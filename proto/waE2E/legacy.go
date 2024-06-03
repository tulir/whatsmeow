package waE2E

// Deprecated: Use GetKeyID
func (x *AppStateSyncKey) GetKeyId() *AppStateSyncKeyId {
	return x.GetKeyID()
}

// Deprecated: Use GetKeyID
func (x *AppStateSyncKeyId) GetKeyId() []byte {
	return x.GetKeyID()
}

// Deprecated: Use GetStanzaID
func (x *PeerDataOperationRequestResponseMessage) GetStanzaId() string {
	return x.GetStanzaID()
}

// Deprecated: Use GetMentionedJID
func (x *ContextInfo) GetMentionedJid() []string {
	return x.GetMentionedJID()
}

// Deprecated: Use GetRemoteJID
func (x *ContextInfo) GetRemoteJid() string {
	return x.GetRemoteJID()
}

// Deprecated: Use GetStanzaID
func (x *ContextInfo) GetStanzaId() string {
	return x.GetStanzaID()
}
