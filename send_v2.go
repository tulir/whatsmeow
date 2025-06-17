package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/util/ptr"
	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type ClientV2 struct {
	client *Client

	EncryptMessageForDevicesConcurrentSize int
}

func (cliV1 *Client) UpgradeV2() *ClientV2 {
	return &ClientV2{
		client: cliV1,
	}
}

// SendMessageV2 sends the given message.
//
// This method will wait for the server to acknowledge the message before returning.
// The return value is the timestamp of the message from the server.
//
// Optional parameters like the message ID can be specified with the SendRequestExtra struct.
// Only one extra parameter is allowed, put all necessary parameters in the same struct.
//
// The message itself can contain anything you want (within the protobuf schema).
// e.g. for a simple text message, use the Conversation field:
//
//	cli.client.SendMessageV2(context.Background(), targetJID, &waE2E.Message{
//		Conversation: proto.String("Hello, World!"),
//	})
//
// Things like replies, mentioning users and the "forwarded" flag are stored in ContextInfo,
// which can be put in ExtendedTextMessage and any of the media message types.
//
// For uploading and sending media/attachments, see the Upload method.
//
// For other message types, you'll have to figure it out yourself. Looking at the protobuf schema
// in binary/proto/def.proto may be useful to find out all the allowed fields. Printing the RawMessage
// field in incoming message events to figure out what it contains is also a good way to learn how to
// send the same kind of message.
func (cli *ClientV2) SendMessageV2(ctx context.Context, to types.JID, message *waE2E.Message, extra ...SendRequestExtra) (resp SendResponse, err error) {
	if cli == nil {
		err = ErrClientIsNil
		return
	}
	var req SendRequestExtra
	if len(extra) > 1 {
		err = errors.New("only one extra parameter may be provided to SendMessage")
		return
	} else if len(extra) == 1 {
		req = extra[0]
	}
	if to.Device > 0 && !req.Peer {
		err = ErrRecipientADJID
		return
	}
	ownID := cli.client.getOwnID()
	if ownID.IsEmpty() {
		err = ErrNotLoggedIn
		return
	}

	if req.Timeout == 0 {
		req.Timeout = defaultRequestTimeout
	}
	if len(req.ID) == 0 {
		req.ID = cli.client.GenerateMessageID()
	}
	if to.Server == types.NewsletterServer {
		// TODO somehow deduplicate this with the code in sendNewsletter?
		if message.EditedMessage != nil {
			req.ID = types.MessageID(message.GetEditedMessage().GetMessage().GetProtocolMessage().GetKey().GetID())
		} else if message.ProtocolMessage != nil && message.ProtocolMessage.GetType() == waE2E.ProtocolMessage_REVOKE {
			req.ID = types.MessageID(message.GetProtocolMessage().GetKey().GetID())
		}
	}
	resp.ID = req.ID

	isInlineBotMode := false

	if !req.InlineBotJID.IsEmpty() {
		if !req.InlineBotJID.IsBot() {
			err = ErrInvalidInlineBotID
			return
		}
		isInlineBotMode = true
	}

	isBotMode := isInlineBotMode || to.IsBot()
	needsMessageSecret := isBotMode
	var extraParams nodeExtraParams

	if needsMessageSecret {
		if message.MessageContextInfo == nil {
			message.MessageContextInfo = &waE2E.MessageContextInfo{}
		}
		if message.MessageContextInfo.MessageSecret == nil {
			message.MessageContextInfo.MessageSecret = random.Bytes(32)
		}
	}

	if isBotMode {
		if message.MessageContextInfo.BotMetadata == nil {
			message.MessageContextInfo.BotMetadata = &waE2E.BotMetadata{
				PersonaID: proto.String("867051314767696$760019659443059"),
			}
		}

		if isInlineBotMode {
			// inline mode specific code
			messageSecret := message.GetMessageContextInfo().GetMessageSecret()
			message = &waE2E.Message{
				BotInvokeMessage: &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						ExtendedTextMessage: message.ExtendedTextMessage,
						MessageContextInfo: &waE2E.MessageContextInfo{
							BotMetadata: message.MessageContextInfo.BotMetadata,
						},
					},
				},
				MessageContextInfo: message.MessageContextInfo,
			}

			botMessage := &waE2E.Message{
				BotInvokeMessage: message.BotInvokeMessage,
				MessageContextInfo: &waE2E.MessageContextInfo{
					BotMetadata:      message.MessageContextInfo.BotMetadata,
					BotMessageSecret: applyBotMessageHKDF(messageSecret),
				},
			}

			messagePlaintext, _, marshalErr := marshalMessage(req.InlineBotJID, botMessage)
			if marshalErr != nil {
				err = marshalErr
				return
			}

			participantNodes, _ := cli.client.encryptMessageForDevices(ctx, []types.JID{req.InlineBotJID}, resp.ID, messagePlaintext, nil, waBinary.Attrs{})
			extraParams.botNode = &waBinary.Node{
				Tag:     "bot",
				Attrs:   nil,
				Content: participantNodes,
			}
		}
	}

	var groupParticipants []types.JID
	if to.Server == types.GroupServer || to.Server == types.BroadcastServer {
		start := time.Now()
		if to.Server == types.GroupServer {
			var cachedData *groupMetaCache
			cachedData, err = cli.client.getCachedGroupData(ctx, to)
			if err != nil {
				err = fmt.Errorf("failed to get group members: %w", err)
				return
			}
			groupParticipants = cachedData.Members
			// TODO this is fairly hacky, is there a proper way to determine which identity the message is sent with?
			if cachedData.AddressingMode == types.AddressingModeLID {
				ownID = cli.client.getOwnLID()
				extraParams.addressingMode = types.AddressingModeLID
				if req.Meta == nil {
					req.Meta = &types.MsgMetaInfo{}
				}
				req.Meta.DeprecatedLIDSession = ptr.Ptr(false)
			} else if cachedData.CommunityAnnouncementGroup && req.Meta != nil {
				ownID = cli.client.getOwnLID()
				// Why is this set to PN?
				extraParams.addressingMode = types.AddressingModePN
			}
		} else {
			// TODO use context
			groupParticipants, err = cli.client.getBroadcastListParticipants(to)
			if err != nil {
				err = fmt.Errorf("failed to get broadcast list members: %w", err)
				return
			}
		}
		resp.DebugTimings.GetParticipants = time.Since(start)
	} else if to.Server == types.HiddenUserServer {
		ownID = cli.client.getOwnLID()
		extraParams.addressingMode = types.AddressingModeLID
		// if req.Meta == nil {
		// 	req.Meta = &types.MsgMetaInfo{}
		// }
		// req.Meta.DeprecatedLIDSession = ptr.Ptr(false)
	}
	if req.Meta != nil {
		extraParams.metaNode = &waBinary.Node{
			Tag:   "meta",
			Attrs: waBinary.Attrs{},
		}
		if req.Meta.DeprecatedLIDSession != nil {
			extraParams.metaNode.Attrs["deprecated_lid_session"] = *req.Meta.DeprecatedLIDSession
		}
		if req.Meta.ThreadMessageID != "" {
			extraParams.metaNode.Attrs["thread_msg_id"] = req.Meta.ThreadMessageID
			extraParams.metaNode.Attrs["thread_msg_sender_jid"] = req.Meta.ThreadMessageSenderJID
		}
	}

	resp.Sender = ownID

	start := time.Now()
	// Sending multiple messages at a time can cause weird issues and makes it harder to retry safely
	cli.client.messageSendLock.Lock()
	resp.DebugTimings.Queue = time.Since(start)
	defer cli.client.messageSendLock.Unlock()

	respChan := cli.client.waitResponse(req.ID)
	// Peer message retries aren't implemented yet
	if !req.Peer {
		cli.client.addRecentMessage(to, req.ID, message, nil)
	}

	if message.GetMessageContextInfo().GetMessageSecret() != nil {
		err = cli.client.Store.MsgSecrets.PutMessageSecret(to, ownID, req.ID, message.GetMessageContextInfo().GetMessageSecret())
		if err != nil {
			cli.client.Log.Warnf("Failed to store message secret key for outgoing message %s: %v", req.ID, err)
		} else {
			cli.client.Log.Debugf("Stored message secret key for outgoing message %s", req.ID)
		}
	}
	var phash string
	var data []byte
	switch to.Server {
	case types.GroupServer, types.BroadcastServer:
		phash, data, err = cli.sendGroupV2(ctx, to, groupParticipants, req.ID, message, &resp.DebugTimings, extraParams)
	case types.DefaultUserServer, types.BotServer:
		if req.Peer {
			data, err = cli.client.sendPeerMessage(to, req.ID, message, &resp.DebugTimings)
		} else {
			data, err = cli.client.sendDM(ctx, ownID, to, req.ID, message, &resp.DebugTimings, extraParams)
		}
	case types.NewsletterServer:
		data, err = cli.client.sendNewsletter(to, req.ID, message, req.MediaHandle, &resp.DebugTimings)
	default:
		err = fmt.Errorf("%w %s", ErrUnknownServer, to.Server)
	}
	start = time.Now()
	if err != nil {
		cli.client.cancelResponse(req.ID, respChan)
		return
	}
	var respNode *waBinary.Node
	var timeoutChan <-chan time.Time
	if req.Timeout > 0 {
		timeoutChan = time.After(req.Timeout)
	} else {
		timeoutChan = make(<-chan time.Time)
	}
	select {
	case respNode = <-respChan:
	case <-timeoutChan:
		cli.client.cancelResponse(req.ID, respChan)
		err = ErrMessageTimedOut
		return
	case <-ctx.Done():
		cli.client.cancelResponse(req.ID, respChan)
		err = ctx.Err()
		return
	}
	resp.DebugTimings.Resp = time.Since(start)
	if isDisconnectNode(respNode) {
		start = time.Now()
		respNode, err = cli.client.retryFrame("message send", req.ID, data, respNode, ctx, 0)
		resp.DebugTimings.Retry = time.Since(start)
		if err != nil {
			return
		}
	}
	ag := respNode.AttrGetter()
	resp.ServerID = types.MessageServerID(ag.OptionalInt("server_id"))
	resp.Timestamp = ag.UnixTime("t")
	if errorCode := ag.Int("error"); errorCode != 0 {
		err = fmt.Errorf("%w %d", ErrServerReturnedError, errorCode)
	}
	expectedPHash := ag.OptionalString("phash")
	if len(expectedPHash) > 0 && phash != expectedPHash {
		cli.client.Log.Warnf("Server returned different participant list hash when sending to %s. Some devices may not have received the message.", to)
		// TODO also invalidate device list caches
		cli.client.groupCacheLock.Lock()
		delete(cli.client.groupCache, to)
		cli.client.groupCacheLock.Unlock()
	}
	return
}

