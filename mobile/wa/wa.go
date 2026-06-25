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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver, registers as "sqlite"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const historyBridgeBatchSize = 50

// maxOutboundMediaBytes bounds how much we download for an outbound media send.
// WhatsApp's practical image/video ceiling is ~16 MB; we read one byte past it
// so we can detect (and reject) anything larger instead of streaming forever.
const maxOutboundMediaBytes = 16 * 1024 * 1024

// Events is implemented on the host (Swift) side to receive async updates.
//
// All methods may be invoked from a background goroutine; the host is
// responsible for dispatching to the main thread before touching UI.
type Events interface {
	OnConnected()
	OnLoggedOut()
	OnPairSuccess()
	// OnMessage delivers a single live message as a JSON object (see waMessage).
	OnMessage(payload string)
	// OnHistorySync delivers a small batch of historical messages as JSON each
	// time the phone pushes a history blob. Large blobs are split before they
	// cross the Swift/JS bridge to avoid transient memory spikes.
	OnHistorySync(payload string)
	OnError(stage string, message string)
}

var (
	mu        sync.Mutex
	client    *whatsmeow.Client
	container *sqlstore.Container
	dbConn    *sql.DB
	rootCtx   context.Context
	cancelCtx context.CancelFunc

	// evtMu guards currentEvt, which always points at the most recent host
	// listener. React Native hands us a fresh Events bridge on every screen
	// mount / fast-refresh, so the event handler resolves it dynamically
	// instead of capturing a single bridge for the life of the client.
	evtMu      sync.RWMutex
	currentEvt Events
)

func setEvt(e Events) {
	evtMu.Lock()
	currentEvt = e
	evtMu.Unlock()
}

func getEvt() Events {
	evtMu.RLock()
	defer evtMu.RUnlock()
	return currentEvt
}

// CoreLinked reports that the whatsmeow core linked into the framework.
// Useful as a trivial bridge sanity check from Swift.
func CoreLinked() bool { return true }

// Start opens (or creates) the session database under storeDir and connects.
// If a paired session already exists it resumes; otherwise it connects so
// that RequestPairingCode can be called next. Calling Start again is safe: it
// refreshes the host listener and reconnects the socket if it has dropped.
func Start(storeDir string, evt Events) error {
	mu.Lock()
	defer mu.Unlock()
	if evt == nil {
		return errors.New("events listener is required")
	}

	// How this device appears in WhatsApp > Linked Devices. WhatsApp renders its
	// own icon from PlatformType (custom images aren't possible); DESKTOP makes
	// it show the Os string verbatim ("MeGPT") instead of a "Browser (OS)" label.
	// These props are only sent at pair time, so changing them requires
	// unlinking and pairing again.
	store.DeviceProps.Os = proto.String("MeGPT")
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_DESKTOP.Enum()

	// Always route callbacks to the latest host listener.
	setEvt(evt)

	// Already initialized. If the device was unlinked — by us, or from the phone's
	// WhatsApp > Linked Devices — whatsmeow deletes the session store (marking it
	// Deleted) and then refuses to reconnect or re-pair on that client. Reusing it
	// is exactly what makes the next pair attempt hang on "Logging in…" in
	// WhatsApp: a code is issued, but the new session keys can't be persisted to a
	// deleted store, so the link never completes. Tear the dead client down here
	// so we rebuild a fresh, pairable one below. Otherwise reuse it (including the
	// normal not-yet-paired state, where Store.ID is nil) and ensure the socket.
	if client != nil {
		if client.Store == nil || client.Store.Deleted {
			teardownLocked()
		} else {
			if !client.IsConnected() {
				if err := client.Connect(); err != nil {
					return fmt.Errorf("reconnect: %w", err)
				}
			}
			return nil
		}
	}

	rootCtx, cancelCtx = context.WithCancel(context.Background())

	dbPath := filepath.Join(storeDir, "whatsmeow.db")
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	dbConn = db

	container = sqlstore.NewWithDB(db, "sqlite3", waLog.Stdout("WA-DB", "WARN", false))
	if err := container.Upgrade(rootCtx); err != nil {
		return fmt.Errorf("upgrade db: %w", err)
	}

	device, err := container.GetFirstDevice(rootCtx)
	if err != nil {
		return fmt.Errorf("get device: %w", err)
	}

	client = whatsmeow.NewClient(device, waLog.Stdout("WA", "INFO", false))
	c := client
	// Capture this client's root context for the handler's lifetime so JID
	// canonicalization (LID lookups) shares the client's cancellation scope.
	handlerCtx := rootCtx
	client.AddEventHandler(func(raw interface{}) {
		if e := getEvt(); e != nil {
			dispatch(handlerCtx, c, e, raw)
		}
	})

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

// IsConnected reports whether the websocket is currently connected. This is
// independent of login state (an unpaired client can be connected while it
// waits to be linked).
func IsConnected() bool {
	mu.Lock()
	defer mu.Unlock()
	return client != nil && client.IsConnected()
}

// RequestPairingCode links this device to the given phone number (full
// international format, digits only, no leading +). Returns an 8-character
// code the user types into WhatsApp > Linked Devices > Link with phone number.
// It ensures the websocket is connected first, since pairing requires it.
func RequestPairingCode(phone string) (string, error) {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return "", errors.New("not started")
	}
	// Pairing happens while we're intentionally unpaired, so we only need the
	// websocket up here — NOT a logged-in session (that's what we're creating).
	if err := ensureSocket(c, 15*time.Second); err != nil {
		return "", err
	}
	code, err := c.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (macOS)")
	if err != nil {
		return "", fmt.Errorf("pair: %w", err)
	}
	return code, nil
}

