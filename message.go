package whatsapp

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type MediaType string

const (
	MediaImage    MediaType = "WhatsApp Image Keys"
	MediaVideo    MediaType = "WhatsApp Video Keys"
	MediaAudio    MediaType = "WhatsApp Audio Keys"
	MediaDocument MediaType = "WhatsApp Document Keys"
)

var msgInfo MessageInfo

func (wac *Conn) Send(msg interface{}) (string, error) {
	var err error
	var ch <-chan string
	var msgProto *proto.WebMessageInfo

	switch m := msg.(type) {
	case *proto.WebMessageInfo:
		ch, err = wac.sendProto(m)
	case TextMessage:
		msgProto = getTextProto(m)
		msgInfo = getMessageInfo(msgProto)
		ch, err = wac.sendProto(msgProto)
	case ImageMessage:
		m.url, m.mediaKey, m.fileEncSha256, m.fileSha256, m.fileLength, err = wac.Upload(m.Content, MediaImage)
		if err != nil {
			return "ERROR", fmt.Errorf("image upload failed: %v", err)
		}
		msgProto = getImageProto(m)
		msgInfo = getMessageInfo(msgProto)
		ch, err = wac.sendProto(msgProto)
	case VideoMessage:
		m.url, m.mediaKey, m.fileEncSha256, m.fileSha256, m.fileLength, err = wac.Upload(m.Content, MediaVideo)
		if err != nil {
			return "ERROR", fmt.Errorf("video upload failed: %v", err)
		}
		msgProto = getVideoProto(m)
		msgInfo = getMessageInfo(msgProto)
		ch, err = wac.sendProto(msgProto)
	case DocumentMessage:
		m.url, m.mediaKey, m.fileEncSha256, m.fileSha256, m.fileLength, err = wac.Upload(m.Content, MediaDocument)
		if err != nil {
			return "ERROR", fmt.Errorf("document upload failed: %v", err)
		}
		msgProto = getDocumentProto(m)
		msgInfo = getMessageInfo(msgProto)
		ch, err = wac.sendProto(msgProto)
	case AudioMessage:
		m.url, m.mediaKey, m.fileEncSha256, m.fileSha256, m.fileLength, err = wac.Upload(m.Content, MediaAudio)
		if err != nil {
			return "ERROR", fmt.Errorf("audio upload failed: %v", err)
		}
		msgProto = getAudioProto(m)
		msgInfo = getMessageInfo(msgProto)
		ch, err = wac.sendProto(msgProto)
	default:
		return "ERROR", fmt.Errorf("cannot match type %T, use message types declared in the package", msg)
	}

	if err != nil {
		return "ERROR", fmt.Errorf("could not send proto: %v", err)
	}

	select {
	case response := <-ch:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(response), &resp); err != nil {
			return "ERROR", fmt.Errorf("error decoding sending response: %v\n", err)
		}
		if int(resp["status"].(float64)) != 200 {
			return "ERROR", fmt.Errorf("message sending responded with %d", resp["status"])
		}
		if int(resp["status"].(float64)) == 200 {
			return msgInfo.Id, nil
		}
	case <-time.After(wac.msgTimeout):
		return "ERROR", fmt.Errorf("sending message timed out")
	}

	return "ERROR", nil
}

func (wac *Conn) sendProto(p *proto.WebMessageInfo) (<-chan string, error) {
	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "relay",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{p},
	}
	return wac.writeBinary(n, message, ignore, p.Key.GetId())
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

/*
MessageInfo contains general message information. It is part of every of every message type.
*/
type MessageInfo struct {
	Id              string
	RemoteJid       string
	SenderJid       string
	FromMe          bool
	Timestamp       uint64
	PushName        string
	Status          MessageStatus
	QuotedMessageID string

	Source *proto.WebMessageInfo
}

type MessageStatus int

const (
	Error       MessageStatus = 0
	Pending                   = 1
	ServerAck                 = 2
	DeliveryAck               = 3
	Read                      = 4
	Played                    = 5
)

func getMessageInfo(msg *proto.WebMessageInfo) MessageInfo {
	return MessageInfo{
		Id:        msg.GetKey().GetId(),
		RemoteJid: msg.GetKey().GetRemoteJid(),
		SenderJid: msg.GetKey().GetParticipant(),
		FromMe:    msg.GetKey().GetFromMe(),
		Timestamp: msg.GetMessageTimestamp(),
		Status:    MessageStatus(msg.GetStatus()),
		PushName:  msg.GetPushName(),
		Source:    msg,
	}
}

