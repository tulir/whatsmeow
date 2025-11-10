-- v10 (compatible with v8+): Add lid migration timestamp to device table
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS lid_migration_ts BIGINT NOT NULL DEFAULT 0;
