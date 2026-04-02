// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

type MsgSecretType string

const (
	EncSecretPollVote      MsgSecretType = "Poll Vote"
	EncSecretReaction      MsgSecretType = "Enc Reaction"
	EncSecretComment       MsgSecretType = "Enc Comment"
	EncSecretReportToken   MsgSecretType = "Report Token"
	EncSecretEventResponse MsgSecretType = "Event Response"
	EncSecretEventEdit     MsgSecretType = "Event Edit"
	EncSecretBotMsg        MsgSecretType = "Bot Message"
)

func applyBotMessageHKDF(messageSecret []byte) []byte {
	return hkdfutil.SHA256(messageSecret, nil, []byte(EncSecretBotMsg), 32)
}

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
	var additionalData []byte
	switch modificationType {
	case EncSecretPollVote, EncSecretEventResponse, "":
		additionalData = fmt.Appendf(nil, "%s\x00%s", origMsgID, modificationSenderStr)
	}

	return secretKey, additionalData
}

func getOrigSenderFromKey(msg *events.Message, key *waCommon.MessageKey) (types.JID, error) {
	if key.GetFromMe() {
		// fromMe always means the poll and vote were sent by the same user
		// TODO this is wrong if the message key used @s.whatsapp.net, but the new event is from @lid
		return msg.Info.Sender, nil
	} else if msg.Info.Chat.Server == types.DefaultUserServer || msg.Info.Chat.Server == types.HiddenUserServer {
		sender, err := types.ParseJID(key.GetRemoteJID())
		if err != nil {
			return types.EmptyJID, fmt.Errorf("failed to parse JID %q of original message sender: %w", key.GetRemoteJID(), err)
		}
		return sender, nil
	} else {
		sender, err := types.ParseJID(key.GetParticipant())
		if sender.Server != types.DefaultUserServer && sender.Server != types.HiddenUserServer {
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

func (cli *Client) decryptMsgSecret(ctx context.Context, msg *events.Message, useCase MsgSecretType, encrypted messageEncryptedSecret, origMsgKey *waCommon.MessageKey) ([]byte, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	origSender, err := getOrigSenderFromKey(msg, origMsgKey)
	if err != nil {
		return nil, err
	}
	baseEncKey, origSender, err := cli.Store.MsgSecrets.GetMessageSecret(ctx, msg.Info.Chat, origSender, origMsgKey.GetID())
	if err != nil {
		return nil, fmt.Errorf("failed to get original message secret key: %w", err)
	}
	if baseEncKey == nil {
		return nil, ErrOriginalMessageSecretNotFound
	}
	secretKey, additionalData := generateMsgSecretKey(useCase, msg.Info.Sender, origMsgKey.GetID(), origSender, baseEncKey)
	plaintext, err := gcmutil.Decrypt(secretKey, encrypted.GetEncIV(), encrypted.GetEncPayload(), additionalData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret message: %w", err)
	}
	return plaintext, nil
}

func (cli *Client) encryptMsgSecret(ctx context.Context, ownID, chat, origSender types.JID, origMsgID types.MessageID, useCase MsgSecretType, plaintext []byte) (ciphertext, iv []byte, err error) {
	if cli == nil {
		return nil, nil, ErrClientIsNil
	} else if ownID.IsEmpty() {
		return nil, nil, ErrNotLoggedIn
	}

	baseEncKey, origSender, err := cli.Store.MsgSecrets.GetMessageSecret(ctx, chat, origSender, origMsgID)
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

func (cli *Client) decryptBotMessage(ctx context.Context, messageSecret []byte, msMsg messageEncryptedSecret, messageID types.MessageID, targetSenderJID types.JID, info *types.MessageInfo) ([]byte, error) {
	newKey, additionalData := generateMsgSecretKey("", info.Sender, messageID, targetSenderJID, applyBotMessageHKDF(messageSecret))

	plaintext, err := gcmutil.Decrypt(newKey, msMsg.GetEncIV(), msMsg.GetEncPayload(), additionalData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret message: %w", err)
	}

	return plaintext, nil
}

// DecryptReaction decrypts a reaction message in a community announcement group.
//
//	if evt.Message.GetEncReactionMessage() != nil {
//		reaction, err := cli.DecryptReaction(evt)
//		if err != nil {
//			fmt.Println(":(", err)
//			return
//		}
//		fmt.Printf("Reaction message: %+v\n", reaction)
//	}
func (cli *Client) DecryptReaction(ctx context.Context, reaction *events.Message) (*waE2E.ReactionMessage, error) {
	encReaction := reaction.Message.GetEncReactionMessage()
	if encReaction == nil {
		return nil, ErrNotEncryptedReactionMessage
	}
	plaintext, err := cli.decryptMsgSecret(ctx, reaction, EncSecretReaction, encReaction, encReaction.GetTargetMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt reaction: %w", err)
	}
	var msg waE2E.ReactionMessage
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reaction protobuf: %w", err)
	}
	return &msg, nil
}

// DecryptComment decrypts a reply/comment message in a community announcement group.
//
//	if evt.Message.GetEncCommentMessage() != nil {
//		comment, err := cli.DecryptComment(evt)
//		if err != nil {
//			fmt.Println(":(", err)
//			return
//		}
//		fmt.Printf("Comment message: %+v\n", comment)
//	}
func (cli *Client) DecryptComment(ctx context.Context, comment *events.Message) (*waE2E.Message, error) {
	encComment := comment.Message.GetEncCommentMessage()
	if encComment == nil {
		return nil, ErrNotEncryptedCommentMessage
	}
	plaintext, err := cli.decryptMsgSecret(ctx, comment, EncSecretComment, encComment, encComment.GetTargetMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt comment: %w", err)
	}
	var msg waE2E.Message
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode comment protobuf: %w", err)
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
func (cli *Client) DecryptPollVote(ctx context.Context, vote *events.Message) (*waE2E.PollVoteMessage, error) {
	pollUpdate := vote.Message.GetPollUpdateMessage()
	if pollUpdate == nil {
		return nil, ErrNotPollUpdateMessage
	}
	plaintext, err := cli.decryptMsgSecret(ctx, vote, EncSecretPollVote, pollUpdate.GetVote(), pollUpdate.GetPollCreationMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt poll vote: %w", err)
	}
	var msg waE2E.PollVoteMessage
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode poll vote protobuf: %w", err)
	}
	return &msg, nil
}

func (cli *Client) DecryptSecretEncryptedMessage(ctx context.Context, evt *events.Message) (*waE2E.Message, error) {
	encMessage := evt.Message.GetSecretEncryptedMessage()
	if encMessage == nil {
		return nil, ErrNotSecretEncryptedMessage
	}
	if encMessage.GetSecretEncType() != waE2E.SecretEncryptedMessage_EVENT_EDIT {
		return nil, fmt.Errorf("unsupported secret enc type: %s", encMessage.SecretEncType.String())
	}
	plaintext, err := cli.decryptMsgSecret(ctx, evt, EncSecretEventEdit, encMessage, encMessage.GetTargetMessageKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}
	var msg waE2E.Message
	err = proto.Unmarshal(plaintext, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message protobuf: %w", err)
	}
	if evt.Message.MessageContextInfo != nil && msg.MessageContextInfo == nil {
		msg.MessageContextInfo = evt.Message.MessageContextInfo
	}
	return &msg, nil
}

func getKeyFromInfo(msgInfo *types.MessageInfo) *waCommon.MessageKey {
	creationKey := &waCommon.MessageKey{
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
func (cli *Client) BuildPollVote(ctx context.Context, pollInfo *types.MessageInfo, optionNames []string) (*waE2E.Message, error) {
	pollUpdate, err := cli.EncryptPollVote(ctx, pollInfo, &waE2E.PollVoteMessage{
		SelectedOptions: HashPollOptions(optionNames),
	})
	return &waE2E.Message{PollUpdateMessage: pollUpdate}, err
}

// BuildPollCreation builds a poll creation message with the given poll name, options and maximum number of selections.
// The built message can be sent normally using Client.SendMessage.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildPollCreation("meow?", []string{"yes", "no"}, 1))
func (cli *Client) BuildPollCreation(name string, optionNames []string, selectableOptionCount int) *waE2E.Message {
	msgSecret := random.Bytes(32)
	if selectableOptionCount < 0 || selectableOptionCount > len(optionNames) {
		selectableOptionCount = 0
	}
	options := make([]*waE2E.PollCreationMessage_Option, len(optionNames))
	for i, option := range optionNames {
		options[i] = &waE2E.PollCreationMessage_Option{OptionName: proto.String(option)}
	}
	return &waE2E.Message{
		PollCreationMessage: &waE2E.PollCreationMessage{
			Name:                   proto.String(name),
			Options:                options,
			SelectableOptionsCount: proto.Uint32(uint32(selectableOptionCount)),
		},
		MessageContextInfo: &waE2E.MessageContextInfo{
			MessageSecret: msgSecret,
		},
	}
}

// EncryptPollVote encrypts a poll vote message. This is a slightly lower-level function, using BuildPollVote is recommended.
func (cli *Client) EncryptPollVote(ctx context.Context, pollInfo *types.MessageInfo, vote *waE2E.PollVoteMessage) (*waE2E.PollUpdateMessage, error) {
	plaintext, err := proto.Marshal(vote)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal poll vote protobuf: %w", err)
	}
	ciphertext, iv, err := cli.encryptMsgSecret(ctx, cli.getOwnID(), pollInfo.Chat, pollInfo.Sender, pollInfo.ID, EncSecretPollVote, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt poll vote: %w", err)
	}
	return &waE2E.PollUpdateMessage{
		PollCreationMessageKey: getKeyFromInfo(pollInfo),
		Vote: &waE2E.PollEncValue{
			EncPayload: ciphertext,
			EncIV:      iv,
		},
		SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
	}, nil
}

func (cli *Client) EncryptComment(ctx context.Context, rootMsgInfo *types.MessageInfo, comment *waE2E.Message) (*waE2E.Message, error) {
	plaintext, err := proto.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal comment protobuf: %w", err)
	}
	// TODO is hardcoding LID here correct? What about polls?
	ciphertext, iv, err := cli.encryptMsgSecret(ctx, cli.getOwnLID(), rootMsgInfo.Chat, rootMsgInfo.Sender, rootMsgInfo.ID, EncSecretComment, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt comment: %w", err)
	}
	return &waE2E.Message{
		EncCommentMessage: &waE2E.EncCommentMessage{
			TargetMessageKey: &waCommon.MessageKey{
				RemoteJID:   proto.String(rootMsgInfo.Chat.String()),
				Participant: proto.String(rootMsgInfo.Sender.ToNonAD().String()),
				FromMe:      proto.Bool(rootMsgInfo.IsFromMe),
				ID:          proto.String(rootMsgInfo.ID),
			},
			EncPayload: ciphertext,
			EncIV:      iv,
		},
	}, nil
}

func (cli *Client) EncryptReaction(ctx context.Context, rootMsgInfo *types.MessageInfo, reaction *waE2E.ReactionMessage) (*waE2E.EncReactionMessage, error) {
	reactionKey := reaction.Key
	reaction.Key = nil
	plaintext, err := proto.Marshal(reaction)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reaction protobuf: %w", err)
	}
	ciphertext, iv, err := cli.encryptMsgSecret(ctx, cli.getOwnLID(), rootMsgInfo.Chat, rootMsgInfo.Sender, rootMsgInfo.ID, EncSecretReaction, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt reaction: %w", err)
	}
	return &waE2E.EncReactionMessage{
		TargetMessageKey: reactionKey,
		EncPayload:       ciphertext,
		EncIV:            iv,
	}, nil
}