func (cli *ClientV2) sendGroupV2(
	ctx context.Context,
	to types.JID,
	participants []types.JID,
	id types.MessageID,
	message *waE2E.Message,
	timings *MessageDebugTimings,
	extraParams nodeExtraParams,
) (string, []byte, error) {
	start := time.Now()
	plaintext, _, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return "", nil, err
	}

	start = time.Now()
	builder := groups.NewGroupSessionBuilder(cli.client.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(to.String(), cli.client.getOwnLID().SignalAddress())
	signalSKDMessage, err := builder.Create(senderKeyName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create sender key distribution message to send %s to %s: %w", id, to, err)
	}
	skdMessage := &waE2E.Message{
		SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{
			GroupID:                             proto.String(to.String()),
			AxolotlSenderKeyDistributionMessage: signalSKDMessage.Serialize(),
		},
	}
	skdPlaintext, err := proto.Marshal(skdMessage)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal sender key distribution message to send %s to %s: %w", id, to, err)
	}

	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.client.Store)
	encrypted, err := cipher.Encrypt(padMessage(plaintext))
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt group message to send %s to %s: %w", id, to, err)
	}
	ciphertext := encrypted.SignedSerialize()
	timings.GroupEncrypt = time.Since(start)

	node, allDevices, err := cli.prepareMessageNodeV2(
		ctx, to, id, message, participants, skdPlaintext, nil, timings, extraParams,
	)
	if err != nil {
		return "", nil, err
	}

	phash := participantListHashV2(allDevices)
	node.Attrs["phash"] = phash
	skMsg := waBinary.Node{
		Tag:     "enc",
		Content: ciphertext,
		Attrs:   waBinary.Attrs{"v": "2", "type": "skmsg"},
	}
	if mediaType := getMediaTypeFromMessage(message); mediaType != "" {
		skMsg.Attrs["mediatype"] = mediaType
	}
	node.Content = append(node.GetChildren(), skMsg)

	start = time.Now()
	data, err := cli.client.sendNodeAndGetData(*node)
	timings.Send = time.Since(start)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return phash, data, nil
}

