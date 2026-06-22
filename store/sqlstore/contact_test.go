package sqlstore

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/types"
)

func mustParseJID(t *testing.T, value string) types.JID {
	t.Helper()
	jid, err := types.ParseJID(value)
	if err != nil {
		t.Fatalf("failed to parse jid %q: %v", value, err)
	}
	return jid
}

func newTestSQLStore(t *testing.T) (*SQLStore, func()) {
	t.Helper()

	ctx := context.Background()
	container, err := New(ctx, "sqlite3", "file::memory:?_foreign_keys=on", nil)
	if err != nil {
		t.Fatalf("failed to create sqlstore container: %v", err)
	}

	device := container.NewDevice()
	device.ID = ptr(mustParseJID(t, "5511999999999:1@s.whatsapp.net"))
	device.Account = &waAdv.ADVSignedDeviceIdentity{}
	if err := device.Save(ctx); err != nil {
		_ = container.Close()
		t.Fatalf("failed to save test device: %v", err)
	}

	sqlStore, ok := device.Contacts.(*SQLStore)
	if !ok {
		_ = container.Close()
		t.Fatalf("expected *SQLStore contacts implementation, got %T", device.Contacts)
	}

	return sqlStore, func() {
		_ = container.Close()
	}
}

func ptr[T any](value T) *T {
	return &value
}

func TestGetContactsPageReturnsOrderedSliceAndTotal(t *testing.T) {
	sqlStore, cleanup := newTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()
	contact1 := mustParseJID(t, "5511999999998@s.whatsapp.net")
	contact2 := mustParseJID(t, "5511999999997@s.whatsapp.net")
	contact3 := mustParseJID(t, "5511999999999@s.whatsapp.net")

	if err := sqlStore.PutContactName(ctx, contact1, "Zulu", "Zulu"); err != nil {
		t.Fatalf("failed to put contact1: %v", err)
	}
	if _, _, err := sqlStore.PutPushName(ctx, contact1, "Zulu"); err != nil {
		t.Fatalf("failed to put push name contact1: %v", err)
	}
	if err := sqlStore.PutContactName(ctx, contact2, "Alpha", "Alpha"); err != nil {
		t.Fatalf("failed to put contact2: %v", err)
	}
	if _, _, err := sqlStore.PutPushName(ctx, contact2, "Alpha"); err != nil {
		t.Fatalf("failed to put push name contact2: %v", err)
	}
	if err := sqlStore.PutContactName(ctx, contact3, "Mike", "Mike"); err != nil {
		t.Fatalf("failed to put contact3: %v", err)
	}
	if _, _, err := sqlStore.PutPushName(ctx, contact3, "Mike"); err != nil {
		t.Fatalf("failed to put push name contact3: %v", err)
	}

	page, err := sqlStore.GetContactsPage(ctx, 2, 1)
	if err != nil {
		t.Fatalf("GetContactsPage returned error: %v", err)
	}

	if page.Total != 3 {
		t.Fatalf("expected total 3, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if page.Items[0].JID.String() != "5511999999998@s.whatsapp.net" {
		t.Fatalf("expected first paged jid to be 5511999999998@s.whatsapp.net, got %s", page.Items[0].JID.String())
	}
	if page.Items[1].JID.String() != "5511999999999@s.whatsapp.net" {
		t.Fatalf("expected second paged jid to be 5511999999999@s.whatsapp.net, got %s", page.Items[1].JID.String())
	}
	if page.Items[0].Info.PushName != "Zulu" {
		t.Fatalf("expected first paged push name to be Zulu, got %q", page.Items[0].Info.PushName)
	}
	if page.Items[1].Info.PushName != "Mike" {
		t.Fatalf("expected second paged push name to be Mike, got %q", page.Items[1].Info.PushName)
	}
}

func TestGetContactsPageReturnsEmptyWhenOffsetExceedsTotal(t *testing.T) {
	sqlStore, cleanup := newTestSQLStore(t)
	defer cleanup()

	ctx := context.Background()
	contact := mustParseJID(t, "5511999999999@s.whatsapp.net")
	if err := sqlStore.PutContactName(ctx, contact, "Only", "Only"); err != nil {
		t.Fatalf("failed to put contact: %v", err)
	}

	page, err := sqlStore.GetContactsPage(ctx, 10, 5)
	if err != nil {
		t.Fatalf("GetContactsPage returned error: %v", err)
	}

	if page.Total != 1 {
		t.Fatalf("expected total 1, got %d", page.Total)
	}
	if len(page.Items) != 0 {
		t.Fatalf("expected empty page, got %d items", len(page.Items))
	}
}