// ensureSocket makes sure the websocket is connected and the Noise handshake has
// completed, reconnecting if necessary.
//
// Unlike Client.WaitForConnection, it does NOT require the client to be logged
// in. That distinction matters: during pairing the client is intentionally
// unpaired, so waiting for a logged-in session there would always time out.
func ensureSocket(c *whatsmeow.Client, timeout time.Duration) error {
	if !c.IsConnected() {
		if err := c.Connect(); err != nil && !errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			return fmt.Errorf("connect: %w", err)
		}
	}
	deadline := time.Now().Add(timeout)
	for !c.IsConnected() {
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for WhatsApp connection")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// SendText sends a plain text message to a recipient. The recipient is either a
// bare phone number (digits only, no +) or a full JID ("<pn>@s.whatsapp.net" or
// the privacy "<id>@lid" form) taken from a known thread — see resolveSendJID.
func SendText(recipient string, text string) error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	if c.Store == nil || c.Store.ID == nil {
		return errors.New("not logged in")
	}
	// Sending requires an authenticated session, so wait for the socket and then
	// for login to complete (the paired client auto-authenticates on connect).
	if err := ensureSocket(c, 15*time.Second); err != nil {
		return err
	}
	if !c.WaitForConnection(15 * time.Second) {
		return errors.New("timed out waiting for WhatsApp login")
	}
	jid, err := resolveSendJID(ctx, c, recipient)
	if err != nil {
		return err
	}
	resp, err := c.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	// whatsmeow never delivers our own client's sends back as events, so echo the
	// message through the same OnMessage path used for captured messages. Without
	// this, messages sent from the app would never reach the host and so would
	// never appear in the chat. The server-assigned ID lets the host dedupe this
	// against any later history-sync copy of the same message.
	echoChatJID := canonicalJID(ctx, c, jid)
	emitSentMessage(ctx, c, echoChatJID, string(resp.ID), text, resp.Timestamp)
	return nil
}

