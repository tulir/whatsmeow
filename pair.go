// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/libsignal/ecc"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/keys"
)

const qrScanTimeout = 30 * time.Second

func (cli *Client) handleIQ(node *waBinary.Node) {
	children := node.GetChildren()
	if len(children) != 1 || node.Attrs["from"] != waBinary.ServerJID {
		return
	}
	switch children[0].Tag {
	case "pair-device":
		cli.handlePairDevice(node)
	case "pair-success":
		cli.handlePairSuccess(node)
	}
}

func (cli *Client) handlePairDevice(node *waBinary.Node) {
	pairDevice := node.GetChildByTag("pair-device")
	err := cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   node.Attrs["from"],
			"id":   node.Attrs["id"],
			"type": "result",
		},
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for pair-device request: %v", err)
	}

	evt := &QREvent{
		Codes:   make([]string, 0, len(pairDevice.GetChildren())),
		Timeout: qrScanTimeout,
	}
	for i, child := range pairDevice.GetChildren() {
		if child.Tag != "ref" {
			cli.Log.Warnf("pair-device node contains unexpected child tag %s at index %d", child.Tag, i)
			continue
		}
		content, ok := child.Content.([]byte)
		if !ok {
			cli.Log.Warnf("pair-device node contains unexpected child content type %T at index %d", child, i)
			continue
		}
		evt.Codes = append(evt.Codes, cli.makeQRData(string(content)))
	}

	cli.dispatchEvent(evt)
}

func (cli *Client) makeQRData(ref string) string {
	noise := base64.StdEncoding.EncodeToString(cli.Store.NoiseKey.Pub[:])
	identity := base64.StdEncoding.EncodeToString(cli.Store.IdentityKey.Pub[:])
	adv := base64.StdEncoding.EncodeToString(cli.Store.AdvSecretKey)
	return strings.Join([]string{ref, noise, identity, adv}, ",")
}

func (cli *Client) handlePairSuccess(node *waBinary.Node) {
	id := node.Attrs["id"].(string)
	pairSuccess := node.GetChildByTag("pair-success")

	deviceIdentityBytes, _ := pairSuccess.GetChildByTag("device-identity").Content.([]byte)
	businessName, _ := pairSuccess.GetChildByTag("biz").Attrs["name"].(string)
	wid, _ := pairSuccess.GetChildByTag("device").Attrs["jid"].(waBinary.JID)
	platform, _ := pairSuccess.GetChildByTag("platform").Attrs["name"].(string)

	go func() {
		err := cli.handlePair(deviceIdentityBytes, id, businessName, platform, wid)
		if err != nil {
			cli.Log.Errorf("Failed to pair device: %v", err)
		} else {
			cli.Log.Infof("Successfully paired with %s", cli.Store.ID)
		}
	}()
}

