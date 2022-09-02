// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var cli *whatsmeow.Client
var log waLog.Logger

var logLevel = "INFO"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")
var requestFullSync = flag.Bool("request-full-sync", false, "Request full (1 year) history sync when logging in?")

func main() {
	waBinary.IndentXML = true
	flag.Parse()

	if *debugLogs {
		logLevel = "DEBUG"
	}
	if *requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
	}
	log = waLog.Stdout("Main", logLevel, true)

	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(*dbDialect, *dbAddress, dbLog)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		return
	}
	device, err := storeContainer.GetFirstDevice()
	if err != nil {
		log.Errorf("Failed to get device: %v", err)
		return
	}

	cli = whatsmeow.NewClient(device, waLog.Stdout("Client", logLevel, true))

	ch, err := cli.GetQRChannel(context.Background())
	if err != nil {
		// This error means that we're already logged in, so ignore it.
		if !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			log.Errorf("Failed to get QR channel: %v", err)
		}
	} else {
		go func() {
			for evt := range ch {
				if evt.Event == "code" {
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				} else {
					log.Infof("QR channel result: %s", evt.Event)
				}
			}
		}()
	}

	cli.AddEventHandler(handler)
	err = cli.Connect()
	if err != nil {
		log.Errorf("Failed to connect: %v", err)
		return
	}

	c := make(chan os.Signal)
	input := make(chan string)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(input)
		scan := bufio.NewScanner(os.Stdin)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if len(line) > 0 {
				input <- line
			}
		}
	}()
	for {
		select {
		case <-c:
			log.Infof("Interrupt received, exiting")
			cli.Disconnect()
			return
		case cmd := <-input:
			if len(cmd) == 0 {
				log.Infof("Stdin closed, exiting")
				cli.Disconnect()
				return
			}
			args := strings.Fields(cmd)
			cmd = args[0]
			args = args[1:]
			go handleCmd(strings.ToLower(cmd), args)
		}
	}
}

func parseJID(arg string) (types.JID, bool) {
	if arg[0] == '+' {
		arg = arg[1:]
	}
	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), true
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			log.Errorf("Invalid JID %s: %v", arg, err)
			return recipient, false
		} else if recipient.User == "" {
			log.Errorf("Invalid JID %s: no server specified", arg)
			return recipient, false
		}
		return recipient, true
	}
}

