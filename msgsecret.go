// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"crypto/sha256"
	"fmt"
	"time"

	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

type MsgSecretType string

const (
	EncSecretPollVote MsgSecretType = "Poll Vote"
	EncSecretReaction MsgSecretType = "Enc Reaction"
)

func generateMsgSecretKey(
	modificationType MsgSecretType, modificationSender types.JID,
	origMsgID types.MessageID, origMsgSender types.JID, origMsgSecret []byte,
) ([]byte, []byte) {
	origMsgSenderStr := origMsgSender.ToNonAD().String()
	modificationSenderStr := modificationSender.ToNonAD().String()

	useCaseSecret := make([]byte, 0, len(origMsgID)+len(origMsgSenderStr)+len(modificationSenderStr)+len(modificationType))
	useCaseSecret = append(useCaseSecret, origMsgID...)
	useCaseSecret = append(useCaseSecret, origMsgSenderStr...)
	useCaseSecret = append(useCaseSecret, modificationSenderStr...)
	useCaseSecret = append(useCaseSecret, modificationType...)

	secretKey := hkdfutil.SHA256(origMsgSecret, nil, useCaseSecret, 32)
	additionalData := []byte(fmt.Sprintf("%s\x00%s", origMsgID, modificationSenderStr))

	return secretKey, additionalData
}

func getOrigSenderFromKey(msg *events.Message, key *waProto.MessageKey) (types.JID, error) {
	if key.GetFromMe() {
		// fromMe always means the poll and vote were sent by the same user
		return msg.Info.Sender, nil
	} else if msg.Info.Chat.Server == types.DefaultUserServer {
		sender, err := types.ParseJID(key.GetRemoteJid())
		if err != nil {
			return types.EmptyJID, fmt.Errorf("failed to parse JID %q of original message sender: %w", key.GetRemoteJid(), err)
		}
		return sender, nil
	} else {
		sender, err := types.ParseJID(key.GetParticipant())
		if sender.Server != types.DefaultUserServer {
			err = fmt.Errorf("unexpected server")
		}
		if err != nil {
			return types.EmptyJID, fmt.Errorf("failed to parse JID %q of original message sender: %w", key.GetParticipant(), err)
		}
		return sender, nil
	}
}

type messageEncryptedSecret interface {
	GetEncIV() []byte
	GetEncPayload() []byte
}

func (cli *Client) decryptMsgSecret(msg *events.Message, useCase MsgSecretType, encrypted messageEncryptedSecret, origMsgKey *waProto.MessageKey) ([]byte, error) {
	pollSender, err := getOrigSenderFromKey(msg, origMsgKey)
	if err != nil {
		return nil, err
	}
	baseEncKey, err := cli.Store.MsgSecrets.GetMessageSecret(msg.Info.Chat, pollSender, origMsgKey.GetId())
	if err != nil {
		return nil, fmt.Errorf("failed to get original message secret key: %w", err)
	} else if baseEncKey == nil {
		return nil, ErrOriginalMessageSecretNotFound
	}
	secretKey, additionalData := generateMsgSecretKey(useCase, msg.Info.Sender, origMsgKey.GetId(), pollSender, baseEncKey)
	plaintext, err := gcmutil.Decrypt(secretKey, encrypted.GetEncIV(), encrypted.GetEncPayload(), additionalData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret message: %w", err)
	}
	return plaintext, nil
}

func (cli *Client) encryptMsgSecret(chat, origSender types.JID, origMsgID types.MessageID, useCase MsgSecretType, plaintext []byte) (ciphertext, iv []byte, err error) {
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		return nil, nil, ErrNotLoggedIn
	}

	baseEncKey, err := cli.Store.MsgSecrets.GetMessageSecret(chat, origSender, origMsgID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get original message secret key: %w", err)
	} else if baseEncKey == nil {
		return nil, nil, ErrOriginalMessageSecretNotFound
	}
	secretKey, additionalData := generateMsgSecretKey(useCase, ownID, origMsgID, origSender, baseEncKey)

	iv = random.Bytes(12)
	ciphertext, err = gcmutil.Encrypt(secretKey, iv, plaintext, additionalData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt secret message: %w", err)
	}
	return ciphertext, iv, nil
}

// DecryptReaction decrypts a reaction update message. This form of reactions hasn't been rolled out yet,
// so this function is likely not of much use.
//
//	if evt.Message.GetEncReactionMessage() != nil {
//		reaction, err := cli.DecryptReaction(evt)
//		if err != nil {
//			fmt.Println(":(", err)
//			return
//		}
//		fmt.Printf("Reaction message: %+v\n", reaction)
//	}
func (cli *Client) DecryptReaction(reaction *events.Message) (*waProto.ReactionMessage, error) {
	encReaction := reaction.Message.GetEncReactionMessage()
	if encReaction == nil {
		return nil, ErrNotEncryptedReactionMessage
	}
	plaintext, err := cli.decryptMsgSecret(reaction, EncSecretReaction, encReaction, encReaction.GetTargetMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt reaction: %w", err)
	}
	var msg waProto.ReactionMessage
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reaction protobuf: %w", err)
	}
	return &msg, nil
}

