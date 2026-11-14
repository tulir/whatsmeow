package whatsmeow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/hkdf"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/gcmutil"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func TestSubmitGroupSuspensionAppeal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		req      ReqGroupSuspensionAppeal
		body     string
		wantCode string
		wantJSON string
	}{
		{
			name:     "new appeal with default options",
			body:     `{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS","appeal_creation_time":"1700000100"}}}`,
			wantCode: "SUCCESS",
			wantJSON: `{"variables":{"input":{"group_jid":"120363000000000000","debug_info":"{}"}}}`,
		},
		{
			name:     "existing appeal with explicit options",
			req:      ReqGroupSuspensionAppeal{Reason: "Please review this group.", DebugInfo: `{"source":"test"}`},
			body:     `{"data":{"wa_create_group_suspension_appeal":{"response_code":"APPEAL_ALREADY_EXISTS","appeal_creation_time":1700000100}}}`,
			wantCode: "APPEAL_ALREADY_EXISTS",
			wantJSON: `{"variables":{"input":{"group_jid":"120363000000000000","appeal_reason":"Please review this group.","debug_info":"{\"source\":\"test\"}"}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli, requests := newGroupAppealTestClient(t, groupAppealMexResponse(tc.body))
			result, err := cli.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("120363000000000000", types.GroupServer), tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if result == nil || result.ResponseCode != tc.wantCode || result.ErrorMessage != "" || !result.AppealCreationTime.Equal(time.Unix(1700000100, 0)) {
				t.Fatalf("appeal result = %+v", result)
			}
			request := <-requests
			if request.Tag != "iq" || request.Attrs["xmlns"] != "w:mex" || request.Attrs["type"] != "get" || request.Attrs["to"] != types.ServerJID {
				t.Fatalf("appeal IQ = %+v", request)
			}
			query := request.GetChildByTag("query")
			if query.Attrs["query_id"] != "25946115325088226" {
				t.Fatalf("query ID = %v", query.Attrs["query_id"])
			}
			payload, ok := query.Content.([]byte)
			if !ok {
				t.Fatalf("query payload type = %T", query.Content)
			}
			var got, want any
			if err = json.Unmarshal(payload, &got); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal([]byte(tc.wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("query variables = %s, want %s", payload, tc.wantJSON)
			}
		})
	}
}

func TestSubmitGroupSuspensionAppealRejectsUnsuccessfulResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		wantCode string
	}{
		{"business rejection", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"NOT_AUTHORIZED","error_message":"Only admins may appeal"}}}`, "NOT_AUTHORIZED"},
		{"unknown response code", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"FUTURE_CODE"}}}`, "FUTURE_CODE"},
		{"missing payload", `{"data":{}}`, ""},
		{"null payload", `{"data":{"wa_create_group_suspension_appeal":null}}`, ""},
		{"missing response code", `{"data":{"wa_create_group_suspension_appeal":{}}}`, ""},
		{"malformed data", `{"data":[]}`, ""},
		{"invalid timestamp", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS","appeal_creation_time":"invalid"}}}`, ""},
		{"fractional timestamp", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS","appeal_creation_time":1700000100.5}}}`, ""},
		{"overflowing timestamp", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS","appeal_creation_time":"9223372036854775808"}}}`, ""},
		{"GraphQL error", `{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS"}},"errors":[{"message":"not authorized"}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli, _ := newGroupAppealTestClient(t, groupAppealMexResponse(tc.body))
			result, err := cli.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("120363000000000000", types.GroupServer), ReqGroupSuspensionAppeal{})
			if err == nil {
				t.Fatalf("unsuccessful response accepted: %+v", result)
			}
			if tc.wantCode != "" && (result == nil || result.ResponseCode != tc.wantCode) {
				t.Fatalf("server response code was lost: %+v (%v)", result, err)
			}
			if tc.wantCode == "NOT_AUTHORIZED" && (result.ErrorMessage != "Only admins may appeal" || !strings.Contains(err.Error(), "NOT_AUTHORIZED")) {
				t.Fatalf("server rejection details were lost: %+v (%v)", result, err)
			}
		})
	}
}

func TestSubmitGroupSuspensionAppealAllowsMissingCreationTime(t *testing.T) {
	for _, timestamp := range []string{"", `,"appeal_creation_time":null`, `,"appeal_creation_time":"0"`} {
		t.Run(timestamp, func(t *testing.T) {
			cli, _ := newGroupAppealTestClient(t, groupAppealMexResponse(`{"data":{"wa_create_group_suspension_appeal":{"response_code":"SUCCESS"`+timestamp+`}}}`))
			result, err := cli.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("120363000000000000", types.GroupServer), ReqGroupSuspensionAppeal{})
			if err != nil || result == nil || !result.AppealCreationTime.IsZero() {
				t.Fatalf("optional creation time = %+v, %v", result, err)
			}
		})
	}
}

func TestSubmitGroupSuspensionAppealValidatesClientAndGroup(t *testing.T) {
	var nilClient *Client
	if _, err := nilClient.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("123", types.GroupServer), ReqGroupSuspensionAppeal{}); !errors.Is(err, ErrClientIsNil) {
		t.Fatalf("nil client error = %v", err)
	}
	ownID := types.NewJID("15550000000", types.DefaultUserServer)
	cli := NewClient(&store.Device{ID: &ownID}, nil)
	for _, jid := range []types.JID{types.EmptyJID, types.GroupServerJID, types.NewJID("123", types.DefaultUserServer)} {
		if _, err := cli.SubmitGroupSuspensionAppeal(t.Context(), jid, ReqGroupSuspensionAppeal{}); err == nil || errors.Is(err, ErrNotConnected) {
			t.Fatalf("invalid group %s was sent to transport: %v", jid, err)
		}
	}
	if _, err := cli.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("123", types.GroupServer), ReqGroupSuspensionAppeal{}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("disconnected error = %v", err)
	}
}

func TestSubmitGroupSuspensionAppealPreservesIQErrors(t *testing.T) {
	response := &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "error"}, Content: []waBinary.Node{{Tag: "error", Attrs: waBinary.Attrs{"code": "403", "text": "not-authorized"}}}}
	cli, _ := newGroupAppealTestClient(t, response)
	_, err := cli.SubmitGroupSuspensionAppeal(t.Context(), types.NewJID("123", types.GroupServer), ReqGroupSuspensionAppeal{})
	var iqErr *IQError
	if !errors.As(err, &iqErr) || iqErr.Code != 403 {
		t.Fatalf("IQ error = %v", err)
	}
}

func TestSubmitGroupSuspensionAppealPreservesContextCancellation(t *testing.T) {
	cli, requests := newGroupAppealTestClient(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := cli.SubmitGroupSuspensionAppeal(ctx, types.NewJID("123", types.GroupServer), ReqGroupSuspensionAppeal{})
		done <- err
	}()
	select {
	case <-requests:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("appeal request did not reach the peer")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("appeal ignored context cancellation")
	}
}

func groupAppealMexResponse(body string) *waBinary.Node {
	return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"type": "result"}, Content: []waBinary.Node{{Tag: "result", Content: []byte(body)}}}
}

// The peer exchanges real encrypted IQ frames; no WhatsApp connection is used.
func newGroupAppealTestClient(t *testing.T, response *waBinary.Node) (*Client, <-chan *waBinary.Node) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	requests := make(chan *waBinary.Node, 1)
	keys := make([]byte, 64)
	if _, err := io.ReadFull(hkdf.New(sha256.New, nil, []byte(socket.NoiseStartPattern), nil), keys); err != nil {
		t.Fatal(err)
	}
	writeKey, err := gcmutil.Prepare(keys[:32])
	if err != nil {
		t.Fatal(err)
	}
	readKey, err := gcmutil.Prepare(keys[32:])
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		_, frame, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				t.Error(err)
			}
			return
		}
		if len(frame) < 3 {
			t.Error("missing encrypted IQ frame")
			return
		}
		plaintext, err := writeKey.Open(nil, make([]byte, 12), frame[3:], nil)
		if err != nil {
			t.Error(err)
			return
		}
		unpacked, err := waBinary.Unpack(plaintext)
		if err != nil {
			t.Error(err)
			return
		}
		node, err := waBinary.Unmarshal(unpacked)
		if err != nil {
			t.Error(err)
			return
		}
		requests <- node
		if response == nil {
			<-ctx.Done()
			return
		}
		response.Attrs["id"] = node.Attrs["id"]
		payload, err := waBinary.Marshal(*response)
		if err != nil {
			t.Error(err)
			return
		}
		ciphertext := readKey.Seal(nil, make([]byte, 12), payload, nil)
		frame = append([]byte{byte(len(ciphertext) >> 16), byte(len(ciphertext) >> 8), byte(len(ciphertext))}, ciphertext...)
		if err = conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
			t.Error(err)
			return
		}
		_, _, _ = conn.Read(ctx)
	}))
	ownID := types.NewJID("15550000000", types.DefaultUserServer)
	cli := NewClient(&store.Device{ID: &ownID}, nil)
	fs := socket.NewFrameSocket(waLog.Noop, server.Client())
	fs.URL, fs.Header = "ws"+strings.TrimPrefix(server.URL, "http"), nil
	if err = fs.Connect(ctx); err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	noise := socket.NewNoiseHandshake()
	noise.Start(socket.NoiseStartPattern, nil)
	cli.socket, err = noise.Finish(fs, cli.makeFrameHandler(nil), func(context.Context, *socket.NoiseSocket, bool) {})
	if err != nil {
		cancel()
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); cli.socket.Stop(true, true); server.Close() })
	return cli, requests
}
