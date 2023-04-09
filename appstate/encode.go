package appstate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/cbcutil"
	"google.golang.org/protobuf/proto"
	"time"
)

const (
	MuteTime1Hour  = int64(60 * 60 * 1000)
	MuteTime8Hours = int64(8 * MuteTime1Hour)
	MuteTime1Week  = int64(7 * 24 * MuteTime1Hour)
)

var (
	operation = waProto.SyncdMutation_SET
)

type MutationInfo struct {
	Index   []string
	Version int32
	Value   *waProto.SyncActionValue
}

type PatchInfo struct {
	Timestamp int64
	Type      WAPatchName
	Mutations []MutationInfo
}

func NewMutePatchInfo(target types.JID, mute bool, muteEndTimestamp *int64) PatchInfo {
	ts := time.Now().UnixMilli()
	if !mute {
		muteEndTimestamp = nil
	}

	if muteEndTimestamp != nil {
		*muteEndTimestamp += ts
	}

	return PatchInfo{
		Timestamp: ts,
		Type:      WAPatchRegularHigh,
		Mutations: []MutationInfo{
			{
				Index:   []string{"mute", target.String()},
				Version: 2,
				Value: &waProto.SyncActionValue{
					MuteAction: &waProto.MuteAction{
						Muted:            &mute,
						MuteEndTimestamp: muteEndTimestamp,
					},
				},
			},
		},
	}
}

func newPinMutationInfo(target types.JID, pin bool) MutationInfo {
	return MutationInfo{
		Index:   []string{"pin_v1", target.String()},
		Version: 5,
		Value: &waProto.SyncActionValue{
			PinAction: &waProto.PinAction{
				Pinned: &pin,
			},
		},
	}
}

func NewPinPatchInfo(target types.JID, pin bool) PatchInfo {
	return PatchInfo{
		Timestamp: time.Now().UnixMilli(),
		Type:      WAPatchRegularLow,
		Mutations: []MutationInfo{
			newPinMutationInfo(target, pin),
		},
	}
}

func (proc *Processor) NewArchivePatchInfo(target types.JID, archive bool) (PatchInfo, error) {
	// From the research, LastMessageTimestamp should be the LastMessageTimestamp that fromMe=false unless no such
	// message. in that case the LastMessageTimestamp will be a message with fromMe=true.
	// messages should include all messages since LastMessageTimestamp descending.
	// LastSystemMessageTimestamp should be the LastSystemMessageTimestamp and should not be included in the messages
	// array.
	// Currently, this is working with just specifying the last message, but this might be unsafe in the future.
	historicMessage, err := proc.Store.HistoricMessages.GetHistoricMessage(target)
	if err != nil {
		return PatchInfo{}, err
	}

	archiveMutationInfo := MutationInfo{
		Index:   []string{"archive", target.String()},
		Version: 3,
		Value: &waProto.SyncActionValue{
			ArchiveChatAction: &waProto.ArchiveChatAction{
				Archived: &archive,
				MessageRange: &waProto.SyncActionMessageRange{
					LastMessageTimestamp: historicMessage.LastMessageTimestamp,
					//todo: currently not using this info because the pkg doesn't handle all system message
					//LastSystemMessageTimestamp: historicMessage.LastSystemMessageTimestamp,
				},
			},
		},
	}

	if historicMessage.LastMessageId != nil {
		chatStr := historicMessage.Chat.String()
		archiveMutationInfo.Value.ArchiveChatAction.MessageRange.Messages = []*waProto.SyncActionMessage{
			{
				Key: &waProto.MessageKey{
					RemoteJid: &chatStr,
					FromMe:    historicMessage.LastMessageFromMe,
					Id:        historicMessage.LastMessageId,
				},
				Timestamp: historicMessage.LastMessageTimestamp,
			},
		}
	}

	mutations := []MutationInfo{archiveMutationInfo}
	if archive {
		mutations = append(mutations, newPinMutationInfo(target, false))
	}

	result := PatchInfo{
		Timestamp: time.Now().UnixMilli(),
		Type:      WAPatchRegularLow,
		Mutations: mutations,
	}

	return result, nil
}

func (proc *Processor) EncodePatch(state HashState, patchInfo PatchInfo) ([]byte, error) {
	latestKeyID, err := proc.Store.AppStateKeys.GetLatestAppStateSyncKeyID()
	if err != nil {
		proc.Log.Errorf("unable to encode archive patch: %v", err)
		return nil, err
	}

	keys, err := proc.getAppStateKey(latestKeyID)
	if err != nil {
		proc.Log.Errorf("unable to encode archive patch: %v", err)
		return nil, err
	}

	mutations := make([]*waProto.SyncdMutation, 0)
	for _, mutationInfo := range patchInfo.Mutations {
		mutationInfo.Value.Timestamp = &patchInfo.Timestamp

		indexBytes, err := json.Marshal(mutationInfo.Index)
		if err != nil {
			proc.Log.Errorf("unable to encode archive patch: %v", err)
			return nil, err
		}

		pbObj := &waProto.SyncActionData{
			Index:   indexBytes,
			Value:   mutationInfo.Value,
			Padding: make([]byte, 0),
			Version: &mutationInfo.Version,
		}

		content, err := proto.Marshal(pbObj)
		if err != nil {
			proc.Log.Errorf("unable to encode archive patch: %v", err)
			return nil, err
		}

		encryptedContent, err := cbcutil.Encrypt(keys.ValueEncryption, nil, content)
		if err != nil {
			proc.Log.Errorf("unable to encode archive patch: %v", err)
			return nil, err
		}

		valueMac := generateContentMAC(operation, encryptedContent, latestKeyID, keys.ValueMAC)
		indexMac := concatAndHMAC(sha256.New, keys.Index, indexBytes)

		mutations = append(mutations, &waProto.SyncdMutation{
			Operation: &operation,
			Record: &waProto.SyncdRecord{
				Index: &waProto.SyncdIndex{
					Blob: indexMac,
				},
				Value: &waProto.SyncdValue{
					Blob: append(encryptedContent, valueMac...),
				},
				KeyId: &waProto.KeyId{
					Id: latestKeyID,
				},
			},
		})
	}

	var warn []error
	warn, err = state.updateHash(mutations, func(indexMAC []byte, _ int) ([]byte, error) {
		return proc.Store.AppState.GetAppStateMutationMAC(string(patchInfo.Type), indexMAC)
	})
	if len(warn) > 0 {
		proc.Log.Warnf("Warnings while updating hash for %s: %+v", patchInfo.Type, warn)
	}
	if err != nil {
		err = fmt.Errorf("failed to update state hash: %w", err)
		return nil, err
	}

	state.Version += 1

	syncdPatch := &waProto.SyncdPatch{
		SnapshotMac: state.generateSnapshotMAC(patchInfo.Type, keys.SnapshotMAC),
		KeyId: &waProto.KeyId{
			Id: latestKeyID,
		},
		Mutations: mutations,
	}

	syncdPatch.PatchMac = generatePatchMAC(syncdPatch, patchInfo.Type, keys.PatchMAC, state.Version)

	result, err := proto.Marshal(syncdPatch)
	if err != nil {
		proc.Log.Errorf("unable to encode archive patch: %v", err)
		return nil, err
	}

	return result, nil
}
