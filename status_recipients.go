// Explicit status audiences retain the normal sender-key broadcast send path.
package whatsmeow

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/types"
)

// Normalize to PN as the existing status sender uses the own PN identity.
// Never mix PN and LID or guess a phone number from a LID's numeric user.
func (cli *Client) statusRecipientPN(ctx context.Context, jid types.JID) (types.JID, error) {
	jid = jid.ToNonAD()
	if jid.Server == types.HiddenUserServer {
		pn, err := cli.Store.LIDs.GetPNForLID(ctx, jid)
		if err != nil || pn.IsEmpty() {
			return types.JID{}, fmt.Errorf("status LID has no known PN mapping")
		}
		jid = pn.ToNonAD()
	}
	if jid.Server != types.DefaultUserServer || jid.User == "" {
		return types.JID{}, fmt.Errorf("invalid status recipient")
	}
	return jid, nil
}

func (cli *Client) getExplicitStatusBroadcastRecipients(ctx context.Context, input []types.JID) ([]types.JID, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("explicit status audience must not be empty")
	}
	privacy, err := cli.GetStatusPrivacy(ctx)
	if err != nil {
		return nil, fmt.Errorf("status privacy: %w", err)
	}
	if len(privacy) == 0 {
		return nil, fmt.Errorf("missing status privacy")
	}
	return cli.resolveExplicitStatusRecipients(ctx, input, privacy[0])
}

// Split from the network privacy query so filtering can be tested offline.
func (cli *Client) resolveExplicitStatusRecipients(ctx context.Context, input []types.JID, policy types.StatusPrivacy) ([]types.JID, error) {
	if policy.Type != types.StatusPrivacyTypeContacts && policy.Type != types.StatusPrivacyTypeWhitelist && policy.Type != types.StatusPrivacyTypeBlacklist {
		return nil, fmt.Errorf("unsupported status privacy mode")
	}
	// Fail closed if an exclusion cannot be mapped; do not bypass phone privacy.
	privacySet := make(map[types.JID]bool)
	for _, id := range policy.List {
		pn, err := cli.statusRecipientPN(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("status privacy mapping: %w", err)
		}
		privacySet[pn] = true
	}
	own := cli.getOwnID().ToNonAD()
	if own.IsEmpty() {
		return nil, ErrNotLoggedIn
	}
	contacts := make(map[types.JID]bool)
	if policy.Type != types.StatusPrivacyTypeWhitelist {
		if cli.Store.Contacts == nil {
			return nil, fmt.Errorf("status contacts unavailable")
		}
		stored, err := cli.Store.Contacts.GetAllContacts(ctx)
		if err != nil {
			return nil, fmt.Errorf("status contacts: %w", err)
		}
		for jid, contact := range stored {
			if contact.FullName == "" {
				continue
			}
			pn, err := cli.statusRecipientPN(ctx, jid)
			if err != nil {
				return nil, fmt.Errorf("status contact mapping: %w", err)
			}
			contacts[pn] = true
		}
	}
	seen := make(map[types.JID]bool)
	output := make([]types.JID, 0, len(input)+1)
	for _, id := range input {
		pn, err := cli.statusRecipientPN(ctx, id)
		if err != nil {
			return nil, err
		}
		if pn != own && policy.Type != types.StatusPrivacyTypeWhitelist && !contacts[pn] {
			return nil, fmt.Errorf("explicit status recipient is not a contact")
		}
		if pn != own && ((policy.Type == types.StatusPrivacyTypeWhitelist && !privacySet[pn]) ||
			(policy.Type == types.StatusPrivacyTypeBlacklist && privacySet[pn])) {
			return nil, fmt.Errorf("explicit status recipient excluded by phone privacy")
		}
		if !seen[pn] {
			seen[pn] = true
			output = append(output, pn)
		}
	}
	if !seen[own] {
		output = append(output, own)
	}
	return output, nil
}
