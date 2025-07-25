-- v4: Add privacy tokens table
CREATE TABLE whatsmeow_privacy_tokens (
	our_jid   TEXT,
	their_jid TEXT,
	token     bytea  NOT NULL,
	timestamp BIGINT NOT NULL,
	PRIMARY KEY (our_jid, their_jid)
);
