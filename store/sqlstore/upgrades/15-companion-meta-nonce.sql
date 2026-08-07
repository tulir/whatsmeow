-- v15 (compatible with v8+): Add companion meta nonce column to device table
ALTER TABLE whatsmeow_device ADD COLUMN companion_meta_nonce TEXT NOT NULL DEFAULT '';
