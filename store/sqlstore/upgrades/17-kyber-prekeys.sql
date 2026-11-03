-- v17 (compatible with v8+): Add ML-KEM-1024 prekeys for post-quantum sessions
CREATE TABLE whatsmeow_kyber_pre_keys (
	jid         TEXT,
	key_id      INTEGER CHECK ( key_id >= 0 AND key_id < 16777216 ),
	seed        bytea   NOT NULL CHECK ( length(seed) = 64 ),
	signature   bytea   NOT NULL CHECK ( length(signature) = 64 ),
	timestamp   BIGINT  NOT NULL,
	last_resort BOOLEAN NOT NULL,
	uploaded    BOOLEAN NOT NULL,

	PRIMARY KEY (jid, key_id),
	FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
);
