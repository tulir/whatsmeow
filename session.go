package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/curve25519"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
)

//represents the WhatsAppWeb client version
var waVersion = []int{2, 2104, 9}

/*
Session contains session individual information. To be able to resume the connection without scanning the qr code
every time you should save the Session returned by Login and use RestoreWithSession the next time you want to login.
Every successful created connection returns a new Session. The Session(ClientToken, ServerToken) is altered after
every re-login and should be saved every time.
*/
type Session struct {
	ClientID    string
	ClientToken string
	ServerToken string
	EncKey      []byte
	MacKey      []byte
	Wid         string
}

/*
SetClientName sets the long and short client names that are sent to WhatsApp when logging in and displayed in the
WhatsApp Web device list. As the values are only sent when logging in, changing them after logging in is not possible.
*/
func (wac *Conn) SetClientName(long, short, version string) error {
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return fmt.Errorf("cannot change client name after logging in")
	}
	wac.longClientName, wac.shortClientName, wac.clientVersion = long, short, version
	return nil
}

/*
SetClientVersion sets WhatsApp client version
Default value is 0.4.2080
*/
func (wac *Conn) SetClientVersion(major int, minor int, patch int) {
	waVersion = []int{major, minor, patch}
}

func (wac *Conn) adminInitRequest(clientID string) (string, time.Duration, error) {
	login := []interface{}{"admin", "init", waVersion, []string{wac.longClientName, wac.shortClientName, wac.clientVersion}, clientID, true}
	loginChan, err := wac.writeJSON(login)
	if err != nil {
		return "", 0, fmt.Errorf("error writing login: %w", err)
	}

	var r string
	select {
	case r = <-loginChan:
		wac.adminInited = true
	case <-time.After(wac.msgTimeout):
		return "", 0, fmt.Errorf("login connection timed out")
	}

	var resp map[string]interface{}
	if err = json.Unmarshal([]byte(r), &resp); err != nil {
		return "", 0, fmt.Errorf("error decoding login resp: %w", err)
	}

	return resp["ref"].(string), time.Duration(resp["ttl"].(float64)) * time.Millisecond, nil
}

func (wac *Conn) adminRerefRequest() (string, time.Duration, error) {
	reref := []interface{}{"admin", "Conn", "reref"}
	rerefChan, err := wac.writeJSON(reref)
	if err != nil {
		return "", 0, fmt.Errorf("error writing reref: %w", err)
	}

	var r string
	select {
	case r = <-rerefChan:
	case <-time.After(wac.msgTimeout):
		return "", 0, fmt.Errorf("reref connection timed out")
	}

	var resp map[string]interface{}
	if err = json.Unmarshal([]byte(r), &resp); err != nil {
		return "", 0, fmt.Errorf("error decoding reref resp: %w", err)
	}

	statusCode := int(resp["status"].(float64))
	if statusCode != 200 {
		return "", 0, fmt.Errorf("reref error status: %d", statusCode)
	}

	return resp["ref"].(string), time.Duration(resp["ttl"].(float64)) * time.Millisecond, nil
}

// GetClientVersion returns WhatsApp client version
func (wac *Conn) GetClientVersion() []int {
	return waVersion
}

func (wac *Conn) Login(qrChan chan<- string, ctx context.Context) (Session, JID, error) {
	session := Session{}
	//Makes sure that only a single Login or Restore can happen at the same time
	if !atomic.CompareAndSwapUint32(&wac.sessionLock, 0, 1) {
		return session, "", ErrLoginInProgress
	}
	wac.sessionWait.Add(1)
	defer wac.sessionWait.Done()
	defer atomic.StoreUint32(&wac.sessionLock, 0)

	if wac.loggedIn {
		return session, "", ErrAlreadyLoggedIn
	}

	if err := wac.connect(); err != nil && err != ErrAlreadyConnected {
		return session, "", err
	}

	//logged in?!?
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return session, "", ErrSessionExists
	}

	clientID := make([]byte, 16)
	_, err := rand.Read(clientID)
	if err != nil {
		return session, "", fmt.Errorf("error creating random ClientID: %w", err)
	}

	session.ClientID = base64.StdEncoding.EncodeToString(clientID)

	priv, pub, err := curve25519.GenerateKey()
	if err != nil {
		return session, "", fmt.Errorf("error generating keys: %w", err)
	}

	//listener for Login response
	s1 := make(chan string, 1)
	wac.listener.add(s1, nil, false, "s1")

	ref, ttl, err := wac.adminInitRequest(session.ClientID)
	if err != nil {
		return session, "", err
	}
	qrChan <- fmt.Sprintf("%v,%v,%v", ref, base64.StdEncoding.EncodeToString(pub[:]), session.ClientID)

	wac.loginSessionLock.Lock()
	defer wac.loginSessionLock.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	var resp []json.RawMessage
	maxRetries := 6
