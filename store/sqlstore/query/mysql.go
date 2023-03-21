package query

import "fmt"

type MySql struct{}

// whatsmeow_version

func (a *MySql) CreateTableVersion() string {
	return "CREATE TABLE IF NOT EXISTS whatsmeow_version (version INTEGER)"
} // OK

func (a *MySql) GetVersion() string {
	return "SELECT version FROM whatsmeow_version LIMIT 1"
} // OK

func (a *MySql) DeleteAllVersions() string {
	return "DELETE FROM whatsmeow_version"
} // OK

func (a *MySql) InsertNewVersion() string {
	return "INSERT INTO whatsmeow_version (version) VALUES (?)"
} // OK

// whatsmeow_device

func (a *MySql) CreateTableDevice() string {
	return `CREATE TABLE whatsmeow_device (
		jid VARCHAR(50) PRIMARY KEY,

		registration_id BIGINT NOT NULL CHECK ( registration_id >= 0 AND registration_id < 4294967296 ),

		noise_key    BINARY(32) NOT NULL CHECK ( length(noise_key) = 32 ),
		identity_key BINARY(32) NOT NULL CHECK ( length(identity_key) = 32 ),

		signed_pre_key     BINARY(32)   NOT NULL CHECK ( length(signed_pre_key) = 32 ),
		signed_pre_key_id  INTEGER NOT NULL CHECK ( signed_pre_key_id >= 0 AND signed_pre_key_id < 16777216 ),
		signed_pre_key_sig BINARY(64)   NOT NULL CHECK ( length(signed_pre_key_sig) = 64 ),

		adv_key         BLOB NOT NULL,
		adv_details     BLOB NOT NULL,
		adv_account_sig BINARY(64) NOT NULL CHECK ( length(adv_account_sig) = 64 ),
		adv_device_sig  BINARY(64) NOT NULL CHECK ( length(adv_device_sig) = 64 ),

		platform      TEXT NOT NULL DEFAULT '',
		business_name TEXT NOT NULL DEFAULT '',
		push_name     TEXT NOT NULL DEFAULT ''
	)`
} // OK

func (a *MySql) AlterTableDevice_AddColumnSigKey() string {
	return "ALTER TABLE whatsmeow_device ADD COLUMN adv_account_sig_key BINARY(32) NOT NULL"
} // OK

func (a *MySql) FillSigKey() string {
	return `UPDATE whatsmeow_device
	INNER JOIN whatsmeow_identity_keys ON (our_jid=jid AND their_id=concat(split_part(jid, '.', 1), ':0'))
	SET adv_account_sig_key=identity;
	DELETE FROM whatsmeow_device WHERE adv_account_sig_key IS NULL;
	ALTER TABLE whatsmeow_device MODIFY COLUMN adv_account_sig_key BINARY(32) NOT NULL;`
} // OK

func (a *MySql) GetAllDevices() string {
	return `SELECT jid, registration_id, noise_key, identity_key,
		   signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
		   adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
		   platform, business_name, push_name
	FROM whatsmeow_device
	`
} // OK

func (a *MySql) GetDevice() string {
	return fmt.Sprintf("%s %s", a.GetAllDevices(), "WHERE jid=?")
} // OK

func (a *MySql) InsertDevice() string {
	return `INSERT INTO whatsmeow_device (jid, registration_id, noise_key, identity_key,
			signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
			adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
			platform, business_name, push_name)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
	platform=VALUES(platform), business_name=VALUES(business_name), push_name=VALUES(push_name)
`
} // OK

func (a *MySql) DeleteDevice() string {
	return `DELETE FROM whatsmeow_device WHERE jid=?`
} // OK

// whatsmeow_identity_keys

