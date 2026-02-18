package shared

import "time"

type ChatStatus struct {
	JID          string    `json:"jid"`
	Name         string    `json:"name"`
	MessageCount int       `json:"message_count"`
	MediaCount   int       `json:"media_count"`
	PrivacyNotes []string  `json:"privacy_notes"`
	ImportStatus string    `json:"import_status"`
	LastImported time.Time `json:"last_imported"`
	IsGroup      bool      `json:"is_group"`
}

type MessageData struct {
	ID        string `json:"id"`
	ChatJID   string `json:"chat_jid"`
	Sender    string `json:"sender"`
	Text      string `json:"text"`
	Caption   string `json:"caption,omitempty"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	FromMe    bool   `json:"from_me"`
	Status    string `json:"status"`
	HasMedia  bool   `json:"has_media"`
	MimeType  string `json:"mime_type,omitempty"`
	FileName  string `json:"file_name,omitempty"`

	// Media metadata for "links" export and reconstruction
	DirectPath    string `json:"direct_path,omitempty"`
	MediaKey      []byte `json:"media_key,omitempty"`
	FileSHA256    []byte `json:"file_sha256,omitempty"`
	FileEncSHA256 []byte `json:"file_enc_sha256,omitempty"`
}

type StateData struct {
	ImportStats   map[string]*ChatStatus `json:"import_stats"`
	ChatMessages  map[string][]MessageData `json:"chat_messages"`
	MediaMessages []MessageData            `json:"media_messages"`
}
