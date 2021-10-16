module go.mau.fi/whatsmeow/mdtest

go 1.17

require (
	github.com/mattn/go-sqlite3 v1.14.8
	github.com/mdp/qrterminal/v3 v3.0.0
	go.mau.fi/whatsmeow v0.0.0-20211015203634-46b9eb90dc19
	google.golang.org/protobuf v1.27.1
	maunium.net/go/maulogger/v2 v2.2.4
)

require (
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/RadicalApp/complete v0.0.0-20170329192659-17e6c0ee499b // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	go.mau.fi/libsignal v0.0.0-20211016125744-b84e562375e1 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	rsc.io/qr v0.2.0 // indirect
)

replace go.mau.fi/whatsmeow => ../
