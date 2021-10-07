module go.mau.fi/whatsmeow

go 1.16

require (
	github.com/RadicalApp/libsignal-protocol-go v0.0.0-20170414202031-d09bcab9f18e
	github.com/gorilla/websocket v1.4.2
	github.com/mdp/qrterminal/v3 v3.0.0
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
	google.golang.org/protobuf v1.26.0
	maunium.net/go/maulogger/v2 v2.2.4
)

replace github.com/RadicalApp/libsignal-protocol-go => github.com/crossle/libsignal-protocol-go v0.0.0-20200729065236-21cc516a6fbf
