-- v8 (compatible with v8+): Add tables for LID<->JID mapping
CREATE TABLE whatsmeow_lid_map (
	lid TEXT PRIMARY KEY,
	pn  TEXT UNIQUE NOT NULL
);