func (a *MySql) CreateTableIdentityKeys() string {
	return `CREATE TABLE whatsmeow_identity_keys (
		our_jid  VARCHAR(50),
		their_id VARCHAR(50),
		identity BINARY(32) NOT NULL CHECK ( length(identity) = 32 ),

		PRIMARY KEY (our_jid, their_id),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutIdentity() string {
	return `INSERT INTO whatsmeow_identity_keys (our_jid, their_id, identity)
	VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE identity=VALUES(identity)
`
} // OK

func (a *MySql) DeleteAllIdentities() string {
	return `DELETE FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id LIKE ?`
} // OK

func (a *MySql) DeleteIdentity() string {
	return `DELETE FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id=?`
} // OK

func (a *MySql) GetIdentity() string {
	return `SELECT identity FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id=?`
} // OK

// whatsmeow_pre_keys

func (a *MySql) CreateTablePreKeys() string {
	return fmt.Sprintf(`CREATE TABLE whatsmeow_pre_keys (
		jid TEXT,
		key_id INTEGER CHECK (key_id >= 0 AND key_id < 16777216),
		%s BINARY(32) NOT NULL,
		uploaded BOOLEAN NOT NULL,
		PRIMARY KEY (jid, key_id),
		FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`, "`key`")
} // OK

func (a *MySql) GetLastPreKeyID() string {
	return `SELECT MAX(key_id) FROM whatsmeow_pre_keys WHERE jid=?`
} // OK

func (a *MySql) InsertPreKey() string {
	return `INSERT INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES (?, ?, ?, ?)`
} // OK

func (a *MySql) GetUnUploadedPreKeys() string {
	return `SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=? AND uploaded=false ORDER BY key_id LIMIT ?`
} // OK

func (a *MySql) GetPreKey() string {
	return `SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=? AND key_id=?`
} // OK

func (a *MySql) DeletePreKey() string {
	return `DELETE FROM whatsmeow_pre_keys WHERE jid=? AND key_id=?`
} // OK

func (a *MySql) MarkPreKeysAsUploaded() string {
	return `UPDATE whatsmeow_pre_keys SET uploaded=true WHERE jid=? AND key_id<=?`
} // OK

func (a *MySql) GetUploadedPreKeyCount() string {
	return `SELECT COUNT(*) FROM whatsmeow_pre_keys WHERE jid=? AND uploaded=true`
} // OK

// whatsmeow_sessions

func (a *MySql) CreateTableSessions() string {
	return `CREATE TABLE whatsmeow_sessions (
		our_jid  VARCHAR(50),
		their_id VARCHAR(50),
		session  BLOB,

		PRIMARY KEY (our_jid, their_id),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) GetSession() string {
	return `SELECT session FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?`
} // OK

func (a *MySql) HasSession() string {
	return `SELECT true FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?`
} // OK

func (a *MySql) PutSession() string {
	return `INSERT INTO whatsmeow_sessions (our_jid, their_id, session) VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE session=VALUES(session)`
} // OK

func (a *MySql) DeleteAllSessions() string {
	return `DELETE FROM whatsmeow_sessions WHERE our_jid=? AND their_id LIKE ?`
} // OK

func (a *MySql) DeleteSession() string {
	return `DELETE FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?`
} // OK

// whatsmeow_sender_keys

func (a *MySql) CreateTableSenderKeys() string {
	return `CREATE TABLE whatsmeow_sender_keys (
		our_jid    TEXT,
		chat_id    TEXT,
		sender_id  TEXT,
		sender_key BLOB NOT NULL,
	
		PRIMARY KEY (our_jid, chat_id, sender_id),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) GetSenderKey() string {
	return `SELECT sender_key FROM whatsmeow_sender_keys WHERE our_jid=? AND chat_id=? AND sender_id=?`
} // OK

func (a *MySql) PutSenderKey() string {
	return `INSERT INTO whatsmeow_sender_keys (our_jid, chat_id, sender_id, sender_key) 
	VALUES (?, ?, ?, ?) 
	ON DUPLICATE KEY UPDATE sender_key=VALUES(sender_key)`
} // OK

// whatsmeow_app_state_sync_keys

func (a *MySql) CreateTableStateSyncKeys() string {
	return `CREATE TABLE whatsmeow_app_state_sync_keys (
		jid         TEXT,
		key_id      BLOB,
		key_data    BLOB NOT NULL,
		timestamp   BIGINT NOT NULL,
		fingerprint BLOB NOT NULL,
	
		PRIMARY KEY (jid, key_id),
		FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutAppStateSyncKey() string {
	return `
	INSERT INTO whatsmeow_app_state_sync_keys (jid, key_id, key_data, timestamp, fingerprint) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT (jid, key_id) DO UPDATE
		SET key_data=excluded.key_data, timestamp=excluded.timestamp, fingerprint=excluded.fingerprint
		WHERE excluded.timestamp > whatsmeow_app_state_sync_keys.timestamp
`
} // TODO ???

func (a *MySql) GetAppStateSyncKey() string {
	return `SELECT key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys WHERE jid=? AND key_id=?`
} // OK

// whatsmeow_app_state_version

func (a *MySql) CreateTableStateVersion() string {
	return `CREATE TABLE whatsmeow_app_state_version (
		jid     TEXT,
		name    TEXT,
		version BIGINT NOT NULL,
		hash    BINARY(128) NOT NULL,
	
		PRIMARY KEY (jid, name),
		FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutAppStateVersion() string {
	return `
	INSERT INTO whatsmeow_app_state_version (jid, name, version, hash) VALUES (?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE version=VALUES(version), hash=VALUES(hash)
`
} // OK

func (a *MySql) GetAppStateVersion() string {
	return `SELECT version, hash FROM whatsmeow_app_state_version WHERE jid=? AND name=?`
} // OK

func (a *MySql) DeleteAppStateVersion() string {
	return `DELETE FROM whatsmeow_app_state_version WHERE jid=? AND name=?`
} // OK

// whatsmeow_app_state_mutation_macs

func (a *MySql) CreateTableStateMutationMacs() string {
	return `CREATE TABLE whatsmeow_app_state_mutation_macs (
		jid       TEXT,
		name      TEXT,
		version   BIGINT,
		index_mac BINARY(32),
		value_mac BINARY(32) NOT NULL,
		PRIMARY KEY (jid, name, version, index_mac),
		FOREIGN KEY (jid, name) REFERENCES whatsmeow_app_state_version(jid, name) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutAppStateMutationMACs() string {
	return `INSERT INTO whatsmeow_app_state_mutation_macs (jid, name, version, index_mac, value_mac) VALUES `
} // OK ??

func (a *MySql) DeleteAppStateMutationMACs() string {
	return `DELETE FROM whatsmeow_app_state_mutation_macs WHERE jid=? AND name=? AND index_mac IN (?)`
} // OK ??

func (a *MySql) GetAppStateMutationMAC() string {
	return `SELECT value_mac FROM whatsmeow_app_state_mutation_macs WHERE jid=? AND name=? AND index_mac=? ORDER BY version DESC LIMIT 1`
} // OK

// whatsmeow_contacts

func (a *MySql) CreateTableContacts() string {
	return `CREATE TABLE whatsmeow_contacts (
		our_jid       TEXT,
		their_jid     TEXT,
		first_name    TEXT,
		full_name     TEXT,
		push_name     TEXT,
		business_name TEXT,

		PRIMARY KEY (our_jid, their_jid),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutContactName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name) 
	VALUES (?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE first_name=VALUES(first_name), full_name=VALUES(full_name)`
} // OK

func (a *MySql) PutManyContactNames() string {
	return `
	INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name)
	VALUES %s
	ON CONFLICT (our_jid, their_jid) DO UPDATE SET first_name=excluded.first_name, full_name=excluded.full_name
`
} // TODO ????

func (a *MySql) PutPushName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, push_name) 
	VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE push_name=VALUES(push_name)`
} // OK

func (a *MySql) PutBusinessName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, business_name) VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE business_name=VALUES(business_name)`
} // OK

func (a *MySql) GetContact() string {
	return `SELECT first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=? AND their_jid=?`
} // OK

func (a *MySql) GetAllContacts() string {
	return `SELECT their_jid, first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=?`
} // OK

// whatsmeow_chat_settings

func (a *MySql) CreateTableChatSettings() string {
	return `CREATE TABLE whatsmeow_chat_settings (
		our_jid       TEXT,
		chat_jid      TEXT,
		muted_until   BIGINT  NOT NULL DEFAULT 0,
		pinned        BOOLEAN NOT NULL DEFAULT false,
		archived      BOOLEAN NOT NULL DEFAULT false,

		PRIMARY KEY (our_jid, chat_jid),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutChatSetting() string {
	return `INSERT INTO whatsmeow_chat_settings (our_jid, chat_jid, %[1]s) VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE %[1]s=VALUES(%[1]s)`
} // OK

func (a *MySql) GetChatSettings() string {
	return `SELECT muted_until, pinned, archived FROM whatsmeow_chat_settings WHERE our_jid=? AND chat_jid=?`
} // OK

// whatsmeow_message_secrets

func (a *MySql) CreateTableMessageSecrets() string {
	return `CREATE TABLE whatsmeow_message_secrets (
		our_jid    TEXT,
		chat_jid   TEXT,
		sender_jid TEXT,
		message_id TEXT,
		key        BLOB NOT NULL,

		PRIMARY KEY (our_jid, chat_jid, sender_jid, message_id),
		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
	)`
} // OK

func (a *MySql) PutMsgSecret() string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_message_secrets (our_jid, chat_jid, sender_jid, message_id, %[1]s)
	VALUES (?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE %[1]s=%[1]s`, "`key`")
} // OK

func (a *MySql) GetMsgSecret() string {
	return fmt.Sprintf(`SELECT %s FROM whatsmeow_message_secrets WHERE our_jid=? AND chat_jid=? AND sender_jid=? AND message_id=?`, "`key`")
} // OK

// whatsmeow_privacy_tokens

func (a *MySql) CreateTablePrivacyTokens() string {
	return `CREATE TABLE whatsmeow_privacy_tokens (
		our_jid   TEXT,
		their_jid TEXT,
		token     VARBINARY(255)  NOT NULL,
		timestamp BIGINT NOT NULL,
		PRIMARY KEY (our_jid, their_jid)
	)`
} // OK ???

func (a *MySql) PutPrivacyTokens() string {
	return `INSERT INTO whatsmeow_privacy_tokens (our_jid, their_jid, token, timestamp)
	VALUES (?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE token=VALUES(token), timestamp=VALUES(timestamp)`
} // OK

func (a *MySql) GetPrivacyToken() string {
	return `SELECT token, timestamp FROM whatsmeow_privacy_tokens WHERE our_jid=? AND their_jid=?`
} // OK

// placeholder ???

func (a *MySql) PlaceholderSyntax() string {
	return "(?, ?, ?, ?, ?)"
} // OK ??