func getInfoProto(info *MessageInfo) *proto.WebMessageInfo {
	if info.Id == "" || len(info.Id) < 2 {
		b := make([]byte, 10)
		rand.Read(b)
		info.Id = strings.ToUpper(hex.EncodeToString(b))
	}
	if info.Timestamp == 0 {
		info.Timestamp = uint64(time.Now().Unix())
	}
	info.FromMe = true

	status := proto.WebMessageInfo_STATUS(info.Status)

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

/*
TextMessage represents a text message.
*/
type TextMessage struct {
	Info MessageInfo
	Text string
}

func getTextMessage(msg *proto.WebMessageInfo) TextMessage {
	text := TextMessage{Info: getMessageInfo(msg)}
	if m := msg.GetMessage().GetExtendedTextMessage(); m != nil {
		text.Text = m.GetText()
		text.Info.QuotedMessageID = m.GetContextInfo().GetStanzaId()
	} else {
		text.Text = msg.GetMessage().GetConversation()
	}
	return text
}

func getTextProto(msg TextMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		Conversation: &msg.Text,
	}
	return p
}

/*
ImageMessage represents a image message. Unexported fields are needed for media up/downloading and media validation.
Provide a io.Reader as Content for message sending.
*/
type ImageMessage struct {
	Info          MessageInfo
	Caption       string
	Thumbnail     []byte
	Type          string
	Content       io.Reader
	url           string
	mediaKey      []byte
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    uint64
}

func getImageMessage(msg *proto.WebMessageInfo) ImageMessage {
	image := msg.GetMessage().GetImageMessage()
	return ImageMessage{
		Info:          getMessageInfo(msg),
		Caption:       image.GetCaption(),
		Thumbnail:     image.GetJpegThumbnail(),
		url:           image.GetUrl(),
		mediaKey:      image.GetMediaKey(),
		Type:          image.GetMimetype(),
		fileEncSha256: image.GetFileEncSha256(),
		fileSha256:    image.GetFileSha256(),
		fileLength:    image.GetFileLength(),
	}
}

func getImageProto(msg ImageMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		ImageMessage: &proto.ImageMessage{
			Caption:       &msg.Caption,
			JpegThumbnail: msg.Thumbnail,
			Url:           &msg.url,
			MediaKey:      msg.mediaKey,
			Mimetype:      &msg.Type,
			FileEncSha256: msg.fileEncSha256,
			FileSha256:    msg.fileSha256,
			FileLength:    &msg.fileLength,
		},
	}
	return p
}

/*
Download is the function to retrieve media data. The media gets downloaded, validated and returned.
*/
func (m *ImageMessage) Download() ([]byte, error) {
	return Download(m.url, m.mediaKey, MediaImage, int(m.fileLength))
}

/*
VideoMessage represents a video message. Unexported fields are needed for media up/downloading and media validation.
Provide a io.Reader as Content for message sending.
*/
type VideoMessage struct {
	Info          MessageInfo
	Caption       string
	Thumbnail     []byte
	Length        uint32
	Type          string
	Content       io.Reader
	url           string
	mediaKey      []byte
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    uint64
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
		Type:          vid.GetMimetype(),
		fileEncSha256: vid.GetFileEncSha256(),
		fileSha256:    vid.GetFileSha256(),
		fileLength:    vid.GetFileLength(),
	}
}

func getVideoProto(msg VideoMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		VideoMessage: &proto.VideoMessage{
			Caption:       &msg.Caption,
			JpegThumbnail: msg.Thumbnail,
			Url:           &msg.url,
			MediaKey:      msg.mediaKey,
			Seconds:       &msg.Length,
			FileEncSha256: msg.fileEncSha256,
			FileSha256:    msg.fileSha256,
			FileLength:    &msg.fileLength,
			Mimetype:      &msg.Type,
		},
	}
	return p
}

/*
Download is the function to retrieve media data. The media gets downloaded, validated and returned.
*/
func (m *VideoMessage) Download() ([]byte, error) {
	return Download(m.url, m.mediaKey, MediaVideo, int(m.fileLength))
}