// DecryptPollVote decrypts a poll update message. The vote itself includes SHA-256 hashes of the selected options.
//
//	if evt.Message.GetPollUpdateMessage() != nil {
//		pollVote, err := cli.DecryptPollVote(evt)
//		if err != nil {
//			fmt.Println(":(", err)
//			return
//		}
//		fmt.Println("Selected hashes:")
//		for _, hash := range pollVote.GetSelectedOptions() {
//			fmt.Printf("- %X\n", hash)
//		}
//	}
func (cli *Client) DecryptPollVote(vote *events.Message) (*waProto.PollVoteMessage, error) {
	pollUpdate := vote.Message.GetPollUpdateMessage()
	if pollUpdate == nil {
		return nil, ErrNotPollUpdateMessage
	}
	plaintext, err := cli.decryptMsgSecret(vote, EncSecretPollVote, pollUpdate.GetVote(), pollUpdate.GetPollCreationMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt poll vote: %w", err)
	}
	var msg waProto.PollVoteMessage
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode poll vote protobuf: %w", err)
	}
	return &msg, nil
}

func getKeyFromInfo(msgInfo *types.MessageInfo) *waProto.MessageKey {
	creationKey := &waProto.MessageKey{
		RemoteJID: proto.String(msgInfo.Chat.String()),
		FromMe:    proto.Bool(msgInfo.IsFromMe),
		ID:        proto.String(msgInfo.ID),
	}
	if msgInfo.IsGroup {
		creationKey.Participant = proto.String(msgInfo.Sender.String())
	}
	return creationKey
}

// HashPollOptions hashes poll option names using SHA-256 for voting.
// This is used by BuildPollVote to convert selected option names to hashes.
func HashPollOptions(optionNames []string) [][]byte {
	optionHashes := make([][]byte, len(optionNames))
	for i, option := range optionNames {
		optionHash := sha256.Sum256([]byte(option))
		optionHashes[i] = optionHash[:]
	}
	return optionHashes
}

// BuildPollVote builds a poll vote message using the given poll message info and option names.
// The built message can be sent normally using Client.SendMessage.
//
// For example, to vote for the first option after receiving a message event (*events.Message):
//
//	if evt.Message.GetPollCreationMessage() != nil {
//		pollVoteMsg, err := cli.BuildPollVote(&evt.Info, []string{evt.Message.GetPollCreationMessage().GetOptions()[0].GetOptionName()})
//		if err != nil {
//			fmt.Println(":(", err)
//			return
//		}
//		resp, err := cli.SendMessage(context.Background(), evt.Info.Chat, pollVoteMsg)
//	}
func (cli *Client) BuildPollVote(pollInfo *types.MessageInfo, optionNames []string) (*waProto.Message, error) {
	pollUpdate, err := cli.EncryptPollVote(pollInfo, &waProto.PollVoteMessage{
		SelectedOptions: HashPollOptions(optionNames),
	})
	return &waProto.Message{PollUpdateMessage: pollUpdate}, err
}

// BuildPollCreation builds a poll creation message with the given poll name, options and maximum number of selections.
// The built message can be sent normally using Client.SendMessage.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildPollCreation("meow?", []string{"yes", "no"}, 1))
func (cli *Client) BuildPollCreation(name string, optionNames []string, selectableOptionCount int) *waProto.Message {
	msgSecret := random.Bytes(32)
	if selectableOptionCount < 0 || selectableOptionCount > len(optionNames) {
		selectableOptionCount = 0
	}
	options := make([]*waProto.PollCreationMessage_Option, len(optionNames))
	for i, option := range optionNames {
		options[i] = &waProto.PollCreationMessage_Option{OptionName: proto.String(option)}
	}
	return &waProto.Message{
		PollCreationMessage: &waProto.PollCreationMessage{
			Name:                   proto.String(name),
			Options:                options,
			SelectableOptionsCount: proto.Uint32(uint32(selectableOptionCount)),
		},
		MessageContextInfo: &waProto.MessageContextInfo{
			MessageSecret: msgSecret,
		},
	}
}

// EncryptPollVote encrypts a poll vote message. This is a slightly lower-level function, using BuildPollVote is recommended.
func (cli *Client) EncryptPollVote(pollInfo *types.MessageInfo, vote *waProto.PollVoteMessage) (*waProto.PollUpdateMessage, error) {
	plaintext, err := proto.Marshal(vote)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal poll vote protobuf: %w", err)
	}
	ciphertext, iv, err := cli.encryptMsgSecret(pollInfo.Chat, pollInfo.Sender, pollInfo.ID, EncSecretPollVote, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt poll vote: %w", err)
	}
	return &waProto.PollUpdateMessage{
		PollCreationMessageKey: getKeyFromInfo(pollInfo),
		Vote: &waProto.PollEncValue{
			EncPayload: ciphertext,
			EncIV:      iv,
		},
		SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
	}, nil
}
