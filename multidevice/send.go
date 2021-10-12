// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/RadicalApp/libsignal-protocol-go/keys/prekey"
	"github.com/RadicalApp/libsignal-protocol-go/protocol"
	"github.com/RadicalApp/libsignal-protocol-go/session"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

func GenerateMessageID() string {
	id := make([]byte, 16)
	_, err := rand.Read(id)
	if err != nil {
		// Out of entropy
		panic(err)
	}
	return hex.EncodeToString(id)
}

var ErrNoSession = errors.New("no signal session established")

func (cli *Client) encryptMessageForDevice(plaintext []byte, to waBinary.FullJID, bundle *prekey.Bundle) (*waBinary.Node, bool, error) {
	builder := session.NewBuilderFromSignal(cli.Session, to.SignalAddress(), pbSerializer)
	if !cli.Session.ContainsSession(to.SignalAddress()) {
		if bundle != nil {
			cli.Log.Debugln("Processing prekey bundle for", to)
			err := builder.ProcessBundle(bundle)
			if err != nil {
				return nil, false, fmt.Errorf("failed to process prekey bundle: %w", err)
			}
		} else {
			return nil, false, ErrNoSession
		}
	}
	cipher := session.NewCipher(builder, to.SignalAddress())
	ciphertext, err := cipher.Encrypt(plaintext)
	if err != nil {
		return nil, false, fmt.Errorf("cipher encryption failed: %w", err)
	}

	encType := "msg"
	if ciphertext.Type() == protocol.PREKEY_TYPE {
		encType = "pkmsg"
	}

	return &waBinary.Node{
		Tag: "to",
		Attrs: map[string]interface{}{
			"jid": to,
		},
		Content: []waBinary.Node{{
			Tag: "enc",
			Attrs: map[string]interface{}{
				"v":    "2",
				"type": encType,
			},
			Content: ciphertext.Serialize(),
		}},
	}, encType == "pkmsg", nil
}

func marshalMessage(to waBinary.FullJID, message *waProto.Message) (plaintext, dsmPlaintext []byte, err error) {
	plaintext, err = proto.Marshal(message)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return
	}
	plaintext = padMessage(plaintext)

	dsmPlaintext, err = proto.Marshal(&waProto.DeviceSentMessage{
		DestinationJid: proto.String(to.String()),
		Message:        message,
	})
	if err != nil {
		err = fmt.Errorf("failed to marshal message (for own devices): %w", err)
		return
	}
	dsmPlaintext = padMessage(dsmPlaintext)

	return
}

