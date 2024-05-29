package waServerSync

// Deprecated: Use GetKeyID
func (x *SyncdRecord) GetKeyId() *KeyId {
	return x.GetKeyID()
}

// Deprecated: Use GetKeyID
func (x *SyncdSnapshot) GetKeyId() *KeyId {
	return x.GetKeyID()
}

// Deprecated: Use GetKeyID
func (x *SyncdPatch) GetKeyId() *KeyId {
	return x.GetKeyID()
}

// Deprecated: Use GetSnapshotMAC
func (x *SyncdPatch) GetSnapshotMac() []byte {
	return x.GetSnapshotMAC()
}

// Deprecated: Use GetPatchMAC
func (x *SyncdPatch) GetPatchMac() []byte {
	return x.GetPatchMAC()
}

// Deprecated: Use GetID
func (x *KeyId) GetId() []byte {
	return x.GetID()
}