// SendImageURL sends an image to a recipient (bare phone number or full JID, see
// resolveSendJID) with an optional caption. The image bytes are fetched from
// mediaURL here, inside the wrapper, rather than passed across the
// gomobile/Swift/JS bridge: that keeps large binaries off the bridge (the host
// only hands us a URL string) and lets whatsmeow encrypt + upload the raw bytes
// exactly as the protocol expects.
//
// Flow mirrors whatsmeow's documented media send: download -> Upload (encrypt +
// upload, returns keys/URL) -> build an ImageMessage from those keys -> Send.
func SendImageURL(recipient string, mediaURL string, caption string) error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	if c.Store == nil || c.Store.ID == nil {
		return errors.New("not logged in")
	}
	if strings.TrimSpace(mediaURL) == "" {
		return errors.New("media url is required")
	}
	if err := ensureSocket(c, 15*time.Second); err != nil {
		return err
	}
	if !c.WaitForConnection(15 * time.Second) {
		return errors.New("timed out waiting for WhatsApp login")
	}

	data, mimeType, err := downloadMedia(ctx, mediaURL)
	if err != nil {
		return fmt.Errorf("download media: %w", err)
	}

	uploaded, err := c.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	img := &waE2E.ImageMessage{
		Mimetype:      proto.String(mimeType),
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
	}
	if caption != "" {
		img.Caption = proto.String(caption)
	}

	jid, err := resolveSendJID(ctx, c, recipient)
	if err != nil {
		return err
	}
	resp, err := c.SendMessage(ctx, jid, &waE2E.Message{ImageMessage: img})
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	// Echo the send so it surfaces in the host chat, same as SendText. Media
	// echoes don't carry the image yet, so show the caption (or a photo marker)
	// to reflect that something was sent. The server-assigned ID lets the host
	// dedupe this against any later history-sync copy.
	echoText := caption
	if echoText == "" {
		echoText = "\U0001F4F7 Photo"
	}
	echoChatJID := canonicalJID(ctx, c, jid)
	emitSentMessage(ctx, c, echoChatJID, string(resp.ID), echoText, resp.Timestamp)
	return nil
}

// PostStatusImageURL posts an image to the user's WhatsApp Status ("story"),
// with an optional caption. Like SendImageURL it fetches the bytes inside the
// wrapper from mediaURL (keeping large binaries off the gomobile/Swift/JS
// bridge), then uploads + sends — but the destination is the special
// status@broadcast JID instead of a 1:1 chat.
//
// whatsmeow handles the broadcast fan-out: sending to StatusBroadcastJID makes
// it resolve the user's status-privacy recipient list and encrypt the media
// message for each of them. Status posts are not echoed back as chat messages
// (they don't belong to any DM thread); the server posts the user-facing
// confirmation instead.
func PostStatusImageURL(mediaURL string, caption string) error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	if c.Store == nil || c.Store.ID == nil {
		return errors.New("not logged in")
	}
	if strings.TrimSpace(mediaURL) == "" {
		return errors.New("media url is required")
	}
	if err := ensureSocket(c, 15*time.Second); err != nil {
		return err
	}
	if !c.WaitForConnection(15 * time.Second) {
		return errors.New("timed out waiting for WhatsApp login")
	}

	data, mimeType, err := downloadMedia(ctx, mediaURL)
	if err != nil {
		return fmt.Errorf("download media: %w", err)
	}

	uploaded, err := c.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	img := &waE2E.ImageMessage{
		Mimetype:      proto.String(mimeType),
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uploaded.FileLength),
	}
	if caption != "" {
		img.Caption = proto.String(caption)
	}

	if _, err := c.SendMessage(ctx, types.StatusBroadcastJID, &waE2E.Message{ImageMessage: img}); err != nil {
		return fmt.Errorf("post status: %w", err)
	}
	return nil
}

