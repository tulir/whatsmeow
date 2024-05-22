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
var pairRejectChan = make(chan bool, 1)

func main() {
	waBinary.IndentXML = true
	flag.Parse()

	if *debugLogs {
		logLevel = "DEBUG"
	}
	if *requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
		store.DeviceProps.HistorySyncConfig = &waProto.DeviceProps_HistorySyncConfig{
			FullSyncDaysLimit:   proto.Uint32(3650),
			FullSyncSizeMbLimit: proto.Uint32(102400),
			StorageQuotaMb:      proto.Uint32(102400),
		}
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
	var isWaitingForPair atomic.Bool
	cli.PrePairCallback = func(jid types.JID, platform, businessName string) bool {
		isWaitingForPair.Store(true)
		defer isWaitingForPair.Store(false)
		log.Infof("Pairing %s (platform: %q, business name: %q). Type r within 3 seconds to reject pair", jid, platform, businessName)
		select {
		case reject := <-pairRejectChan:
			if reject {
				log.Infof("Rejecting pair")
				return false
			}
		case <-time.After(3 * time.Second):
		}
		log.Infof("Accepting pair")
		return true
	}

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

	c := make(chan os.Signal, 1)
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
			if isWaitingForPair.Load() {
				if cmd == "r" {
					pairRejectChan <- true
				} else if cmd == "a" {
					pairRejectChan <- false
				}
				continue
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
	case "pair-phone":
		if len(args) < 1 {
			log.Errorf("Usage: pair-phone <number>")
			return
		}
		linkingCode, err := cli.PairPhone(args[0], true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			panic(err)
		}
		fmt.Println("Linking code:", linkingCode)
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
	case "unavailable-request":
		if len(args) < 3 {
			log.Errorf("Usage: unavailable-request <chat JID> <sender JID> <message ID>")
			return
		}
		chat, ok := parseJID(args[0])
		if !ok {
			return
		}
		sender, ok := parseJID(args[1])
		if !ok {
			return
		}
		resp, err := cli.SendMessage(
			context.Background(),
			cli.Store.ID.ToNonAD(),
			cli.BuildUnavailableMessageRequest(chat, sender, args[2]),
			whatsmeow.SendRequestExtra{Peer: true},
		)
		fmt.Println(resp)
		fmt.Println(err)
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
		if len(args) == 0 {
			log.Errorf("Usage: presence <available/unavailable>")
			return
		}
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
	case "setprivacysetting":
		if len(args) < 2 {
			log.Errorf("Usage: setprivacysetting <setting> <value>")
			return
		}
		setting := types.PrivacySettingType(args[0])
		value := types.PrivacySetting(args[1])
		resp, err := cli.SetPrivacySetting(setting, value)
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
	case "mediaconn":
		conn, err := cli.DangerousInternals().RefreshMediaConn(false)
		if err != nil {
			log.Errorf("Failed to get media connection: %v", err)
		} else {
			log.Infof("Media connection: %+v", conn)
		}
	case "raw":
		var node waBinary.Node
		if err := json.Unmarshal([]byte(strings.Join(args, " ")), &node); err != nil {
			log.Errorf("Failed to parse args as JSON into XML node: %v", err)
		} else if err = cli.DangerousInternals().SendNode(node); err != nil {
			log.Errorf("Error sending node: %v", err)
		} else {
			log.Infof("Node sent")
		}
	case "listnewsletters":
		newsletters, err := cli.GetSubscribedNewsletters()
		if err != nil {
			log.Errorf("Failed to get subscribed newsletters: %v", err)
			return
		}
		for _, newsletter := range newsletters {
			log.Infof("* %s: %s", newsletter.ID, newsletter.ThreadMeta.Name.Text)
		}
	case "getnewsletter":
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		meta, err := cli.GetNewsletterInfo(jid)
		if err != nil {
			log.Errorf("Failed to get info: %v", err)
		} else {
			log.Infof("Got info: %+v", meta)
		}
	case "getnewsletterinvite":
		meta, err := cli.GetNewsletterInfoWithInvite(args[0])
		if err != nil {
			log.Errorf("Failed to get info: %v", err)
		} else {
			log.Infof("Got info: %+v", meta)
		}
	case "livesubscribenewsletter":
		if len(args) < 1 {
			log.Errorf("Usage: livesubscribenewsletter <jid>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		dur, err := cli.NewsletterSubscribeLiveUpdates(context.TODO(), jid)
		if err != nil {
			log.Errorf("Failed to subscribe to live updates: %v", err)
		} else {
			log.Infof("Subscribed to live updates for %s for %s", jid, dur)
		}
	case "getnewslettermessages":
		if len(args) < 1 {
			log.Errorf("Usage: getnewslettermessages <jid> [count] [before id]")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		count := 100
		var err error
		if len(args) > 1 {
			count, err = strconv.Atoi(args[1])
			if err != nil {
				log.Errorf("Invalid count: %v", err)
				return
			}
		}
		var before types.MessageServerID
		if len(args) > 2 {
			before, err = strconv.Atoi(args[2])
			if err != nil {
				log.Errorf("Invalid message ID: %v", err)
				return
			}
		}
		messages, err := cli.GetNewsletterMessages(jid, &whatsmeow.GetNewsletterMessagesParams{Count: count, Before: before})
		if err != nil {
			log.Errorf("Failed to get messages: %v", err)
		} else {
			for _, msg := range messages {
				log.Infof("%d: %+v (viewed %d times)", msg.MessageServerID, msg.Message, msg.ViewsCount)
			}
		}
	case "createnewsletter":
		if len(args) < 1 {
			log.Errorf("Usage: createnewsletter <name>")
			return
		}
		resp, err := cli.CreateNewsletter(whatsmeow.CreateNewsletterParams{
			Name: strings.Join(args, " "),
		})
		if err != nil {
			log.Errorf("Failed to create newsletter: %v", err)
		} else {
			log.Infof("Created newsletter %+v", resp)
		}
	case "getavatar":
		if len(args) < 1 {
			log.Errorf("Usage: getavatar <jid> [existing ID] [--preview] [--community]")
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
		var preview, isCommunity bool
		for _, arg := range args {
			if arg == "--preview" {
				preview = true
			} else if arg == "--community" {
				isCommunity = true
			}
		}
		pic, err := cli.GetProfilePictureInfo(jid, &whatsmeow.GetProfilePictureParams{
			Preview:     preview,
			IsCommunity: isCommunity,
			ExistingID:  existingID,
		})
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
	case "subgroups":
		if len(args) < 1 {
			log.Errorf("Usage: subgroups <jid>")
			return
		}
		group, ok := parseJID(args[0])
		if !ok {
			return
		} else if group.Server != types.GroupServer {
			log.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return
		}
		resp, err := cli.GetSubGroups(group)
		if err != nil {
			log.Errorf("Failed to get subgroups: %v", err)
		} else {
			for _, sub := range resp {
				log.Infof("Subgroup: %+v", sub)
			}
		}
	case "communityparticipants":
		if len(args) < 1 {
			log.Errorf("Usage: communityparticipants <jid>")
			return
		}
		group, ok := parseJID(args[0])
		if !ok {
			return
		} else if group.Server != types.GroupServer {
			log.Errorf("Input must be a group JID (@%s)", types.GroupServer)
			return
		}
		resp, err := cli.GetLinkedGroupsParticipants(group)
		if err != nil {
			log.Errorf("Failed to get community participants: %v", err)
		} else {
			log.Infof("Community participants: %+v", resp)
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
	case "updateparticipant":
		if len(args) < 3 {
			log.Errorf("Usage: updateparticipant <jid> <action> <numbers...>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		action := whatsmeow.ParticipantChange(args[1])
		switch action {
		case whatsmeow.ParticipantChangeAdd, whatsmeow.ParticipantChangeRemove, whatsmeow.ParticipantChangePromote, whatsmeow.ParticipantChangeDemote:
		default:
			log.Errorf("Valid actions: add, remove, promote, demote")
			return
		}
		users := make([]types.JID, len(args)-2)
		for i, arg := range args[2:] {
			users[i], ok = parseJID(arg)
			if !ok {
				return
			}
		}
		resp, err := cli.UpdateGroupParticipants(jid, users, action)
		if err != nil {
			log.Errorf("Failed to add participant: %v", err)
			return
		}
		for _, item := range resp {
			if action == whatsmeow.ParticipantChangeAdd && item.Error == 403 && item.AddRequest != nil {
				log.Infof("Participant is private: %d %s %s %v", item.Error, item.JID, item.AddRequest.Code, item.AddRequest.Expiration)
				cli.SendMessage(context.TODO(), item.JID, &waProto.Message{
					GroupInviteMessage: &waProto.GroupInviteMessage{
						InviteCode:       proto.String(item.AddRequest.Code),
						InviteExpiration: proto.Int64(item.AddRequest.Expiration.Unix()),
						GroupJID:         proto.String(jid.String()),
						GroupName:        proto.String("Test group"),
						Caption:          proto.String("This is a test group"),
					},
				})
			} else if item.Error == 409 {
				log.Infof("Participant already in group: %d %s %+v", item.Error, item.JID)
			} else if item.Error == 0 {
				log.Infof("Added participant: %d %s %+v", item.Error, item.JID)
			} else {
				log.Infof("Unknown status: %d %s %+v", item.Error, item.JID)
			}
		}
	case "getrequestparticipant":
		if len(args) < 1 {
			log.Errorf("Usage: getrequestparticipant <jid>")
			return
		}
		group, ok := parseJID(args[0])
		if !ok {
			log.Errorf("Invalid JID")
			return
		}
		resp, err := cli.GetGroupRequestParticipants(group)
		if err != nil {
			log.Errorf("Failed to get request participants: %v", err)
		} else {
			log.Infof("Request participants: %+v", resp)
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
	case "setdefaultdisappeartimer":
		if len(args) < 1 {
			log.Errorf("Usage: setdefaultdisappeartimer <days>")
			return
		}
		days, err := strconv.Atoi(args[0])
		if err != nil {
			log.Errorf("Invalid duration: %v", err)
			return
		}
		err = cli.SetDefaultDisappearingTimer(time.Duration(days) * 24 * time.Hour)
		if err != nil {
			log.Errorf("Failed to set default disappearing timer: %v", err)
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
		resp, err := cli.SendMessage(context.Background(), recipient, msg)
		if err != nil {
			log.Errorf("Error sending message: %v", err)
		} else {
			log.Infof("Message sent (server timestamp: %s)", resp.Timestamp)
		}
	case "sendpoll":
		if len(args) < 7 {
			log.Errorf("Usage: sendpoll <jid> <max answers> <question> -- <option 1> / <option 2> / ...")
			return
		}
		recipient, ok := parseJID(args[0])
		if !ok {
			return
		}
		maxAnswers, err := strconv.Atoi(args[1])
		if err != nil {
			log.Errorf("Number of max answers must be an integer")
			return
		}
		remainingArgs := strings.Join(args[2:], " ")
		question, optionsStr, _ := strings.Cut(remainingArgs, "--")
		question = strings.TrimSpace(question)
		options := strings.Split(optionsStr, "/")
		for i, opt := range options {
			options[i] = strings.TrimSpace(opt)
		}
		resp, err := cli.SendMessage(context.Background(), recipient, cli.BuildPollCreation(question, options, maxAnswers))
		if err != nil {
			log.Errorf("Error sending message: %v", err)
		} else {
			log.Infof("Message sent (server timestamp: %s)", resp.Timestamp)
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
					RemoteJID: proto.String(recipient.String()),
					FromMe:    proto.Bool(fromMe),
					ID:        proto.String(messageID),
				},
				Text:              proto.String(reaction),
				SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
			},
		}
		resp, err := cli.SendMessage(context.Background(), recipient, msg)
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
		resp, err := cli.SendMessage(context.Background(), recipient, cli.BuildRevoke(recipient, types.EmptyJID, messageID))
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
		var uploaded whatsmeow.UploadResponse
		if recipient.Server == types.NewsletterServer {
			uploaded, err = cli.UploadNewsletter(context.Background(), data, whatsmeow.MediaImage)
		} else {
			uploaded, err = cli.Upload(context.Background(), data, whatsmeow.MediaImage)
		}
		if err != nil {
			log.Errorf("Failed to upload file: %v", err)
			return
		}
		msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(strings.Join(args[2:], " ")),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(http.DetectContentType(data)),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		}}
		resp, err := cli.SendMessage(context.Background(), recipient, msg, whatsmeow.SendRequestExtra{
			MediaHandle: uploaded.Handle,
		})
		if err != nil {
			log.Errorf("Error sending image message: %v", err)
		} else {
			log.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
		}
	case "setpushname":
		if len(args) == 0 {
			log.Errorf("Usage: setpushname <name>")
			return
		}
		err := cli.SendAppState(appstate.BuildSettingPushName(strings.Join(args, " ")))
		if err != nil {
			log.Errorf("Error setting push name: %v", err)
		} else {
			log.Infof("Push name updated")
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
	case "archive":
		if len(args) < 2 {
			log.Errorf("Usage: archive <jid> <action>")
			return
		}
		target, ok := parseJID(args[0])
		if !ok {
			return
		}
		action, err := strconv.ParseBool(args[1])
		if err != nil {
			log.Errorf("invalid second argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildArchive(target, action, time.Time{}, nil))
		if err != nil {
			log.Errorf("Error changing chat's archive state: %v", err)
		}
	case "mute":
		if len(args) < 2 {
			log.Errorf("Usage: mute <jid> <action>")
			return
		}
		target, ok := parseJID(args[0])
		if !ok {
			return
		}
		action, err := strconv.ParseBool(args[1])
		if err != nil {
			log.Errorf("invalid second argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildMute(target, action, 1*time.Hour))
		if err != nil {
			log.Errorf("Error changing chat's mute state: %v", err)
		}
	case "pin":
		if len(args) < 2 {
			log.Errorf("Usage: pin <jid> <action>")
			return
		}
		target, ok := parseJID(args[0])
		if !ok {
			return
		}
		action, err := strconv.ParseBool(args[1])
		if err != nil {
			log.Errorf("invalid second argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildPin(target, action))
		if err != nil {
			log.Errorf("Error changing chat's pin state: %v", err)
		}
	case "getblocklist":
		blocklist, err := cli.GetBlocklist()
		if err != nil {
			log.Errorf("Failed to get blocked contacts list: %v", err)
		} else {
			log.Infof("Blocklist: %+v", blocklist)
		}
	case "block":
		if len(args) < 1 {
			log.Errorf("Usage: block <jid>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		resp, err := cli.UpdateBlocklist(jid, events.BlocklistChangeActionBlock)
		if err != nil {
			log.Errorf("Error updating blocklist: %v", err)
		} else {
			log.Infof("Blocklist updated: %+v", resp)
		}
	case "unblock":
		if len(args) < 1 {
			log.Errorf("Usage: unblock <jid>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		resp, err := cli.UpdateBlocklist(jid, events.BlocklistChangeActionUnblock)
		if err != nil {
			log.Errorf("Error updating blocklist: %v", err)
		} else {
			log.Infof("Blocklist updated: %+v", resp)
		}
	case "labelchat":
		if len(args) < 3 {
			log.Errorf("Usage: labelchat <jid> <labelID> <action>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		labelID := args[1]
		action, err := strconv.ParseBool(args[2])
		if err != nil {
			log.Errorf("invalid third argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildLabelChat(jid, labelID, action))
		if err != nil {
			log.Errorf("Error changing chat's label state: %v", err)
		}
	case "labelmessage":
		if len(args) < 4 {
			log.Errorf("Usage: labelmessage <jid> <labelID> <messageID> <action>")
			return
		}
		jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		labelID := args[1]
		messageID := args[2]
		action, err := strconv.ParseBool(args[3])
		if err != nil {
			log.Errorf("invalid fourth argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildLabelMessage(jid, labelID, messageID, action))
		if err != nil {
			log.Errorf("Error changing message's label state: %v", err)
		}
	case "editlabel":
		if len(args) < 4 {
			log.Errorf("Usage: editlabel <labelID> <name> <color> <action>")
			return
		}
		labelID := args[0]
		name := args[1]
		color, err := strconv.Atoi(args[2])
		if err != nil {
			log.Errorf("invalid third argument: %v", err)
			return
		}
		action, err := strconv.ParseBool(args[3])
		if err != nil {
			log.Errorf("invalid fourth argument: %v", err)
			return
		}

		err = cli.SendAppState(appstate.BuildLabelEdit(labelID, name, int32(color), action))
		if err != nil {
			log.Errorf("Error editing label: %v", err)
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
		if evt.IsViewOnceV2 {
			metaParts = append(metaParts, "ephemeral (v2)")
		}
		if evt.IsDocumentWithCaption {
			metaParts = append(metaParts, "document with caption")
		}
		if evt.IsEdit {
			metaParts = append(metaParts, "edit")
		}

		log.Infof("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)

		if evt.Message.GetPollUpdateMessage() != nil {
			decrypted, err := cli.DecryptPollVote(evt)
			if err != nil {
				log.Errorf("Failed to decrypt vote: %v", err)
			} else {
				log.Infof("Selected options in decrypted vote:")
				for _, option := range decrypted.SelectedOptions {
					log.Infof("- %X", option)
				}
			}
		} else if evt.Message.GetEncReactionMessage() != nil {
			decrypted, err := cli.DecryptReaction(evt)
			if err != nil {
				log.Errorf("Failed to decrypt encrypted reaction: %v", err)
			} else {
				log.Infof("Decrypted reaction: %+v", decrypted)
			}
		}

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
		if evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf {
			log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == types.ReceiptTypeDelivered {
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
	case *events.KeepAliveRestored:
		log.Debugf("Keepalive restored")
	case *events.Blocklist:
		log.Infof("Blocklist event: %+v", evt)
	}
}
