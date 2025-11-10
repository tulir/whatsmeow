-- v6: Add facebook_uuid column to device table
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS facebook_uuid uuid;
