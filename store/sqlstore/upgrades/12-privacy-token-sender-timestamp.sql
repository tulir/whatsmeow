-- v12: Add sender timestamp column for privacy tokens
ALTER TABLE whatsmeow_privacy_tokens ADD COLUMN sender_timestamp BIGINT;