// downloadMedia fetches bytes from a URL with a bounded size and resolves a
// usable image MIME type, preferring the server's Content-Type header and
// falling back to content sniffing. It rejects non-image and oversized bodies so
// a bad URL fails fast instead of being uploaded as a broken attachment.
func downloadMedia(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxOutboundMediaBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("empty media body")
	}
	if len(data) > maxOutboundMediaBytes {
		return nil, "", fmt.Errorf("media exceeds %d byte limit", maxOutboundMediaBytes)
	}

	mimeType := resp.Header.Get("Content-Type")
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}
	mimeType = strings.TrimSpace(mimeType)
	if !strings.HasPrefix(mimeType, "image/") {
		// Header missing or generic (e.g. application/octet-stream from a CDN):
		// sniff the actual bytes so the recipient gets the right content type.
		mimeType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, "", fmt.Errorf("unsupported media type %q", mimeType)
	}
	return data, mimeType, nil
}

// IsOnWhatsApp reports whether a phone number is registered on WhatsApp. The
// phone is digits only with no leading + (same convention as SendText); we add
// the international + prefix that whatsmeow expects. This is a server query, so
// it requires a logged-in session — callers use it to avoid sending into the
// void when a contact isn't on WhatsApp.
func IsOnWhatsApp(phone string) (bool, error) {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return false, errors.New("not started")
	}
	if c.Store == nil || c.Store.ID == nil {
		return false, errors.New("not logged in")
	}
	if err := ensureSocket(c, 15*time.Second); err != nil {
		return false, err
	}
	if !c.WaitForConnection(15 * time.Second) {
		return false, errors.New("timed out waiting for WhatsApp login")
	}
	query := phone
	if !strings.HasPrefix(query, "+") {
		query = "+" + query
	}
	resp, err := c.IsOnWhatsApp(ctx, []string{query})
	if err != nil {
		return false, fmt.Errorf("is-on-whatsapp: %w", err)
	}
	if len(resp) == 0 {
		return false, nil
	}
	return resp[0].IsIn, nil
}

// Disconnect closes the websocket but keeps the session on disk.
func Disconnect() {
	mu.Lock()
	defer mu.Unlock()
	if client != nil {
		client.Disconnect()
	}
}

// Logout unlinks this device from the account and clears the local session,
// then tears down the in-memory client so the next Start builds a fresh one.
// whatsmeow can't cleanly re-pair on a client object that has already been
// logged out, so without this reset a re-link hangs on "Logging in…" until the
// app is force-quit.
func Logout() error {
	mu.Lock()
	c, ctx := client, rootCtx
	mu.Unlock()
	if c == nil {
		return errors.New("not started")
	}
	if err := c.Logout(ctx); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	resetClient()
	return nil
}

// resetClient tears down the in-memory client, store handle, and context so the
// next Start rebuilds everything from scratch: a fresh, unpaired client ready to
// pair again. Only call after the on-disk session has been cleared (e.g. after a
// successful Logout), otherwise the next Start would just resume the old device.
// resetClient tears down the in-memory client from outside an mu-locked section
// (e.g. after Logout). It is the locking wrapper around teardownLocked.
func resetClient() {
	mu.Lock()
	defer mu.Unlock()
	teardownLocked()
}

// teardownLocked cancels the root context, closes the store handle, and clears
// every cached reference so the next Start builds a fresh client. Callers must
// already hold mu (e.g. Start rebuilding a dead client in place).
func teardownLocked() {
	if cancelCtx != nil {
		cancelCtx()
	}
	if dbConn != nil {
		_ = dbConn.Close()
	}
	client = nil
	container = nil
	dbConn = nil
	rootCtx = nil
	cancelCtx = nil
}

