// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

func getMediaRetryKey(mediaKey []byte) (cipherKey []byte) {
	return hkdfutil.SHA256(mediaKey, nil, []byte("WhatsApp Media Retry Notification"), 32)
}

func encryptMediaRetryReceipt(messageID types.MessageID, mediaKey []byte) (ciphertext, iv []byte, err error) {
	receipt := &waProto.ServerErrorReceipt{
		StanzaID: proto.String(messageID),
	}
	var plaintext []byte
	plaintext, err = proto.Marshal(receipt)
	if err != nil {
		err = fmt.Errorf("failed to marshal payload: %w", err)
		return
	}
	iv = random.Bytes(12)
	ciphertext, err = gcmutil.Encrypt(getMediaRetryKey(mediaKey), iv, plaintext, []byte(messageID))
	return
}

// SendMediaRetryReceipt sends a request to the phone to re-upload the media in a message.
//
// This is mostly relevant when handling history syncs and getting a 404 or 410 error downloading media.
// Rough example on how to use it (will not work out of the box, you must adjust it depending on what you need exactly):
//
//	var mediaRetryCache map[types.MessageID]*waProto.ImageMessage
//
//	evt, err := cli.ParseWebMessage(chatJID, historyMsg.GetMessage())
//	imageMsg := evt.Message.GetImageMessage() // replace this with the part of the message you want to download
//	data, err := cli.Download(imageMsg)
//	if errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404) || errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410) {
//	  err = cli.SendMediaRetryReceipt(&evt.Info, imageMsg.GetMediaKey())
//	  // You need to store the event data somewhere as it's necessary for handling the retry response.
//	  mediaRetryCache[evt.Info.ID] = imageMsg
//	}
//
// The response will come as an *events.MediaRetry. The response will then have to be decrypted
// using DecryptMediaRetryNotification and the same media key passed here. If the media retry was successful,
// the decrypted notification should contain an updated DirectPath, which can be used to download the file.
//
//	func eventHandler(rawEvt interface{}) {
//	  switch evt := rawEvt.(type) {
//	  case *events.MediaRetry:
//	    imageMsg := mediaRetryCache[evt.MessageID]
//	    retryData, err := whatsmeow.DecryptMediaRetryNotification(evt, imageMsg.GetMediaKey())
//	    if err != nil || retryData.GetResult != waProto.MediaRetryNotification_SUCCESS {
//	      return
//	    }
//	    // Use the new path to download the attachment
//	    imageMsg.DirectPath = retryData.DirectPath
//	    data, err := cli.Download(imageMsg)
//	    // Alternatively, you can use cli.DownloadMediaWithPath and provide the individual fields manually.
//	  }
//	}
func (cli *Client) SendMediaRetryReceipt(message *types.MessageInfo, mediaKey []byte) error {
	ciphertext, iv, err := encryptMediaRetryReceipt(message.ID, mediaKey)
	if err != nil {
		return fmt.Errorf("failed to prepare encrypted retry receipt: %w", err)
	}
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		return ErrNotLoggedIn
	}

	rmrAttrs := waBinary.Attrs{
		"jid":     message.Chat,
		"from_me": message.IsFromMe,
	}
	if message.IsGroup {
		rmrAttrs["participant"] = message.Sender
	}

	encryptedRequest := []waBinary.Node{
		{Tag: "enc_p", Content: ciphertext},
		{Tag: "enc_iv", Content: iv},
	}

	err = cli.sendNode(waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"id":   message.ID,
			"to":   ownID,
			"type": "server-error",
		},
		Content: []waBinary.Node{
			{Tag: "encrypt", Content: encryptedRequest},
			{Tag: "rmr", Attrs: rmrAttrs},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

// DecryptMediaRetryNotification decrypts a media retry notification using the media key.
// See Client.SendMediaRetryReceipt for more info on how to use this.
func DecryptMediaRetryNotification(evt *events.MediaRetry, mediaKey []byte) (*waProto.MediaRetryNotification, error) {
	var notif waProto.MediaRetryNotification
	if evt.Error != nil && evt.Ciphertext == nil {
		if evt.Error.Code == 2 {
			return nil, ErrMediaNotAvailableOnPhone
		}
		return nil, fmt.Errorf("%w (code: %d)", ErrUnknownMediaRetryError, evt.Error.Code)
	} else if plaintext, err := gcmutil.Decrypt(getMediaRetryKey(mediaKey), evt.IV, evt.Ciphertext, []byte(evt.MessageID)); err != nil {
		return nil, fmt.Errorf("failed to decrypt notification: %w", err)
	} else if err = proto.Unmarshal(plaintext, &notif); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notification (invalid encryption key?): %w", err)
	} else {
		return &notif, nil
	}
}

func parseMediaRetryNotification(node *waBinary.Node) (*events.MediaRetry, error) {
	ag := node.AttrGetter()
	var evt events.MediaRetry
	evt.Timestamp = ag.UnixTime("t")
	evt.MessageID = types.MessageID(ag.String("id"))
	if !ag.OK() {
		return nil, ag.Error()
	}
	rmr, ok := node.GetOptionalChildByTag("rmr")
	if !ok {
		return nil, &ElementMissingError{Tag: "rmr", In: "retry notification"}
	}
	rmrAG := rmr.AttrGetter()
	evt.ChatID = rmrAG.JID("jid")
	evt.FromMe = rmrAG.Bool("from_me")
	evt.SenderID = rmrAG.OptionalJIDOrEmpty("participant")
	if !rmrAG.OK() {
		return nil, fmt.Errorf("missing attributes in <rmr> tag: %w", rmrAG.Error())
	}

	errNode, ok := node.GetOptionalChildByTag("error")
	if ok {
		evt.Error = &events.MediaRetryError{
			Code: errNode.AttrGetter().Int("code"),
		}
		return &evt, nil
	}

	evt.Ciphertext, ok = node.GetChildByTag("encrypt", "enc_p").Content.([]byte)
	if !ok {
		return nil, &ElementMissingError{Tag: "enc_p", In: fmt.Sprintf("retry notification %s", evt.MessageID)}
	}
	evt.IV, ok = node.GetChildByTag("encrypt", "enc_iv").Content.([]byte)
	if !ok {
		return nil, &ElementMissingError{Tag: "enc_iv", In: fmt.Sprintf("retry notification %s", evt.MessageID)}
	}
	return &evt, nil
}

func (cli *Client) handleMediaRetryNotification(node *waBinary.Node) {
	evt, err := parseMediaRetryNotification(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse media retry notification: %v", err)
		return
	}
	cli.dispatchEvent(evt)
}
