package query

import (
	"fmt"
	"strings"
)

type Sqlite struct {
	Default Adapter
}

// whatsmeow_version

func (a *Sqlite) CreateTableVersion() string {
	return a.Default.CreateTableVersion()
}

func (a *Sqlite) GetVersion() string {
	return a.Default.GetVersion()
}

func (a *Sqlite) DeleteAllVersions() string {
	return a.Default.DeleteAllVersions()
}

func (a *Sqlite) InsertNewVersion() string {
	return a.Default.InsertNewVersion()
}

// whatsmeow_device

func (a *Sqlite) CreateTableDevice() string {
	return a.Default.CreateTableDevice()
}

func (a *Sqlite) AlterTableDevice_AddColumnSigKey() string {
	return a.Default.AlterTableDevice_AddColumnSigKey()
}

func (a *Sqlite) FillSigKey() string {
	return `UPDATE whatsmeow_device SET adv_account_sig_key=(
        SELECT identity
        FROM whatsmeow_identity_keys
        WHERE our_jid=whatsmeow_device.jid
            AND their_id=substr(whatsmeow_device.jid, 0, instr(whatsmeow_device.jid, '.')) || ':0'
    )`
}

func (a *Sqlite) DeleteNullSigKeys() string {
	return a.Default.DeleteNullSigKeys()
}

func (a *Sqlite) AlterTableDevice_SetNotNull() string {
	return ""
}

func (a *Sqlite) GetAllDevices() string {
	return a.Default.GetAllDevices()
}

func (a *Sqlite) GetDevice() string {
	return a.Default.GetDevice()
}

func (a *Sqlite) InsertDevice() string {
	return a.Default.InsertDevice()
}

func (a *Sqlite) DeleteDevice() string {
	return a.Default.DeleteDevice()
}

// whatsmeow_identity_keys

func (a *Sqlite) CreateTableIdentityKeys() string {
	return a.Default.CreateTableIdentityKeys()
}

func (a *Sqlite) PutIdentity() string {
	return a.Default.PutIdentity()
}

func (a *Sqlite) DeleteAllIdentities() string {
	return a.Default.DeleteAllIdentities()
}

func (a *Sqlite) DeleteIdentity() string {
	return a.Default.DeleteIdentity()
}

func (a *Sqlite) GetIdentity() string {
	return a.Default.GetIdentity()
}

// whatsmeow_pre_keys

func (a *Sqlite) CreateTablePreKeys() string {
	return a.Default.CreateTablePreKeys()
}

func (a *Sqlite) GetLastPreKeyID() string {
	return a.Default.GetLastPreKeyID()
}

func (a *Sqlite) InsertPreKey() string {
	return a.Default.InsertPreKey()
}

func (a *Sqlite) GetUnUploadedPreKeys() string {
	return a.Default.GetUnUploadedPreKeys()
}

func (a *Sqlite) GetPreKey() string {
	return a.Default.GetPreKey()
}

func (a *Sqlite) DeletePreKey() string {
	return a.Default.DeletePreKey()
}

func (a *Sqlite) MarkPreKeysAsUploaded() string {
	return a.Default.MarkPreKeysAsUploaded()
}

func (a *Sqlite) GetUploadedPreKeyCount() string {
	return a.Default.GetUploadedPreKeyCount()
}

// whatsmeow_sessions

func (a *Sqlite) CreateTableSessions() string {
	return a.Default.CreateTableSessions()
}

func (a *Sqlite) GetSession() string {
	return a.Default.GetSession()
}

func (a *Sqlite) HasSession() string {
	return a.Default.HasSession()
}

func (a *Sqlite) PutSession() string {
	return a.Default.PutSession()
}

func (a *Sqlite) DeleteAllSessions() string {
	return a.Default.DeleteAllSessions()
}

func (a *Sqlite) DeleteSession() string {
	return a.Default.DeleteSession()
}

// whatsmeow_sender_keys

func (a *Sqlite) CreateTableSenderKeys() string {
	return a.Default.CreateTableSenderKeys()
}

func (a *Sqlite) GetSenderKey() string {
	return a.Default.GetSenderKey()
}

