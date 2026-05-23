package events

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// HistoricalPollUpdates flattens every previously-bundled poll vote record
// embedded in this history sync blob into a single slice. The primary
// phone assembles these records on WebMessageInfo.PollUpdates for each
// poll-creation message so newly paired companions can render existing
// tallies they never received as live PollUpdateMessage events.
//
// The returned hashes are SHA-256(optionName) digests — identical in shape
// to what [whatsmeow.Client.DecryptPollVote] yields for live votes — so a
// consumer's tallying code path is uniform across live and historical
// sources.
//
// Returns nil when the blob carries no poll-update records.
func (h *HistorySync) HistoricalPollUpdates() []HistoricalPollVote {
	if h == nil || h.Data == nil {
		return nil
	}
	var out []HistoricalPollVote
	for _, conv := range h.Data.GetConversations() {
		chatJIDStr := conv.GetID()
		if chatJIDStr == "" {
			continue
		}
		chatJID, err := types.ParseJID(chatJIDStr)
		if err != nil {
			continue
		}
		for _, m := range conv.GetMessages() {
			wm := m.GetMessage()
			if wm == nil {
				continue
			}
			updates := wm.GetPollUpdates()
			if len(updates) == 0 {
				continue
			}
			key := wm.GetKey()
			if key == nil {
				continue
			}
			pollID := types.MessageID(key.GetID())
			pollFromMe := key.GetFromMe()
			for _, pu := range updates {
				vote := pu.GetVote()
				if vote == nil {
					continue
				}
				voteKey := pu.GetPollUpdateMessageKey()
				var voter types.JID
				if voteKey != nil {
					if p := voteKey.GetParticipant(); p != "" {
						if vj, perr := types.ParseJID(p); perr == nil {
							voter = vj
						}
					} else if !voteKey.GetFromMe() {
						// 1:1 polls have no Participant on the key; the
						// voter is the chat peer (except when the vote
						// is from us, which the caller can detect via
						// PollUpdateMessageKey.FromMe).
						voter = chatJID
					}
				}
				var ts time.Time
				if ms := pu.GetSenderTimestampMS(); ms > 0 {
					ts = time.UnixMilli(ms)
				} else {
					ts = time.Unix(int64(wm.GetMessageTimestamp()), 0)
				}
				out = append(out, HistoricalPollVote{
					Chat:                 chatJID,
					PollCreationID:       pollID,
					Voter:                voter,
					SelectedOptionHashes: vote.GetSelectedOptions(),
					Timestamp:            ts,
					PollCreationFromMe:   pollFromMe,
				})
			}
		}
	}
	return out
}
