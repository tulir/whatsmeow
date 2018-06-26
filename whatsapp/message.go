package whatsapp

import (
	"encoding/hex"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary/proto"
	"math/rand"
	"strings"
	"time"
)

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
