-- v16 (compatible with v8+): Only keep the latest MAC for each app state mutation index
-- transaction: sqlite-fkey-off
-- only: postgres until "end only"
DELETE FROM whatsmeow_app_state_mutation_macs AS old
USING whatsmeow_app_state_mutation_macs AS new
WHERE old.jid = new.jid
  AND old.name = new.name
  AND old.index_mac = new.index_mac
  AND old.version < new.version;
ALTER TABLE whatsmeow_app_state_mutation_macs DROP CONSTRAINT whatsmeow_app_state_mutation_macs_pkey;
ALTER TABLE whatsmeow_app_state_mutation_macs ADD PRIMARY KEY (jid, name, index_mac);
-- end only postgres
-- only: sqlite until "end only"
CREATE TABLE whatsmeow_app_state_mutation_macs_v16 (
	jid       TEXT,
	name      TEXT,
	version   BIGINT,
	index_mac bytea          CHECK ( length(index_mac) = 32 ),
	value_mac bytea NOT NULL CHECK ( length(value_mac) = 32 ),

	PRIMARY KEY (jid, name, index_mac),
	FOREIGN KEY (jid, name) REFERENCES whatsmeow_app_state_version(jid, name) ON DELETE CASCADE ON UPDATE CASCADE
);
INSERT INTO whatsmeow_app_state_mutation_macs_v16 (jid, name, version, index_mac, value_mac)
SELECT jid, name, version, index_mac, value_mac
FROM (
	SELECT *, ROW_NUMBER() OVER (PARTITION BY jid, name, index_mac ORDER BY version DESC) AS rn
	FROM whatsmeow_app_state_mutation_macs
) WHERE rn = 1;
DROP TABLE whatsmeow_app_state_mutation_macs;
ALTER TABLE whatsmeow_app_state_mutation_macs_v16 RENAME TO whatsmeow_app_state_mutation_macs;
-- end only sqlite
