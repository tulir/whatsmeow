package binary

import "fmt"

var SingleTokens = [...]string{"", "", "", "200", "400", "404", "500", "501", "502", "action", "add",
	"after", "archive", "author", "available", "battery", "before", "body",
	"broadcast", "chat", "clear", "code", "composing", "contacts", "count",
	"create", "debug", "delete", "demote", "duplicate", "encoding", "error",
	"false", "filehash", "from", "g.us", "group", "groups_v2", "height", "id",
	"image", "in", "index", "invis", "item", "jid", "kind", "last", "leave",
	"live", "log", "media", "message", "mimetype", "missing", "modify", "name",
	"notification", "notify", "out", "owner", "participant", "paused",
	"picture", "played", "presence", "preview", "promote", "query", "raw",
	"read", "receipt", "received", "recipient", "recording", "relay",
	"remove", "response", "resume", "retry", "s.whatsapp.net", "seconds",
	"set", "size", "status", "subject", "subscribe", "t", "text", "to", "true",
	"type", "unarchive", "unavailable", "url", "user", "value", "web", "width",
	"mute", "read_only", "admin", "creator", "short", "update", "powersave",
	"checksum", "epoch", "block", "previous", "409", "replaced", "reason",
	"spam", "modify_tag", "message_info", "delivery", "emoji", "title",
	"description", "canonical-url", "matched-text", "star", "unstar",
	"media_key", "filename", "identity", "unread", "page", "page_count",
	"search", "media_message", "security", "call_log", "profile", "ciphertext",
	"invite", "gif", "vcard", "frequent", "privacy", "blacklist", "whitelist",
	"verify", "location", "document", "elapsed", "revoke_invite", "expiration",
	"unsubscribe", "disable", "vname", "old_jid", "new_jid", "announcement",
	"locked", "prop", "label", "color", "call", "offer", "call-id"}

var doubleTokens = [...]string{}

func GetToken(i int) (string, error) {
	if i < 3 || i >= len(SingleTokens) {
		return "", fmt.Errorf("index out of token bounds %d", i)
	}

	return SingleTokens[i], nil
}

func GetTokenDouble(index1 int, index2 int) (string, error) {
	n := 256*index1 + index2
	if n < 0 || n >= len(doubleTokens) {
		return "", fmt.Errorf("index out of double token bounds %d", n)
	}

	return doubleTokens[n], nil
}

func IndexOfToken(token string) int {
	for i, t := range SingleTokens {
		if t == token {
			return i
		}
	}

	return -1
}

const (
	LIST_EMPTY      = 0
	STREAM_END      = 2
	DICTIONARY_0    = 236
	DICTIONARY_1    = 237
	DICTIONARY_2    = 238
	DICTIONARY_3    = 239
	LIST_8          = 248
	LIST_16         = 249
	JID_PAIR        = 250
	HEX_8           = 251
	BINARY_8        = 252
	BINARY_20       = 253
	BINARY_32       = 254
	NIBBLE_8        = 255
	SINGLE_BYTE_MAX = 256
	PACKED_MAX      = 254
)

type AppInfo string

const (
	IMAGE    AppInfo = "WhatsApp Image Keys"
	VIDEO    AppInfo = "WhatsApp Video Keys"
	AUDIO    AppInfo = "WhatsApp Audio Keys"
	DOCUMENT AppInfo = "WhatsApp Document Keys"
)

type Metric byte

const (
	DEBUG_LOG Metric = iota + 1
	QUERY_RESUME
	QUERY_RECEIPT
	QUERY_MEDIA
	QUERY_CHAT
	QUERY_CONTACTS
	QUERY_MESSAGES
	PRESENCE
	PRESENCE_SUBSCRIBE
	GROUP
	READ
	CHAT
	RECEIVED
	PIC
	STATUS
	MESSAGE
	QUERY_ACTIONS
	BLOCK
	QUERY_GROUP
	QUERY_PREVIEW
	QUERY_EMOJI
	QUERY_MESSAGE_INFO
	SPAM
	QUERY_SEARCH
	QUERY_IDENTITY
	QUERY_URL
	PROFILE
	CONTACT
	QUERY_VCARD
	QUERY_STATUS
	QUERY_STATUS_UPDATE
	PRIVACY_STATUS
	QUERY_LIVE_LOCATIONS
	LIVE_LOCATION
	QUERY_VNAME
	QUERY_LABELS
	CALL
	QUERY_CALL
	QUERY_QUICK_REPLIES
)

type Flag byte

const (
	IGNORE Flag = 1 << (7 - iota)
	ACKREQUEST
	AVAILABLE
	NOTAVAILABLE
	EXPIRES
	SKIPOFFLINE
)
