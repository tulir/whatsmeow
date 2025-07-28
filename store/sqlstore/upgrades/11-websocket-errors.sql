-- v11: Add websocket errors table
CREATE TABLE whatsmeow_websocket_errors (
	id           SERIAL PRIMARY KEY,
	client_jid   TEXT NOT NULL,
	error_msg    TEXT NOT NULL,
	timestamp    BIGINT NOT NULL,
	created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_whatsmeow_websocket_errors_client_jid ON whatsmeow_websocket_errors(client_jid);
CREATE INDEX idx_whatsmeow_websocket_errors_timestamp ON whatsmeow_websocket_errors(timestamp);
