package types

// internal package
import types "go.mau.fi/whatsmeow/types"

///////////////////////////
//    globals structs    //
///////////////////////////

// incoming message
type ST_G_IncomingMessage struct {
	Subdomain,
	CountryCode,
	IncomingMessageText,
	IncomingAttachmentURL,
	IncomingAttachmentMimeType,
	PushName string
	JID types.JID
}

// outgoing message
type ST_G_OutgoingMessage struct {
	ToPhone,
	MessageText,
	FromPhone string
}

// customer registration info
type ST_G_CustomerRegistrationInfo struct {
	IsRegistered,
	IsNameFirstSaved,
	IsNameLastSaved,
	IsCitySaved bool
}
