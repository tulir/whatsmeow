// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"bytes"
	"crypto/rand"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/RadicalApp/libsignal-protocol-go/protocol"
	"github.com/RadicalApp/libsignal-protocol-go/serialize"
	"github.com/RadicalApp/libsignal-protocol-go/session"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

var pbSerializer = serialize.NewProtoBufSerializer()

func (cli *Client) decryptMessage(child *waBinary.Node, addr waBinary.FullJID) ([]byte, error) {
	content, _ := child.Content.([]byte)

	encType, ok := child.Attrs["type"].(string)
	if !ok {
		return nil, fmt.Errorf("message doesn't have a valid type")
	}
	builder := session.NewBuilderFromSignal(cli.Session, addr.SignalAddress(), serialize.NewJSONSerializer())
	cipher := session.NewCipher(builder, addr.SignalAddress())
	if encType == "pkmsg" {
		preKeyMsg, err := protocol.NewPreKeySignalMessageFromBytes(content, pbSerializer.PreKeySignalMessage, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse prekey message: %w", err)
		}
		plaintext, _, err := cipher.DecryptMessageReturnKey(preKeyMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt prekey message: %w", err)
		}
		return unpadMessage(plaintext)
	} else {
		msg, err := protocol.NewSignalMessageFromBytes(content, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse normal message: %w", err)
		}
		plaintext, err := cipher.Decrypt(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt normal message: %w", err)
		}
		return unpadMessage(plaintext)
	}
}

var CheckPadding = true

func unpadMessage(plaintext []byte) ([]byte, error) {
	if CheckPadding {
		lastByte := plaintext[len(plaintext)-1]
		expectedPadding := bytes.Repeat([]byte{lastByte}, int(lastByte))
		if !bytes.HasSuffix(plaintext, expectedPadding) {
			return nil, fmt.Errorf("plaintext doesn't have expected padding")
		}
	}
	return plaintext[:len(plaintext)-int(plaintext[len(plaintext)-1])], nil
}

func padMessage(plaintext []byte) []byte {
	var pad [1]byte
	_, err := rand.Read(pad[:])
	if err != nil {
		panic(err)
	}
	plaintext = append(plaintext, bytes.Repeat(pad[:], int(pad[0]&0xf))...)
	return plaintext
}

func (cli *Client) decryptMessages(addr waBinary.FullJID, nodes []waBinary.Node) {
	cli.Log.Debugln("Decrypting", len(nodes), "messages from", addr)
	for _, child := range nodes {
		if child.Tag != "enc" {
			continue
		}
		decrypted, err := cli.decryptMessage(&child, addr)
		if err != nil {
			cli.Log.Warnfln("Error decrypting message from %s: %v", addr, err)
			continue
		}

		var msg waProto.Message
		err = proto.Unmarshal(decrypted, &msg)
		if err != nil {
			cli.Log.Warnfln("Error unmarshaling decrypted message from %s: %v", addr, err)
		}

		fmt.Printf("%+v\n", &msg)
	}
}

func (cli *Client) handleMessage(node *waBinary.Node) bool {
	if node.Tag != "message" {
		return false
	}
	addr, ok := node.Attrs["from"].(waBinary.FullJID)
	if !ok {
		cli.Log.Warnln("Didn't find valid from attribute in incoming message")
		return false
	}

	go cli.decryptMessages(addr, node.GetChildren())

	return true
}
