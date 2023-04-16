package query

func NewByDialect(dialect string) Adapter {
	switch dialect {
	case "postgres", "pgx":
		return &Postgres{}
	case "sqlite", "sqlite3":
		return &Sqlite{
			Default: &Postgres{},
		}
	case "mysql":
		return &MySql{}
	default:
		return &Postgres{}
	}
}

// Adapter store all available queries to easily change the drivers
type Adapter interface {
	Version
	Device
	IdentityKeys
	PreKeys
	Sessions
	SenderKeys
	AppStateSyncKeys
	AppStateVersion
	AppStateMutationMacs
	Contacts
	ChatSettings
	MessageSecrets
	PrivacyTokens
	PlaceholderCreate(int, int) string
}

// Version represents the table whatsmeow_version
type Version interface {
	CreateTableVersion() string
	GetVersion() string
	DeleteAllVersions() string
	InsertNewVersion() string
}

// Device represents the table whatsmeow_device
type Device interface {
	CreateTableDevice() string
	AlterTableDevice_AddColumnSigKey() string
	FillSigKey() string
	DeleteNullSigKeys() string
	AlterTableDevice_SetNotNull() string
	GetAllDevices() string
	GetDevice() string
	InsertDevice() string
	DeleteDevice() string
}

// IdentityKeys represents the table whatsmeow_identity_keys
type IdentityKeys interface {
	CreateTableIdentityKeys() string
	PutIdentity() string
	DeleteAllIdentities() string
	DeleteIdentity() string
	GetIdentity() string
}

// PreKeys represents the table whatsmeow_pre_keys
type PreKeys interface {
	CreateTablePreKeys() string
	GetLastPreKeyID() string
	InsertPreKey() string
	GetUnUploadedPreKeys() string
	GetPreKey() string
	DeletePreKey() string
	MarkPreKeysAsUploaded() string
	GetUploadedPreKeyCount() string
}

// Sessions represents the table whatsmeow_sessions
type Sessions interface {
	CreateTableSessions() string
	GetSession() string
	HasSession() string
	PutSession() string
	DeleteAllSessions() string
	DeleteSession() string
}

// SenderKeys represents the table whatsmeow_sender_keys
type SenderKeys interface {
	CreateTableSenderKeys() string
	GetSenderKey() string
	PutSenderKey() string
}

// AppStateSyncKeys represents the table whatsmeow_app_state_sync_keys
type AppStateSyncKeys interface {
	CreateTableStateSyncKeys() string
	PutAppStateSyncKey() string
	GetAppStateSyncKey() string
}

// AppStateVersion represents the table whatsmeow_app_state_version
type AppStateVersion interface {
	CreateTableStateVersion() string
	PutAppStateVersion() string
	GetAppStateVersion() string
	DeleteAppStateVersion() string
}

// AppStateMutationMacs represents the table whatsmeow_app_state_mutation_macs
type AppStateMutationMacs interface {
	CreateTableStateMutationMacs() string
	PutAppStateMutationMACs(string) string
	DeleteAppStateMutationMACs(string) string
	GetAppStateMutationMAC() string
}

// Contacts represents the table whatsmeow_contacts
type Contacts interface {
	CreateTableContacts() string
	PutContactName() string
	PutManyContactNames(string) string
	PutPushName() string
	PutBusinessName() string
	GetContact() string
	GetAllContacts() string
}

// ChatSettings represents the table whatsmeow_chat_settings
type ChatSettings interface {
	CreateTableChatSettings() string
	PutChatSetting(string) string
	GetChatSettings() string
}

// MessageSecrets represents the table whatsmeow_message_secrets
type MessageSecrets interface {
	CreateTableMessageSecrets() string
	PutMsgSecret() string
	GetMsgSecret() string
}

// PrivacyTokens represents the table whatsmeow_privacy_tokens
type PrivacyTokens interface {
	CreateTablePrivacyTokens() string
	PutPrivacyTokens(string) string
	GetPrivacyToken() string
}
