package whatsapp

import "github.com/Rhymen/go-whatsapp/whatsapp/binary/proto"

type MessageInfo struct {
	Id        string
	From      string
	FromMe    bool
	Timestamp uint64
	PushName  string
}

func getMessageInfo(msg *proto.WebMessageInfo) MessageInfo {
	return MessageInfo{
		Id:        msg.GetKey().GetId(),
		From:      msg.GetKey().GetRemoteJid(),
		FromMe:    msg.GetKey().GetFromMe(),
		Timestamp: msg.GetMessageTimestamp(),
		PushName:  msg.GetPushName(),
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

type ImageMessage struct {
	Info      MessageInfo
	Caption   string
	Thumbnail []byte
	url       string
	mediaKey  []byte
	ImageType string
}

func getImageMessage(msg *proto.WebMessageInfo) ImageMessage {
	image := msg.GetMessage().GetImageMessage()
	return ImageMessage{
		Info:      getMessageInfo(msg),
		Caption:   image.GetCaption(),
		Thumbnail: image.GetJpegThumbnail(),
		url:       image.GetUrl(),
		mediaKey:  image.GetMediaKey(),
		ImageType: image.GetMimetype()}
}
