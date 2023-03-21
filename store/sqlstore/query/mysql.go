package query

import (
	"fmt"
	"strings"
)

type MySql struct{}

// whatsmeow_version

func (a *MySql) CreateTableVersion() string {
	return "CREATE TABLE IF NOT EXISTS whatsmeow_version (version INTEGER)"
}

func (a *MySql) GetVersion() string {
	return "SELECT version FROM whatsmeow_version LIMIT 1"
}

func (a *MySql) DeleteAllVersions() string {
	return "DELETE FROM whatsmeow_version"
}

func (a *MySql) InsertNewVersion() string {
	return "INSERT INTO whatsmeow_version (version) VALUES (?)"
}

// whatsmeow_device

func (a *MySql) CreateTableDevice() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_device (
        jid VARCHAR(50) PRIMARY KEY,

        registration_id BIGINT NOT NULL CHECK ( registration_id >= 0 AND registration_id < 4294967296 ),

        noise_key    BINARY(32) NOT NULL,
        identity_key BINARY(32) NOT NULL,

        signed_pre_key     BINARY(32)   NOT NULL,
        signed_pre_key_id  INTEGER NOT NULL CHECK ( signed_pre_key_id >= 0 AND signed_pre_key_id < 16777216 ),
        signed_pre_key_sig BINARY(64),

        adv_key         BLOB NOT NULL,
        adv_details     BLOB NOT NULL,
        adv_account_sig BINARY(64) NOT NULL,
        adv_device_sig  BINARY(64) NOT NULL,

        platform      TEXT NOT NULL,
        business_name TEXT NOT NULL,
        push_name     TEXT NOT NULL
	)`
}

func (a *MySql) AlterTableDevice_AddColumnSigKey() string {
	return "ALTER TABLE whatsmeow_device ADD COLUMN adv_account_sig_key BINARY(32) NOT NULL"
}

func (a *MySql) FillSigKey() string {
	return `UPDATE whatsmeow_device
    INNER JOIN whatsmeow_identity_keys ON (our_jid=jid AND their_id=CONCAT(SUBSTRING_INDEX(whatsmeow_device.jid, '.', 1), ':0'))
    SET adv_account_sig_key=identity`
}

func (a *MySql) DeleteNullSigKeys() string {
	return "DELETE FROM whatsmeow_device WHERE adv_account_sig_key IS NULL"
}

func (a *MySql) AlterTableDevice_SetNotNull() string {
	return "ALTER TABLE whatsmeow_device MODIFY COLUMN adv_account_sig_key BINARY(32) NOT NULL;"
}

func (a *MySql) GetAllDevices() string {
	return `SELECT jid, registration_id, noise_key, identity_key,
           signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
           adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
           platform, business_name, push_name
    FROM whatsmeow_device`
}

func (a *MySql) GetDevice() string {
	return fmt.Sprintf("%s %s", a.GetAllDevices(), "WHERE jid=?")
}

func (a *MySql) InsertDevice() string {
	return `INSERT INTO whatsmeow_device (jid, registration_id, noise_key, identity_key,
            signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
            adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
            platform, business_name, push_name)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
    platform=VALUES(platform), business_name=VALUES(business_name), push_name=VALUES(push_name)`
}

func (a *MySql) DeleteDevice() string {
	return "DELETE FROM whatsmeow_device WHERE jid=?"
}

// whatsmeow_identity_keys

func (a *MySql) CreateTableIdentityKeys() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_identity_keys (
        our_jid  VARCHAR(50),
        their_id VARCHAR(50),
        identity BINARY(32) NOT NULL CHECK ( length(identity) = 32 ),

        PRIMARY KEY (our_jid, their_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutIdentity() string {
	return `INSERT INTO whatsmeow_identity_keys (our_jid, their_id, identity)
    VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE identity=VALUES(identity)`
}

func (a *MySql) DeleteAllIdentities() string {
	return "DELETE FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id LIKE ?"
}

func (a *MySql) DeleteIdentity() string {
	return "DELETE FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id=?"
}

func (a *MySql) GetIdentity() string {
	return "SELECT identity FROM whatsmeow_identity_keys WHERE our_jid=? AND their_id=?"
}

// whatsmeow_pre_keys

func (a *MySql) CreateTablePreKeys() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS whatsmeow_pre_keys (
        jid VARCHAR(50),
        key_id INTEGER CHECK (key_id >= 0 AND key_id < 16777216),
        %s BINARY(32) NOT NULL,
        uploaded BOOLEAN NOT NULL,
        
        PRIMARY KEY (jid, key_id),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`, "`key`")
}

func (a *MySql) GetLastPreKeyID() string {
	return "SELECT MAX(key_id) FROM whatsmeow_pre_keys WHERE jid=?"
}

func (a *MySql) InsertPreKey() string {
	return "INSERT INTO whatsmeow_pre_keys (jid, key_id, `key`, uploaded) VALUES (?, ?, ?, ?)"
}