func (cli *ClientV2) prepareMessageNodeV2(
	ctx context.Context,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	participants []types.JID,
	plaintext, dsmPlaintext []byte,
	timings *MessageDebugTimings,
	extraParams nodeExtraParams,
) (*waBinary.Node, []types.JID, error) {
	start := time.Now()
	allDevices, err := cli.client.GetUserDevicesContext(ctx, participants)
	timings.GetDevices = time.Since(start)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device list: %w", err)
	}

	msgType := getTypeFromMessage(message)
	encAttrs := waBinary.Attrs{}
	// Only include encMediaType for 1:1 messages (groups don't have a device-sent message plaintext)
	if encMediaType := getMediaTypeFromMessage(message); dsmPlaintext != nil && encMediaType != "" {
		encAttrs["mediatype"] = encMediaType
	}
	attrs := waBinary.Attrs{
		"id":   id,
		"type": msgType,
		"to":   to,
	}
	// TODO this is a very hacky hack for announcement group messages, why is it pn anyway?
	if extraParams.addressingMode != "" {
		attrs["addressing_mode"] = string(extraParams.addressingMode)
	}
	if editAttr := getEditAttribute(message); editAttr != "" {
		attrs["edit"] = string(editAttr)
		encAttrs["decrypt-fail"] = string(events.DecryptFailHide)
	}
	if msgType == "reaction" || message.GetPollUpdateMessage() != nil {
		encAttrs["decrypt-fail"] = string(events.DecryptFailHide)
	}

	start = time.Now()
	var participantNodes []waBinary.Node
	var includeIdentity bool
	if cli.EncryptMessageForDevicesConcurrentSize > 0 {
		participantNodes, includeIdentity = cli.encryptMessageForDevicesConcurrent(
			ctx, allDevices, id, plaintext, dsmPlaintext, encAttrs,
		)
	} else {
		participantNodes, includeIdentity = cli.client.encryptMessageForDevices(
			ctx, allDevices, id, plaintext, dsmPlaintext, encAttrs,
		)
	}
	timings.PeerEncrypt = time.Since(start)
	participantNode := waBinary.Node{
		Tag:     "participants",
		Content: participantNodes,
	}
	return &waBinary.Node{
		Tag:   "message",
		Attrs: attrs,
		Content: cli.client.getMessageContent(
			participantNode, message, attrs, includeIdentity, extraParams,
		),
	}, allDevices, nil
}