// waMessage is the JSON shape delivered to the host for both live and
// historical messages. It is intentionally flat and text-only for now.
//
// ChatJID/SenderJID are canonical (LID-preferred, see canonicalJID) so an
// identity stays stable across the forms WhatsApp uses. Because a LID carries no
// phone number, we additionally resolve — best effort, on-device — the dialable
// phone for each JID and the user's saved address-book name for the chat
// counterparty. The host uses the phone to unify a chat with an existing contact
// and the contact name to label it, instead of surfacing the opaque LID id.
type waMessage struct {
	ChatJID       string `json:"chatJID"`
	SenderJID     string `json:"senderJID"`
	MessageID     string `json:"messageID"`
	TimestampSecs int64  `json:"timestampSecs"`
	Text          string `json:"text"`
	PushName      string `json:"pushName"`
	FromMe        bool   `json:"fromMe"`
	// Dialable phone (digits only, no +) resolved from each JID's LID->PN
	// mapping; empty when the device knows no phone for that identity.
	SenderPhoneNumber string `json:"senderPhoneNumber"`
	ChatPhoneNumber   string `json:"chatPhoneNumber"`
	// The owner's saved address-book name for the chat counterparty, if any.
	ContactName string `json:"contactName"`
}

type historySyncPayload struct {
	Messages   []waMessage `json:"messages"`
	SyncType   string      `json:"syncType"`
	ChunkOrder uint32      `json:"chunkOrder"`
	Progress   uint32      `json:"progress"`
	BatchIndex uint32      `json:"batchIndex"`
}

// messageText pulls plain text out of a message, covering both simple
// conversation messages and extended (link/quote) text. Non-text messages
// (media, stickers, reactions, ...) return "" and are skipped while scope is
// text-only.
func messageText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if t := msg.GetConversation(); t != "" {
		return t
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}

// toWaMessage maps a parsed whatsmeow message to the host payload shape. ok is
// false when the message should be skipped: group chats (scope is 1:1 DMs) and
// non-text messages.
func toWaMessage(ctx context.Context, c *whatsmeow.Client, m *events.Message) (waMessage, bool) {
	if m == nil || m.Info.IsGroup {
		return waMessage{}, false
	}
	text := messageText(m.Message)
	if text == "" {
		return waMessage{}, false
	}
	return waMessage{
		ChatJID:           canonicalJID(ctx, c, m.Info.Chat).String(),
		SenderJID:         canonicalJID(ctx, c, m.Info.Sender).String(),
		MessageID:         string(m.Info.ID),
		TimestampSecs:     m.Info.Timestamp.Unix(),
		Text:              text,
		PushName:          m.Info.PushName,
		FromMe:            m.Info.IsFromMe,
		SenderPhoneNumber: dialablePhone(ctx, c, m.Info.Sender),
		ChatPhoneNumber:   dialablePhone(ctx, c, m.Info.Chat),
		ContactName:       deviceContactName(ctx, c, m.Info.Chat),
	}, true
}

// canonicalJID reduces any 1:1 user JID to a single stable identity so the same
// person maps to one JID no matter how we observed them. WhatsApp exposes the
// same user under several forms — phone-number JIDs (`<pn>@s.whatsapp.net`), the
// newer privacy LID JIDs (`<id>@lid`), and per-device "AD" variants
// (`…:<device>@…`) — and they arrive inconsistently: received events carry the
// raw sender/chat, while our own sends are normalized. If we forwarded those raw
// forms, one contact (or the owner) would split into multiple users, which is
// what makes the owner's own name leak onto a thread (an owner AD/LID variant
// that doesn't match the stored account JID gets treated as the counterparty).
//
// We strip the device (ToNonAD) and prefer the LID form (matching WhatsApp's own
// direction), falling back to the phone JID when no LID mapping exists. The
// mapping is deterministic at a given time, so both the receive and self-send
// paths converge on the same value for the same identity.
func canonicalJID(ctx context.Context, c *whatsmeow.Client, jid types.JID) types.JID {
	id := jid.ToNonAD()
	if c == nil || c.Store == nil || id.Server != types.DefaultUserServer {
		return id
	}
	lid, err := c.Store.LIDs.GetLIDForPN(ctx, id)
	if err != nil || lid.IsEmpty() {
		return id
	}
	return lid.ToNonAD()
}

