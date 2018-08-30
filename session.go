package whatsapp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/curve25519"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
)

/*
Session contains session individual information. To be able to resume the connection without scanning the qr code
every time you should save the Session returned by Login and use RestoreSession the next time you want to login.
Every successful created connection returns a new Session. The Session(ClientToken, ServerToken) is altered after
every re-login and should be saved every time.
*/
type Session struct {
	ClientId    string
	ClientToken string
	ServerToken string
	EncKey      []byte
	MacKey      []byte
	Wid         string
}

type Info struct {
	Battery   int
	Platform  string
	Connected bool
	Pushname  string
	Wid       string
	Lc        string
	Phone     *PhoneInfo
	Plugged   bool
	Tos       int
	Lg        string
	Is24h     bool
}

type PhoneInfo struct {
	Mcc                string
	Mnc                string
	OsVersion          string
	DeviceManufacturer string
	DeviceModel        string
	OsBuildNumber      string
	WaVersion          string
}

func newInfoFromReq(info map[string]interface{}) *Info {
	phoneInfo := info["phone"].(map[string]interface{})

	ret := &Info{
		Battery:   int(info["battery"].(float64)),
		Platform:  info["platform"].(string),
		Connected: info["connected"].(bool),
		Pushname:  info["pushname"].(string),
		Wid:       info["wid"].(string),
		Lc:        info["lc"].(string),
		Phone: &PhoneInfo{
			phoneInfo["mcc"].(string),
			phoneInfo["mnc"].(string),
			phoneInfo["os_version"].(string),
			phoneInfo["device_manufacturer"].(string),
			phoneInfo["device_model"].(string),
			phoneInfo["os_build_number"].(string),
			phoneInfo["wa_version"].(string),
		},
		Plugged: info["plugged"].(bool),
		Lg:      info["lg"].(string),
		Tos:     int(info["tos"].(float64)),
	}

	if is24h, ok := info["is24h"]; ok {
		ret.Is24h = is24h.(bool)
	}

	return ret
}

/*
SetClientName sets the long and short client names that are sent to WhatsApp when logging in and displayed in the
WhatsApp Web device list. As the values are only sent when logging in, changing them after logging in is not possible.
*/
func (wac *Conn) SetClientName(long, short string) error {
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return fmt.Errorf("cannot change client name after logging in")
	}
	wac.longClientName, wac.shortClientName = long, short
	return nil
}

/*
Login is the function that creates a new whatsapp session and logs you in. If you do not want to scan the qr code
every time, you should save the returned session and use RestoreSession the next time. Login takes a writable channel
as an parameter. This channel is used to push the data represented by the qr code back to the user. The received data
should be displayed as an qr code in a way you prefer. To print a qr code to console you can use:
github.com/Baozisoftware/qrcode-terminal-go Example login procedure:
	wac, err := whatsapp.NewConn(5 * time.Second)
	if err != nil {
		panic(err)
	}

	qr := make(chan string)
	go func() {
		terminal := qrcodeTerminal.New()
		terminal.Get(<-qr).Print()
	}()

	session, err := wac.Login(qr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error during login: %v\n", err)
	}
	fmt.Printf("login successful, session: %v\n", session)
*/
func (wac *Conn) Login(qrChan chan<- string) (Session, error) {
	session := Session{}

	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return session, fmt.Errorf("already logged in")
	}

	clientId := make([]byte, 16)
	_, err := rand.Read(clientId)
	if err != nil {
		return session, fmt.Errorf("error creating random ClientId: %v", err)
	}

	session.ClientId = base64.StdEncoding.EncodeToString(clientId)
	//oldVersion=8691
	login := []interface{}{"admin", "init", []int{0, 3, 225}, []string{wac.longClientName, wac.shortClientName}, session.ClientId, true}
	loginChan, err := wac.write(login)
	if err != nil {
		return session, fmt.Errorf("error writing login: %v\n", err)
	}

	var r string
	select {
	case r = <-loginChan:
	case <-time.After(wac.msgTimeout):
		return session, fmt.Errorf("login connection timed out")
	}

	var resp map[string]interface{}
	if err = json.Unmarshal([]byte(r), &resp); err != nil {
		return session, fmt.Errorf("error decoding login resp: %v\n", err)
	}

	ref := resp["ref"].(string)

	priv, pub, err := curve25519.GenerateKey()
	if err != nil {
		return session, fmt.Errorf("error generating keys: %v\n", err)
	}

	//listener for Login response
	messageTag := "s1"
	wac.listener[messageTag] = make(chan string, 1)

	qrChan <- fmt.Sprintf("%v,%v,%v", ref, base64.StdEncoding.EncodeToString(pub[:]), session.ClientId)

	var resp2 []interface{}
	select {
	case r1 := <-wac.listener[messageTag]:
		if err := json.Unmarshal([]byte(r1), &resp2); err != nil {
			return session, fmt.Errorf("error decoding qr code resp: %v", err)
		}
	case <-time.After(time.Duration(resp["ttl"].(float64)) * time.Millisecond):
		return session, fmt.Errorf("qr code scan timed out")
	}

	info := resp2[1].(map[string]interface{})

	wac.Info = newInfoFromReq(info)

	session.ClientToken = info["clientToken"].(string)
	session.ServerToken = info["serverToken"].(string)
	session.Wid = info["wid"].(string)
	s := info["secret"].(string)
	decodedSecret, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return session, fmt.Errorf("error decoding secret: %v", err)
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
		return session, fmt.Errorf("hkdf error: %v", err)
	}

	//login validation
	checkSecret := make([]byte, 112)
	copy(checkSecret[:32], decodedSecret[:32])
	copy(checkSecret[32:], decodedSecret[64:])
	h2 := hmac.New(hash, sharedSecretExtended[32:64])
	h2.Write(checkSecret)
	if !hmac.Equal(h2.Sum(nil), decodedSecret[32:64]) {
		return session, fmt.Errorf("abort login")
	}

	keysEncrypted := make([]byte, 96)
	copy(keysEncrypted[:16], sharedSecretExtended[64:])
	copy(keysEncrypted[16:], decodedSecret[64:])

	keyDecrypted, err := cbc.Decrypt(sharedSecretExtended[:32], nil, keysEncrypted)
	if err != nil {
		return session, fmt.Errorf("error decryptAes: %v", err)
	}

	session.EncKey = keyDecrypted[:32]
	session.MacKey = keyDecrypted[32:64]
	wac.session = &session

	return session, nil
}