Loop:
	for {
		select {
		case r1 := <-s1:
			if err := json.Unmarshal([]byte(r1), &resp); err != nil {
				return session, "", fmt.Errorf("error decoding qr code resp: %w", err)
			}
			break Loop
		case <-time.After(ttl):
			maxRetries--
			if maxRetries < 0 {
				return session, "", ErrLoginTimedOut
			}
			ref, ttl, err = wac.adminRerefRequest()
			if err != nil {
				return session, "", err
			}
			qrChan <- fmt.Sprintf("%v,%v,%v", ref, base64.StdEncoding.EncodeToString(pub[:]), session.ClientID)
		case <-ctx.Done():
			return session, "", ErrLoginCancelled
		}
	}

	var msgType JSONMessageType
	err = json.Unmarshal(resp[0], &msgType)
	if err != nil {
		return session, "", fmt.Errorf("error decoding qr code response type: %w", err)
	}
	var info ConnInfo
	err = json.Unmarshal(resp[1], &info)
	if err != nil {
		return session, "", fmt.Errorf("error decoding qr code response data: %w", err)
	}

	session.ClientToken = info.ClientToken
	session.ServerToken = info.ServerToken
	session.Wid = info.WID
	decodedSecret, err := base64.StdEncoding.DecodeString(info.Secret)
	if err != nil {
		return session, "", fmt.Errorf("error decoding secret: %w", err)
	}

	var pubKey [32]byte
	copy(pubKey[:], decodedSecret[:32])

	sharedSecret := curve25519.GenerateSharedSecret(*priv, pubKey)

	hash := sha256.New

	nullKey := make([]byte, 32)
	h := hmac.New(hash, nullKey)
	h.Write(sharedSecret)

	sharedSecretExtended, err := hkdf.Expand(h.Sum(nil), 80, "")
	if err != nil {
		return session, "", fmt.Errorf("hkdf error: %w", err)
	}

	//login validation
	checkSecret := make([]byte, 112)
	copy(checkSecret[:32], decodedSecret[:32])
	copy(checkSecret[32:], decodedSecret[64:])
	h2 := hmac.New(hash, sharedSecretExtended[32:64])
	h2.Write(checkSecret)
	if !hmac.Equal(h2.Sum(nil), decodedSecret[32:64]) {
		return session, "", ErrAbortLogin
	}

	keysEncrypted := make([]byte, 96)
	copy(keysEncrypted[:16], sharedSecretExtended[64:])
	copy(keysEncrypted[16:], decodedSecret[64:])

	keyDecrypted, err := cbc.Decrypt(sharedSecretExtended[:32], nil, keysEncrypted)
	if err != nil {
		return session, "", fmt.Errorf("error decryptAes: %w", err)
	}

	session.EncKey = keyDecrypted[:32]
	session.MacKey = keyDecrypted[32:64]
	wac.session = &session
	wac.loggedIn = true

	return session, info.WID, nil
}

func (wac *Conn) SetSession(session Session) {
	if !wac.loggedIn {
		wac.session = &session
	}
}

