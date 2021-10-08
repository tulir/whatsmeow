// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	log "maunium.net/go/maulogger/v2"

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

func main() {
	log.DefaultLogger.PrintLevel = 0

	sess := loadSession()
	cli := multidevice.NewClient(sess, log.DefaultLogger)
	err := cli.Connect()
	if err != nil {
		log.Fatalln("Failed to connect:", err)
		return
	}
	cli.AddEventHandler(handler)
	defer saveSession(sess)

	c := make(chan os.Signal)
	q := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(q, syscall.SIGQUIT)
	for {
		select {
		case <-c:
			cli.Disconnect()
			return
		case <-q:
			cli.Disconnect()
			err = cli.Connect()
			if err != nil {
				log.Fatalln("Failed to connect:", err)
				return
			}
		}
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