func handleCmd(cmd string, args []string) {
	switch cmd {
	case "reconnect":
		cli.Disconnect()
		err := cli.Connect()
		if err != nil {
			log.Errorf("Failed to connect: %v", err)
		}
	case "logout":
		err := cli.Logout()
		if err != nil {
			log.Errorf("Error logging out: %v", err)
		} else {
			log.Infof("Successfully logged out")
		}
	case "appstate":
		if len(args) < 1 {
			log.Errorf("Usage: appstate <types...>")
			return
		}
		names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
		if args[0] == "all" {
			names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
		}
		resync := len(args) > 1 && args[1] == "resync"
		for _, name := range names {
			err := cli.FetchAppState(name, resync, false)
			if err != nil {
				log.Errorf("Failed to sync app state: %v", err)
			}
		}
	case "request-appstate-key":
		if len(args) < 1 {
			log.Errorf("Usage: request-appstate-key <ids...>")
			return
		}
		var keyIDs = make([][]byte, len(args))
		for i, id := range args {
			decoded, err := hex.DecodeString(id)
			if err != nil {
				log.Errorf("Failed to decode %s as hex: %v", id, err)
				return
			}
			keyIDs[i] = decoded
		}
		cli.DangerousInternals().RequestAppStateKeys(context.Background(), keyIDs)
	case "checkuser":
		if len(args) < 1 {
			log.Errorf("Usage: checkuser <phone numbers...>")
			return
		}
		resp, err := cli.IsOnWhatsApp(args)
		if err != nil {
			log.Errorf("Failed to check if users are on WhatsApp:", err)
		} else {
			for _, item := range resp {
				if item.VerifiedName != nil {
					log.Infof("%s: on whatsapp: %t, JID: %s, business name: %s", item.Query, item.IsIn, item.JID, item.VerifiedName.Details.GetVerifiedName())
				} else {
					log.Infof("%s: on whatsapp: %t, JID: %s", item.Query, item.IsIn, item.JID)
				}
			}
		}
	case "checkupdate":
		resp, err := cli.CheckUpdate()
		if err != nil {
			log.Errorf("Failed to check for updates: %v", err)
		} else {
			log.Debugf("Version data: %#v", resp)
			if resp.ParsedVersion == store.GetWAVersion() {
				log.Infof("Client is up to date")
			} else if store.GetWAVersion().LessThan(resp.ParsedVersion) {
				log.Warnf("Client is outdated")
			} else {
				log.Infof("Client is newer than latest")
			}
		}
	case "subscribepresence":
		if len(args) < 1 {
			log.Errorf("Usage: subscribepresence <jid>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		err := cli.SubscribePresence(jid)
		if err != nil {
			fmt.Println(err)
		}
	case "presence":
		fmt.Println(cli.SendPresence(types.Presence(args[0])))
	case "chatpresence":
		if len(args) == 2 {
			args = append(args, "")
		} else if len(args) < 2 {
			log.Errorf("Usage: chatpresence <jid> <composing/paused> [audio]")
			return
		}
		jid, _ := types.ParseJID(args[0])
		fmt.Println(cli.SendChatPresence(jid, types.ChatPresence(args[1]), types.ChatPresenceMedia(args[2])))
	case "privacysettings":
		resp, err := cli.TryFetchPrivacySettings(false)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("%+v\n", resp)
		}
	case "getuser":
		if len(args) < 1 {
			log.Errorf("Usage: getuser <jids...>")
			return
		}
		var jids []types.JID
		for _, arg := range args {
			jid, ok := parseJID(arg)
			if !ok {
				return
			}
			jids = append(jids, jid)
		}
		resp, err := cli.GetUserInfo(jids)
		if err != nil {
			log.Errorf("Failed to get user info: %v", err)
		} else {
			for jid, info := range resp {
				log.Infof("%s: %+v", jid, info)
			}
		}
	case "getavatar":
		if len(args) < 1 {
			log.Errorf("Usage: getavatar <jid> [existing ID] [--preview]")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		existingID := ""
		if len(args) > 2 {
			existingID = args[2]
		}
		pic, err := cli.GetProfilePictureInfo(jid, args[len(args)-1] == "--preview", existingID)
		if err != nil {
			log.Errorf("Failed to get avatar: %v", err)
		} else if pic != nil {
			log.Infof("Got avatar ID %s: %s", pic.ID, pic.URL)
		} else {
			log.Infof("No avatar found")
		}
	case "getgroup":
		if len(args) < 1 {
			log.Errorf("Usage: getgroup <jid>")
			return
		}
		group, ok := parseJID(args[0])
		if !ok {
			return
		} else if group.Server != types.GroupServer {
			log.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return
		}
		resp, err := cli.GetGroupInfo(group)
		if err != nil {
			log.Errorf("Failed to get group info: %v", err)
		} else {
			log.Infof("Group info: %+v", resp)
		}
	case "listgroups":
		groups, err := cli.GetJoinedGroups()
		if err != nil {
			log.Errorf("Failed to get group list: %v", err)
		} else {
			for _, group := range groups {
				log.Infof("%+v", group)
			}
		}
	case "getinvitelink":
		if len(args) < 1 {
			log.Errorf("Usage: getinvitelink <jid> [--reset]")
			return
		}
		group, ok := parseJID(args[0])
		if !ok {
			return
		} else if group.Server != types.GroupServer {
			log.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return
		}
		resp, err := cli.GetGroupInviteLink(group, len(args) > 1 && args[1] == "--reset")
		if err != nil {
			log.Errorf("Failed to get group invite link: %v", err)
		} else {
			log.Infof("Group invite link: %s", resp)
		}
	case "queryinvitelink":
		if len(args) < 1 {
			log.Errorf("Usage: queryinvitelink <link>")
			return
		}
		resp, err := cli.GetGroupInfoFromLink(args[0])
		if err != nil {
			log.Errorf("Failed to resolve group invite link: %v", err)
		} else {
			log.Infof("Group info: %+v", resp)
		}
	case "querybusinesslink":
		if len(args) < 1 {
			log.Errorf("Usage: querybusinesslink <link>")
			return
		}
		resp, err := cli.ResolveBusinessMessageLink(args[0])
		if err != nil {
			log.Errorf("Failed to resolve business message link: %v", err)
		} else {
			log.Infof("Business info: %+v", resp)
		}
	case "joininvitelink":
		if len(args) < 1 {
			log.Errorf("Usage: acceptinvitelink <link>")
			return
		}
		groupID, err := cli.JoinGroupWithLink(args[0])
		if err != nil {
			log.Errorf("Failed to join group via invite link: %v", err)
		} else {
			log.Infof("Joined %s", groupID)
		}
	case "getstatusprivacy":
		resp, err := cli.GetStatusPrivacy()
		fmt.Println(err)
		fmt.Println(resp)
	case "setdisappeartimer":
		if len(args) < 2 {
			log.Errorf("Usage: setdisappeartimer <jid> <days>")
			return
		}
		days, err := strconv.Atoi(args[1])
		if err != nil {
			log.Errorf("Invalid duration: %v", err)
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		err = cli.SetDisappearingTimer(recipient, time.Duration(days)*24*time.Hour)
		if err != nil {
			log.Errorf("Failed to set disappearing timer: %v", err)
		}
	case "send":
		if len(args) < 2 {
			log.Errorf("Usage: send <jid> <text>")
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		resp, err := cli.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			log.Errorf("Error sending message: %v", err)
		} else {
			log.Infof("Message sent (server timestamp: %s)", resp.Timestamp)
		}
	case "multisend":
		if len(args) < 3 {
			log.Errorf("Usage: multisend <jids...> -- <text>")
			return
		}
		var recipients []types.JID
		for len(args) > 0 && args[0] != "--" {
			recipient, ok := parseJID(args[0])
			args = args[1:]
			if !ok {
				return
			}
			recipients = append(recipients, recipient)
		}
		if len(args) == 0 {
			log.Errorf("Usage: multisend <jids...> -- <text> (the -- is required)")
			return
		}
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		for _, recipient := range recipients {
			go func(recipient types.JID) {
				resp, err := cli.SendMessage(context.Background(), recipient, "", msg)
				if err != nil {
					log.Errorf("Error sending message to %s: %v", recipient, err)
				} else {
					log.Infof("Message sent to %s (server timestamp: %s)", recipient, resp.Timestamp)
				}
			}(recipient)
		}
	case "react":
		if len(args) < 3 {
			log.Errorf("Usage: react <jid> <message ID> <reaction>")
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		messageID := args[1]
		fromMe := false
		if strings.HasPrefix(messageID, "me:") {
			fromMe = true
			messageID = messageID[len("me:"):]
		}
		reaction := args[2]
		if reaction == "remove" {
			reaction = ""
		}
		msg := &waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJid: proto.String(recipient.String()),
					FromMe:    proto.Bool(fromMe),
					Id:        proto.String(messageID),
				},
				Text:              proto.String(reaction),
				SenderTimestampMs: proto.Int64(time.Now().UnixMilli()),
			},
		}
		resp, err := cli.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			log.Errorf("Error sending reaction: %v", err)
		} else {
			log.Infof("Reaction sent (server timestamp: %s)", resp.Timestamp)
		}
	case "revoke":
		if len(args) < 2 {
			log.Errorf("Usage: revoke <jid> <message ID>")
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		messageID := args[1]
		resp, err := cli.RevokeMessage(recipient, messageID)
		if err != nil {
			log.Errorf("Error sending revocation: %v", err)
		} else {
			log.Infof("Revocation sent (server timestamp: %s)", resp.Timestamp)
		}
	case "sendimg":
		if len(args) < 2 {
			log.Errorf("Usage: sendimg <jid> <image path> [caption]")
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			log.Errorf("Failed to read %s: %v", args[0], err)
			return
		}
		uploaded, err := cli.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			log.Errorf("Failed to upload file: %v", err)
			return
		}
		msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(strings.Join(args[2:], " ")),
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		}}
		resp, err := cli.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			log.Errorf("Error sending image message: %v", err)
		} else {
			log.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
		}
	case "setstatus":
		if len(args) == 0 {
			log.Errorf("Usage: setstatus <message>")
			return
		}
		err := cli.SetStatusMessage(strings.Join(args, " "))
		if err != nil {
			log.Errorf("Error setting status message: %v", err)
		} else {
			log.Infof("Status updated")
		}
	}
}

