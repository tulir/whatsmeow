# WhatsApp iOS App

A native iOS application built with Swift and SwiftUI that uses the [whatsmeow](https://github.com/tulir/whatsmeow) Go library for WhatsApp Web API.

## Features

- QR code login (link as WhatsApp Web device)
- Send and receive text messages
- Send images, documents, and media
- Group chat support
- Real-time message delivery and read receipts
- Contact management
- Typing indicators
- Message history sync

## Architecture

```
ios/
├── WhatsApp/
│   ├── WhatsApp.xcodeproj/     # Xcode project
│   └── WhatsApp/
│       ├── Models/              # Data models (Message, Contact, Chat)
│       ├── Views/               # SwiftUI views
│       ├── ViewModels/          # MVVM view models
│       ├── Services/            # WhatsApp client wrapper
│       └── Assets.xcassets/     # App icons and colors
├── Frameworks/                   # Go mobile framework (generated)
└── Scripts/                      # Build scripts
```

## Requirements

- macOS 13.0 or later
- Xcode 15.0 or later
- iOS 17.0+ deployment target
- Go 1.21 or later (for building the framework)
- gomobile tool

## Building

### 1. Install Prerequisites

```bash
# Install Go (if not installed)
brew install go

# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

### 2. Build the Go Framework

```bash
# From the repository root
cd /path/to/whatsmeow

# Build the iOS framework
./ios/Scripts/build-framework.sh
```

This will generate `WhatsApp.xcframework` in the `ios/Frameworks/` directory.

### 3. Open and Build in Xcode

```bash
open ios/WhatsApp/WhatsApp.xcodeproj
```

1. Select your development team in Signing & Capabilities
2. Choose your target device or simulator
3. Press Cmd+R to build and run

## Project Structure

### Models

- **Message.swift** - WhatsApp message representation
- **Contact.swift** - Contact information
- **Chat.swift** - Chat/conversation data

### Views

- **ContentView.swift** - Main app entry with state routing
- **QRCodeView.swift** - QR code display for login
- **ChatListView.swift** - List of chats
- **ChatView.swift** - Individual chat conversation
- **MessageBubbleView.swift** - Message bubble UI
- **ContactsView.swift** - Contacts list
- **SettingsView.swift** - App settings
- **NewChatView.swift** - Start new chat/group

### Services

- **WhatsAppClient.swift** - Go framework bridge
- **Extensions.swift** - Swift extensions and utilities

### ViewModels

- **ChatViewModel.swift** - Main app state management

## Go Mobile Integration

The app uses gomobile to bind the Go whatsmeow library to iOS. The binding layer is in the `/mobile` package:

```go
// mobile/client.go
type Client struct {
    // WhatsApp client wrapper
}

type EventCallback interface {
    OnQRCode(code string)
    OnConnected()
    OnMessage(msg *Message)
    // ...
}
```

### Building Custom Framework

If you need to modify the Go bindings:

1. Edit `mobile/client.go`
2. Run `./ios/Scripts/build-framework.sh`
3. Clean and rebuild the Xcode project

## Usage

### QR Code Login

The app displays a QR code when first launched. Scan this with WhatsApp on your phone:

1. Open WhatsApp on your phone
2. Go to Settings > Linked Devices
3. Tap "Link a Device"
4. Scan the QR code displayed in the app

### Sending Messages

```swift
// Via ViewModel
await viewModel.sendMessage(to: chatJID, text: "Hello!")

// Or directly via client
let messageID = try await WhatsAppClient.shared.sendTextMessage(
    to: "1234567890@s.whatsapp.net",
    text: "Hello, World!"
)
```

### Receiving Messages

Messages are received through the delegate pattern:

```swift
extension ChatViewModel: WhatsAppClientDelegate {
    func didReceiveMessage(_ message: Message) {
        // Handle incoming message
        messages[message.chatJID]?.append(message)
    }
}
```

## Demo Mode

For testing without a real WhatsApp connection, the app includes a Demo Mode:

1. Launch the app
2. On the QR code screen, tap "Demo Mode (Skip QR)"
3. The app will load with sample data

## Customization

### Colors

Edit `Assets.xcassets/AccentColor.colorset` or use the color extensions in `Extensions.swift`:

```swift
extension Color {
    static let whatsappGreen = Color(red: 37/255, green: 211/255, blue: 102/255)
    static let whatsappDarkGreen = Color(red: 18/255, green: 140/255, blue: 126/255)
}
```

### App Icon

Replace the app icon in `Assets.xcassets/AppIcon.appiconset/` with your own 1024x1024 PNG.

## Troubleshooting

### Framework Not Found

If Xcode reports "Framework not found", ensure:

1. The framework is built: `./ios/Scripts/build-framework.sh`
2. Framework search paths in Xcode include `$(PROJECT_DIR)/../Frameworks`

### Signing Issues

1. Open the project in Xcode
2. Select the WhatsApp target
3. Go to Signing & Capabilities
4. Select your development team

### Connection Issues

- Ensure you have internet connectivity
- Check that your phone's WhatsApp is up to date
- Try re-linking the device (logout and scan QR again)

## Security Notes

- The app stores session keys in the local SQLite database
- Consider implementing Keychain storage for production
- Messages are end-to-end encrypted by the WhatsApp protocol
- The app never sees message content in plaintext on WhatsApp servers

## License

This iOS app is part of the whatsmeow project and is licensed under the Mozilla Public License v2.0.

## Contributing

Contributions are welcome! Please read the main repository's contributing guidelines.