func (wac *Conn) Restore(takeover bool, ctx context.Context) error {
	//Makes sure that only a single Login or Restore can happen at the same time
	if !atomic.CompareAndSwapUint32(&wac.sessionLock, 0, 1) {
		return ErrLoginInProgress
	}
	wac.sessionWait.Add(1)
	defer wac.sessionWait.Done()
	defer atomic.StoreUint32(&wac.sessionLock, 0)

	if wac.session == nil {
		return ErrInvalidSession
	}

	if err := wac.connect(); err != nil && err != ErrAlreadyConnected {
		return err
	}

	if wac.loggedIn {
		return ErrAlreadyLoggedIn
	}

	var initChan <-chan string
	var err error
	if !wac.adminInited {
		//admin init
		init := []interface{}{"admin", "init", waVersion, []string{wac.longClientName, wac.shortClientName, wac.clientVersion}, wac.session.ClientID, true}
		initChan, err = wac.writeJSON(init)
		if err != nil {
			return fmt.Errorf("error writing admin init: %w", err)
		}
	}

	restoreType := "reconnect"
	if takeover {
		restoreType = "takeover"
	}
	login := []interface{}{"admin", "login", wac.session.ClientToken, wac.session.ServerToken, wac.session.ClientID, restoreType}
	loginChan, retry, err := wac.writeJSONRetry(login, false)
	if err != nil {
		return fmt.Errorf("error writing admin login: %w", err)
	}

	if !wac.adminInited {
		select {
		case r := <-initChan:
			resp := StatusResponse{RequestType: "init"}
			if err = json.Unmarshal([]byte(r), &resp); err != nil {
				return fmt.Errorf("error decoding login connResp: %w", err)
			} else if resp.Status != 200 {
				wac.timeTag = ""
				return resp
			}
			wac.adminInited = true
		case <-time.After(wac.msgTimeout):
			wac.timeTag = ""
			return ErrRestoreSessionInitTimeout
		case <-wac.ws.ctx.Done():
			return ErrWebsocketClosedBeforeLogin
		}

		resends := wac.listener.getResendables()
		for _, waiter := range resends {
			wac.log.Debugln("Resending request", waiter.tags[0])
			err = waiter.resend()
			if err != nil {
				wac.log.Warnfln("Failed to resend %s: %v", waiter.tags[0], err)
			}
		}
	}

	retryCounter := 0

	//check for login 200 --> login success
Loop:
	for {
		select {
		case r := <-loginChan:
			resp := StatusResponse{RequestType: "admin login"}
			if err = json.Unmarshal([]byte(r), &resp); err != nil {
				wac.timeTag = ""
				return fmt.Errorf("error decoding login connResp: %w", err)
			} else if resp.Status != 200 {
				wac.timeTag = ""
				return fmt.Errorf("admin login errored: %w", wac.getAdminLoginResponseError(resp))
			} else {
				break Loop
			}
		case <-time.After(wac.msgTimeout):
			retryCounter++
			err = retry()
			if err != nil {
				return fmt.Errorf("failed to send login retry (#%d): %w", retryCounter, err)
			}
		case <-ctx.Done():
			wac.timeTag = ""
			return fmt.Errorf("login context finished: %w", ctx.Err())
		case <-wac.ws.ctx.Done():
			return ErrWebsocketClosedBeforeLogin
		}
	}

	wac.loggedIn = true

	return nil
}

func (wac *Conn) getAdminLoginResponseError(resp StatusResponse) error {
	switch resp.Status {
	case 400:
		return ErrBadRequest
	case 401:
		return ErrUnpaired
	case 403:
		return fmt.Errorf("%w - tos: %d", ErrAccessDenied, resp.TermsOfService)
	case 405:
		return ErrLoggedIn
	case 409:
		return ErrReplaced
	}
	return fmt.Errorf("%d (unknown error)", status)
}

func (wac *Conn) resolveChallenge(challenge string) error {
	decoded, err := base64.StdEncoding.DecodeString(challenge)
	if err != nil {
		return err
	}

	h2 := hmac.New(sha256.New, wac.session.MacKey)
	h2.Write(decoded)

	ch := []interface{}{"admin", "challenge", base64.StdEncoding.EncodeToString(h2.Sum(nil)), wac.session.ServerToken, wac.session.ClientID}
	challengeChan, err := wac.writeJSON(ch)
	if err != nil {
		return fmt.Errorf("error writing challenge: %w", err)
	}

	select {
	case r := <-challengeChan:
		resp := StatusResponse{RequestType: "login challenge"}
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			return fmt.Errorf("error decoding login resp: %w", err)
		} else if resp.Status != 200 {
			return resp
		}
	case <-time.After(wac.msgTimeout):
		return fmt.Errorf("connection timed out")
	}

	return nil
}

/*
Logout is the function to logout from a WhatsApp session. Logging out means invalidating the current session.
The session can not be resumed and will disappear on your phone in the WhatsAppWeb client list.
*/
func (wac *Conn) Logout() error {
	login := []interface{}{"admin", "Conn", "disconnect"}
	_, err := wac.writeJSON(login)
	if err != nil {
		return fmt.Errorf("error writing logout: %w", err)
	}

	return nil
}
