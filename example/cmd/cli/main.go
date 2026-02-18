package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"sync"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go_client/pkg/shared"
)

var (
	BaseDir      string
	client       *whatsmeow.Client
	dbPath       string
	statePath    string
)

func init() {
	var err error
	BaseDir, err = os.Getwd()
	if err != nil {
		BaseDir = "."
	}
	dbPath = filepath.Join(BaseDir, "examplestore.db")
	statePath = filepath.Join(BaseDir, "cli-data.json")
}

func getClient() (*whatsmeow.Client, error) {
	if client != nil {
		return client, nil
	}

	// Suppress all logs
	dbLog := waLog.Noop
	clientLog := waLog.Noop

	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), dbLog)
	if err != nil {
		return nil, err
	}
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}

	client = whatsmeow.NewClient(deviceStore, clientLog)
	shared.EnableImportCache(client)
	
	// Load previous state
	if err := shared.LoadState(statePath); err != nil {
		fmt.Printf("Warning: Failed to load state: %v\n", err)
	}

	return client, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "whatsapp-cli",
		Short: "A clean WhatsApp CLI client",
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	var loginCmd = &cobra.Command{
		Use:   "login",
		Short: "Login to WhatsApp via QR Code",
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			if cli.Store.ID != nil {
				fmt.Println("Already logged in.")
				return
			}

			// We need to wait for HistorySync
			var wg sync.WaitGroup
			wg.Add(1)
			
			// Custom handler to detect sync finish
			cli.AddEventHandler(func(evt interface{}) {
				switch v := evt.(type) {
				case *events.HistorySync:
					if v.Data.GetProgress() >= 100 {
						fmt.Println("History Sync Complete!")
						wg.Done()
					}
				}
			})

			qrChan, _ := cli.GetQRChannel(context.Background())
			err = cli.Connect()
			if err != nil {
				fmt.Printf("Connection error: %v\n", err)
				return
			}

			fmt.Println("Waiting for QR code...")
			for evt := range qrChan {
				if evt.Event == "code" {
					q, _ := qrcode.New(evt.Code, qrcode.Low)
					fmt.Println(q.ToSmallString(false))
					fmt.Println("Scan the QR code with your WhatsApp mobile app.")
				} else if evt.Event == "success" {
					fmt.Println("Login successful! Waiting for history sync...")
					// Don't return yet, wait for sync
				}
			}

			// Wait for history sync with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				fmt.Println("Initial sync finished.")
			case <-time.After(2 * time.Minute):
				fmt.Println("Timeout waiting for full history sync. Saving what we have.")
			}

			// Save state
			if err := shared.SaveState(statePath); err != nil {
				fmt.Printf("Error saving state: %v\n", err)
			} else {
				fmt.Println("State saved successfully.")
			}
		},
	}

	var logoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Logout from WhatsApp",
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			err = cli.Connect()
			if err != nil {
				fmt.Printf("Connection error: %v\n", err)
				return
			}
			err = cli.Logout(context.Background())
			if err != nil {
				fmt.Printf("Logout error: %v\n", err)
			} else {
				fmt.Println("Logged out successfully.")
				// Clean up session data and cache
				_ = os.Remove(dbPath)
				_ = os.Remove(statePath)
			}
		},
	}

	var chatsCmd = &cobra.Command{
		Use:   "chats",
		Short: "List all chats",
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Connect to get updates
			go cli.Connect()
			
			// Give it a moment to receive new messages if any
			time.Sleep(2 * time.Second)
			
			// Save updated state
			_ = shared.SaveState(statePath)

			chats := shared.GetChats()
			if len(chats) == 0 {
				fmt.Println("No chats found yet. Try 'login' again if this persists.")
				return
			}

			fmt.Printf("%-30s | %-25s | %s\n", "JID", "Name", "Messages")
			fmt.Println(strings.Repeat("-", 70))
			for jid, status := range chats {
				name := status.Name
				if name == "" {
					name = "Unknown"
				}
				fmt.Printf("%-30s | %-25s | %d\n", jid, name, status.MessageCount)
			}
		},
	}

	var contactsCmd = &cobra.Command{
		Use:   "contacts",
		Short: "List all contacts",
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			err = cli.Connect()
			if err != nil {
				fmt.Printf("Connection error: %v\n", err)
				return
			}

			contacts, err := cli.Store.Contacts.GetAllContacts(context.Background())
			if err != nil {
				fmt.Printf("Error getting contacts: %v\n", err)
				return
			}

			fmt.Printf("%-30s | %-25s\n", "JID", "Name")
			fmt.Println(strings.Repeat("-", 60))
			for jid, info := range contacts {
				name := info.FullName
				if name == "" {
					name = info.PushName
				}
				fmt.Printf("%-30s | %-25s\n", jid, name)
			}
		},
	}

	var messagesCmd = &cobra.Command{
		Use:   "messages [jid]",
		Short: "List messages for a chat",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			jid := args[0]
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			go cli.Connect()
			time.Sleep(1 * time.Second)
			_ = shared.SaveState(statePath)

			msgs := shared.GetChatMessages(jid)
			if len(msgs) == 0 {
				fmt.Println("No messages found in cache.")
				return
			}

			for _, m := range msgs {
				sender := "Them"
				if m.FromMe {
					sender = "You"
				}
				text := m.Text
				if text == "" {
					text = fmt.Sprintf("(%s)", m.Type)
				}
				ts := time.Unix(m.Timestamp, 0).Format("15:04:05")
				fmt.Printf("[%s] %s: %s\n", ts, sender, text)
			}
		},
	}

	var mediaCmd = &cobra.Command{
		Use:   "media",
		Short: "List all discovered media",
		Run: func(cmd *cobra.Command, args []string) {
			cli, err := getClient()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			go cli.Connect()
			time.Sleep(1 * time.Second)
			_ = shared.SaveState(statePath)

			media := shared.GetAllMedia()
			if len(media) == 0 {
				fmt.Println("No media found in cache.")
				return
			}

			fmt.Printf("%-20s | %-10s | %s\n", "ID", "Type", "Caption/Filename")
			fmt.Println(strings.Repeat("-", 60))
			for _, m := range media {
				desc := m.Caption
				if desc == "" {
					desc = m.FileName
				}
				fmt.Printf("%-20s | %-10s | %s\n", m.ID, m.Type, desc)
			}
		},
	}

	rootCmd.AddCommand(loginCmd, logoutCmd, chatsCmd, contactsCmd, messagesCmd, mediaCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
