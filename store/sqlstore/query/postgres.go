package query

import (
	"fmt"
	"strings"
)

type Postgres struct{}

// whatsmeow_version

func (a *Postgres) CreateTableVersion() string {
	return "CREATE TABLE IF NOT EXISTS whatsmeow_version (version INTEGER)"
}

func (a *Postgres) GetVersion() string {
	return "SELECT version FROM whatsmeow_version LIMIT 1"
}

func (a *Postgres) DeleteAllVersions() string {
	return "DELETE FROM whatsmeow_version"
}

func (a *Postgres) InsertNewVersion() string {
	return "INSERT INTO whatsmeow_version (version) VALUES ($1)"
}

// whatsmeow_device

func (a *Postgres) CreateTableDevice() string {
	return `CREATE TABLE whatsmeow_device (
        jid TEXT PRIMARY KEY,

        registration_id BIGINT NOT NULL CHECK ( registration_id >= 0 AND registration_id < 4294967296 ),

        noise_key    bytea NOT NULL CHECK ( length(noise_key) = 32 ),
        identity_key bytea NOT NULL CHECK ( length(identity_key) = 32 ),

        signed_pre_key     bytea   NOT NULL CHECK ( length(signed_pre_key) = 32 ),
        signed_pre_key_id  INTEGER NOT NULL CHECK ( signed_pre_key_id >= 0 AND signed_pre_key_id < 16777216 ),
        signed_pre_key_sig bytea   NOT NULL CHECK ( length(signed_pre_key_sig) = 64 ),

        adv_key         bytea NOT NULL,
        adv_details     bytea NOT NULL,
        adv_account_sig bytea NOT NULL CHECK ( length(adv_account_sig) = 64 ),
        adv_device_sig  bytea NOT NULL CHECK ( length(adv_device_sig) = 64 ),

        platform      TEXT NOT NULL DEFAULT '',
        business_name TEXT NOT NULL DEFAULT '',
        push_name     TEXT NOT NULL DEFAULT ''
    )`
}

func (a *Postgres) AlterTableDevice_AddColumnSigKey() string {
	return "ALTER TABLE whatsmeow_device ADD COLUMN adv_account_sig_key bytea CHECK ( length(adv_account_sig_key) = 32 )"
}

func (a *Postgres) FillSigKey() string {
	return `UPDATE whatsmeow_device SET adv_account_sig_key=(
        SELECT identity
        FROM whatsmeow_identity_keys
        WHERE our_jid=whatsmeow_device.jid
          AND their_id=concat(split_part(whatsmeow_device.jid, '.', 1), ':0')
    )`
}

func (a *Postgres) DeleteNullSigKeys() string {
	return "DELETE FROM whatsmeow_device WHERE adv_account_sig_key IS NULL"
}

func (a *Postgres) AlterTableDevice_SetNotNull() string {
	return "ALTER TABLE whatsmeow_device ALTER COLUMN adv_account_sig_key SET NOT NULL"
}

func (a *Postgres) GetAllDevices() string {
	return `SELECT jid, registration_id, noise_key, identity_key,
           signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
           adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
           platform, business_name, push_name
    FROM whatsmeow_device`
}

func (a *Postgres) GetDevice() string {
	return fmt.Sprintf("%s %s", a.GetAllDevices(), "WHERE jid=$1")
}

func (a *Postgres) InsertDevice() string {
	return `INSERT INTO whatsmeow_device (jid, registration_id, noise_key, identity_key,
            signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
            adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
            platform, business_name, push_name)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    ON CONFLICT (jid) DO UPDATE
        SET platform=excluded.platform, business_name=excluded.business_name, push_name=excluded.push_name`
}

func (a *Postgres) DeleteDevice() string {
	return `DELETE FROM whatsmeow_device WHERE jid=$1`
}

// whatsmeow_identity_keys