// phoneJID resolves a 1:1 user JID to its phone-number form
// (`<pn>@s.whatsapp.net`). A phone JID is already in that form; a LID
// (`<id>@lid`) is translated via the device's stored LID->PN mapping. Any other
// server (or an unknown LID) yields an empty JID. This is the inverse of
// canonicalJID, which prefers the LID for stable identity.
func phoneJID(ctx context.Context, c *whatsmeow.Client, jid types.JID) types.JID {
	id := jid.ToNonAD()
	switch id.Server {
	case types.DefaultUserServer:
		return id
	case types.HiddenUserServer:
		if c == nil || c.Store == nil {
			return types.JID{}
		}
		pn, err := c.Store.LIDs.GetPNForLID(ctx, id)
		if err != nil || pn.IsEmpty() {
			return types.JID{}
		}
		return pn.ToNonAD()
	default:
		return types.JID{}
	}
}

// dialablePhone returns the dialable phone number (digits only, no +) for a 1:1
// user JID, or "" when the device knows no phone for that identity (e.g. a LID
// with no stored mapping). The digits match the host's phone convention.
func dialablePhone(ctx context.Context, c *whatsmeow.Client, jid types.JID) string {
	pn := phoneJID(ctx, c, jid)
	if pn.IsEmpty() {
		return ""
	}
	return pn.User
}

// deviceContactName returns the owner's saved address-book name for a 1:1 JID
// (full name, then first name, then business name), or "" when the user isn't a
// saved contact. The contact store is keyed by phone, so we look up both the JID
// as observed and its resolved phone form. PushName is intentionally excluded
// here — it already rides along on the message as PushName.
func deviceContactName(ctx context.Context, c *whatsmeow.Client, jid types.JID) string {
	if c == nil || c.Store == nil || c.Store.Contacts == nil {
		return ""
	}
	candidates := []types.JID{jid.ToNonAD()}
	if pn := phoneJID(ctx, c, jid); !pn.IsEmpty() && pn.String() != jid.ToNonAD().String() {
		candidates = append(candidates, pn)
	}
	for _, candidate := range candidates {
		if candidate.IsEmpty() {
			continue
		}
		info, err := c.Store.Contacts.GetContact(ctx, candidate)
		if err != nil || !info.Found {
			continue
		}
		if name := firstNonEmpty(info.FullName, info.FirstName, info.BusinessName); name != "" {
			return name
		}
	}
	return ""
}

// firstNonEmpty returns the first value that is non-empty after trimming spaces.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveSendJID converts a caller-provided recipient into the JID we actually
// send to. The recipient is either a bare phone number (digits, no +) — the
// historical contract used for contact-resolved sends — or a full JID string
// taken from a known thread. Because of WhatsApp's LID migration a thread's
// counterparty is often only addressable by its `<id>@lid` JID (we never learn a
// dialable phone for them server-side), so phone-only sending can't reach them.
// We resolve a LID target to its phone-number JID when the device knows the
// mapping (the most reliable delivery path); otherwise we send to the JID as
// parsed and let whatsmeow handle LID addressing.
func resolveSendJID(ctx context.Context, c *whatsmeow.Client, recipient string) (types.JID, error) {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return types.JID{}, errors.New("empty recipient")
	}
	if !strings.ContainsRune(recipient, '@') {
		// Bare phone number (digits only, no +): the historical contract.
		return types.NewJID(recipient, types.DefaultUserServer), nil
	}
	jid, err := types.ParseJID(recipient)
	if err != nil {
		return types.JID{}, fmt.Errorf("parse recipient jid %q: %w", recipient, err)
	}
	jid = jid.ToNonAD()
	if jid.Server == types.HiddenUserServer && c != nil && c.Store != nil {
		if pn, perr := c.Store.LIDs.GetPNForLID(ctx, jid); perr == nil && !pn.IsEmpty() {
			return pn.ToNonAD(), nil
		}
	}
	return jid, nil
}