/*
RestoreSession is the function that restores a given session. It will try to reestablish the connection to the
WhatsAppWeb servers with the provided session. If it succeeds it will return a new session. This new session has to be
saved because the Client and Server-Token will change after every login. Logging in with old tokens is possible, but not
suggested. If so, a challenge has to be resolved which is just another possible point of failure.
*/
func (wac *Conn) RestoreSession(session Session) (Session, error) {
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return Session{}, fmt.Errorf("already logged in")
	}

	wac.session = &session

	//listener for Conn or challenge; s1 is not allowed to drop
	wac.listener["s1"] = make(chan string, 1)

	//admin init
	init := []interface{}{"admin", "init", []int{0, 3, 225}, []string{wac.longClientName, wac.shortClientName}, session.ClientId, true}
	initChan, err := wac.write(init)
	if err != nil {
		wac.session = nil
		return Session{}, fmt.Errorf("error writing admin init: %v\n", err)
	}

	//admin login with takeover
	login := []interface{}{"admin", "login", session.ClientToken, session.ServerToken, session.ClientId, "takeover"}
	loginChan, err := wac.write(login)
	if err != nil {
		wac.session = nil
		return Session{}, fmt.Errorf("error writing admin login: %v\n", err)
	}

	select {
	case r := <-initChan:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			wac.session = nil
			return Session{}, fmt.Errorf("error decoding login connResp: %v\n", err)
		}

		if int(resp["status"].(float64)) != 200 {
			wac.session = nil
			return Session{}, fmt.Errorf("init responded with %d", resp["status"])
		}
	case <-time.After(wac.msgTimeout):
		wac.session = nil
		return Session{}, fmt.Errorf("restore session init timed out")
	}

	//wait for s1
	var connResp []interface{}
	select {
	case r1 := <-wac.listener["s1"]:
		if err := json.Unmarshal([]byte(r1), &connResp); err != nil {
			wac.session = nil
			return Session{}, fmt.Errorf("error decoding s1 message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		wac.session = nil
		return Session{}, fmt.Errorf("restore session connection timed out")
	}

	//check if challenge is present
	if len(connResp) == 2 && connResp[0] == "Cmd" && connResp[1].(map[string]interface{})["type"] == "challenge" {
		wac.listener["s2"] = make(chan string, 1)

		if err := wac.resolveChallenge(connResp[1].(map[string]interface{})["challenge"].(string)); err != nil {
			wac.session = nil
			return Session{}, fmt.Errorf("error resolving challenge: %v\n", err)
		}

		select {
		case r := <-wac.listener["s2"]:
			if err := json.Unmarshal([]byte(r), &connResp); err != nil {
				wac.session = nil
				return Session{}, fmt.Errorf("error decoding s2 message: %v\n", err)
			}
		case <-time.After(wac.msgTimeout):
			wac.session = nil
			return Session{}, fmt.Errorf("restore session challenge timed out")
		}
	}

	//check for login 200 --> login success
	select {
	case r := <-loginChan:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			wac.session = nil
			return Session{}, fmt.Errorf("error decoding login connResp: %v\n", err)
		}

		if int(resp["status"].(float64)) != 200 {
			wac.session = nil
			return Session{}, fmt.Errorf("admin login responded with %d", resp["status"])
		}
	case <-time.After(wac.msgTimeout):
		wac.session = nil
		return Session{}, fmt.Errorf("restore session login timed out")
	}

	info := connResp[1].(map[string]interface{})

	wac.Info = newInfoFromReq(info)

	//set new tokens
	session.ClientToken = info["clientToken"].(string)
	session.ServerToken = info["serverToken"].(string)
	session.Wid = info["wid"].(string)

	return *wac.session, nil
}

func (wac *Conn) resolveChallenge(challenge string) error {
	decoded, err := base64.StdEncoding.DecodeString(challenge)
	if err != nil {
		return err
	}

	h2 := hmac.New(sha256.New, wac.session.MacKey)
	h2.Write([]byte(decoded))

	ch := []interface{}{"admin", "challenge", base64.StdEncoding.EncodeToString(h2.Sum(nil)), wac.session.ServerToken, wac.session.ClientId}
	challengeChan, err := wac.write(ch)
	if err != nil {
		return fmt.Errorf("error writing challenge: %v\n", err)
	}

	select {
	case r := <-challengeChan:
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(r), &resp); err != nil {
			return fmt.Errorf("error decoding login resp: %v\n", err)
		}
		if int(resp["status"].(float64)) != 200 {
			return fmt.Errorf("challenge responded with %d\n", resp["status"])
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
	_, err := wac.write(login)
	if err != nil {
		return fmt.Errorf("error writing logout: %v\n", err)
	}

	return nil
}
