module go.mau.fi/whatsmeow/mdtest

go 1.19

require (
	github.com/go-whatsapp/whatsmeow v0.0.0-20231110043443-090e9262d8a0
	github.com/goccy/go-json v0.10.2
	github.com/mattn/go-sqlite3 v1.14.18
	github.com/mdp/qrterminal/v3 v3.0.0
	google.golang.org/protobuf v1.31.0
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/cristalhq/base64 v0.1.2 // indirect
	github.com/go-whatsapp/go-util v0.1.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	go.mau.fi/libsignal v0.1.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	rsc.io/qr v0.2.0 // indirect
)

replace go.mau.fi/whatsmeow => ../
