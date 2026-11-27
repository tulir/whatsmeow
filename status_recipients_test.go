package whatsmeow

import (
	"context"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"reflect"
	"testing"
)

type statusTestContacts struct{ store.ContactStore }

func (statusTestContacts) GetAllContacts(_ context.Context) (map[types.JID]types.ContactInfo, error) {
	return map[types.JID]types.ContactInfo{types.NewJID("200", types.DefaultUserServer): {FullName: "Contact"}}, nil
}

type statusTestLIDs struct{ store.LIDStore }

func (statusTestLIDs) GetPNForLID(_ context.Context, jid types.JID) (types.JID, error) {
	if jid.User == "222" {
		return types.NewJID("200", types.DefaultUserServer), nil
	}
	return types.EmptyJID, nil
}
func TestExplicitStatusPrivacyAndSelf(t *testing.T) {
	ctx := context.Background()
	own := types.NewJID("100", types.DefaultUserServer)
	pn := types.NewJID("200", types.DefaultUserServer)
	lid := types.NewJID("222", types.HiddenUserServer)
	cli := &Client{Store: &store.Device{ID: &own, LIDs: statusTestLIDs{}, Contacts: statusTestContacts{}}}
	for _, tc := range []struct {
		name   string
		policy types.StatusPrivacy
		input  []types.JID
		fail   bool
	}{
		{name: "contacts dedupe LID self", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeContacts}, input: []types.JID{pn, lid, pn}},
		{name: "contacts exclude unknown", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeContacts}, input: []types.JID{types.NewJID("300", types.DefaultUserServer)}, fail: true},
		{name: "whitelist allowed", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeWhitelist, List: []types.JID{lid}}, input: []types.JID{pn}},
		{name: "whitelist excluded", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeWhitelist}, input: []types.JID{pn}, fail: true},
		{name: "blacklist excluded after LID mapping", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeBlacklist, List: []types.JID{lid}}, input: []types.JID{pn}, fail: true},
		{name: "unknown privacy LID fails closed", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeBlacklist, List: []types.JID{types.NewJID("999", types.HiddenUserServer)}}, input: []types.JID{pn}, fail: true},
		{name: "unknown input LID", policy: types.StatusPrivacy{Type: types.StatusPrivacyTypeContacts}, input: []types.JID{types.NewJID("999", types.HiddenUserServer)}, fail: true},
		{name: "unknown privacy", policy: types.StatusPrivacy{Type: "unknown"}, input: []types.JID{pn}, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := cli.resolveExplicitStatusRecipients(ctx, tc.input, tc.policy)
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v", err)
			}
			if !tc.fail && !reflect.DeepEqual(out, []types.JID{pn, own}) {
				t.Fatalf("got %v", out)
			}
		})
	}
	if _, err := cli.getExplicitStatusBroadcastRecipients(ctx, []types.JID{}); err == nil {
		t.Fatal("empty accepted")
	}
	cli.Store.ID = nil
	if _, err := cli.resolveExplicitStatusRecipients(ctx, []types.JID{pn}, types.StatusPrivacy{Type: types.StatusPrivacyTypeContacts}); err != ErrNotLoggedIn {
		t.Fatalf("logged out=%v", err)
	}
}
