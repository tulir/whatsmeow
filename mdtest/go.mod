module go.mau.fi/whatsmeow/mdtest

go 1.17

require (
	github.com/mattn/go-sqlite3 v1.14.12
	github.com/mdp/qrterminal/v3 v3.0.0
	go.mau.fi/whatsmeow v0.0.0-20220502122315-61256be77a41
	google.golang.org/protobuf v1.28.0
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	go.mau.fi/libsignal v0.0.0-20220628090436-4d18b66b087e // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	rsc.io/qr v0.2.0 // indirect
)

replace go.mau.fi/whatsmeow => ../