func (cli *ClientV2) encryptMessageForDevicesConcurrent(
	ctx context.Context,
	allDevices []types.JID,
	id string,
	msgPlaintext, dsmPlaintext []byte,
	encAttrs waBinary.Attrs,
) ([]waBinary.Node, bool) {
	ownJID := cli.client.getOwnID()
	ownLID := cli.client.getOwnLID()
	includeIdentity := atomic.Bool{}
	participantNodes := make([]waBinary.Node, 0, len(allDevices))
	var retryDevices, retryEncryptionIdentities []types.JID
	limitChan := make(chan struct{}, cli.EncryptMessageForDevicesConcurrentSize)
	encryptedChan := make(chan *waBinary.Node)
	retryDeviceChan := make(chan types.JID)
	retryEncryptionIdentityChan := make(chan types.JID)
	wg := sync.WaitGroup{}
	receiverWg := sync.WaitGroup{}

	receiverWg.Add(1)
	go func() {
		defer receiverWg.Done()
		for encrypted := range encryptedChan {
			participantNodes = append(participantNodes, *encrypted)
		}
	}()

	receiverWg.Add(1)
	go func() {
		defer receiverWg.Done()
		for retryDevice := range retryDeviceChan {
			retryDevices = append(retryDevices, retryDevice)
		}
	}()

	receiverWg.Add(1)
	go func() {
		defer receiverWg.Done()
		for retryEncryptionIdentity := range retryEncryptionIdentityChan {
			retryEncryptionIdentities = append(retryEncryptionIdentities, retryEncryptionIdentity)
		}
	}()

	for _, jid := range allDevices {
		plaintext := msgPlaintext
		if (jid.User == ownJID.User || jid.User == ownLID.User) && dsmPlaintext != nil {
			if jid == ownJID {
				continue
			}
			plaintext = dsmPlaintext
		}
		encryptionIdentity := jid
		if jid.Server == types.DefaultUserServer {
			lidForPN, err := cli.client.Store.LIDs.GetLIDForPN(ctx, jid)
			if err != nil {
				cli.client.Log.Warnf("Failed to get LID for %s: %v", jid, err)
			} else if !lidForPN.IsEmpty() {
				cli.client.migrateSessionStore(jid, lidForPN)
				encryptionIdentity = lidForPN
			}
		}

		wg.Add(1)
		limitChan <- struct{}{}
		go func() {
			defer func() {
				<-limitChan
				wg.Done()
			}()
			encrypted, isPreKey, err := cli.client.encryptMessageForDeviceAndWrap(
				plaintext, jid, encryptionIdentity, nil, encAttrs,
			)
			if errors.Is(err, ErrNoSession) {
				retryDeviceChan <- jid
				retryEncryptionIdentityChan <- encryptionIdentity
				return
			} else if err != nil {
				cli.client.Log.Warnf("Failed to encrypt %s for %s: %v", id, jid, err)
				return
			}

			encryptedChan <- encrypted

			if isPreKey {
				includeIdentity.Store(true)
			}
		}()
	}

	wg.Wait()
	close(limitChan)
	close(encryptedChan)
	close(retryDeviceChan)
	close(retryEncryptionIdentityChan)
	receiverWg.Wait()

	if len(retryDevices) > 0 {
		limitChan := make(chan struct{}, cli.EncryptMessageForDevicesConcurrentSize)
		encryptedChan := make(chan *waBinary.Node)
		wg := sync.WaitGroup{}

		receiverWg.Add(1)
		go func() {
			defer receiverWg.Done()
			for encrypted := range encryptedChan {
				participantNodes = append(participantNodes, *encrypted)
			}
		}()

		bundles, err := cli.client.fetchPreKeys(ctx, retryDevices)
		if err != nil {
			cli.client.Log.Warnf("Failed to fetch prekeys for %v to retry encryption: %v", retryDevices, err)
		} else {
			for i, jid := range retryDevices {
				resp := bundles[jid]
				if resp.err != nil {
					cli.client.Log.Warnf("Failed to fetch prekey for %s: %v", jid, resp.err)
					continue
				}

				plaintext := msgPlaintext
				if (jid.User == ownJID.User || jid.User == ownLID.User) && dsmPlaintext != nil {
					plaintext = dsmPlaintext
				}

				limitChan <- struct{}{}
				wg.Add(1)
				go func() {
					defer func() {
						<-limitChan
						wg.Done()
					}()
					encrypted, isPreKey, err := cli.client.encryptMessageForDeviceAndWrap(
						plaintext, jid, retryEncryptionIdentities[i], resp.bundle, encAttrs,
					)
					if err != nil {
						cli.client.Log.Warnf("Failed to encrypt %s for %s (retry): %v", id, jid, err)
						return
					}
					if isPreKey {
						includeIdentity.Store(true)
					}
					encryptedChan <- encrypted
				}()
			}
		}

		wg.Wait()
		close(encryptedChan)
		close(limitChan)
		receiverWg.Wait()
	}
	return participantNodes, includeIdentity.Load()
}
