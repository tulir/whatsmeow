// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow_test

import (
	"context"
	"fmt"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// For single and multiple sessions

var container *sqlstore.Container
var dbLog waLog.Logger
var clientLog waLog.Logger

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}

func start() {
	dbLog = waLog.Stdout("Database", "DEBUG", true)
	var err error
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err = sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	clientLog = waLog.Stdout("Client", "DEBUG", true)
}

func connectClient(client *whatsmeow.Client) *types.JID {

	if client.Store.ID == nil {

		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err := client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				// e.g. qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else if evt.Event == "success" {
				return client.Store.ID
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}

	} else {

		// Already logged in, just connect
		err := client.Connect()
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func getClient(deviceStore *store.Device) *whatsmeow.Client {

	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	return client
}

// For multiple sessions

var jid1 *types.JID // This pretends to be your database
var jid2 *types.JID // This pretends to be your database

func selectJIDFromFakeDatabase(textJID string) *types.JID {

	jid, err := types.ParseJID(textJID) // Fake database search
	if err != nil {
		panic(err)
	}

	return &jid
}

// Exemples

func ExampleSigleSession() {

	start()

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	client := getClient(deviceStore)
	connectClient(client)

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

func ExampleMultipleSessions() {

	start()

	// First run, adding new users / WhatsApp
	deviceStore1 := container.NewDevice()
	deviceStore2 := container.NewDevice()

	client1 := getClient(deviceStore1)
	client2 := getClient(deviceStore2)

	jid1 = connectClient(client1) // Scanning the QR Code and saving the JID in the fake database
	jid2 = connectClient(client2) // Scanning the QR Code and saving the JID in the fake database

	// Here your system may suffer a crash, be restarted or other reasons that may require you to recover old sessions
	fmt.Println("\n\n\n\n\n\n\n\n\n Waiting for initial sync \n\n\n\n\n\n\n\n\n ")
	time.Sleep(30 * time.Second)

	client1.Disconnect() // Ending current session to simulate reconnection
	client2.Disconnect() // Ending current session to simulate reconnection

	fmt.Println("\n\n\n\n\n\n\n\n\n Disconnected customers \n\n\n\n\n\n\n\n\n ")
	time.Sleep(10 * time.Second)

	fmt.Println("\n\n\n\n\n\n\n\n\n Restoring sessions \n\n\n\n\n\n\n\n\n ")
	// Here your system is connecting again and so that you don't have to scan the QR Code again, retrieve the JIDs from your database
	jid1 = selectJIDFromFakeDatabase(jid1.String()) // Fake JID recovery
	jid2 = selectJIDFromFakeDatabase(jid2.String()) // Fake JID recovery

	// Recreating devices and clients for new connection with old session
	var err error
	deviceStore1, err = container.GetDevice(*jid1)
	if err != nil {
		panic(err)
	}

	deviceStore2, err = container.GetDevice(*jid2)
	if err != nil {
		panic(err)
	}

	client1 = getClient(deviceStore1)
	client2 = getClient(deviceStore2)

	// Connecting clients with previous sessions

	connectClient(client1) // Now you will only be connected to WhatsApp and will not need to scan the QR Code
	connectClient(client2) // Now you will only be connected to WhatsApp and will not need to scan the QR Code

	fmt.Println("\n\n\n\n\n\n\n\n\n Clients connected to old sessions \n\n\n\n\n\n\n\n\n ")

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client1.Disconnect()
	client2.Disconnect()
}
