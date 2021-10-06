// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"os"
	"os/signal"
	"syscall"

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

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
