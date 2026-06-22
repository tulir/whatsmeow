// Package wa is a thin, gomobile-friendly wrapper around whatsmeow.
//
// It exposes the minimal surface MeGPT needs from iOS:
//   - Start: open the local session store and connect (resumes if paired)
//   - RequestPairingCode: link this device to a phone via an 8-char code
//   - SendText: send a text message to a phone number
//   - Disconnect / Logout
//
// Only gomobile-supported types are exported (string/bool/error). Async
// updates are delivered to the host (Swift) via the Events callback interface.
//
// Storage uses modernc.org/sqlite (pure Go, no CGo) so it cross-compiles for
// iOS via gomobile without a C toolchain on the device target.
package wa

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // pure-Go sqlite driver, registers as "sqlite"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Events is implemented on the host (Swift) side to receive async updates.
//
// All methods may be invoked from a background goroutine; the host is
// responsible for dispatching to the main thread before touching UI.
type Events interface {
	OnConnected()
	OnLoggedOut()
	OnPairSuccess()
	OnMessage(chatJID string, senderJID string, text string, fromMe bool)
	OnError(stage string, message string)
}

var (
	mu        sync.Mutex
	client    *whatsmeow.Client
	container *sqlstore.Container
	rootCtx   context.Context
	cancelCtx context.CancelFunc
)

// CoreLinked reports that the whatsmeow core linked into the framework.
// Useful as a trivial bridge sanity check from Swift.
func CoreLinked() bool { return true }

// Start opens (or creates) the session database under storeDir and connects.
// If a paired session already exists it resumes; otherwise it connects so
// that RequestPairingCode can be called next. Calling Start more than once is
// a no-op.
func Start(storeDir string, evt Events) error {
	mu.Lock()
	defer mu.Unlock()
	if client != nil {
		return nil
	}
	if evt == nil {
		return errors.New("events listener is required")
	}

	rootCtx, cancelCtx = context.WithCancel(context.Background())

	dbPath := filepath.Join(storeDir, "whatsmeow.db")
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	container = sqlstore.NewWithDB(db, "sqlite3", waLog.Stdout("WA-DB", "WARN", false))
	if err := container.Upgrade(rootCtx); err != nil {
		return fmt.Errorf("upgrade db: %w", err)
	}

	device, err := container.GetFirstDevice(rootCtx)
	if err != nil {
		return fmt.Errorf("get device: %w", err)
	}

	client = whatsmeow.NewClient(device, waLog.Stdout("WA", "INFO", false))
	client.AddEventHandler(func(raw interface{}) { dispatch(evt, raw) })

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

// IsLoggedIn reports whether a paired session is currently active.
func IsLoggedIn() bool {
	mu.Lock()
	defer mu.Unlock()
	return client != nil && client.Store != nil && client.Store.ID != nil
}

// RequestPairingCode links this device to the given phone number (full
// international format, digits only, no leading +). Returns an 8-character
// code the user types into WhatsApp > Linked Devices > Link with phone number.
// Start (and its Connect) must have completed first.
func RequestPairingCode(phone string) (string, error) {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return "", errors.New("not started")
	}
	code, err := c.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (macOS)")
	if err != nil {
		return "", fmt.Errorf("pair: %w", err)
	}
	return code, nil
}

// SendText sends a plain text message to a phone number (digits only, no +).
func SendText(phone string, text string) error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	if c.Store == nil || c.Store.ID == nil {
		return errors.New("not logged in")
	}
	jid := types.NewJID(phone, types.DefaultUserServer)
	_, err := c.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

// Disconnect closes the websocket but keeps the session on disk.
func Disconnect() {
	mu.Lock()
	defer mu.Unlock()
	if client != nil {
		client.Disconnect()
	}
}

// Logout unlinks this device from the account and clears the local session.
func Logout() error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	return c.Logout(ctx)
}

func dispatch(evt Events, raw interface{}) {
	switch e := raw.(type) {
	case *events.Connected:
		evt.OnConnected()
	case *events.PairSuccess:
		evt.OnPairSuccess()
	case *events.LoggedOut:
		evt.OnLoggedOut()
	case *events.Message:
		text := e.Message.GetConversation()
		if text == "" && e.Message.GetExtendedTextMessage() != nil {
			text = e.Message.GetExtendedTextMessage().GetText()
		}
		evt.OnMessage(e.Info.Chat.String(), e.Info.Sender.String(), text, e.Info.IsFromMe)
	}
}
