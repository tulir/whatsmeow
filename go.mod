module go.mau.fi/whatsmeow

go 1.17

require (
	github.com/RadicalApp/libsignal-protocol-go v0.0.0-20170414202031-d09bcab9f18e
	github.com/gorilla/websocket v1.4.2
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	google.golang.org/protobuf v1.27.1
)

require (
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/RadicalApp/complete v0.0.0-20170329192659-17e6c0ee499b // indirect
)

replace github.com/RadicalApp/libsignal-protocol-go => github.com/tulir/libsignal-protocol-go v0.0.0-20211015104614-7ac953a1b8f5