// emitSentMessage echoes a message we just sent through the same OnMessage path
// used for received messages. whatsmeow does not deliver our own client's sends
// back as events, so this is the only way our outbound messages reach the host
// (and thus the chat). It marks FromMe so the host treats it as the owner's
// message, and carries the server-assigned id+timestamp so a later history-sync
// copy dedupes against it instead of duplicating.
func emitSentMessage(ctx context.Context, c *whatsmeow.Client, chat types.JID, id string, text string, ts time.Time) {
	evt := getEvt()
	if evt == nil || c.Store == nil || c.Store.ID == nil {
		return
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	payload, err := json.Marshal(waMessage{
		ChatJID:           chat.String(),
		SenderJID:         canonicalJID(ctx, c, *c.Store.ID).String(),
		MessageID:         id,
		TimestampSecs:     ts.Unix(),
		Text:              text,
		PushName:          c.Store.PushName,
		FromMe:            true,
		SenderPhoneNumber: dialablePhone(ctx, c, *c.Store.ID),
		ChatPhoneNumber:   dialablePhone(ctx, c, chat),
		ContactName:       deviceContactName(ctx, c, chat),
	})
	if err != nil {
		evt.OnError("encode_sent", err.Error())
		return
	}
	evt.OnMessage(string(payload))
}

func dispatch(ctx context.Context, c *whatsmeow.Client, evt Events, raw interface{}) {
	switch e := raw.(type) {
	case *events.Connected:
		evt.OnConnected()
	case *events.PairSuccess:
		evt.OnPairSuccess()
	case *events.LoggedOut:
		evt.OnLoggedOut()
	case *events.Message:
		msg, ok := toWaMessage(ctx, c, e)
		if !ok {
			return
		}
		payload, err := json.Marshal(msg)
		if err != nil {
			evt.OnError("encode_message", err.Error())
			return
		}
		evt.OnMessage(string(payload))
	case *events.HistorySync:
		dispatchHistory(ctx, c, evt, e)
	}
}

// dispatchHistory streams a history blob into small, text-only 1:1 batches. A
// full WhatsApp history blob can be large enough to duplicate memory several
// times when marshaled through Go -> Swift -> JS, so never accumulate the whole
// blob before emitting.
func dispatchHistory(ctx context.Context, c *whatsmeow.Client, evt Events, e *events.HistorySync) {
	if c == nil || e == nil || e.Data == nil {
		return
	}
	syncType := e.Data.GetSyncType().String()
	chunkOrder := e.Data.GetChunkOrder()
	progress := e.Data.GetProgress()
	batchIndex := uint32(0)
	batch := make([]waMessage, 0, historyBridgeBatchSize)
	emitBatch := func() {
		if len(batch) == 0 {
			return
		}
		payload, err := json.Marshal(historySyncPayload{
			Messages:   batch,
			SyncType:   syncType,
			ChunkOrder: chunkOrder,
			Progress:   progress,
			BatchIndex: batchIndex,
		})
		if err != nil {
			evt.OnError("encode_history", err.Error())
			return
		}
		evt.OnHistorySync(string(payload))
		batchIndex++
		batch = make([]waMessage, 0, historyBridgeBatchSize)
	}

	for _, conv := range e.Data.GetConversations() {
		for _, histMsg := range conv.GetMessages() {
			// Empty chat JID lets ParseWebMessage derive it from the message key.
			parsed, err := c.ParseWebMessage(types.JID{}, histMsg.GetMessage())
			if err != nil {
				continue
			}
			if msg, ok := toWaMessage(ctx, c, parsed); ok {
				batch = append(batch, msg)
				if len(batch) >= historyBridgeBatchSize {
					emitBatch()
				}
			}
		}
	}
	emitBatch()
}
