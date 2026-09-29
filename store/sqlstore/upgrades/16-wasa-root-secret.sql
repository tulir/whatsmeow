-- v16 (compatible with v8+): Remember the active WASA root secret per bot
ALTER TABLE whatsmeow_chat_settings ADD COLUMN wasa_root_secret_id TEXT NOT NULL DEFAULT '';
