// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/binary/armadillo"
	"go.mau.fi/whatsmeow/binary/armadillo/waArmadilloApplication"
	"go.mau.fi/whatsmeow/binary/armadillo/waCommon"
	"go.mau.fi/whatsmeow/binary/armadillo/waConsumerApplication"
	"go.mau.fi/whatsmeow/binary/armadillo/waMsgApplication"
	"go.mau.fi/whatsmeow/binary/armadillo/waMsgTransport"
	"go.mau.fi/whatsmeow/binary/armadillo/waMultiDevice"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleDecryptedArmadillo(info *types.MessageInfo, decrypted []byte, retryCount int) bool {
	dec, err := decodeArmadillo(decrypted)
	if err != nil {
		cli.Log.Warnf("Failed to decode armadillo message from %s: %v", info.SourceString(), err)
		return false
	}
	if dec.Transport.GetProtocol().GetAncillary().GetSkdm() != nil {
		if !info.IsGroup {
			cli.Log.Warnf("Got sender key distribution message in non-group chat from %s", info.Sender)
		} else {
			skdm := dec.Transport.GetProtocol().GetAncillary().GetSkdm()
			cli.handleSenderKeyDistributionMessage(info.Chat, info.Sender, skdm.AxolotlSenderKeyDistributionMessage)
		}
	}
	switch evtData := dec.Message.(type) {
	case *waConsumerApplication.ConsumerApplication:
		evt := &events.FBConsumerMessage{
			Info:        *info,
			Message:     evtData,
			RetryCount:  retryCount,
			Transport:   dec.Transport,
			Application: dec.Application,
		}
		cli.dispatchEvent(evt)
		// TODO dispatch other events?
	}
	return true
}

type DecodedArmadillo struct {
	Transport   *waMsgTransport.MessageTransport
	Application *waMsgApplication.MessageApplication
	Message     armadillo.MessageApplicationSub
}

func decodeArmadillo(data []byte) (dec DecodedArmadillo, err error) {
	var transport waMsgTransport.MessageTransport
	err = proto.Unmarshal(data, &transport)
	if err != nil {
		return dec, fmt.Errorf("failed to unmarshal transport: %w", err)
	}
	dec.Transport = &transport
	if transport.GetPayload() == nil {
		return
	} else if transport.GetPayload().GetApplicationPayload().GetVersion() != 2 {
		// TODO handle future proof behavior tag?
		return dec, fmt.Errorf("unsupported application payload version: %d", transport.GetPayload().GetApplicationPayload().GetVersion())
	}
	var application waMsgApplication.MessageApplication
	err = proto.Unmarshal(transport.GetPayload().GetApplicationPayload().GetPayload(), &application)
	if err != nil {
		return dec, fmt.Errorf("failed to unmarshal application: %w", err)
	}
	dec.Application = &application
	if application.GetPayload() == nil {
		return
	}

	switch typedContent := application.GetPayload().GetContent().(type) {
	case *waMsgApplication.MessageApplication_Payload_CoreContent:
		err = fmt.Errorf("unsupported core content payload")
	case *waMsgApplication.MessageApplication_Payload_Signal:
		err = fmt.Errorf("unsupported signal payload")
	case *waMsgApplication.MessageApplication_Payload_ApplicationData:
		err = fmt.Errorf("unsupported application data payload")
	case *waMsgApplication.MessageApplication_Payload_SubProtocol:
		var protoMsg proto.Message
		var subData *waCommon.SubProtocol
		switch subProtocol := typedContent.SubProtocol.GetSubProtocol().(type) {
		case *waMsgApplication.MessageApplication_SubProtocolPayload_ConsumerMessage:
			typedSub := &waConsumerApplication.ConsumerApplication{}
			dec.Message = typedSub
			protoMsg = typedSub
			subData = subProtocol.ConsumerMessage
		case *waMsgApplication.MessageApplication_SubProtocolPayload_BusinessMessage:
			subData = subProtocol.BusinessMessage
			dec.Message = (*armadillo.Unsupported_BusinessApplication)(subData)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_PaymentMessage:
			subData = subProtocol.PaymentMessage
			dec.Message = (*armadillo.Unsupported_PaymentApplication)(subData)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_MultiDevice:
			typedSub := &waMultiDevice.MultiDevice{}
			dec.Message = typedSub
			protoMsg = typedSub
			subData = subProtocol.MultiDevice
		case *waMsgApplication.MessageApplication_SubProtocolPayload_Voip:
			subData = subProtocol.Voip
			dec.Message = (*armadillo.Unsupported_Voip)(subData)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_Armadillo:
			typedSub := &waArmadilloApplication.Armadillo{}
			dec.Message = typedSub
			protoMsg = typedSub
			subData = subProtocol.Armadillo
		default:
			return dec, fmt.Errorf("unsupported subprotocol type: %T", subProtocol)
		}
		if protoMsg != nil {
			err = proto.Unmarshal(subData.GetPayload(), protoMsg)
			if err != nil {
				return dec, fmt.Errorf("failed to unmarshal application subprotocol payload (%T v%d): %w", protoMsg, subData.GetVersion(), err)
			}
		}
	default:
		err = fmt.Errorf("unsupported application payload content type: %T", typedContent)
	}
	return
}
