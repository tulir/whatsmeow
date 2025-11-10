-- v7 (compatible with v6+): Add lid column to device table
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS lid TEXT;
