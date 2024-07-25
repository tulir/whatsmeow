package waConsumerApplication

import (
	"go.mau.fi/whatsmeow/proto/armadilloutil"
	"go.mau.fi/whatsmeow/proto/waMediaTransport"
)

type ConsumerApplication_Content_Content = isConsumerApplication_Content_Content

func (*ConsumerApplication) IsMessageApplicationSub() {}

const (
	ImageTransportVersion    = 1
	StickerTransportVersion  = 1
	VideoTransportVersion    = 1
	AudioTransportVersion    = 1
	DocumentTransportVersion = 1
	ContactTransportVersion  = 1
)

func (msg *ConsumerApplication_ImageMessage) Decode() (dec *waMediaTransport.ImageTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.ImageTransport{}, msg.GetImage(), ImageTransportVersion)
}

func (msg *ConsumerApplication_ImageMessage) Set(payload *waMediaTransport.ImageTransport) (err error) {
	msg.Image, err = armadilloutil.Marshal(payload, ImageTransportVersion)
	return
}

func (msg *ConsumerApplication_StickerMessage) Decode() (dec *waMediaTransport.StickerTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.StickerTransport{}, msg.GetSticker(), StickerTransportVersion)
}

func (msg *ConsumerApplication_StickerMessage) Set(payload *waMediaTransport.StickerTransport) (err error) {
	msg.Sticker, err = armadilloutil.Marshal(payload, StickerTransportVersion)
	return
}

func (msg *ConsumerApplication_ExtendedTextMessage) DecodeThumbnail() (dec *waMediaTransport.ImageTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.ImageTransport{}, msg.GetThumbnail(), ImageTransportVersion)
}

func (msg *ConsumerApplication_ExtendedTextMessage) SetThumbnail(payload *waMediaTransport.ImageTransport) (err error) {
	msg.Thumbnail, err = armadilloutil.Marshal(payload, ImageTransportVersion)
	return
}

func (msg *ConsumerApplication_VideoMessage) Decode() (dec *waMediaTransport.VideoTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.VideoTransport{}, msg.GetVideo(), VideoTransportVersion)
}

func (msg *ConsumerApplication_VideoMessage) Set(payload *waMediaTransport.VideoTransport) (err error) {
	msg.Video, err = armadilloutil.Marshal(payload, VideoTransportVersion)
	return
}

func (msg *ConsumerApplication_AudioMessage) Decode() (dec *waMediaTransport.AudioTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.AudioTransport{}, msg.GetAudio(), AudioTransportVersion)
}

func (msg *ConsumerApplication_AudioMessage) Set(payload *waMediaTransport.AudioTransport) (err error) {
	msg.Audio, err = armadilloutil.Marshal(payload, AudioTransportVersion)
	return
}

func (msg *ConsumerApplication_DocumentMessage) Decode() (dec *waMediaTransport.DocumentTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.DocumentTransport{}, msg.GetDocument(), DocumentTransportVersion)
}

func (msg *ConsumerApplication_DocumentMessage) Set(payload *waMediaTransport.DocumentTransport) (err error) {
	msg.Document, err = armadilloutil.Marshal(payload, DocumentTransportVersion)
	return
}

func (msg *ConsumerApplication_ContactMessage) Decode() (dec *waMediaTransport.ContactTransport, err error) {
	return armadilloutil.Unmarshal(&waMediaTransport.ContactTransport{}, msg.GetContact(), ContactTransportVersion)
}

func (msg *ConsumerApplication_ContactMessage) Set(payload *waMediaTransport.ContactTransport) (err error) {
	msg.Contact, err = armadilloutil.Marshal(payload, ContactTransportVersion)
	return
}
