-- v3: Add message secrets table
CREATE TABLE whatsmeow_message_secrets
(
	our_jid    TEXT,
	chat_jid   TEXT,
	sender_jid TEXT,
	message_id TEXT,
	key        bytea NOT NULL,
	PRIMARY KEY (our_jid, chat_jid, sender_jid, message_id),
	FOREIGN KEY (our_jid) REFERENCES whatsmeow_device (jid) ON DELETE CASCADE ON UPDATE CASCADE
);

alter table whatsmeow_message_secrets
	add column created_at INT NOT NULL DEFAULT 0;

CREATE INDEX idx_message_secrets_created_at ON whatsmeow_message_secrets (created_at);