func (cli *Client) handlePair(deviceIdentityBytes []byte, reqID, businessName, platform string, wid waBinary.JID) error {
	var deviceIdentityContainer waProto.ADVSignedDeviceIdentityHMAC
	err := proto.Unmarshal(deviceIdentityBytes, &deviceIdentityContainer)
	if err != nil {
		return fmt.Errorf("failed to parse device identity container in pair success message: %w", err)
	}

	h := hmac.New(sha256.New, cli.Store.AdvSecretKey)
	h.Write(deviceIdentityContainer.Details)
	if !bytes.Equal(h.Sum(nil), deviceIdentityContainer.Hmac) {
		cli.Log.Warnf("Invalid HMAC from pair success message")
		cli.sendNotAuthorized(reqID)
		return fmt.Errorf("invalid device identity HMAC in pair success message")
	}

	var deviceIdentity waProto.ADVSignedDeviceIdentity
	err = proto.Unmarshal(deviceIdentityContainer.Details, &deviceIdentity)
	if err != nil {
		return fmt.Errorf("failed to parse signed device identity in pair success message: %w", err)
	}

	if !verifyDeviceIdentityAccountSignature(&deviceIdentity, cli.Store.IdentityKey) {
		cli.sendNotAuthorized(reqID)
		return fmt.Errorf("invalid device signature in pair success message")
	}

	deviceIdentity.DeviceSignature = generateDeviceSignature(&deviceIdentity, cli.Store.IdentityKey)[:]

	var deviceIdentityDetails waProto.ADVDeviceIdentity
	err = proto.Unmarshal(deviceIdentity.Details, &deviceIdentityDetails)
	if err != nil {
		return fmt.Errorf("failed to parse device identity details in pair success message: %w", err)
	}

	mainDeviceJID := wid
	mainDeviceJID.Device = 0
	mainDeviceIdentity := *(*[32]byte)(deviceIdentity.AccountSignatureKey)
	deviceIdentity.AccountSignatureKey = nil

	cli.Store.Account = proto.Clone(&deviceIdentity).(*waProto.ADVSignedDeviceIdentity)

	selfSignedDeviceIdentity, err := proto.Marshal(&deviceIdentity)
	if err != nil {
		return fmt.Errorf("failed to marshal self-signed device identity: %w", err)
	}

	cli.Store.ID = &wid
	cli.Store.BusinessName = businessName
	cli.Store.Platform = platform
	err = cli.Store.Save()
	if err != nil {
		return fmt.Errorf("failed to save device store: %w", err)
	}
	err = cli.Store.Identities.PutIdentity(mainDeviceJID.SignalAddress().String(), mainDeviceIdentity)
	if err != nil {
		return fmt.Errorf("failed to store main device identity: %w", err)
	}

	err = cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   waBinary.ServerJID,
			"type": "result",
			"id":   reqID,
		},
		Content: []waBinary.Node{{
			Tag: "pair-device-sign",
			Content: []waBinary.Node{{
				Tag: "device-identity",
				Attrs: waBinary.Attrs{
					"key-index": deviceIdentityDetails.GetKeyIndex(),
				},
				Content: selfSignedDeviceIdentity,
			}},
		}},
	})
	if err != nil {
		_ = cli.Store.Delete()
		return fmt.Errorf("failed to send pairing confirmation: %w", err)
	}
	cli.dispatchEvent(&PairSuccessEvent{ID: wid, BusinessName: businessName, Platform: platform})
	return nil
}

func concatBytes(data ...[]byte) []byte {
	length := 0
	for _, item := range data {
		length += len(item)
	}
	output := make([]byte, length)
	ptr := 0
	for _, item := range data {
		ptr += copy(output[ptr:ptr+len(item)], item)
	}
	return output
}

func verifyDeviceIdentityAccountSignature(deviceIdentity *waProto.ADVSignedDeviceIdentity, ikp *keys.KeyPair) bool {
	if len(deviceIdentity.AccountSignatureKey) != 32 || len(deviceIdentity.AccountSignature) != 64 {
		return false
	}

	signatureKey := ecc.NewDjbECPublicKey(*(*[32]byte)(deviceIdentity.AccountSignatureKey))
	signature := *(*[64]byte)(deviceIdentity.AccountSignature)

	message := concatBytes([]byte{6, 0}, deviceIdentity.Details, ikp.Pub[:])
	return ecc.VerifySignature(signatureKey, message, signature)
}

func generateDeviceSignature(deviceIdentity *waProto.ADVSignedDeviceIdentity, ikp *keys.KeyPair) *[64]byte {
	message := concatBytes([]byte{6, 1}, deviceIdentity.Details, ikp.Pub[:], deviceIdentity.AccountSignatureKey)
	sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*ikp.Priv), message)
	return &sig
}

func (cli *Client) sendNotAuthorized(id string) waBinary.Node {
	return waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   waBinary.ServerJID,
			"type": "error",
			"id":   id,
		},
		Content: []waBinary.Node{{
			Tag: "error",
			Attrs: waBinary.Attrs{
				"code": "401",
				"text": "not-authorized",
			},
		}},
	}
}
