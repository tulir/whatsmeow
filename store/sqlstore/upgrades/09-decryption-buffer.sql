-- v9 (compatible with v8+): Add decrypted event buffer
CREATE TABLE IF NOT EXISTS whatsmeow_event_buffer (
	business_id TEXT NOT NULL,
	our_jid          TEXT   NOT NULL,
	ciphertext_hash  bytea  NOT NULL CHECK ( length(ciphertext_hash) = 32 ),
	plaintext        bytea,
	server_timestamp BIGINT NOT NULL,
	insert_timestamp BIGINT NOT NULL,
	PRIMARY KEY (business_id, our_jid, ciphertext_hash),
	FOREIGN KEY (business_id, our_jid) REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE
);
