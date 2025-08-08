module github.com/PakaiWA/whatsmeow

go 1.23.0

toolchain go1.24.5

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/rs/zerolog v1.34.0
	go.mau.fi/libsignal v0.2.0
	go.mau.fi/util v0.8.8
	go.mau.fi/whatsmeow v0.0.0-20250807072145-72ce90b82194
	golang.org/x/crypto v0.41.0
	golang.org/x/net v0.43.0
	google.golang.org/protobuf v1.36.7
)

replace go.mau.fi/whatsmeow v0.0.0-20250807072145-72ce90b82194 => github.com/PakaiWA/whatsmeow v0.25.8-8

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/petermattis/goid v0.0.0-20250508124226-395b08cebbdb // indirect
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