func (a *Postgres) CreateTableIdentityKeys() string {
	return `CREATE TABLE whatsmeow_identity_keys (
        our_jid  TEXT,
        their_id TEXT,
        identity bytea NOT NULL CHECK ( length(identity) = 32 ),

        PRIMARY KEY (our_jid, their_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutIdentity() string {
	return `INSERT INTO whatsmeow_identity_keys (our_jid, their_id, identity) VALUES ($1, $2, $3)
    ON CONFLICT (our_jid, their_id) DO UPDATE SET identity=excluded.identity`
}

func (a *Postgres) DeleteAllIdentities() string {
	return "DELETE FROM whatsmeow_identity_keys WHERE our_jid=$1 AND their_id LIKE $2"
}

func (a *Postgres) DeleteIdentity() string {
	return "DELETE FROM whatsmeow_identity_keys WHERE our_jid=$1 AND their_id=$2"
}

func (a *Postgres) GetIdentity() string {
	return "SELECT identity FROM whatsmeow_identity_keys WHERE our_jid=$1 AND their_id=$2"
}

// whatsmeow_pre_keys

func (a *Postgres) CreateTablePreKeys() string {
	return `CREATE TABLE whatsmeow_pre_keys (
        jid      TEXT,
        key_id   INTEGER          CHECK ( key_id >= 0 AND key_id < 16777216 ),
        key      bytea   NOT NULL CHECK ( length(key) = 32 ),
        uploaded BOOLEAN NOT NULL,

        PRIMARY KEY (jid, key_id),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) GetLastPreKeyID() string {
	return "SELECT MAX(key_id) FROM whatsmeow_pre_keys WHERE jid=$1"
}

func (a *Postgres) InsertPreKey() string {
	return "INSERT INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4)"
}

func (a *Postgres) GetUnUploadedPreKeys() string {
	return "SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=$1 AND uploaded=false ORDER BY key_id LIMIT $2"
}

func (a *Postgres) GetPreKey() string {
	return "SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=$1 AND key_id=$2"
}

func (a *Postgres) DeletePreKey() string {
	return "DELETE FROM whatsmeow_pre_keys WHERE jid=$1 AND key_id=$2"
}

func (a *Postgres) MarkPreKeysAsUploaded() string {
	return "UPDATE whatsmeow_pre_keys SET uploaded=true WHERE jid=$1 AND key_id<=$2"
}

func (a *Postgres) GetUploadedPreKeyCount() string {
	return "SELECT COUNT(*) FROM whatsmeow_pre_keys WHERE jid=$1 AND uploaded=true"
}

// whatsmeow_sessions

func (a *Postgres) CreateTableSessions() string {
	return `CREATE TABLE whatsmeow_sessions (
        our_jid  TEXT,
        their_id TEXT,
        session  bytea,

        PRIMARY KEY (our_jid, their_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) GetSession() string {
	return "SELECT session FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id=$2"
}

func (a *Postgres) HasSession() string {
	return "SELECT true FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id=$2"
}

func (a *Postgres) PutSession() string {
	return `INSERT INTO whatsmeow_sessions (our_jid, their_id, session)
    VALUES ($1, $2, $3)
    ON CONFLICT (our_jid, their_id) DO UPDATE SET session=excluded.session`
}

func (a *Postgres) DeleteAllSessions() string {
	return "DELETE FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id LIKE $2"
}

func (a *Postgres) DeleteSession() string {
	return "DELETE FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id=$2"
}

// whatsmeow_sender_keys

func (a *Postgres) CreateTableSenderKeys() string {
	return `CREATE TABLE whatsmeow_sender_keys (
        our_jid    TEXT,
        chat_id    TEXT,
        sender_id  TEXT,
        sender_key bytea NOT NULL,

        PRIMARY KEY (our_jid, chat_id, sender_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) GetSenderKey() string {
	return "SELECT sender_key FROM whatsmeow_sender_keys WHERE our_jid=$1 AND chat_id=$2 AND sender_id=$3"
}

func (a *Postgres) PutSenderKey() string {
	return `INSERT INTO whatsmeow_sender_keys (our_jid, chat_id, sender_id, sender_key)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (our_jid, chat_id, sender_id) DO UPDATE
        SET sender_key=excluded.sender_key`
}

// whatsmeow_app_state_sync_keys

func (a *Postgres) CreateTableStateSyncKeys() string {
	return `CREATE TABLE whatsmeow_app_state_sync_keys (
        jid         TEXT,
        key_id      bytea,
        key_data    bytea  NOT NULL,
        timestamp   BIGINT NOT NULL,
        fingerprint bytea  NOT NULL,

        PRIMARY KEY (jid, key_id),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutAppStateSyncKey() string {
	return `INSERT INTO whatsmeow_app_state_sync_keys (jid, key_id, key_data, timestamp, fingerprint)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (jid, key_id) DO UPDATE
        SET key_data=excluded.key_data, timestamp=excluded.timestamp, fingerprint=excluded.fingerprint
        WHERE excluded.timestamp > whatsmeow_app_state_sync_keys.timestamp
`
}

func (a *Postgres) GetAppStateSyncKey() string {
	return "SELECT key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys WHERE jid=$1 AND key_id=$2"
}

// whatsmeow_app_state_version

func (a *Postgres) CreateTableStateVersion() string {
	return `CREATE TABLE whatsmeow_app_state_version (
        jid     TEXT,
        name    TEXT,
        version BIGINT NOT NULL,
        hash    bytea  NOT NULL CHECK ( length(hash) = 128 ),

        PRIMARY KEY (jid, name),
        FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutAppStateVersion() string {
	return `INSERT INTO whatsmeow_app_state_version (jid, name, version, hash)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (jid, name) DO UPDATE
        SET version=excluded.version, hash=excluded.hash`
}

func (a *Postgres) GetAppStateVersion() string {
	return "SELECT version, hash FROM whatsmeow_app_state_version WHERE jid=$1 AND name=$2"
}

func (a *Postgres) DeleteAppStateVersion() string {
	return "DELETE FROM whatsmeow_app_state_version WHERE jid=$1 AND name=$2"
}

// whatsmeow_app_state_mutation_macs

func (a *Postgres) CreateTableStateMutationMacs() string {
	return `CREATE TABLE whatsmeow_app_state_mutation_macs (
        jid       TEXT,
        name      TEXT,
        version   BIGINT,
        index_mac bytea          CHECK ( length(index_mac) = 32 ),
        value_mac bytea NOT NULL CHECK ( length(value_mac) = 32 ),

        PRIMARY KEY (jid, name, version, index_mac),
        FOREIGN KEY (jid, name) REFERENCES whatsmeow_app_state_version(jid, name) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutAppStateMutationMACs(placeholder string) string {
	return fmt.Sprintf("INSERT INTO whatsmeow_app_state_mutation_macs (jid, name, version, index_mac, value_mac) VALUES %s", placeholder)
}

func (a *Postgres) DeleteAppStateMutationMACs(_ string) string {
	return "DELETE FROM whatsmeow_app_state_mutation_macs WHERE jid=$1 AND name=$2 AND index_mac=ANY($3::bytea[])"
}

func (a *Postgres) GetAppStateMutationMAC() string {
	return "SELECT value_mac FROM whatsmeow_app_state_mutation_macs WHERE jid=$1 AND name=$2 AND index_mac=$3 ORDER BY version DESC LIMIT 1"
}

// whatsmeow_contacts

func (a *Postgres) CreateTableContacts() string {
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
}

func (a *Postgres) PutContactName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (our_jid, their_jid) DO UPDATE
        SET first_name=excluded.first_name, full_name=excluded.full_name`
}

func (a *Postgres) PutManyContactNames(placeholder string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_contacts (our_jid, their_jid, first_name, full_name)
    VALUES %s
    ON CONFLICT (our_jid, their_jid) DO UPDATE
        SET first_name=excluded.first_name, full_name=excluded.full_name`, placeholder)
}

func (a *Postgres) PutPushName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, push_name)
    VALUES ($1, $2, $3)
    ON CONFLICT (our_jid, their_jid) DO UPDATE
        SET push_name=excluded.push_name`
}

func (a *Postgres) PutBusinessName() string {
	return `INSERT INTO whatsmeow_contacts (our_jid, their_jid, business_name)
    VALUES ($1, $2, $3)
    ON CONFLICT (our_jid, their_jid) DO UPDATE
        SET business_name=excluded.business_name`
}

func (a *Postgres) GetContact() string {
	return "SELECT first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=$1 AND their_jid=$2"
}

func (a *Postgres) GetAllContacts() string {
	return "SELECT their_jid, first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE our_jid=$1"
}

// whatsmeow_chat_settings

func (a *Postgres) CreateTableChatSettings() string {
	return `CREATE TABLE whatsmeow_chat_settings (
        our_jid       TEXT,
        chat_jid      TEXT,
        muted_until   BIGINT  NOT NULL DEFAULT 0,
        pinned        BOOLEAN NOT NULL DEFAULT false,
        archived      BOOLEAN NOT NULL DEFAULT false,

        PRIMARY KEY (our_jid, chat_jid),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutChatSetting(field string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_chat_settings (our_jid, chat_jid, %[1]s)
    VALUES ($1, $2, $3)
    ON CONFLICT (our_jid, chat_jid) DO UPDATE
        SET %[1]s=excluded.%[1]s`, field)
}

func (a *Postgres) GetChatSettings() string {
	return "SELECT muted_until, pinned, archived FROM whatsmeow_chat_settings WHERE our_jid=$1 AND chat_jid=$2"
}

// whatsmeow_message_secrets

func (a *Postgres) CreateTableMessageSecrets() string {
	return `CREATE TABLE whatsmeow_message_secrets (
        our_jid    TEXT,
        chat_jid   TEXT,
        sender_jid TEXT,
        message_id TEXT,
        key        bytea NOT NULL,

        PRIMARY KEY (our_jid, chat_jid, sender_jid, message_id),
        FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
    )`
}

func (a *Postgres) PutMsgSecret() string {
	return `INSERT INTO whatsmeow_message_secrets (our_jid, chat_jid, sender_jid, message_id, key)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (our_jid, chat_jid, sender_jid, message_id) DO NOTHING`
}

func (a *Postgres) GetMsgSecret() string {
	return "SELECT key FROM whatsmeow_message_secrets WHERE our_jid=$1 AND chat_jid=$2 AND sender_jid=$3 AND message_id=$4"
}

// whatsmeow_privacy_tokens

func (a *Postgres) CreateTablePrivacyTokens() string {
	return `CREATE TABLE whatsmeow_privacy_tokens (
        our_jid   TEXT,
        their_jid TEXT,
        token     bytea  NOT NULL,
        timestamp BIGINT NOT NULL,
        PRIMARY KEY (our_jid, their_jid)
    )`
}

func (a *Postgres) PutPrivacyTokens(placeholders string) string {
	return fmt.Sprintf(`INSERT INTO whatsmeow_privacy_tokens (our_jid, their_jid, token, timestamp)
    VALUES %s
    ON CONFLICT (our_jid, their_jid) DO UPDATE
        SET token=EXCLUDED.token, timestamp=EXCLUDED.timestamp`, placeholders)
}

func (a *Postgres) GetPrivacyToken() string {
	return "SELECT token, timestamp FROM whatsmeow_privacy_tokens WHERE our_jid=$1 AND their_jid=$2"
}

// helper

func (a *Postgres) PlaceholderCreate(size, repeat int) string {
	var multipart []string
	for i := 0; i < repeat; i++ {
		var placeholders []string
		for j := 1; j < size+1; j++ {
			placeholders = append(placeholders, fmt.Sprintf("$%d", (i*size)+j))
		}
		multipart = append(multipart, fmt.Sprintf("(%s)", strings.Join(placeholders, ",")))
	}
	return strings.Join(multipart, ", ")
}
