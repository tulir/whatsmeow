# whatsmeow

[![Go Reference](https://pkg.go.dev/badge/go.mau.fi/whatsmeow.svg)](https://pkg.go.dev/go.mau.fi/whatsmeow)
[![License](https://img.shields.io/badge/License-MPL%202.0-blue.svg)](https://mozilla.org/MPL/2.0/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tulir/whatsmeow)](https://go.dev/)

whatsmeow is a Go library for the WhatsApp Web multidevice API, providing a robust and comprehensive interface for building WhatsApp-integrated applications.

## 📋 Table of Contents

- [Why whatsmeow?](#-why-whatsmeow)
- [Features](#-features)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Authentication](#-authentication)
- [Sending Messages](#-sending-messages)
- [Receiving Messages](#-receiving-messages)
- [Group Management](#-group-management)
- [Advanced Configuration](#-advanced-configuration)
- [API Reference](#-api-reference)
- [Project Structure](#-project-structure)
- [Development](#-development)
- [Troubleshooting](#-troubleshooting--faq)
- [Discussion](#-discussion)
- [License](#-license)

## 💡 Why whatsmeow?

- **🔄 Multidevice Support**: Full implementation of WhatsApp's multidevice protocol
- **🚀 High Performance**: Built in Go for speed and efficiency
- **🔐 Secure**: End-to-end encryption with proper key management
- **📦 Easy Integration**: Simple API with comprehensive documentation
- **🎯 Production Ready**: Battle-tested in production environments
- **🔧 Actively Maintained**: Regular updates and bug fixes

## ✨ Features

### Core Features ✅

- ✉️ **Messaging**
  - Send and receive text messages
  - Send and receive media (images, videos, audio, documents)
  - Message reactions and replies
  - Location sharing
  - Contact sharing
  - Delivery and read receipts
  - Typing notifications

- 👥 **Group Management**
  - Create and manage groups
  - Add/remove participants
  - Promote/demote admins
  - Change group settings (name, description, icon)
  - Handle group invites and invite links
  - Group events and notifications

- 🔐 **Security & Privacy**
  - End-to-end encryption
  - Device verification
  - Privacy settings management
  - Secure session storage

- 📱 **App State Sync**
  - Contact list synchronization
  - Chat pin/mute status
  - Archive status
  - Message star status

- 🔄 **Advanced Features**
  - History sync
  - Media retry mechanism
  - Automatic message decryption retry
  - Status/Story messages (experimental)
  - Newsletter support

### Not Yet Implemented ⏳

- Broadcast list messages (not supported on WhatsApp Web)
- Voice/Video calls (signals only)

## 🚀 Quick Start

Here's a minimal example to get you started:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    _ "github.com/mattn/go-sqlite3"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
    // Initialize logger
    dbLog := waLog.Stdout("Database", "INFO", true)
    
    // Initialize store
    container, err := sqlstore.New(context.Background(), "sqlite3", "file:whatsmeow.db?_foreign_keys=on", dbLog)
    if err != nil {
        panic(err)
    }
    
    // Get device store
    deviceStore, err := container.GetFirstDevice(ctx)
    if err != nil {
        panic(err)
    }
    
    // Create client
    client := whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
    
    // Add event handler
    client.AddEventHandler(func(evt interface{}) {
        switch v := evt.(type) {
        case *events.Message:
            fmt.Println("Received message:", v.Message.GetConversation())
        }
    })
    
    // Connect and login
    if client.Store.ID == nil {
        // No ID stored, new login required
        qrChan, _ := client.GetQRChannel(context.Background())
        err = client.Connect()
        if err != nil {
            panic(err)
        }
        for evt := range qrChan {
            if evt.Event == "code" {
                fmt.Println("QR code:", evt.Code)
            } else {
                fmt.Println("Login event:", evt.Event)
            }
        }
    } else {
        err = client.Connect()
        if err != nil {
            panic(err)
        }
    }
    
    // Wait for interrupt
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
    
    client.Disconnect()
}
```

## 📖 Installation

### Prerequisites

- **Go**: Version 1.24.0 or higher
- **Database Driver**: SQLite, PostgreSQL, or MySQL driver

### Install the Library

```bash
go get go.mau.fi/whatsmeow
```

### Install Database Driver

For SQLite (recommended for getting started):
```bash
go get github.com/mattn/go-sqlite3
```

For PostgreSQL:
```bash
go get github.com/lib/pq
```

For MySQL:
```bash
go get github.com/go-sql-driver/mysql
```

### Basic Project Setup

```bash
# Create a new project
mkdir my-whatsapp-bot
cd my-whatsapp-bot
go mod init my-whatsapp-bot

# Install dependencies
go get go.mau.fi/whatsmeow
go get github.com/mattn/go-sqlite3
```

## 🔐 Authentication

whatsmeow supports two authentication methods: QR Code scanning and Phone Number pairing.

### QR Code Authentication

```go
package main

import (
    "context"
    "fmt"
    
    _ "github.com/mattn/go-sqlite3"
    "github.com/mdp/qrterminal/v3"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
    ctx := context.Background()
    dbLog := waLog.Stdout("Database", "INFO", true)
    container, err := sqlstore.New(ctx, "sqlite3", "file:store.db?_foreign_keys=on", dbLog)
    if err != nil {
        panic(err)
    }
    
    deviceStore, err := container.GetFirstDevice(ctx)
    if err != nil {
        panic(err)
    }
    
    client := whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
    
    if client.Store.ID == nil {
        qrChan, _ := client.GetQRChannel(context.Background())
        err = client.Connect()
        if err != nil {
            panic(err)
        }
        
        for evt := range qrChan {
            if evt.Event == "code" {
                // Display QR code in terminal
                qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
                fmt.Println("QR code:", evt.Code)
            } else {
                fmt.Println("Login event:", evt.Event)
            }
        }
    } else {
        err = client.Connect()
        if err != nil {
            panic(err)
        }
    }
    
    fmt.Println("Successfully connected!")
}
```

### Phone Number Pairing

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"
    
    _ "github.com/mattn/go-sqlite3"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
    ctx := context.Background()
    container, err := sqlstore.New(ctx, "sqlite3", "file:store.db?_foreign_keys=on", waLog.Noop)
    if err != nil {
        panic(err)
    }
    
    deviceStore, err := container.GetFirstDevice(ctx)
    if err != nil {
        panic(err)
    }
    
    client := whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "INFO", true))
    
    if client.Store.ID == nil {
        // Start connection first
        qrChan, _ := client.GetQRChannel(context.Background())
        err = client.Connect()
        if err != nil {
            panic(err)
        }
        
        // Wait for QR channel to be ready (or sleep briefly)
        <-qrChan // Read and discard the first QR code event
        
        // Request pairing code
        fmt.Print("Enter your phone number (with country code): ")
        scanner := bufio.NewScanner(os.Stdin)
        scanner.Scan()
        phoneNumber := strings.TrimSpace(scanner.Text())
        
        code, err := client.PairPhone(phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
        if err != nil {
            panic(err)
        }
        
        fmt.Println("Pairing code:", code)
        fmt.Println("Enter this code in WhatsApp: Settings > Linked Devices > Link a Device")
        
        // Wait for successful pairing
        for evt := range qrChan {
            if evt.Event == "success" {
                fmt.Println("Successfully paired!")
                break
            }
        }
    }
}
```

## 💬 Sending Messages

### Text Messages

```go
import (
    "context"
    
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/proto/waE2E"
    "go.mau.fi/whatsmeow/types"
    "google.golang.org/protobuf/proto"
)

// Send a simple text message
func sendTextMessage(client *whatsmeow.Client, recipient string, text string) error {
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    
    message := &waE2E.Message{
        Conversation: proto.String(text),
    }
    
    resp, err := client.SendMessage(context.Background(), recipientJID, message)
    if err != nil {
        return err
    }
    
    fmt.Println("Message sent! ID:", resp.ID)
    return nil
}

// Send a message with reply
func sendReplyMessage(client *whatsmeow.Client, recipient string, text string, quotedID string) error {
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    
    message := &waE2E.Message{
        ExtendedTextMessage: &waE2E.ExtendedTextMessage{
            Text: proto.String(text),
            ContextInfo: &waE2E.ContextInfo{
                StanzaID:      proto.String(quotedID),
                Participant:   proto.String(recipient),
                QuotedMessage: &waE2E.Message{Conversation: proto.String("")},
            },
        },
    }
    
    _, err := client.SendMessage(context.Background(), recipientJID, message)
    return err
}
```

### Image Messages

```go
import (
    "context"
    "os"
    
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/proto/waE2E"
    "google.golang.org/protobuf/proto"
)

func sendImageMessage(client *whatsmeow.Client, recipient string, imagePath string, caption string) error {
    // Read image file
    imageData, err := os.ReadFile(imagePath)
    if err != nil {
        return err
    }
    
    // Upload image
    uploaded, err := client.Upload(context.Background(), imageData, whatsmeow.MediaImage)
    if err != nil {
        return err
    }
    
    // Create image message
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    message := &waE2E.Message{
        ImageMessage: &waE2E.ImageMessage{
            Caption:       proto.String(caption),
            URL:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String("image/jpeg"),
            FileEncSHA256: uploaded.FileEncSHA256,
            FileSHA256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uploaded.FileLength),
        },
    }
    
    _, err = client.SendMessage(context.Background(), recipientJID, message)
    return err
}
```

### Document Messages

```go
func sendDocumentMessage(client *whatsmeow.Client, recipient string, documentPath string, fileName string) error {
    // Read document file
    docData, err := os.ReadFile(documentPath)
    if err != nil {
        return err
    }
    
    // Upload document
    uploaded, err := client.Upload(context.Background(), docData, whatsmeow.MediaDocument)
    if err != nil {
        return err
    }
    
    // Create document message
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    message := &waE2E.Message{
        DocumentMessage: &waE2E.DocumentMessage{
            URL:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String("application/pdf"),
            FileEncSHA256: uploaded.FileEncSHA256,
            FileSHA256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uploaded.FileLength),
            FileName:      proto.String(fileName),
        },
    }
    
    _, err = client.SendMessage(context.Background(), recipientJID, message)
    return err
}
```

### Video and Audio Messages

```go
func sendVideoMessage(client *whatsmeow.Client, recipient string, videoPath string) error {
    videoData, err := os.ReadFile(videoPath)
    if err != nil {
        return err
    }
    
    uploaded, err := client.Upload(context.Background(), videoData, whatsmeow.MediaVideo)
    if err != nil {
        return err
    }
    
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    message := &waE2E.Message{
        VideoMessage: &waE2E.VideoMessage{
            URL:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String("video/mp4"),
            FileEncSHA256: uploaded.FileEncSHA256,
            FileSHA256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uploaded.FileLength),
        },
    }
    
    _, err = client.SendMessage(context.Background(), recipientJID, message)
    return err
}

func sendAudioMessage(client *whatsmeow.Client, recipient string, audioPath string) error {
    audioData, err := os.ReadFile(audioPath)
    if err != nil {
        return err
    }
    
    uploaded, err := client.Upload(context.Background(), audioData, whatsmeow.MediaAudio)
    if err != nil {
        return err
    }
    
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    message := &waE2E.Message{
        AudioMessage: &waE2E.AudioMessage{
            URL:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String("audio/ogg; codecs=opus"),
            FileEncSHA256: uploaded.FileEncSHA256,
            FileSHA256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uploaded.FileLength),
        },
    }
    
    _, err = client.SendMessage(context.Background(), recipientJID, message)
    return err
}
```

### Location Messages

```go
func sendLocationMessage(client *whatsmeow.Client, recipient string, latitude, longitude float64, name string) error {
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    
    message := &waE2E.Message{
        LocationMessage: &waE2E.LocationMessage{
            DegreesLatitude:  proto.Float64(latitude),
            DegreesLongitude: proto.Float64(longitude),
            Name:             proto.String(name),
        },
    }
    
    _, err := client.SendMessage(context.Background(), recipientJID, message)
    return err
}
```

### Typing Indicator

```go
import "go.mau.fi/whatsmeow/types"

func sendTypingIndicator(client *whatsmeow.Client, recipient string, isTyping bool) error {
    recipientJID := types.NewJID(recipient, types.DefaultUserServer)
    
    var presence types.Presence
    if isTyping {
        presence = types.PresenceComposing
    } else {
        presence = types.PresencePaused
    }
    
    return client.SendChatPresence(recipientJID, presence, "")
}
```

## 📨 Receiving Messages

### Basic Event Handler

```go
import (
    "fmt"
    
    "go.mau.fi/whatsmeow/types/events"
)

func setupEventHandlers(client *whatsmeow.Client) {
    client.AddEventHandler(func(evt interface{}) {
        switch v := evt.(type) {
        case *events.Message:
            handleMessage(v)
        case *events.Receipt:
            handleReceipt(v)
        case *events.Presence:
            handlePresence(v)
        case *events.HistorySync:
            handleHistorySync(v)
        case *events.Connected:
            fmt.Println("Connected to WhatsApp!")
        case *events.Disconnected:
            fmt.Println("Disconnected from WhatsApp")
        }
    })
}

func handleMessage(evt *events.Message) {
    fmt.Printf("Received message from %s: %s\n", 
        evt.Info.Sender.User, 
        evt.Message.GetConversation())
    
    // Access message info
    messageID := evt.Info.ID
    sender := evt.Info.Sender
    timestamp := evt.Info.Timestamp
    isFromMe := evt.Info.IsFromMe
    
    // Get message content
    if evt.Message.GetConversation() != "" {
        // Text message
        text := evt.Message.GetConversation()
        fmt.Println("Text:", text)
    } else if evt.Message.GetImageMessage() != nil {
        // Image message
        img := evt.Message.GetImageMessage()
        fmt.Println("Image caption:", img.GetCaption())
    } else if evt.Message.GetDocumentMessage() != nil {
        // Document message
        doc := evt.Message.GetDocumentMessage()
        fmt.Println("Document:", doc.GetFileName())
    }
}

func handleReceipt(evt *events.Receipt) {
    fmt.Printf("Receipt: %s from %s (type: %s)\n", 
        evt.MessageIDs[0], 
        evt.Sender.User, 
        evt.Type)
}

func handlePresence(evt *events.Presence) {
    fmt.Printf("Presence update: %s is %s\n", 
        evt.From.User, 
        evt.Presence)
}
```

### Download Media from Messages

```go
import (
    "os"
    
    "go.mau.fi/whatsmeow/types/events"
)

func downloadImage(client *whatsmeow.Client, evt *events.Message) error {
    img := evt.Message.GetImageMessage()
    if img == nil {
        return fmt.Errorf("not an image message")
    }
    
    // Download the image
    data, err := client.Download(img)
    if err != nil {
        return err
    }
    
    // Save to file
    filename := fmt.Sprintf("image_%s.jpg", evt.Info.ID)
    return os.WriteFile(filename, data, 0644)
}

func downloadDocument(client *whatsmeow.Client, evt *events.Message) error {
    doc := evt.Message.GetDocumentMessage()
    if doc == nil {
        return fmt.Errorf("not a document message")
    }
    
    // Download the document
    data, err := client.Download(doc)
    if err != nil {
        return err
    }
    
    // Save to file
    filename := doc.GetFileName()
    if filename == "" {
        filename = fmt.Sprintf("document_%s", evt.Info.ID)
    }
    return os.WriteFile(filename, data, 0644)
}
```

### Auto-Reply Example

```go
func setupAutoReply(client *whatsmeow.Client) {
    client.AddEventHandler(func(evt interface{}) {
        if msg, ok := evt.(*events.Message); ok {
            // Don't reply to own messages
            if msg.Info.IsFromMe {
                return
            }
            
            text := msg.Message.GetConversation()
            if text == "ping" {
                // Send reply
                reply := &waE2E.Message{
                    Conversation: proto.String("pong"),
                }
                client.SendMessage(context.Background(), msg.Info.Sender, reply)
            }
        }
    })
}
```

## 👥 Group Management

### Create a Group

```go
import (
    "context"
    
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types"
)

func createGroup(client *whatsmeow.Client, name string, participants []string) (*types.GroupInfo, error) {
    participantJIDs := make([]types.JID, len(participants))
    for i, p := range participants {
        participantJIDs[i] = types.NewJID(p, types.DefaultUserServer)
    }
    
    req := whatsmeow.ReqCreateGroup{
        Name:         name,
        Participants: participantJIDs,
    }
    
    return client.CreateGroup(context.Background(), req)
}
```

### Get Group Info

```go
func getGroupInfo(client *whatsmeow.Client, groupJID types.JID) (*types.GroupInfo, error) {
    return client.GetGroupInfo(context.Background(), groupJID)
}

func listGroupParticipants(client *whatsmeow.Client, groupJID types.JID) error {
    info, err := client.GetGroupInfo(context.Background(), groupJID)
    if err != nil {
        return err
    }
    
    fmt.Printf("Group: %s\n", info.Name)
    fmt.Printf("Owner: %s\n", info.Owner.User)
    fmt.Println("Participants:")
    for _, p := range info.Participants {
        fmt.Printf("  - %s (%s)\n", p.JID.User, p.DisplayName)
    }
    
    return nil
}
```

### Add/Remove Participants

```go
func addParticipants(client *whatsmeow.Client, groupJID types.JID, participants []string) error {
    participantJIDs := make([]types.JID, len(participants))
    for i, p := range participants {
        participantJIDs[i] = types.NewJID(p, types.DefaultUserServer)
    }
    
    _, err := client.UpdateGroupParticipants(context.Background(), groupJID, participantJIDs, whatsmeow.ParticipantChangeAdd)
    return err
}

func removeParticipants(client *whatsmeow.Client, groupJID types.JID, participants []string) error {
    participantJIDs := make([]types.JID, len(participants))
    for i, p := range participants {
        participantJIDs[i] = types.NewJID(p, types.DefaultUserServer)
    }
    
    _, err := client.UpdateGroupParticipants(context.Background(), groupJID, participantJIDs, whatsmeow.ParticipantChangeRemove)
    return err
}
```

### Update Group Settings

```go
func updateGroupName(client *whatsmeow.Client, groupJID types.JID, newName string) error {
    return client.SetGroupName(context.Background(), groupJID, newName)
}

func updateGroupDescription(client *whatsmeow.Client, groupJID types.JID, description string) error {
    return client.SetGroupTopic(context.Background(), groupJID, "", "", description)
}

func setGroupPhoto(client *whatsmeow.Client, groupJID types.JID, photoPath string) error {
    photoData, err := os.ReadFile(photoPath)
    if err != nil {
        return err
    }
    
    _, err = client.SetGroupPhoto(context.Background(), groupJID, photoData)
    return err
}
```

### Group Invite Links

```go
func getGroupInviteLink(client *whatsmeow.Client, groupJID types.JID, reset bool) (string, error) {
    code, err := client.GetGroupInviteLink(context.Background(), groupJID, reset)
    if err != nil {
        return "", err
    }
    return "https://chat.whatsapp.com/" + code, nil
}

func joinGroupByLink(client *whatsmeow.Client, inviteCode string) (types.JID, error) {
    return client.JoinGroupWithLink(context.Background(), inviteCode)
}
```

### Leave Group

```go
func leaveGroup(client *whatsmeow.Client, groupJID types.JID) error {
    return client.LeaveGroup(context.Background(), groupJID)
}
```

## ⚙️ Advanced Configuration

### Store Options

whatsmeow supports multiple database backends through the `sqlstore` package:

```go
import (
    "context"
    
    _ "github.com/mattn/go-sqlite3"
    _ "github.com/lib/pq"
    _ "github.com/go-sql-driver/mysql"
    
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

// SQLite
func newSQLiteStore(ctx context.Context) (*sqlstore.Container, error) {
    return sqlstore.New(ctx, "sqlite3", "file:whatsmeow.db?_foreign_keys=on", waLog.Noop)
}

// PostgreSQL
func newPostgresStore(ctx context.Context) (*sqlstore.Container, error) {
    return sqlstore.New(ctx, "postgres", "host=localhost user=whatsapp password=secret dbname=whatsapp sslmode=disable", waLog.Noop)
}

// MySQL
func newMySQLStore(ctx context.Context) (*sqlstore.Container, error) {
    return sqlstore.New(ctx, "mysql", "user:password@tcp(localhost:3306)/whatsapp?parseTime=true", waLog.Noop)
}
```

### Logging Configuration

```go
import (
    "os"
    
    "github.com/rs/zerolog"
    waLog "go.mau.fi/whatsmeow/util/log"
)

// Basic stdout logging
func basicLogger() waLog.Logger {
    return waLog.Stdout("Client", "INFO", true)
}

// Custom zerolog logger
func customLogger() waLog.Logger {
    logger := zerolog.New(os.Stdout).
        With().
        Timestamp().
        Logger().
        Level(zerolog.InfoLevel)
    
    return waLog.Zerolog(logger)
}

// File logging
func fileLogger() (waLog.Logger, error) {
    file, err := os.OpenFile("whatsapp.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        return nil, err
    }
    
    logger := zerolog.New(file).
        With().
        Timestamp().
        Logger()
    
    return waLog.Zerolog(logger), nil
}
```

### Proxy Setup

```go
import (
    "net/http"
    "net/url"
    
    "go.mau.fi/whatsmeow"
)

func setupProxy(client *whatsmeow.Client, proxyURL string) error {
    proxy, err := url.Parse(proxyURL)
    if err != nil {
        return err
    }
    
    client.SetProxyAddress(proxyURL, func(client *http.Client) {
        client.Transport = &http.Transport{
            Proxy: http.ProxyURL(proxy),
        }
    })
    
    return nil
}

// Example: SOCKS5 proxy
func setupSOCKS5Proxy(client *whatsmeow.Client) error {
    return setupProxy(client, "socks5://localhost:1080")
}

// Example: HTTP proxy
func setupHTTPProxy(client *whatsmeow.Client) error {
    return setupProxy(client, "http://proxy.example.com:8080")
}
```

### Client Configuration Options

```go
import "go.mau.fi/whatsmeow"

func configureClient(client *whatsmeow.Client) {
    // Enable auto-reconnect
    client.EnableAutoReconnect = true
    client.AutoReconnectHook = func(err error) bool {
        fmt.Printf("Auto-reconnect failed: %v\n", err)
        return true // Return false to stop auto-reconnect
    }
    
    // Configure message handling
    client.SynchronousAck = true // Wait for all handlers before sending acks
    client.EnableDecryptedEventBuffer = true
    
    // Configure app state sync
    client.EmitAppStateEventsOnFullSync = false
    
    // Enable automatic message rerequest from phone
    client.AutomaticMessageRerequestFromPhone = true
}
```

## 🔧 API Reference

### Core Client Methods

| Method | Description |
|--------|-------------|
| `NewClient(store, log)` | Create a new WhatsApp client |
| `Connect()` | Connect to WhatsApp servers |
| `Disconnect()` | Disconnect from WhatsApp |
| `GetQRChannel(ctx)` | Get channel for QR code authentication |
| `PairPhone(phone, showPush, clientType, name)` | Pair using phone number |
| `SendMessage(ctx, to, message)` | Send a message |
| `Download(msg)` | Download media from message |
| `Upload(ctx, data, mediaType)` | Upload media to WhatsApp |
| `AddEventHandler(handler)` | Add event handler function |
| `RemoveEventHandler(id)` | Remove event handler by ID |

### Group Methods

| Method | Description |
|--------|-------------|
| `CreateGroup(ctx, req)` | Create a new group |
| `GetGroupInfo(ctx, jid)` | Get group information |
| `UpdateGroupParticipants(ctx, jid, participants, action)` | Add/remove participants |
| `SetGroupName(ctx, jid, name)` | Change group name |
| `SetGroupTopic(ctx, jid, prevID, prevSetAt, topic)` | Set group description |
| `SetGroupPhoto(ctx, jid, photo)` | Set group photo |
| `GetGroupInviteLink(ctx, jid, reset)` | Get/reset group invite link |
| `JoinGroupWithLink(ctx, code)` | Join group using invite link |
| `LeaveGroup(ctx, jid)` | Leave a group |

### Event Types

| Event Type | Description |
|------------|-------------|
| `*events.Message` | Incoming message |
| `*events.Receipt` | Message delivery/read receipt |
| `*events.Presence` | User presence update (online/offline) |
| `*events.HistorySync` | Chat history synchronization |
| `*events.Connected` | Client connected to server |
| `*events.Disconnected` | Client disconnected |
| `*events.LoggedOut` | Client was logged out |
| `*events.QR` | QR code for authentication |
| `*events.PairSuccess` | Phone pairing successful |
| `*events.GroupInfo` | Group information update |
| `*events.JoinedGroup` | Joined a group |

### Error Handling

```go
import (
    "errors"
    
    "go.mau.fi/whatsmeow"
)

func handleErrors(err error) {
    if errors.Is(err, whatsmeow.ErrNotConnected) {
        fmt.Println("Client is not connected")
    } else if errors.Is(err, whatsmeow.ErrNotLoggedIn) {
        fmt.Println("Client is not logged in")
    } else if errors.Is(err, whatsmeow.ErrBroadcastListUnsupported) {
        fmt.Println("Broadcast lists not supported")
    } else {
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

## 📁 Project Structure

```
whatsmeow/
├── appstate/           # App state synchronization (contacts, settings, etc.)
├── argo/              # Argo protocol implementation
├── binary/            # Binary protocol encoding/decoding
│   ├── proto/        # Protocol buffer definitions
│   └── token/        # Token handling
├── proto/             # Generated protobuf code
├── socket/            # WebSocket connection handling
├── store/             # Database storage layer
│   └── sqlstore/     # SQL-based storage implementation
├── types/             # Type definitions
│   └── events/       # Event type definitions
├── util/              # Utility functions
│   ├── keys/         # Cryptographic key handling
│   └── log/          # Logging utilities
├── client.go          # Main client implementation
├── send.go            # Message sending logic
├── message.go         # Message handling and decryption
├── group.go           # Group management functions
├── user.go            # User-related functions
├── upload.go          # Media upload functionality
├── download.go        # Media download functionality
├── pair.go            # QR code pairing
├── pair-code.go       # Phone number pairing
└── README.md          # This file
```

### Key Files

- **`client.go`**: Core client implementation, connection management
- **`send.go`**: Message sending and encryption
- **`message.go`**: Message receiving and decryption
- **`group.go`**: All group-related operations
- **`upload.go` / `download.go`**: Media handling
- **`store/`**: Session and message storage
- **`types/events/`**: All event type definitions

## 🛠️ Development

### Building from Source

```bash
# Clone the repository
git clone https://go.mau.fi/whatsmeow
cd whatsmeow

# Download dependencies
go mod download

# Build
go build ./...

# Run tests
go test ./...
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests verbosely
go test -v ./...

# Run specific package tests
go test ./store/sqlstore/
```

### Contributing

Contributions are welcome! Here's how you can help:

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/my-new-feature`
3. **Make your changes** and add tests if applicable
4. **Run tests**: `go test ./...`
5. **Format code**: `go fmt ./...`
6. **Commit changes**: `git commit -am 'Add new feature'`
7. **Push to branch**: `git push origin feature/my-new-feature`
8. **Submit a Pull Request**

### Code Style

- Follow standard Go formatting (`gofmt`)
- Write clear, self-documenting code
- Add comments for complex logic
- Include examples in documentation
- Write tests for new features

## ❓ Troubleshooting & FAQ

### Common Issues

**Q: I'm getting "failed to get own ID" errors**  
A: Make sure you're logged in before trying to send messages. Check `client.Store.ID != nil`.

**Q: QR code scanning doesn't work**  
A: Ensure your WhatsApp is on a recent version that supports multidevice. Also make sure you haven't reached the maximum number of linked devices (4).

**Q: Media downloads fail**  
A: Check your network connection and ensure the media message is valid. Some older messages may have expired media links.

**Q: Getting disconnected frequently**  
A: Enable auto-reconnect: `client.EnableAutoReconnect = true`. Also check your network stability.

**Q: Database is locked (SQLite)**  
A: Ensure you're not opening multiple connections to the same SQLite database. Use `_foreign_keys=on` parameter in connection string.

**Q: Messages not being received**  
A: Make sure your event handler is properly registered before connecting. Check logs for any errors.

### Performance Tips

1. **Use connection pooling** for database operations
2. **Enable auto-reconnect** to handle network issues
3. **Process heavy operations asynchronously** in event handlers
4. **Implement rate limiting** for sending messages
5. **Cache frequently accessed data** (group info, contacts)
6. **Use appropriate log levels** (INFO or WARN in production)

### Security Considerations

- **Store session data securely** - the database contains sensitive keys
- **Don't share your database file** - it can be used to impersonate your session
- **Use strong database passwords** in production
- **Implement rate limiting** to avoid being blocked
- **Validate user inputs** before processing
- **Keep the library updated** for security patches

### Debug Mode

```go
// Enable detailed logging
logger := waLog.Stdout("Client", "DEBUG", true)
client := whatsmeow.NewClient(deviceStore, logger)

// Log all binary protocol messages
client.SetLogLevel("TRACE")
```

## 💬 Discussion

Need help or want to discuss the library?

- **Matrix Room**: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)
- **GitHub Discussions**: [WhatsApp Protocol Q&A](https://github.com/tulir/whatsmeow/discussions/categories/whatsapp-protocol-q-a)
- **Issues**: [GitHub Issues](https://github.com/tulir/whatsmeow/issues)

For questions about the WhatsApp protocol (like how to send a specific type of message), use the WhatsApp protocol Q&A section on GitHub discussions.

## 📄 License

This project is licensed under the Mozilla Public License Version 2.0 (MPL 2.0).

Copyright (c) 2021 Tulir Asokan

See the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Tulir Asokan** - Original author and maintainer
- **Contributors** - All the developers who have contributed to this project
- **WhatsApp** - For providing the multidevice API
- **Go Community** - For the excellent tools and libraries

## 🔗 Resources

- **Documentation**: [pkg.go.dev/go.mau.fi/whatsmeow](https://pkg.go.dev/go.mau.fi/whatsmeow)
- **Source Code**: [go.mau.fi/whatsmeow](https://go.mau.fi/whatsmeow)
- **Examples**: See the [godoc examples](https://pkg.go.dev/go.mau.fi/whatsmeow#example-package)

---

**Note**: This library is not affiliated with, sponsored by, or endorsed by WhatsApp or Meta Platforms, Inc.
