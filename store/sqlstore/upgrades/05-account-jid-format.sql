-- v5: Update account JID format
UPDATE whatsmeow_device SET jid=REPLACE(jid, '.0', '');
