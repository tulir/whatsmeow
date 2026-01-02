// Package mobile provides gomobile bindings for the whatsmeow library.
//
// This package enables iOS and Android applications to use the whatsmeow
// WhatsApp library through gomobile bindings.
//
// # Building for iOS
//
// To build the iOS framework, you need to have Go and gomobile installed:
//
//	go install golang.org/x/mobile/cmd/gomobile@latest
//	gomobile init
//	gomobile bind -target=ios -o WhatsApp.xcframework go.mau.fi/whatsmeow/mobile
//
// # Building for Android
//
// To build the Android library:
//
//	gomobile bind -target=android -o whatsapp.aar go.mau.fi/whatsmeow/mobile
//
// # Usage
//
// Create a client with a database path and event callback:
//
//	client, err := mobile.NewClient("/path/to/whatsapp.db", callback)
//	if err != nil {
//	    // handle error
//	}
//
//	// Connect to WhatsApp
//	err = client.Connect()
//
// The callback will receive QR codes to display to the user for pairing.
// Once paired, the callback will receive message events.
//
// # Event Callback
//
// Implement the EventCallback interface to receive events:
//
//	type EventCallback interface {
//	    OnQRCode(code string)
//	    OnConnected()
//	    OnDisconnected(reason string)
//	    OnLoggedOut(reason string)
//	    OnMessage(msg *Message)
//	    OnReceipt(receipt *Receipt)
//	    OnPresence(presence *Presence)
//	    OnHistorySync(progress int, total int)
//	    OnError(err string)
//	}
package mobile
