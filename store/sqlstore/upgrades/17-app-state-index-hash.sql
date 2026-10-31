-- v17 (compatible with v8+): Remember the logical index of app state mutation MACs
ALTER TABLE whatsmeow_app_state_mutation_macs ADD COLUMN index_hash bytea;
CREATE INDEX whatsmeow_app_state_mutation_macs_index_hash_idx ON whatsmeow_app_state_mutation_macs (jid, name, index_hash);
