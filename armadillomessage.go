// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	armadillo "go.mau.fi/whatsmeow/proto"
	"go.mau.fi/whatsmeow/proto/armadilloutil"
	"go.mau.fi/whatsmeow/proto/instamadilloTransportPayload"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waMsgApplication"
	"go.mau.fi/whatsmeow/proto/waMsgTransport"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleDecryptedArmadillo(ctx context.Context, info *types.MessageInfo, decrypted []byte, retryCount int) (handlerFailed, protobufFailed bool) {
	dec, err := decodeArmadillo(decrypted)
	if err != nil {
		cli.Log.Warnf("Failed to decode armadillo message from %s: %v", info.SourceString(), err)
		protobufFailed = true
		return
	}
	dec.Info = *info
	dec.RetryCount = retryCount
	if dec.Transport.GetProtocol().GetAncillary().GetSkdm() != nil {
		if !info.IsGroup {
			cli.Log.Warnf("Got sender key distribution message in non-group chat from %s", info.Sender)
		} else {
			skdm := dec.Transport.GetProtocol().GetAncillary().GetSkdm()
			cli.handleSenderKeyDistributionMessage(ctx, info.Chat, info.Sender, skdm.AxolotlSenderKeyDistributionMessage)
		}
	}
	if dec.Message != nil || dec.FBApplication != nil {
		handlerFailed = cli.dispatchEvent(&dec)
	}
	return
}

func decodeArmadillo(data []byte) (dec events.FBMessage, err error) {
	var transport waMsgTransport.MessageTransport
	err = proto.Unmarshal(data, &transport)
	if err != nil {
		return dec, fmt.Errorf("failed to unmarshal transport: %w", err)
	}
	dec.Transport = &transport
	if transport.GetPayload() == nil {
		return
	}
	appPayloadVer := transport.GetPayload().GetApplicationPayload().GetVersion()
	switch appPayloadVer {
	case waMsgTransport.FBMessageApplicationVersion:
		return decodeFBArmadillo(&transport)
	case waMsgTransport.IGMessageApplicationVersion:
		return decodeIGArmadillo(&transport)
	default:
		return dec, fmt.Errorf("%w %d in MessageTransport", armadilloutil.ErrUnsupportedVersion, appPayloadVer)
	}
}

func decodeFBArmadillo(transport *waMsgTransport.MessageTransport) (dec events.FBMessage, err error) {
	var application *waMsgApplication.MessageApplication
	application, err = transport.GetPayload().DecodeFB()
	if err != nil {
		return dec, fmt.Errorf("failed to unmarshal application: %w", err)
	}
	dec.FBApplication = application
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
			dec.Message, err = subProtocol.Decode()
		case *waMsgApplication.MessageApplication_SubProtocolPayload_BusinessMessage:
			dec.Message = (*armadillo.Unsupported_BusinessApplication)(subProtocol.BusinessMessage)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_PaymentMessage:
			dec.Message = (*armadillo.Unsupported_PaymentApplication)(subProtocol.PaymentMessage)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_MultiDevice:
			dec.Message, err = subProtocol.Decode()
		case *waMsgApplication.MessageApplication_SubProtocolPayload_Voip:
			dec.Message = (*armadillo.Unsupported_Voip)(subProtocol.Voip)
		case *waMsgApplication.MessageApplication_SubProtocolPayload_Armadillo:
			dec.Message, err = subProtocol.Decode()
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

func decodeIGArmadillo(transport *waMsgTransport.MessageTransport) (dec events.FBMessage, err error) {
	innerTransport, err := transport.GetPayload().DecodeIG()
	if err != nil {
		return dec, fmt.Errorf("failed to unmarshal IG transport: %w", err)
	}
	dec.IGTransport = innerTransport
	switch typedContent := innerTransport.GetTransportPayload().(type) {
	case *instamadilloTransportPayload.TransportPayload_Add:
		dec.Message = typedContent.Add
	case *instamadilloTransportPayload.TransportPayload_Supplement:
		dec.Message = typedContent.Supplement
	case *instamadilloTransportPayload.TransportPayload_Delete:
		dec.Message = typedContent.Delete
	}
	return
}
