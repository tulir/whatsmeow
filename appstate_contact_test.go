// Copyright (c) 2026 Kizuno18
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var _ store.ContactStore = (*legacyContactStore)(nil)

func TestDispatchAppStateContactRemoveUsesCompatibleStore(t *testing.T) {
	ctx := context.Background()
	contactJID := types.NewJID("15550001111", types.DefaultUserServer)
	missingJID := types.NewJID("15550002222", types.DefaultUserServer)
	contacts := &legacyContactStore{contacts: map[types.JID]types.ContactInfo{
		contactJID: {
			Found:         true,
			FirstName:     "Ada",
			FullName:      "Ada Lovelace",
			PushName:      "Ada Push",
			BusinessName:  "Analytical Engines",
			RedactedPhone: "1555******1",
		},
	}}
	client := &Client{Store: &store.Device{Contacts: contacts}}

	event := client.dispatchAppState(ctx, appstate.WAPatchCriticalUnblockLow, contactRemoveMutation(contactJID), false)
	contactEvent, ok := event.(*events.Contact)
	if !ok || !contactEvent.Removed || contactEvent.JID != contactJID {
		t.Fatalf("unexpected contact removal event: %#v", event)
	}
	stored, err := contacts.GetContact(ctx, contactJID)
	if err != nil {
		t.Fatalf("get removed contact: %v", err)
	}
	if stored.FirstName != "" || stored.FullName != "" {
		t.Fatalf("contact names were not cleared: %#v", stored)
	}
	if !stored.Found || stored.PushName != "Ada Push" || stored.BusinessName != "Analytical Engines" || stored.RedactedPhone != "1555******1" {
		t.Fatalf("non-name contact data changed during removal: %#v", stored)
	}

	client.dispatchAppState(ctx, appstate.WAPatchCriticalUnblockLow, contactRemoveMutation(missingJID), false)
	if _, exists := contacts.contacts[missingJID]; exists {
		t.Fatal("removing a missing contact created an empty contact")
	}
	if contacts.clearNameCalls != 2 {
		t.Fatalf("expected both removals through PutContactName, got %d calls", contacts.clearNameCalls)
	}
}

func contactRemoveMutation(jid types.JID) appstate.Mutation {
	patch := appstate.BuildContact(jid, "", false)
	mutation := patch.Mutations[0]
	return appstate.Mutation{
		Operation: patch.Operation,
		Action:    mutation.Value,
		Version:   mutation.Version,
		Index:     mutation.Index,
	}
}

type legacyContactStore struct {
	contacts       map[types.JID]types.ContactInfo
	clearNameCalls int
}

func (s *legacyContactStore) PutPushName(_ context.Context, user types.JID, pushName string) (bool, string, error) {
	contact := s.contacts[user]
	previous := contact.PushName
	contact.Found = true
	contact.PushName = pushName
	s.contacts[user] = contact
	return previous != pushName, previous, nil
}

func (s *legacyContactStore) PutBusinessName(_ context.Context, user types.JID, businessName string) (bool, string, error) {
	contact := s.contacts[user]
	previous := contact.BusinessName
	contact.Found = true
	contact.BusinessName = businessName
	s.contacts[user] = contact
	return previous != businessName, previous, nil
}

func (s *legacyContactStore) PutContactName(_ context.Context, user types.JID, firstName, fullName string) error {
	if firstName == "" && fullName == "" {
		s.clearNameCalls++
	}
	contact, exists := s.contacts[user]
	if !exists && firstName == "" && fullName == "" {
		return nil
	}
	contact.Found = true
	contact.FirstName = firstName
	contact.FullName = fullName
	s.contacts[user] = contact
	return nil
}

func (s *legacyContactStore) PutAllContactNames(_ context.Context, contacts []store.ContactEntry) error {
	for _, entry := range contacts {
		if err := s.PutContactName(context.Background(), entry.JID, entry.FirstName, entry.FullName); err != nil {
			return err
		}
	}
	return nil
}

func (s *legacyContactStore) PutManyRedactedPhones(_ context.Context, entries []store.RedactedPhoneEntry) error {
	for _, entry := range entries {
		contact := s.contacts[entry.JID]
		contact.Found = true
		contact.RedactedPhone = entry.RedactedPhone
		s.contacts[entry.JID] = contact
	}
	return nil
}

func (s *legacyContactStore) GetContact(_ context.Context, user types.JID) (types.ContactInfo, error) {
	return s.contacts[user], nil
}

func (s *legacyContactStore) GetAllContacts(context.Context) (map[types.JID]types.ContactInfo, error) {
	return s.contacts, nil
}
