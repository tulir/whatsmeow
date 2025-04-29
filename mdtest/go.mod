module go.mau.fi/whatsmeow/mdtest

go 1.23.0

toolchain go1.24.0

require (
	github.com/mattn/go-sqlite3 v1.14.24
	github.com/mdp/qrterminal/v3 v3.0.0
	go.mau.fi/whatsmeow v0.0.0-20240327124018-350073db195c
	google.golang.org/protobuf v1.36.6
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	go.mau.fi/libsignal v0.1.2 // indirect
	go.mau.fi/util v0.8.6 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	rsc.io/qr v0.2.0 // indirect
)

replace go.mau.fi/whatsmeow => ../
