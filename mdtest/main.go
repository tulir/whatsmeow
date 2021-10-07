// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	log "maunium.net/go/maulogger/v2"

	"go.mau.fi/whatsmeow/multidevice"
)

func main() {
	log.DefaultLogger.PrintLevel = 0

	cli := multidevice.NewClient(log.DefaultLogger)
	err := cli.Connect()
	if err != nil {
		log.Fatalln("Failed to connect:", err)
		return
	}
	cli.AddEventHandler(handler)

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *multidevice.QREvent:
		go printQRs(evt)
	}
}

func printQRs(evt *multidevice.QREvent) {
	for _, qr := range evt.Codes {
		fmt.Println("\033[38;2;255;255;255m\u001B[48;2;0;0;0m")
		qrterminal.GenerateHalfBlock(qr, qrterminal.L, os.Stdout)
		fmt.Println("\033[0m")
		time.Sleep(evt.Timeout)
	}
}