/*
AudioMessage represents a audio message. Unexported fields are needed for media up/downloading and media validation.
Provide a io.Reader as Content for message sending.
*/
type AudioMessage struct {
	Info          MessageInfo
	Length        uint32
	Type          string
	Content       io.Reader
	url           string
	mediaKey      []byte
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    uint64
}

func getAudioMessage(msg *proto.WebMessageInfo) AudioMessage {
	aud := msg.GetMessage().GetAudioMessage()
	return AudioMessage{
		Info:          getMessageInfo(msg),
		url:           aud.GetUrl(),
		mediaKey:      aud.GetMediaKey(),
		Length:        aud.GetSeconds(),
		Type:          aud.GetMimetype(),
		fileEncSha256: aud.GetFileEncSha256(),
		fileSha256:    aud.GetFileSha256(),
		fileLength:    aud.GetFileLength(),
	}
}

func getAudioProto(msg AudioMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		AudioMessage: &proto.AudioMessage{
			Url:           &msg.url,
			MediaKey:      msg.mediaKey,
			Seconds:       &msg.Length,
			FileEncSha256: msg.fileEncSha256,
			FileSha256:    msg.fileSha256,
			FileLength:    &msg.fileLength,
			Mimetype:      &msg.Type,
		},
	}
	return p
}

/*
Download is the function to retrieve media data. The media gets downloaded, validated and returned.
*/
func (m *AudioMessage) Download() ([]byte, error) {
	return Download(m.url, m.mediaKey, MediaAudio, int(m.fileLength))
}

/*
DocumentMessage represents a document message. Unexported fields are needed for media up/downloading and media
validation. Provide a io.Reader as Content for message sending.
*/
type DocumentMessage struct {
	Info          MessageInfo
	Title         string
	PageCount     uint32
	Type          string
	FileName      string
	Thumbnail     []byte
	Content       io.Reader
	url           string
	mediaKey      []byte
	fileEncSha256 []byte
	fileSha256    []byte
	fileLength    uint64
}

func getDocumentMessage(msg *proto.WebMessageInfo) DocumentMessage {
	doc := msg.GetMessage().GetDocumentMessage()
	return DocumentMessage{
		Info:          getMessageInfo(msg),
		Title:         doc.GetTitle(),
		PageCount:     doc.GetPageCount(),
		Type:          doc.GetMimetype(),
		FileName:      doc.GetFileName(),
		Thumbnail:     doc.GetJpegThumbnail(),
		url:           doc.GetUrl(),
		mediaKey:      doc.GetMediaKey(),
		fileEncSha256: doc.GetFileEncSha256(),
		fileSha256:    doc.GetFileSha256(),
		fileLength:    doc.GetFileLength(),
	}
}

func getDocumentProto(msg DocumentMessage) *proto.WebMessageInfo {
	p := getInfoProto(&msg.Info)
	p.Message = &proto.Message{
		DocumentMessage: &proto.DocumentMessage{
			JpegThumbnail: msg.Thumbnail,
			Url:           &msg.url,
			MediaKey:      msg.mediaKey,
			FileEncSha256: msg.fileEncSha256,
			FileSha256:    msg.fileSha256,
			FileLength:    &msg.fileLength,
			PageCount:     &msg.PageCount,
			Title:         &msg.Title,
			Mimetype:      &msg.Type,
		},
	}
	return p
}

/*
Download is the function to retrieve media data. The media gets downloaded, validated and returned.
*/
func (m *DocumentMessage) Download() ([]byte, error) {
	return Download(m.url, m.mediaKey, MediaDocument, int(m.fileLength))
}

func parseProtoMessage(msg *proto.WebMessageInfo) interface{} {
	switch {

	case msg.GetMessage().GetAudioMessage() != nil:
		return getAudioMessage(msg)

	case msg.GetMessage().GetImageMessage() != nil:
		return getImageMessage(msg)

	case msg.GetMessage().GetVideoMessage() != nil:
		return getVideoMessage(msg)

	case msg.GetMessage().GetDocumentMessage() != nil:
		return getDocumentMessage(msg)

	case msg.GetMessage().GetConversation() != "":
		return getTextMessage(msg)

	case msg.GetMessage().GetExtendedTextMessage() != nil:
		return getTextMessage(msg)

	default:
		//cannot match message
	}

	return nil
}
