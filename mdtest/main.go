// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
var log = waLog.Stdout("Main", "DEBUG", true)

var logLevel = "INFO"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")

// getDevice connects to the database and returns the first device stored there.
// If there are no devices, a new device store is returned.
func getDevice() *store.Device {
	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(*dbDialect, *dbAddress, dbLog)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		return nil
	}
	devices, err := storeContainer.GetAllDevices()
	if err != nil {
		log.Errorf("Failed to get devices from database: %v", err)
		return nil
	}
	if len(devices) == 0 {
		return storeContainer.NewDevice()
	} else {
		return devices[0]
	}
}

func main() {
	waBinary.IndentXML = true
	flag.Parse()

	if *debugLogs {
		logLevel = "DEBUG"
	}

	device := getDevice()
	if device == nil {
		return
	}

	cli = whatsmeow.NewClient(device, waLog.Stdout("Client", logLevel, true))
	err := cli.Connect()
	if err != nil {
		log.Errorf("Failed to connect: %v", err)
		return
	}
	cli.AddEventHandler(handler)

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

func handleCmd(cmd string, args []string) {
	switch cmd {
	case "reconnect":
		cli.Disconnect()
		err := cli.Connect()
		if err != nil {
			log.Errorf("Failed to connect: %v", err)
			return
		}
	case "appstate":
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
	case "checkuser":
		resp, err := cli.IsOnWhatsApp(args)
		fmt.Println(err)
		fmt.Printf("%+v\n", resp)
	case "presence":
		fmt.Println(cli.SendPresence(types.Presence(args[0])))
	case "chatpresence":
		jid, _ := types.ParseJID(args[1])
		fmt.Println(cli.SendChatPresence(types.ChatPresence(args[0]), jid))
	case "getuser":
		var jids []types.JID
		for _, jid := range args {
			jids = append(jids, types.NewJID(jid, types.DefaultUserServer))
		}
		resp, err := cli.GetUserInfo(jids)
		fmt.Println(err)
		fmt.Printf("%+v\n", resp)
	case "getavatar":
		jid := types.NewJID(args[0], types.DefaultUserServer)
		if len(args) > 1 && args[1] == "group" {
			jid.Server = types.GroupServer
			args = args[1:]
		}
		pic, err := cli.GetProfilePictureInfo(jid, len(args) > 1 && args[1] == "preview")
		fmt.Println(err)
		fmt.Printf("%+v\n", pic)
	case "getgroup":
		resp, err := cli.GetGroupInfo(types.NewJID(args[0], types.GroupServer))
		fmt.Println(err)
		fmt.Printf("%+v\n", resp)
	case "send", "gsend":
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		recipient := types.NewJID(args[0], types.DefaultUserServer)
		if cmd == "gsend" {
			recipient.Server = types.GroupServer
		}
		err := cli.SendMessage(recipient, "", msg)
		fmt.Println("Send message response:", err)
	case "sendimg", "gsendimg":
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Printf("Failed to read %s: %v\n", args[0], err)
			return
		}
		uploaded, err := cli.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			fmt.Println("Failed to upload file:", err)
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
		recipient := types.NewJID(args[0], types.DefaultUserServer)
		if cmd == "gsendimg" {
			recipient.Server = types.GroupServer
		}
		err = cli.SendMessage(recipient, "", msg)
		fmt.Println("Send image error:", err)
	}
}

func handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.QR:
		go printQRs(evt)
	case *events.PairSuccess:
		select {
		case stopQRs <- struct{}{}:
		default:
		}
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
		if evt.Type == events.ReceiptTypeRead {
			log.Infof("%s was read by %s at %s", evt.MessageSource, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			log.Infof("%s was delivered to %s at %s", evt.MessageID, evt.SourceString(), evt.Timestamp)
		}
	case *events.AppState:
		log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	}
}

var stopQRs = make(chan struct{})

func printQRs(evt *events.QR) {
	for _, qr := range evt.Codes {
		qrterminal.GenerateHalfBlock(qr, qrterminal.L, os.Stdout)
		select {
		case <-time.After(evt.Timeout):
		case <-stopQRs:
			return
		}
	}
}