func (cli *Client) GetUSyncDevices(jids []waBinary.FullJID, ignorePrimary bool) ([]waBinary.FullJID, error) {
	userList := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		userList[i].Tag = "user"
		userList[i].Attrs = map[string]interface{}{"jid": waBinary.NewJID(jid.User, waBinary.DefaultUserServer)}
	}
	res, err := cli.sendIQ(InfoQuery{
		Namespace: "usync",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "usync",
			Attrs: map[string]interface{}{
				"sid":     cli.generateRequestID(),
				"mode":    "query",
				"last":    "true",
				"index":   "0",
				"context": "message",
			},
			Content: []waBinary.Node{
				{Tag: "query", Content: []waBinary.Node{{
					Tag: "devices",
					Attrs: map[string]interface{}{
						"version": "2",
					},
				}}},
				{Tag: "list", Content: userList},
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send usync query: %w", err)
	}
	usync := res.GetChildByTag("usync")
	if usync.Tag != "usync" {
		return nil, fmt.Errorf("unexpected children in response to usync query")
	}
	list := usync.GetChildByTag("list")
	if list.Tag != "list" {
		return nil, fmt.Errorf("missing list inside usync tag")
	}

	var devices []waBinary.FullJID
	for _, user := range list.GetChildren() {
		jid, jidOK := user.Attrs["jid"].(waBinary.FullJID)
		if user.Tag != "user" || !jidOK {
			continue
		}
		deviceNode := user.GetChildByTag("devices")
		deviceList := deviceNode.GetChildByTag("device-list")
		if deviceNode.Tag != "devices" || deviceList.Tag != "device-list" {
			continue
		}
		for _, device := range deviceList.GetChildren() {
			deviceID, ok := device.AttrGetter().GetInt64("id", true)
			if device.Tag != "device" || !ok {
				continue
			}
			deviceJID := waBinary.NewADJID(jid.User, 0, byte(deviceID))
			if (deviceJID.Device > 0 || !ignorePrimary) && deviceJID != *cli.Session.ID {
				devices = append(devices, deviceJID)
			}
		}
	}

	return devices, nil
}

func (cli *Client) sendDM(to waBinary.FullJID, id string, message *waProto.Message) error {
	messagePlaintext, deviceSentMessagePlaintext, err := marshalMessage(to, message)
	if err != nil {
		return err
	}

	//participantNodes := []waBinary.Node{
	//	cli.encryptMessageForDevice(messagePlaintext, waBinary.NewADJID(to.User, 0, 0)),
	//	cli.encryptMessageForDevice(deviceSentMessagePlaintext, waBinary.NewADJID(cli.Session.ID.User, 0, 0)),
	//}
	includeIdentity := false
	allDevices, err := cli.GetUSyncDevices([]waBinary.FullJID{to, *cli.Session.ID}, false)
	if err != nil {
		return fmt.Errorf("failed to get device list: %w", err)
	}
	participantNodes := make([]waBinary.Node, 0, len(allDevices))
	var retryDevices []waBinary.FullJID
	for _, jid := range allDevices {
		plaintext := messagePlaintext
		if jid.User == cli.Session.ID.User {
			plaintext = deviceSentMessagePlaintext
		}
		var encrypted *waBinary.Node
		var isPreKey bool
		encrypted, isPreKey, err = cli.encryptMessageForDevice(plaintext, jid, nil)
		if errors.Is(err, ErrNoSession) {
			retryDevices = append(retryDevices, jid)
			continue
		} else if err != nil {
			cli.Log.Warnfln("Failed to encrypt %s for %s: %v", id, jid, err)
			continue
		}
		participantNodes = append(participantNodes, *encrypted)
		if isPreKey {
			includeIdentity = true
		}
	}
	if len(retryDevices) > 0 {
		bundles, err := cli.fetchPreKeys(retryDevices)
		if err != nil {
			cli.Log.Warnln("Failed to fetch prekeys for", retryDevices, "to retry encryption:", err)
		} else {
			for _, jid := range retryDevices {
				fmt.Printf("Retrying JID{User: %s, Server: %s, Device: %d, Agent: %d, AD: %t}\n", jid.User, jid.Server, jid.Device, jid.Agent, jid.AD)
				resp := bundles[jid]
				if resp.err != nil {
					cli.Log.Warnfln("Failed to fetch prekey for %s: %v", jid, resp.err)
					continue
				}
				plaintext := messagePlaintext
				if jid.User == cli.Session.ID.User {
					plaintext = deviceSentMessagePlaintext
				}
				var encrypted *waBinary.Node
				var isPreKey bool
				encrypted, isPreKey, err = cli.encryptMessageForDevice(plaintext, jid, resp.bundle)
				if err != nil {
					cli.Log.Warnfln("Failed to encrypt %s for %s (retry): %v", id, jid, err)
					continue
				}
				participantNodes = append(participantNodes, *encrypted)
				if isPreKey {
					includeIdentity = true
				}
			}
		}
	}

	node := waBinary.Node{
		Tag: "message",
		Attrs: map[string]interface{}{
			"id":   id,
			"type": "text",
			"to":   to,
		},
		Content: []waBinary.Node{{
			Tag:     "participants",
			Content: participantNodes,
		}},
	}
	if includeIdentity {
		deviceIdentity, err := proto.Marshal(cli.Session.Account)
		if err != nil {
			return fmt.Errorf("failed to marshal device identity: %w", err)
		}
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "device-identity",
			Content: deviceIdentity,
		})
	}
	err = cli.sendNode(node)
	if err != nil {
		return fmt.Errorf("failed to send message node: %w", err)
	}
	return nil
}

func (cli *Client) SendMessage(to waBinary.FullJID, id string, message *waProto.Message) error {
	if to.AD {
		return fmt.Errorf("message recipient must be non-AD JID")
	}

	if len(id) == 0 {
		id = GenerateMessageID()
	}

	if to.Server == waBinary.GroupServer {
		// TODO
		return fmt.Errorf("sending group messages is not yet implemented")
	} else {
		return cli.sendDM(to, id, message)
	}
}
