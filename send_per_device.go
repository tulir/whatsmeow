package whatsmeow

import (
	"fmt"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func isOwnMessageDevice(jid, ownPN, ownLID types.JID) bool {
	// Hosted companions use their account's PN/LID in a distinct server namespace.
	switch jid.Server {
	case types.HostedServer:
		jid.Server = types.DefaultUserServer
	case types.HostedLIDServer:
		jid.Server = types.HiddenUserServer
	}
	return (!ownPN.IsEmpty() && jid.ToNonAD() == ownPN.ToNonAD()) ||
		(!ownLID.IsEmpty() && jid.ToNonAD() == ownLID.ToNonAD())
}

func marshalPerDeviceMessages(to types.JID, devices []types.JID, ownPN, ownLID types.JID, hook func(types.JID) *waE2E.Message) (map[types.JID][]byte, error) {
	if hook == nil {
		return nil, nil
	}
	if to.Server != types.DefaultUserServer && to.Server != types.HiddenUserServer {
		return nil, fmt.Errorf("per-device plaintext requires a PN/LID DM")
	}
	out := make(map[types.JID][]byte)
	for _, jid := range devices {
		if isOwnMessageDevice(jid, ownPN, ownLID) {
			continue
		}
		if msg := hook(jid); msg != nil {
			data, err := proto.Marshal(msg)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal per-device message: %w", err)
			}
			out[jid] = data
		}
	}
	return out, nil
}

func selectDevicePlaintext(jid, ownPN, ownLID types.JID, base, dsm []byte, variants ...map[types.JID][]byte) []byte {
	if isOwnMessageDevice(jid, ownPN, ownLID) {
		if dsm != nil {
			return dsm
		}
		return base
	}
	if len(variants) > 0 {
		if data, ok := variants[0][jid]; ok {
			return data
		}
	}
	return base
}

// Keep retry selection consistent without mutating the shared recent-message cache.
func (rm *RecentMessage) forDeviceRetry(jid, ownPN, ownLID types.JID, skip bool) *RecentMessage {
	if rm.perDeviceMessage == nil || skip || isOwnMessageDevice(jid, ownPN, ownLID) {
		return rm
	}
	if msg := rm.perDeviceMessage(jid); msg != nil {
		result := *rm
		result.wa = proto.Clone(msg).(*waE2E.Message)
		return &result
	}
	return rm
}
