// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	phoneNumber = flag.String("phone", "", "Phone number in international format (e.g., +1234567890)")
	daysAgo     = flag.Int("days", 7, "Number of days ago to retrieve messages from")
	maxMessages = flag.Int("max", 100, "Maximum number of messages to retrieve")
)

func main() {
	flag.Parse()

	if *phoneNumber == "" {
		fmt.Println("Usage: go run main.go -phone +1234567890 [-days 7] [-max 100]")
		fmt.Println("\nOptions:")
		fmt.Println("  -phone    Phone number in international format (required)")
		fmt.Println("  -days     Number of days ago to retrieve messages from (default: 7)")
		fmt.Println("  -max      Maximum number of messages to retrieve (default: 100)")
		return
	}

	// Setup logging
	log := waLog.Stdout("Main", "INFO", true)

	// Initialize database container
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New("sqlite3", "file:whatsmeow.db?_foreign_keys=on", dbLog)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		return
	}

	// Get first device (or create new one)
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		log.Errorf("Failed to get device: %v", err)
		return
	}

	// Create WhatsApp client
	client := whatsmeow.NewClient(deviceStore, log)

	// Register event handler to show connection status
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			log.Infof("Connected to WhatsApp")
		case *events.PushNameSetting:
			log.Infof("Push name set to: %s", v.Name)
		case *events.Message:
			// Show incoming messages while waiting
			log.Infof("Received message from %s: %v", v.Info.Sender, v.Message.GetConversation())
		}
	})

	// Connect to WhatsApp
	if client.Store.ID == nil {
		// No previous session, need to pair
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			log.Errorf("Failed to get QR channel: %v", err)
			return
		}

		err = client.Connect()
		if err != nil {
			log.Errorf("Failed to connect: %v", err)
			return
		}

		fmt.Println("Scan this QR code with WhatsApp:")
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println(evt.Code)
			} else if evt.Event == "success" {
				fmt.Println("QR code scanned successfully!")
				break
			} else {
				log.Infof("QR channel event: %s", evt.Event)
			}
		}
	} else {
		// Already paired, just connect
		err = client.Connect()
		if err != nil {
			log.Errorf("Failed to connect: %v", err)
			return
		}
	}

	// Parse the phone number to JID
	jid, err := types.ParseJID(*phoneNumber + "@s.whatsapp.net")
	if err != nil {
		log.Errorf("Failed to parse phone number: %v", err)
		return
	}

	log.Infof("Retrieving messages from %s for the last %d days (max %d messages)...", jid, *daysAgo, *maxMessages)

	// Calculate date range
	now := time.Now()
	startTime := now.AddDate(0, 0, -*daysAgo)

	// Create query
	query := whatsmeow.DateRangeQuery{
		ChatJID:     jid,
		StartTime:   startTime,
		EndTime:     now,
		MaxMessages: *maxMessages,
		Timeout:     60 * time.Second, // Give more time for larger queries
	}

	// Retrieve messages in date range
	messages, err := client.GetMessagesInDateRange(context.Background(), query)
	if err != nil {
		log.Errorf("Failed to retrieve messages: %v", err)
		client.Disconnect()
		return
	}

	// Display results
	fmt.Printf("\n=== Retrieved %d messages from %s ===\n\n", len(messages), jid)

	for i, msg := range messages {
		fmt.Printf("[%d] %s - From: %s (IsFromMe: %v)\n",
			i+1,
			msg.Info.Timestamp.Format("2006-01-02 15:04:05"),
			msg.Info.Sender,
			msg.Info.IsFromMe,
		)

		// Display message content based on type
		if msg.Message.GetConversation() != "" {
			fmt.Printf("    Text: %s\n", msg.Message.GetConversation())
		} else if ext := msg.Message.GetExtendedTextMessage(); ext != nil {
			fmt.Printf("    Text: %s\n", ext.GetText())
		} else if img := msg.Message.GetImageMessage(); img != nil {
			fmt.Printf("    Image: %s (Caption: %s)\n", img.GetMimetype(), img.GetCaption())
		} else if vid := msg.Message.GetVideoMessage(); vid != nil {
			fmt.Printf("    Video: %s (Caption: %s)\n", vid.GetMimetype(), vid.GetCaption())
		} else if doc := msg.Message.GetDocumentMessage(); doc != nil {
			fmt.Printf("    Document: %s (%s)\n", doc.GetFileName(), doc.GetMimetype())
		} else if audio := msg.Message.GetAudioMessage(); audio != nil {
			fmt.Printf("    Audio: %s (Duration: %ds)\n", audio.GetMimetype(), audio.GetSeconds())
		} else if contact := msg.Message.GetContactMessage(); contact != nil {
			fmt.Printf("    Contact: %s\n", contact.GetDisplayName())
		} else if loc := msg.Message.GetLocationMessage(); loc != nil {
			fmt.Printf("    Location: %.6f, %.6f\n", loc.GetDegreesLatitude(), loc.GetDegreesLongitude())
		} else {
			// For other message types, show the protobuf structure
			fmt.Printf("    Type: %s\n", getMessageType(msg.Message))
		}
		fmt.Println()
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total messages retrieved: %d\n", len(messages))
	fmt.Printf("Date range: %s to %s\n",
		startTime.Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"),
	)

	// Wait for interrupt signal to disconnect
	fmt.Println("\nPress Ctrl+C to disconnect...")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
	fmt.Println("Disconnected")
}

// getMessageType returns a human-readable message type
func getMessageType(msg *waE2E.Message) string {
	switch {
	case msg.Conversation != nil:
		return "Text"
	case msg.ExtendedTextMessage != nil:
		return "Extended Text"
	case msg.ImageMessage != nil:
		return "Image"
	case msg.VideoMessage != nil:
		return "Video"
	case msg.AudioMessage != nil:
		return "Audio"
	case msg.DocumentMessage != nil:
		return "Document"
	case msg.ContactMessage != nil:
		return "Contact"
	case msg.LocationMessage != nil:
		return "Location"
	case msg.StickerMessage != nil:
		return "Sticker"
	case msg.ProtocolMessage != nil:
		return fmt.Sprintf("Protocol (%s)", msg.ProtocolMessage.GetType())
	case msg.ReactionMessage != nil:
		return "Reaction"
	default:
		return fmt.Sprintf("Unknown (%s)", proto.MessageName(msg))
	}
}