func (a *MySql) GetUnUploadedPreKeys() string {
	return "SELECT key_id, `key` FROM whatsmeow_pre_keys WHERE jid=? AND uploaded=false ORDER BY key_id LIMIT ?"
}

func (a *MySql) GetPreKey() string {
	return "SELECT key_id, `key` FROM whatsmeow_pre_keys WHERE jid=? AND key_id=?"
}

func (a *MySql) DeletePreKey() string {
	return "DELETE FROM whatsmeow_pre_keys WHERE jid=? AND key_id=?"
}

func (a *MySql) MarkPreKeysAsUploaded() string {
	return "UPDATE whatsmeow_pre_keys SET uploaded=true WHERE jid=? AND key_id<=?"
}

func (a *MySql) GetUploadedPreKeyCount() string {
	return "SELECT COUNT(*) FROM whatsmeow_pre_keys WHERE jid=? AND uploaded=true"
}

// whatsmeow_sessions

func (a *MySql) CreateTableSessions() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_sessions (
        our_jid  VARCHAR(50),
        their_id VARCHAR(50),
        session  BLOB,

        PRIMARY KEY (our_jid, their_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) GetSession() string {
	return "SELECT session FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?"
}

func (a *MySql) HasSession() string {
	return "SELECT true FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?"
}

func (a *MySql) PutSession() string {
	return `INSERT INTO whatsmeow_sessions (our_jid, their_id, session)
	VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE session=VALUES(session)`
}

func (a *MySql) DeleteAllSessions() string {
	return "DELETE FROM whatsmeow_sessions WHERE our_jid=? AND their_id LIKE ?"
}

func (a *MySql) DeleteSession() string {
	return "DELETE FROM whatsmeow_sessions WHERE our_jid=? AND their_id=?"
}

// whatsmeow_sender_keys

func (a *MySql) CreateTableSenderKeys() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_sender_keys (
        our_jid    VARCHAR(50),
        chat_id    VARCHAR(50),
        sender_id  VARCHAR(50),
        sender_key BLOB NOT NULL,
    
        PRIMARY KEY (our_jid, chat_id, sender_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) GetSenderKey() string {
	return "SELECT sender_key FROM whatsmeow_sender_keys WHERE our_jid=? AND chat_id=? AND sender_id=?"
}

func (a *MySql) PutSenderKey() string {
	return `INSERT INTO whatsmeow_sender_keys (our_jid, chat_id, sender_id, sender_key) 
    VALUES (?, ?, ?, ?) 
    ON DUPLICATE KEY UPDATE sender_key=VALUES(sender_key)`
}

// whatsmeow_app_state_sync_keys

func (a *MySql) CreateTableStateSyncKeys() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_app_state_sync_keys (
        jid         VARCHAR(50),
        key_id      BINARY(6),
        key_data    BLOB NOT NULL,
        timestamp   BIGINT NOT NULL,
        fingerprint BLOB NOT NULL,
    
        PRIMARY KEY (jid, key_id),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutAppStateSyncKey() string {
	return `INSERT INTO whatsmeow_app_state_sync_keys (jid, key_id, key_data, timestamp, fingerprint)
    VALUES (?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
        key_data=IF(VALUES(timestamp) > timestamp, VALUES(key_data), key_data),
        timestamp=IF(VALUES(timestamp) > timestamp, VALUES(timestamp), timestamp),
        fingerprint=IF(VALUES(timestamp) > timestamp, VALUES(fingerprint), fingerprint)`
}

func (a *MySql) GetAppStateSyncKey() string {
	return "SELECT key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys WHERE jid=? AND key_id=?"
}

// whatsmeow_app_state_version

func (a *MySql) CreateTableStateVersion() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_app_state_version (
        jid     VARCHAR(50),
        name    VARCHAR(20),
        version BIGINT NOT NULL,
        hash    BINARY(128) NOT NULL,

        PRIMARY KEY (jid, name),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutAppStateVersion() string {
	return `INSERT INTO whatsmeow_app_state_version (jid, name, version, hash)
	VALUES (?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE version=VALUES(version), hash=VALUES(hash)`
}

func (a *MySql) GetAppStateVersion() string {
	return "SELECT version, hash FROM whatsmeow_app_state_version WHERE jid=? AND name=?"
}

func (a *MySql) DeleteAppStateVersion() string {
	return "DELETE FROM whatsmeow_app_state_version WHERE jid=? AND name=?"
}

// whatsmeow_app_state_mutation_macs

func (a *MySql) CreateTableStateMutationMacs() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_app_state_mutation_macs (
        jid       VARCHAR(50),
        name      VARCHAR(20),
        version   BIGINT,
        index_mac BINARY(32),
        value_mac BINARY(32) NOT NULL,
        PRIMARY KEY (jid, name, version, index_mac),
        FOREIGN KEY (jid, name) REFERENCES whatsmeow_app_state_version(jid, name) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutAppStateMutationMACs(placeholder string) string {
	return fmt.Sprintf("INSERT INTO whatsmeow_app_state_mutation_macs (jid, name, version, index_mac, value_mac) VALUES %s", placeholder)
}

func (a *MySql) DeleteAppStateMutationMACs(placeholder string) string {
	return fmt.Sprintf("DELETE FROM whatsmeow_app_state_mutation_macs WHERE jid=? AND name=? AND index_mac IN %s", placeholder)
}

func (a *MySql) GetAppStateMutationMAC() string {
	return "SELECT value_mac FROM whatsmeow_app_state_mutation_macs WHERE jid=? AND name=? AND index_mac=? ORDER BY version DESC LIMIT 1"
}

// whatsmeow_contacts

func (a *MySql) CreateTableContacts() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_contacts (
        our_jid       VARCHAR(50),
        their_jid     VARCHAR(50),
        first_name    TEXT,
        full_name     TEXT,
        push_name     TEXT,
        business_name TEXT,

        PRIMARY KEY (our_jid, their_jid),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutContactName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name) 
    VALUES (?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE first_name=VALUES(first_name), full_name=VALUES(full_name)`
}

func (a *MySql) PutManyContactNames(placeholder string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name)
    VALUES %s
    ON DUPLICATE KEY UPDATE first_name=VALUES(first_name), full_name=VALUES(full_name)`, placeholder)
}

func (a *MySql) PutPushName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, push_name) 
    VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE push_name=VALUES(push_name)`
}

func (a *MySql) PutBusinessName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, business_name)
	VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE business_name=VALUES(business_name)`
}

func (a *MySql) GetContact() string {
	return "SELECT first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=? AND their_jid=?"
}

func (a *MySql) GetAllContacts() string {
	return "SELECT their_jid, first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=?"
}

// whatsmeow_chat_settings

func (a *MySql) CreateTableChatSettings() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_chat_settings (
        our_jid       VARCHAR(50),
        chat_jid      VARCHAR(50),
        muted_until   BIGINT  NOT NULL DEFAULT 0,
        pinned        BOOLEAN NOT NULL DEFAULT false,
        archived      BOOLEAN NOT NULL DEFAULT false,

        PRIMARY KEY (our_jid, chat_jid),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *MySql) PutChatSetting(field string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_chat_settings (our_jid, chat_jid, %[1]s)
	VALUES (?, ?, ?)
    ON DUPLICATE KEY UPDATE %[1]s=VALUES(%[1]s)`, field)
}

func (a *MySql) GetChatSettings() string {
	return "SELECT muted_until, pinned, archived FROM whatsmeow_chat_settings WHERE our_jid=? AND chat_jid=?"
}

// whatsmeow_message_secrets

func (a *MySql) CreateTableMessageSecrets() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS whatsmeow_message_secrets (
        our_jid    VARCHAR(254),
        chat_jid   VARCHAR(50),
        sender_jid VARCHAR(50),
        message_id VARCHAR(50),
        %s        BLOB NOT NULL,

        PRIMARY KEY (our_jid, chat_jid, sender_jid, message_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`, "`key`")
}

func (a *MySql) PutMsgSecret() string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_message_secrets (our_jid, chat_jid, sender_jid, message_id, %[1]s)
    VALUES (?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE %[1]s=%[1]s`, "`key`")
}

func (a *MySql) GetMsgSecret() string {
	return "SELECT `key` FROM whatsmeow_message_secrets WHERE our_jid=? AND chat_jid=? AND sender_jid=? AND message_id=?"
}

// whatsmeow_privacy_tokens

func (a *MySql) CreateTablePrivacyTokens() string {
	return `CREATE TABLE IF NOT EXISTS whatsmeow_privacy_tokens (
        our_jid   VARCHAR(50),
        their_jid VARCHAR(50),
        token     VARBINARY(255)  NOT NULL,
        timestamp BIGINT NOT NULL,
        PRIMARY KEY (our_jid, their_jid)
    )`
}

func (a *MySql) PutPrivacyTokens(placeholder string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_privacy_tokens (our_jid, their_jid, token, timestamp)
    VALUES %s
    ON DUPLICATE KEY UPDATE token=VALUES(token), timestamp=VALUES(timestamp)`, placeholder)
}

func (a *MySql) GetPrivacyToken() string {
	return "SELECT token, timestamp FROM whatsmeow_privacy_tokens WHERE our_jid=? AND their_jid=?"
}

// helper

func (a *MySql) PlaceholderCreate(size, repeat int) string {
	var multipart []string
	for i := 0; i < repeat; i++ {
		var placeholders []string
		for i := 1; i < size+1; i++ {
			placeholders = append(placeholders, "?")
		}
		multipart = append(multipart, fmt.Sprintf("(%s)", strings.Join(placeholders, ",")))
	}
	return strings.Join(multipart, ", ")
}
