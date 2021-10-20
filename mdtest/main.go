// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bufio"
	"context"
	"database/sql"
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
	log "maunium.net/go/maulogger/v2"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/events"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var cli *whatsmeow.Client

type waLogger struct {
	l log.Logger
}

func (w *waLogger) Debugf(msg string, args ...interface{}) {
	w.l.Debugfln(msg, args...)
}

func (w *waLogger) Infof(msg string, args ...interface{}) {
	w.l.Infofln(msg, args...)
}

func (w *waLogger) Warnf(msg string, args ...interface{}) {
	w.l.Warnfln(msg, args...)
}

func (w *waLogger) Errorf(msg string, args ...interface{}) {
	w.l.Errorfln(msg, args...)
}

func (w *waLogger) Sub(module string) waLog.Logger {
	return &waLogger{l: w.l.Sub(module)}
}

func getDevice() *store.Device {
	db, err := sql.Open("sqlite3", "file:mdtest.db?_foreign_keys=on")
	if err != nil {
		log.Fatalln("Failed to open mdtest.db:", err)
		return nil
	}
	storeContainer := store.NewSQLContainerWithDB(db, "sqlite3", &waLogger{log.DefaultLogger.Sub("Database")})
	err = storeContainer.Upgrade()
	if err != nil {
		log.Fatalln("Failed to upgrade database:", err)
		return nil
	}
	devices, err := storeContainer.GetAllDevices()
	if err != nil {
		log.Fatalln("Failed to get devices from database:", err)
		return nil
	}
	if len(devices) == 0 {
		return storeContainer.NewDevice()
	} else {
		return devices[0]
	}
}

func main() {
	log.DefaultLogger.PrintLevel = 0
	waBinary.IndentXML = true

	device := getDevice()
	if device == nil {
		return
	}

	cli = whatsmeow.NewClient(device, &waLogger{log.DefaultLogger.Sub("Client")})
	err := cli.Connect()
	if err != nil {
		log.Fatalln("Failed to connect:", err)
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
			cli.Disconnect()
			return
		case cmd := <-input:
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
			log.Fatalln("Failed to connect:", err)
			return
		}
	case "appstate":
		err := cli.FetchAppState(appstate.WAPatchName(args[0]), len(args) > 1 && args[1] == "resync")
		if err != nil {
			log.Errorln("Failed to sync app state:", err)
		}
	case "checkuser":
		resp, err := cli.IsOnWhatsApp(args)
		fmt.Println(err)
		fmt.Printf("%+v\n", resp)
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

var stopQRs = make(chan struct{})

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
		log.Infofln("Received message: %+v", evt)
		img := evt.Message.GetImageMessage()
		if img != nil {
			data, err := cli.Download(img)
			if err != nil {
				fmt.Println("Failed to download image:", err)
				//return
			}
			exts, _ := mime.ExtensionsByType(img.GetMimetype())
			path := fmt.Sprintf("%s%s", evt.Info.ID, exts[0])
			err = os.WriteFile(path, data, 0600)
			if err != nil {
				fmt.Println("Failed to save image:", err)
				return
			}
			fmt.Println("Saved image to", path)
		}
	case *events.Receipt:
		log.Infofln("Received receipt: %+v", evt)
	case *events.AppState:
		log.Debugfln("App state event: %+v", evt)
	}
}

func printQRs(evt *events.QR) {
	for _, qr := range evt.Codes {
		fmt.Println("\033[38;2;255;255;255m\u001B[48;2;0;0;0m")
		qrterminal.GenerateHalfBlock(qr, qrterminal.L, os.Stdout)
		fmt.Println("\033[0m")
		select {
		case <-time.After(evt.Timeout):
		case <-stopQRs:
			return
		}
	}
}
