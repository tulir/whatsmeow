package waArmadilloApplication

import (
	"go.mau.fi/whatsmeow/proto/armadilloutil"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waMediaTransport"
)

func (*Armadillo) IsMessageApplicationSub() {}

const (
	RavenImageTransportVersion = 1
	RavenVideoTransportVersion = 1
)

func (rm *Armadillo_Content_RavenMessage) DecodeImageMessage() (*waMediaTransport.ImageTransport, error) {
	var transport waMediaTransport.ImageTransport
	return armadilloutil.Unmarshal(&transport, rm.GetImageMessage(), RavenImageTransportVersion)
}

func (rm *Armadillo_Content_RavenMessage) SetImageMessage(payload *waMediaTransport.ImageTransport) error {
	content, err := armadilloutil.Marshal(payload, RavenImageTransportVersion)
	if err != nil {
		return err
	}
	rm.MediaContent = &Armadillo_Content_RavenMessage_ImageMessage{ImageMessage: content}
	return nil
}

func (rm *Armadillo_Content_RavenMessage) DecodeVideoMessage() (*waMediaTransport.VideoTransport, error) {
	var transport waMediaTransport.VideoTransport
	return armadilloutil.Unmarshal(&transport, rm.GetVideoMessage(), RavenVideoTransportVersion)
}

func (rm *Armadillo_Content_RavenMessage) SetVideoMessage(payload *waMediaTransport.VideoTransport) error {
	content, err := armadilloutil.Marshal(payload, RavenVideoTransportVersion)
	if err != nil {
		return err
	}
	rm.MediaContent = &Armadillo_Content_RavenMessage_VideoMessage{VideoMessage: content}
	return nil
}

func (igm *Armadillo_Content_ImageGalleryMessage) Decode() ([]*waMediaTransport.ImageTransport, error) {
	transports := make([]*waMediaTransport.ImageTransport, len(igm.GetImages()))
	for i, img := range igm.GetImages() {
		var transport waMediaTransport.ImageTransport
		if _, err := armadilloutil.Unmarshal(&transport, img, RavenImageTransportVersion); err != nil {
			return nil, err
		}
		transports[i] = &transport
	}
	return transports, nil
}

func (igm *Armadillo_Content_ImageGalleryMessage) Set(payloads []*waMediaTransport.ImageTransport) (err error) {
	contents := make([]*waCommon.SubProtocol, len(payloads))
	for i, payload := range payloads {
		contents[i], err = armadilloutil.Marshal(payload, RavenImageTransportVersion)
		if err != nil {
			return
		}
	}
	igm.Images = contents
	return
}
