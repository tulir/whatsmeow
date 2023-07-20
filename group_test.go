package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestParseparseParticipantListFromMessage(t *testing.T) {
	/* Example message:

	<message count="2" from="0000000000@broadcast" id="00000000000000000000000000000000" notify="Name"
		participant="000000000000@s.whatsapp.net" t="0" type="text">
		<participants>
			<to eph_setting="X" jid="0000000001@s.whatsapp.net" />
			<to eph_setting="Y" jid="0000000002@s.whatsapp.net" />
		</participants>
		<enc type="pkmsg" v="2"><!-- 165 bytes --></enc>
		<enc type="skmsg" v="2"><!-- 154 bytes --></enc>
	</message>
	*/

	firstParticipant := types.NewJID("0000000001", types.DefaultUserServer)
	secondParticipant := types.NewJID("0000000002", types.DefaultUserServer)

	node := &binary.Node{
		Tag:   "participants",
		Attrs: map[string]any{},
		Content: []binary.Node{{
			Tag: "to",
			Attrs: map[string]any{
				"eph_setting": "X",
				"jid":         firstParticipant,
			},
			Content: nil,
		}, {
			Tag: "to",
			Attrs: map[string]any{
				"eph_setting": "Y",
				"jid":         secondParticipant,
			},
			Content: nil,
		}},
	}

	participants := parseParticipantList(node)
	if actual := len(participants); actual != 2 {
		t.Fatalf("len(participants), Expected %d, Actual %d", 2, actual)
	}

	if actual := participants[0]; actual != firstParticipant {
		t.Fatalf("participants[0], Expected %s, Actual %s", firstParticipant, actual)
	}

	if actual := participants[1]; actual != secondParticipant {
		t.Fatalf("participants[1], Expected %s, Actual %s", secondParticipant, actual)
	}
}