var historySyncID int32
var startupTime = time.Now().Unix()

func handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := cli.SendPresence(types.PresenceAvailable)
			if err != nil {
				log.Warnf("Failed to send available presence: %v", err)
			} else {
				log.Infof("Marked self as available")
			}
		}
	case *events.Connected, *events.PushNameSetting:
		if len(cli.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := cli.SendPresence(types.PresenceAvailable)
		if err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		} else {
			log.Infof("Marked self as available")
		}
	case *events.StreamReplaced:
		os.Exit(0)
	case *events.Message:
		metaParts := []string{fmt.Sprintf("pushname: %s", evt.Info.PushName), fmt.Sprintf("timestamp: %s", evt.Info.Timestamp)}
		if evt.Info.Type != "" {
			metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
		}
		if evt.Info.Category != "" {
			metaParts = append(metaParts, fmt.Sprintf("category: %s", evt.Info.Category))
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "view once")
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "ephemeral")
		}

		log.Infof("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)

		img := evt.Message.GetImageMessage()
		if img != nil {
			data, err := cli.Download(img)
			if err != nil {
				log.Errorf("Failed to download image: %v", err)
				return
			}
			exts, _ := mime.ExtensionsByType(img.GetMimetype())
			path := fmt.Sprintf("%s%s", evt.Info.ID, exts[0])
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				log.Errorf("Failed to save image: %v", err)
				return
			}
			log.Infof("Saved image in message to %s", path)
		}
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			log.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				log.Infof("%s is now offline", evt.From)
			} else {
				log.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
			}
		} else {
			log.Infof("%s is now online", evt.From)
		}
	case *events.HistorySync:
		id := atomic.AddInt32(&historySyncID, 1)
		fileName := fmt.Sprintf("history-%d-%d.json", startupTime, id)
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Errorf("Failed to open file to write history sync: %v", err)
			return
		}
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		err = enc.Encode(evt.Data)
		if err != nil {
			log.Errorf("Failed to write history sync: %v", err)
			return
		}
		log.Infof("Wrote history sync to %s", fileName)
		_ = file.Close()
	case *events.AppState:
		log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	case *events.KeepAliveTimeout:
		log.Debugf("Keepalive timeout event: %+v", evt)
		if evt.ErrorCount > 3 {
			log.Debugf("Got >3 keepalive timeouts, forcing reconnect")
			go func() {
				cli.Disconnect()
				err := cli.Connect()
				if err != nil {
					log.Errorf("Error force-reconnecting after keepalive timeouts: %v", err)
				}
			}()
		}
	case *events.KeepAliveRestored:
		log.Debugf("Keepalive restored")
	}
}
