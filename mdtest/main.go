// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"google.golang.org/protobuf/proto"
	log "maunium.net/go/maulogger/v2"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/multidevice"
	"go.mau.fi/whatsmeow/multidevice/session"
)

const SessionFileName = "mdtest.gob"

func loadSession() *session.Session {
	var sess session.Session
	if file, err := os.OpenFile(SessionFileName, os.O_RDONLY, 0600); errors.Is(err, fs.ErrNotExist) {
		return session.NewSession()
	} else if err != nil {
		log.Fatalln("Failed to open session file for reading:", err)
	} else if err = gob.NewDecoder(file).Decode(&sess); err != nil {
		log.Fatalln("Failed to decode session:", err)
	} else {
		if err = file.Close(); err != nil {
			log.Warnln("Failed to close session file after reading:", err)
		}
		return &sess
	}
	os.Exit(2)
	return nil
}

func saveSession(sess *session.Session) {
	if file, err := os.OpenFile(SessionFileName, os.O_CREATE|os.O_WRONLY, 0600); err != nil {
		log.Fatalln("Failed to open session file for writing:", err)
	} else if err = gob.NewEncoder(file).Encode(sess); err != nil {
		log.Fatalln("Failed to encode session:", err)
	} else if err = file.Close(); err != nil {
		log.Warnln("Failed to close session file after writing:", err)
	}
}

var cli *multidevice.Client

func main() {
	log.DefaultLogger.PrintLevel = 0

	sess := loadSession()
	cli = multidevice.NewClient(sess, log.DefaultLogger)
	err := cli.Connect()
	if err != nil {
		log.Fatalln("Failed to connect:", err)
		return
	}
	cli.AddEventHandler(handler)
	defer saveSession(sess)

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
			handleCmd(strings.ToLower(cmd), args)
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
	case "usync":
		var jids []waBinary.FullJID
		for _, jid := range args {
			jids = append(jids, waBinary.NewJID(jid, waBinary.DefaultUserServer))
		}
		res, err := cli.GetUSyncDevices(jids, false)
		fmt.Println(err)
		fmt.Println(res)
	case "getgroup":
		resp, err := cli.GetGroupInfo(waBinary.NewJID(args[0], waBinary.GroupServer))
		fmt.Println(err)
		fmt.Printf("%+v\n", resp)
	case "send", "gsend":
		msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
		recipient := waBinary.NewJID(args[0], waBinary.DefaultUserServer)
		if cmd == "gsend" {
			recipient.Server = waBinary.GroupServer
		}
		err := cli.SendMessage(recipient, "", msg)
		fmt.Println("Send message response:", err)
	}
}

var stopQRs = make(chan struct{})

func handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *multidevice.QREvent:
		go printQRs(evt)
	case *multidevice.PairSuccessEvent:
		select {
		case stopQRs <- struct{}{}:
		default:
		}
	}
}

func printQRs(evt *multidevice.QREvent) {
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
