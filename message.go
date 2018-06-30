package whatsapp_connection

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type messageType string

const (
	IMAGE    messageType = "WhatsApp Image Keys"
	VIDEO    messageType = "WhatsApp Video Keys"
	AUDIO    messageType = "WhatsApp Audio Keys"
	DOCUMENT messageType = "WhatsApp Document Keys"
)

func (wac *conn) Send(msg interface{}) error {
	var err error
	var ch <-chan string

	switch m := msg.(type) {
	case TextMessage:
		ch, err = wac.sendProto(getTextProto(m))
	default:
		return fmt.Errorf("cannot match type %T, use messagetypes declared in the package", msg)
	}
	if err != nil {
		return fmt.Errorf("error sending message:%v", err)
	}

	select {
	case response := <-ch:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(response), &resp); err != nil {
			return fmt.Errorf("error decoding sending response: %v\n", err)
		}
		if int(resp["status"].(float64)) != 200 {
			return fmt.Errorf("message sending responded with %d", resp["status"])
		}
	case <-time.After(wac.msgTimeout):
		return fmt.Errorf("sending message timed out")
	}

	return nil
}

func (wac *conn) sendProto(p *proto.WebMessageInfo) (<-chan string, error) {
	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "relay",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{p},
	}
	return wac.writeBinary(n, MESSAGE, IGNORE, p.Key.GetId())
}

type MessageInfo struct {
	Id        string
	RemoteJid string
	FromMe    bool
	Timestamp uint64
	PushName  string
}

func getMessageInfo(msg *proto.WebMessageInfo) MessageInfo {
	return MessageInfo{
		Id:        msg.GetKey().GetId(),
		RemoteJid: msg.GetKey().GetRemoteJid(),
		FromMe:    msg.GetKey().GetFromMe(),
		Timestamp: msg.GetMessageTimestamp(),
		PushName:  msg.GetPushName(),
	}
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func getInfoProto(info *MessageInfo) *proto.WebMessageInfo {
	status := proto.WebMessageInfo_STATUS(1)
	if info.Id == "" || len(info.Id) < 2 {
		b := make([]byte, 10)
		rand.Read(b)
		info.Id = strings.ToUpper(hex.EncodeToString(b))
	}
	if info.Timestamp == 0 {
		info.Timestamp = uint64(time.Now().Unix())
	}
	info.FromMe = true

	return &proto.WebMessageInfo{
		Key: &proto.MessageKey{
			FromMe:    &info.FromMe,
			RemoteJid: &info.RemoteJid,
			Id:        &info.Id,
		},
		MessageTimestamp: &info.Timestamp,
		Status:           &status,
	}
}

type TextMessage struct {
	Info MessageInfo
	Text string
}

func getTextMessage(msg *proto.WebMessageInfo) TextMessage {
	return TextMessage{
		Info: getMessageInfo(msg),
		Text: msg.GetMessage().GetConversation(),
	}
}

func getTextProto(msg TextMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		Conversation: &msg.Text,
	}
	return p
}

type ImageMessage struct {
	Info          MessageInfo
	Caption       string
	Thumbnail     []byte
	url           string
	mediaKey      []byte
	ImageType     string
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    int
}

func (m *ImageMessage) Download() ([]byte, error) {
	fmt.Printf("A:%v\nD:%v\n", m.fileEncSha256, m.fileSha256)
	return download(m.url, m.mediaKey, IMAGE, m.fileLength)
}

func getImageMessage(msg *proto.WebMessageInfo) ImageMessage {
	image := msg.GetMessage().GetImageMessage()
	return ImageMessage{
		Info:          getMessageInfo(msg),
		Caption:       image.GetCaption(),
		Thumbnail:     image.GetJpegThumbnail(),
		url:           image.GetUrl(),
		mediaKey:      image.GetMediaKey(),
		ImageType:     image.GetMimetype(),
		fileEncSha256: image.GetFileEncSha256(),
		fileSha256:    image.GetFileSha256(),
		fileLength:    int(image.GetFileLength()),
	}
}

type VideoMessage struct {
	Info          MessageInfo
	Caption       string
	Thumbnail     []byte
	url           string
	mediaKey      []byte
	Length        uint32
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    int
}

func (m *VideoMessage) Download() ([]byte, error) {
	return download(m.url, m.mediaKey, VIDEO, m.fileLength)
}

func getVideoMessage(msg *proto.WebMessageInfo) VideoMessage {
	vid := msg.GetMessage().GetVideoMessage()
	return VideoMessage{
		Info:          getMessageInfo(msg),
		Caption:       vid.GetCaption(),
		Thumbnail:     vid.GetJpegThumbnail(),
		url:           vid.GetUrl(),
		mediaKey:      vid.GetMediaKey(),
		Length:        vid.GetSeconds(),
		fileEncSha256: vid.GetFileEncSha256(),
		fileSha256:    vid.GetFileSha256(),
		fileLength:    int(vid.GetFileLength()),
	}
}

func parseProtoMessage(msg *proto.WebMessageInfo) interface{} {
	switch {

	case msg.GetMessage().GetAudioMessage() != nil:
		//dp.handle(getAudioMessage(msg))

	case msg.GetMessage().GetImageMessage() != nil:
		return getImageMessage(msg)

	case msg.GetMessage().GetVideoMessage() != nil:
		return getVideoMessage(msg)

	case msg.GetMessage().GetConversation() != "":
		return getTextMessage(msg)

	default:
		//cannot match message
	}

	return nil
}