-- v0 -> v11 (compatible with v8+): Latest schema
CREATE TABLE whatsmeow_device (
  	business_id TEXT NOT NULL,
	jid TEXT PRIMARY KEY,
	lid TEXT,

	facebook_uuid uuid,

	registration_id BIGINT NOT NULL CHECK ( registration_id >= 0 AND registration_id < 4294967296 ),

	noise_key    bytea NOT NULL CHECK ( length(noise_key) = 32 ),
	identity_key bytea NOT NULL CHECK ( length(identity_key) = 32 ),

	signed_pre_key     bytea   NOT NULL CHECK ( length(signed_pre_key) = 32 ),
	signed_pre_key_id  INTEGER NOT NULL CHECK ( signed_pre_key_id >= 0 AND signed_pre_key_id < 16777216 ),
	signed_pre_key_sig bytea   NOT NULL CHECK ( length(signed_pre_key_sig) = 64 ),

	adv_key             bytea NOT NULL,
	adv_details         bytea NOT NULL,
	adv_account_sig     bytea NOT NULL CHECK ( length(adv_account_sig) = 64 ),
	adv_account_sig_key bytea NOT NULL CHECK ( length(adv_account_sig_key) = 32 ),
	adv_device_sig      bytea NOT NULL CHECK ( length(adv_device_sig) = 64 ),

	platform      TEXT NOT NULL DEFAULT '',
	business_name TEXT NOT NULL DEFAULT '',
	push_name     TEXT NOT NULL DEFAULT '',

	lid_migration_ts BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE whatsmeow_identity_keys (
    business_id TEXT NOT NULL,
	our_jid  TEXT,
	their_id TEXT,
	identity bytea NOT NULL CHECK ( length(identity) = 32 ),

	PRIMARY KEY (business_id, our_jid, their_id),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_pre_keys (
	business_id TEXT NOT NULL,
	jid      TEXT,
	key_id   INTEGER          CHECK ( key_id >= 0 AND key_id < 16777216 ),
	key      bytea   NOT NULL CHECK ( length(key) = 32 ),
	uploaded BOOLEAN NOT NULL,

	PRIMARY KEY (business_id, jid, key_id),
	FOREIGN KEY (business_id, jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_sessions (
	business_id TEXT NOT NULL,
	our_jid  TEXT,
	their_id TEXT,
	session  bytea,

	PRIMARY KEY (business_id, our_jid, their_id),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_sender_keys (
    business_id TEXT NOT NULL,
	our_jid    TEXT,
	chat_id    TEXT,
	sender_id  TEXT,
	sender_key bytea NOT NULL,

	PRIMARY KEY (business_id, our_jid, chat_id, sender_id),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_app_state_sync_keys (
	business_id TEXT NOT NULL,
	jid         TEXT,
	key_id      bytea,
	key_data    bytea  NOT NULL,
	timestamp   BIGINT NOT NULL,
	fingerprint bytea  NOT NULL,

	PRIMARY KEY (business_id, jid, key_id),
	FOREIGN KEY (business_id, jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_app_state_version (
	business_id TEXT NOT NULL,
	jid     TEXT,
	name    TEXT,
	version BIGINT NOT NULL,
	hash    bytea  NOT NULL CHECK ( length(hash) = 128 ),

	PRIMARY KEY (business_id, jid, name),
	FOREIGN KEY (business_id, jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_app_state_mutation_macs (
	business_id TEXT NOT NULL,
	jid       TEXT,
	name      TEXT,
	version   BIGINT,
	index_mac bytea          CHECK ( length(index_mac) = 32 ),
	value_mac bytea NOT NULL CHECK ( length(value_mac) = 32 ),

	PRIMARY KEY (business_id, jid, name, version, index_mac),
	FOREIGN KEY (business_id, jid, name) REFERENCES whatsmeow_app_state_version(business_id, jid, name) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_contacts (
	business_id TEXT NOT NULL,
	our_jid        TEXT,
	their_jid      TEXT,
	first_name     TEXT,
	full_name      TEXT,
	push_name      TEXT,
	business_name  TEXT,
	redacted_phone TEXT,

	PRIMARY KEY (business_id, our_jid, their_jid),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_chat_settings (
	business_id TEXT NOT NULL,
	our_jid       TEXT,
	chat_jid      TEXT,
	muted_until   BIGINT  NOT NULL DEFAULT 0,
	pinned        BOOLEAN NOT NULL DEFAULT false,
	archived      BOOLEAN NOT NULL DEFAULT false,

	PRIMARY KEY (business_id, our_jid, chat_jid),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_message_secrets (
	business_id TEXT NOT NULL,
	our_jid    TEXT,
	chat_jid   TEXT,
	sender_jid TEXT,
	message_id TEXT,
	key        bytea NOT NULL,

	PRIMARY KEY (business_id, our_jid, chat_jid, sender_jid, message_id),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE whatsmeow_privacy_tokens (
	business_id TEXT NOT NULL,
	our_jid   TEXT,
	their_jid TEXT,
	token     bytea  NOT NULL,
	timestamp BIGINT NOT NULL,
	PRIMARY KEY (business_id, our_jid, their_jid)
);

CREATE TABLE whatsmeow_lid_map (
	business_id TEXT NOT NULL,
	lid TEXT PRIMARY KEY,
	pn  TEXT UNIQUE NOT NULL
);

CREATE TABLE whatsmeow_event_buffer (
	business_id TEXT NOT NULL,
	our_jid          TEXT   NOT NULL,
	ciphertext_hash  bytea  NOT NULL CHECK ( length(ciphertext_hash) = 32 ),
	plaintext        bytea,
	server_timestamp BIGINT NOT NULL,
	insert_timestamp BIGINT NOT NULL,
	PRIMARY KEY (business_id, our_jid, ciphertext_hash),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);
