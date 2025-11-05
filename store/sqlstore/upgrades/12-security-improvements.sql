-- v12: Security and Performance Improvements for Multitenancy

-- Add indexes on business_id for efficient tenant filtering and query performance
CREATE INDEX IF NOT EXISTS idx_identity_keys_business ON whatsmeow_identity_keys(business_id);
CREATE INDEX IF NOT EXISTS idx_sessions_business ON whatsmeow_sessions(business_id);
CREATE INDEX IF NOT EXISTS idx_pre_keys_business ON whatsmeow_pre_keys(business_id);
CREATE INDEX IF NOT EXISTS idx_sender_keys_business ON whatsmeow_sender_keys(business_id);
CREATE INDEX IF NOT EXISTS idx_app_state_sync_keys_business ON whatsmeow_app_state_sync_keys(business_id);
CREATE INDEX IF NOT EXISTS idx_app_state_version_business ON whatsmeow_app_state_version(business_id);
CREATE INDEX IF NOT EXISTS idx_app_state_mutation_macs_business ON whatsmeow_app_state_mutation_macs(business_id);
CREATE INDEX IF NOT EXISTS idx_contacts_business ON whatsmeow_contacts(business_id);
CREATE INDEX IF NOT EXISTS idx_redacted_phones_business ON whatsmeow_redacted_phones(business_id);
CREATE INDEX IF NOT EXISTS idx_chat_settings_business ON whatsmeow_chat_settings(business_id);
CREATE INDEX IF NOT EXISTS idx_message_secrets_business ON whatsmeow_message_secrets(business_id);
CREATE INDEX IF NOT EXISTS idx_privacy_tokens_business ON whatsmeow_privacy_tokens(business_id);
CREATE INDEX IF NOT EXISTS idx_lid_map_business ON whatsmeow_lid_map(business_id);
CREATE INDEX IF NOT EXISTS idx_event_buffer_business ON whatsmeow_event_buffer(business_id);

-- Add composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_sessions_business_jid ON whatsmeow_sessions(business_id, our_jid);
CREATE INDEX IF NOT EXISTS idx_contacts_business_jid ON whatsmeow_contacts(business_id, our_jid);
CREATE INDEX IF NOT EXISTS idx_identity_keys_business_jid ON whatsmeow_identity_keys(business_id, our_jid);
