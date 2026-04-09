package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
)

func getStorePath() string {
	p := os.Getenv("WACLI_STORE_PATH")
	if p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return "file:" + home + "/.picoclaw/workspace/whatsapp/store.db?_foreign_keys=on"
}

func newClient() (*whatsmeow.Client, error) {
	logger := waLog.Noop
	container, err := sqlstore.New(context.Background(), "sqlite3", getStorePath(), logger)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("device: %w", err)
	}
	client := whatsmeow.NewClient(device, logger)
	return client, nil
}

func cmdAuth() {
	client, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if client.Store.ID == nil {
		fmt.Println("Starting authentication...")
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "QR channel error: %v\n", err)
			os.Exit(1)
		}
		err = client.Connect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connect error: %v\n", err)
			os.Exit(1)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code with WhatsApp (Linked Devices):")
				qrterminal.GenerateWithConfig(evt.Code, qrterminal.Config{
					Level:      qrterminal.L,
					Writer:     os.Stdout,
					HalfBlocks: true,
				})
			} else {
				fmt.Printf("QR event: %s\n", evt.Event)
				if evt.Event == "success" {
					fmt.Println("Authenticated successfully!")
				}
			}
		}
		// Wait for signal
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		client.Disconnect()
	} else {
		err = client.Connect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connect error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Already authenticated and connected")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		client.Disconnect()
	}
}

func cmdAuthStatus() {
	client, err := newClient()
	if err != nil {
		fmt.Println("Not authenticated")
		os.Exit(1)
	}
	if client.Store.ID == nil {
		fmt.Println("Not authenticated")
		os.Exit(0)
	}
	err = client.Connect()
	if err != nil {
		fmt.Println("Not authenticated")
		os.Exit(1)
	}
	fmt.Println("Logged in and connected")
	client.Disconnect()
}

func cmdAuthLogout() {
	client, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if client.Store.ID == nil {
		fmt.Println("Not logged in")
		return
	}
	err = client.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connect error: %v\n", err)
		os.Exit(1)
	}
	err = client.Logout(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Logout error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Logged out successfully")
	client.Disconnect()
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: wacli auth [status|logout]")
		os.Exit(1)
	}

	switch args[0] {
	case "auth":
		if len(args) > 1 {
			switch args[1] {
			case "status":
				cmdAuthStatus()
			case "logout":
				cmdAuthLogout()
			default:
				fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[1])
				os.Exit(1)
			}
		} else {
			cmdAuth()
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}
