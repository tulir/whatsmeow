-- v3: Add message secrets table
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
