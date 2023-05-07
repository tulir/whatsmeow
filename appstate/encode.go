package appstate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/cbcutil"
)

// MutationInfo contains information about a single mutation to the app state.
type MutationInfo struct {
	// Index contains the thing being mutated (like `mute` or `pin_v1`), followed by parameters like the target JID.
	Index []string
	// Version is a static number that depends on the thing being mutated.
	Version int32
	// Value contains the data for the mutation.
	Value *waProto.SyncActionValue
}

// PatchInfo contains information about a patch to the app state.
// A patch can contain multiple mutations, as long as all mutations are in the same app state type.
type PatchInfo struct {
	// Timestamp is the time when the patch was created. This will be filled automatically in EncodePatch if it's zero.
	Timestamp time.Time
	// Type is the app state type being mutated.
	Type WAPatchName
	// Mutations contains the individual mutations to apply to the app state in this patch.
	Mutations []MutationInfo
}

// BuildMute builds an app state patch for muting or unmuting a chat.
//
// If mute is true and the mute duration is zero, the chat is muted forever.
func BuildMute(target types.JID, mute bool, muteDuration time.Duration) PatchInfo {
	var muteEndTimestamp *int64
	if muteDuration > 0 {
		muteEndTimestamp = proto.Int64(time.Now().Add(muteDuration).UnixMilli())
	}

	return PatchInfo{
		Type: WAPatchRegularHigh,
		Mutations: []MutationInfo{{
			Index:   []string{IndexMute, target.String()},
			Version: 2,
			Value: &waProto.SyncActionValue{
				MuteAction: &waProto.MuteAction{
					Muted:            proto.Bool(mute),
					MuteEndTimestamp: muteEndTimestamp,
				},
			},
		}},
	}
}

func newPinMutationInfo(target types.JID, pin bool) MutationInfo {
	return MutationInfo{
		Index:   []string{IndexPin, target.String()},
		Version: 5,
		Value: &waProto.SyncActionValue{
			PinAction: &waProto.PinAction{
				Pinned: &pin,
			},
		},
	}
}

// BuildPin builds an app state patch for pinning or unpinning a chat.
func BuildPin(target types.JID, pin bool) PatchInfo {
	return PatchInfo{
		Type: WAPatchRegularLow,
		Mutations: []MutationInfo{
			newPinMutationInfo(target, pin),
		},
	}
}

// BuildArchive builds an app state patch for archiving or unarchiving a chat.
//
// The last message timestamp and last message key are optional and can be set to zero values (`time.Time{}` and `nil`).
//
// Archiving a chat will also unpin it automatically.
func BuildArchive(target types.JID, archive bool, lastMessageTimestamp time.Time, lastMessageKey *waProto.MessageKey) PatchInfo {
	if lastMessageTimestamp.IsZero() {
		lastMessageTimestamp = time.Now()
	}
	archiveMutationInfo := MutationInfo{
		Index:   []string{IndexArchive, target.String()},
		Version: 3,
		Value: &waProto.SyncActionValue{
			ArchiveChatAction: &waProto.ArchiveChatAction{
				Archived: &archive,
				MessageRange: &waProto.SyncActionMessageRange{
					LastMessageTimestamp: proto.Int64(lastMessageTimestamp.Unix()),
					// TODO set LastSystemMessageTimestamp?
				},
			},
		},
	}

	if lastMessageKey != nil {
		archiveMutationInfo.Value.ArchiveChatAction.MessageRange.Messages = []*waProto.SyncActionMessage{{
			Key:       lastMessageKey,
			Timestamp: proto.Int64(lastMessageTimestamp.Unix()),
		}}
	}

	mutations := []MutationInfo{archiveMutationInfo}
	if archive {
		mutations = append(mutations, newPinMutationInfo(target, false))
	}

	result := PatchInfo{
		Type:      WAPatchRegularLow,
		Mutations: mutations,
	}

	return result
}

func (proc *Processor) EncodePatch(keyID []byte, state HashState, patchInfo PatchInfo) ([]byte, error) {
	keys, err := proc.getAppStateKey(keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app state key details with key ID %x: %w", keyID, err)
	}

	if patchInfo.Timestamp.IsZero() {
		patchInfo.Timestamp = time.Now()
	}

	mutations := make([]*waProto.SyncdMutation, 0, len(patchInfo.Mutations))
	for _, mutationInfo := range patchInfo.Mutations {
		mutationInfo.Value.Timestamp = proto.Int64(patchInfo.Timestamp.UnixMilli())

		indexBytes, err := json.Marshal(mutationInfo.Index)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mutation index: %w", err)
		}

		pbObj := &waProto.SyncActionData{
			Index:   indexBytes,
			Value:   mutationInfo.Value,
			Padding: []byte{},
			Version: &mutationInfo.Version,
		}

		content, err := proto.Marshal(pbObj)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal mutation: %w", err)
		}

		encryptedContent, err := cbcutil.Encrypt(keys.ValueEncryption, nil, content)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt mutation: %w", err)
		}

		valueMac := generateContentMAC(waProto.SyncdMutation_SET, encryptedContent, keyID, keys.ValueMAC)
		indexMac := concatAndHMAC(sha256.New, keys.Index, indexBytes)

		mutations = append(mutations, &waProto.SyncdMutation{
			Operation: waProto.SyncdMutation_SET.Enum(),
			Record: &waProto.SyncdRecord{
				Index: &waProto.SyncdIndex{Blob: indexMac},
				Value: &waProto.SyncdValue{Blob: append(encryptedContent, valueMac...)},
				KeyId: &waProto.KeyId{Id: keyID},
			},
		})
	}

	warn, err := state.updateHash(mutations, func(indexMAC []byte, _ int) ([]byte, error) {
		return proc.Store.AppState.GetAppStateMutationMAC(string(patchInfo.Type), indexMAC)
	})
	if len(warn) > 0 {
		proc.Log.Warnf("Warnings while updating hash for %s (sending new app state): %+v", patchInfo.Type, warn)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update state hash: %w", err)
	}

	state.Version += 1

	syncdPatch := &waProto.SyncdPatch{
		SnapshotMac: state.generateSnapshotMAC(patchInfo.Type, keys.SnapshotMAC),
		KeyId:       &waProto.KeyId{Id: keyID},
		Mutations:   mutations,
	}
	syncdPatch.PatchMac = generatePatchMAC(syncdPatch, patchInfo.Type, keys.PatchMAC, state.Version)

	result, err := proto.Marshal(syncdPatch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compiled patch: %w", err)
	}

	return result, nil
}
