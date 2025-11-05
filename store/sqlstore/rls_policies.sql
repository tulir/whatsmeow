-- Row-Level Security (RLS) Policies for Defense-in-Depth Multitenancy
--
-- This file contains PostgreSQL Row-Level Security policies that provide
-- an additional layer of protection against cross-tenant data access.
--
-- IMPORTANT: These policies are OPTIONAL but RECOMMENDED for production deployments.
-- They require PostgreSQL and the application to set session variables.
--
-- Usage:
-- 1. Apply these policies to your database
-- 2. Set the session variable before queries:
--    SET app.current_business_id = 'your-business-id';
-- 3. The database will enforce tenant isolation at the row level

-- Enable RLS on all tables
ALTER TABLE whatsmeow_device ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_identity_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_pre_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_sender_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_sync_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_version ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_mutation_macs ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_redacted_phones ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_chat_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_message_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_privacy_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_lid_map ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_event_buffer ENABLE ROW LEVEL SECURITY;

-- Create policies for whatsmeow_device
CREATE POLICY tenant_isolation_device ON whatsmeow_device
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_device_insert ON whatsmeow_device
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_identity_keys
CREATE POLICY tenant_isolation_identity_keys ON whatsmeow_identity_keys
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_identity_keys_insert ON whatsmeow_identity_keys
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_pre_keys
CREATE POLICY tenant_isolation_pre_keys ON whatsmeow_pre_keys
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_pre_keys_insert ON whatsmeow_pre_keys
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_sessions
CREATE POLICY tenant_isolation_sessions ON whatsmeow_sessions
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_sessions_insert ON whatsmeow_sessions
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_sender_keys
CREATE POLICY tenant_isolation_sender_keys ON whatsmeow_sender_keys
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_sender_keys_insert ON whatsmeow_sender_keys
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_app_state_sync_keys
CREATE POLICY tenant_isolation_app_state_sync_keys ON whatsmeow_app_state_sync_keys
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_app_state_sync_keys_insert ON whatsmeow_app_state_sync_keys
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_app_state_version
CREATE POLICY tenant_isolation_app_state_version ON whatsmeow_app_state_version
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_app_state_version_insert ON whatsmeow_app_state_version
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_app_state_mutation_macs
CREATE POLICY tenant_isolation_app_state_mutation_macs ON whatsmeow_app_state_mutation_macs
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_app_state_mutation_macs_insert ON whatsmeow_app_state_mutation_macs
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_contacts
CREATE POLICY tenant_isolation_contacts ON whatsmeow_contacts
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_contacts_insert ON whatsmeow_contacts
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_redacted_phones
CREATE POLICY tenant_isolation_redacted_phones ON whatsmeow_redacted_phones
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_redacted_phones_insert ON whatsmeow_redacted_phones
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_chat_settings
CREATE POLICY tenant_isolation_chat_settings ON whatsmeow_chat_settings
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_chat_settings_insert ON whatsmeow_chat_settings
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_message_secrets
CREATE POLICY tenant_isolation_message_secrets ON whatsmeow_message_secrets
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_message_secrets_insert ON whatsmeow_message_secrets
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_privacy_tokens
CREATE POLICY tenant_isolation_privacy_tokens ON whatsmeow_privacy_tokens
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_privacy_tokens_insert ON whatsmeow_privacy_tokens
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_lid_map
CREATE POLICY tenant_isolation_lid_map ON whatsmeow_lid_map
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_lid_map_insert ON whatsmeow_lid_map
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Create policies for whatsmeow_event_buffer
CREATE POLICY tenant_isolation_event_buffer ON whatsmeow_event_buffer
    USING (business_id = current_setting('app.current_business_id', true));

CREATE POLICY tenant_isolation_event_buffer_insert ON whatsmeow_event_buffer
    FOR INSERT
    WITH CHECK (business_id = current_setting('app.current_business_id', true));

-- Grant bypass to superuser (for maintenance operations)
-- Note: In production, you might want to create a specific maintenance role
ALTER TABLE whatsmeow_device FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_identity_keys FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_pre_keys FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_sender_keys FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_sync_keys FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_version FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_app_state_mutation_macs FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_contacts FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_redacted_phones FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_chat_settings FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_message_secrets FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_privacy_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_lid_map FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsmeow_event_buffer FORCE ROW LEVEL SECURITY;

-- To disable RLS (for testing or rollback), run:
-- ALTER TABLE whatsmeow_device DISABLE ROW LEVEL SECURITY;
-- (repeat for all tables)

-- To drop all policies (for rollback), run:
-- DROP POLICY IF EXISTS tenant_isolation_device ON whatsmeow_device;
-- (repeat for all tables and their INSERT policies)
