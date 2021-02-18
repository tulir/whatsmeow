package whatsapp

// JID is a WhatsApp user or group ID.
type JID = string

// MessageID is the internal ID of a WhatsApp message.
type MessageID = string

const (
	OldUserSuffix = "@c.us"
	NewUserSuffix = "@s.whatsapp.net"
)