func (a *Sqlite) PutSenderKey() string {
	return a.Default.PutSenderKey()
}

// whatsmeow_app_state_sync_keys

func (a *Sqlite) CreateTableStateSyncKeys() string {
	return a.Default.CreateTableStateSyncKeys()
}

func (a *Sqlite) PutAppStateSyncKey() string {
	return a.Default.PutAppStateSyncKey()
}

func (a *Sqlite) GetAppStateSyncKey() string {
	return a.Default.GetAppStateSyncKey()
}

// whatsmeow_app_state_version

func (a *Sqlite) CreateTableStateVersion() string {
	return a.Default.CreateTableStateVersion()
}

func (a *Sqlite) PutAppStateVersion() string {
	return a.Default.PutAppStateVersion()
}

func (a *Sqlite) GetAppStateVersion() string {
	return a.Default.GetAppStateVersion()
}

func (a *Sqlite) DeleteAppStateVersion() string {
	return a.Default.DeleteAppStateVersion()
}

// whatsmeow_app_state_mutation_macs

func (a *Sqlite) CreateTableStateMutationMacs() string {
	return a.Default.CreateTableStateMutationMacs()
}

func (a *Sqlite) PutAppStateMutationMACs(placeholder string) string {
	return a.Default.PutAppStateMutationMACs(placeholder)
}

func (a *Sqlite) DeleteAppStateMutationMACs(placeholder string) string {
	return fmt.Sprintf("DELETE FROM whatsmeow_app_state_mutation_macs WHERE jid=$1 AND name=$2 AND index_mac IN %s", placeholder)
}

func (a *Sqlite) GetAppStateMutationMAC() string {
	return a.Default.GetAppStateMutationMAC()
}

// whatsmeow_contacts

func (a *Sqlite) CreateTableContacts() string {
	return a.Default.CreateTableContacts()
}

func (a *Sqlite) PutContactName() string {
	return a.Default.PutContactName()
}

func (a *Sqlite) PutManyContactNames(placeholder string) string {
	return a.Default.PutManyContactNames(placeholder)
}

func (a *Sqlite) PutPushName() string {
	return a.Default.PutPushName()
}

func (a *Sqlite) PutBusinessName() string {
	return a.Default.PutBusinessName()
}

func (a *Sqlite) GetContact() string {
	return a.Default.GetContact()
}

func (a *Sqlite) GetAllContacts() string {
	return a.Default.GetAllContacts()
}

// whatsmeow_chat_settings

func (a *Sqlite) CreateTableChatSettings() string {
	return a.Default.CreateTableChatSettings()
}

func (a *Sqlite) PutChatSetting(field string) string {
	return a.Default.PutChatSetting(field)
}

func (a *Sqlite) GetChatSettings() string {
	return a.Default.GetChatSettings()
}

// whatsmeow_message_secrets

func (a *Sqlite) CreateTableMessageSecrets() string {
	return a.Default.CreateTableMessageSecrets()
}

func (a *Sqlite) PutMsgSecret() string {
	return a.Default.PutMsgSecret()
}

func (a *Sqlite) GetMsgSecret() string {
	return a.Default.GetMsgSecret()
}

// whatsmeow_privacy_tokens

func (a *Sqlite) CreateTablePrivacyTokens() string {
	return a.Default.CreateTablePrivacyTokens()
}

func (a *Sqlite) PutPrivacyTokens(placeholder string) string {
	return a.Default.PutPrivacyTokens(placeholder)
}

func (a *Sqlite) GetPrivacyToken() string {
	return a.Default.GetPrivacyToken()
}

// helper

func (a *Sqlite) PlaceholderCreate(size, repeat int) string {
	var multipart []string
	for i := 0; i < repeat; i++ {
		var placeholders []string
		for j := 1; j < size+1; j++ {
			placeholders = append(placeholders, fmt.Sprintf("?%d", (i*size)+j))
		}
		multipart = append(multipart, fmt.Sprintf("(%s)", strings.Join(placeholders, ",")))
	}
	return strings.Join(multipart, ", ")
}
