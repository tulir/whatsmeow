-- v11 (compatible with v8+): Store redacted phone number for LID members in groups
ALTER TABLE whatsmeow_contacts ADD COLUMN IF NOT EXISTS redacted_phone TEXT;
