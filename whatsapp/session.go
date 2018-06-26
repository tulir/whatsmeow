package whatsapp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/curve25519"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
	"time"
)

type Session struct {
	ClientId    string
	ClientToken string
	ServerToken string
	EncKey      []byte
	MacKey      []byte
}

func (wac *conn) Login() (*Session, error) {
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return nil, fmt.Errorf("already logged in")
	}

	wac.session = new(Session)

	clientId := make([]byte, 16)
	_, err := rand.Read(clientId)
	if err != nil {
		return nil, fmt.Errorf("error creating random ClientId: %v", err)
	}

	wac.session.ClientId = base64.StdEncoding.EncodeToString(clientId)
	//oldVersion=8691
	login := []interface{}{"admin", "init", []int{0, 2, 9229}, []string{"Windows 10", "Chrome"}, wac.session.ClientId, true}
	loginChan, err := wac.write(login)
	if err != nil {
		return nil, fmt.Errorf("error writing login: %v\n", err)
	}

	var r string
	select {
	case r = <-loginChan:
	case <-time.After(wac.msgTimeout):
		return nil, fmt.Errorf("login connection timed out")
	}

	var resp map[string]interface{}
	if err = json.Unmarshal([]byte(r), &resp); err != nil {
		return nil, fmt.Errorf("error decoding login resp: %v\n", err)
	}

	ref := resp["ref"].(string)

	priv, pub, err := curve25519.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("error generating keys: %v\n", err)
	}

	//listener for Login response
	messageTag := "s1"
	wac.listener[messageTag] = make(chan string, 1)

	qrData := fmt.Sprintf("%v,%v,%v", ref, base64.StdEncoding.EncodeToString(pub[:]), wac.session.ClientId)

	obj := qrcodeTerminal.New()
	obj.Get(qrData).Print()

	var resp2 []interface{}
	select {
	case r1 := <-wac.listener[messageTag]:
		if err := json.Unmarshal([]byte(r1), &resp2); err != nil {
			return nil, fmt.Errorf("error decoding qr code resp: %v", err)
		}
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("qr code scan timed out")
	}
	wac.session.ClientToken = resp2[1].(map[string]interface{})["clientToken"].(string)
	wac.session.ServerToken = resp2[1].(map[string]interface{})["serverToken"].(string)
	s := resp2[1].(map[string]interface{})["secret"].(string)
	decodedSecret, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("error decoding secret: %v", err)
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
		return nil, fmt.Errorf("hkdf error: %v", err)
	}

	//login validation
	checkSecret := make([]byte, 112)
	copy(checkSecret[:32], decodedSecret[:32])
	copy(checkSecret[32:], decodedSecret[64:])
	h2 := hmac.New(hash, sharedSecretExtended[32:64])
	h2.Write(checkSecret)
	if !hmac.Equal(h2.Sum(nil), decodedSecret[32:64]) {
		return nil, fmt.Errorf("abort login")
	}

	keysEncrypted := make([]byte, 96)
	copy(keysEncrypted[:16], sharedSecretExtended[64:])
	copy(keysEncrypted[16:], decodedSecret[64:])

	keyDecrypted, err := cbc.Decrypt(sharedSecretExtended[:32], nil, keysEncrypted)
	if err != nil {
		return nil, fmt.Errorf("error decryptAes: %v", err)
	}

	wac.session.EncKey = keyDecrypted[:32]
	wac.session.MacKey = keyDecrypted[32:64]

	return wac.session, nil
}

func (wac *conn) RestoreSession(session *Session) (*Session, error) {
	if wac.session != nil && (wac.session.EncKey != nil || wac.session.MacKey != nil) {
		return nil, fmt.Errorf("already logged in")
	}

	wac.session = session

	//listener for conn or challenge; s1 is not allowed to drop
	wac.listener["s1"] = make(chan string, 1)

	//admin init
	init := []interface{}{"admin", "init", []int{0, 2, 9229}, []string{"Windows 10", "Chrome"}, wac.session.ClientId, true}
	initChan, err := wac.write(init)
	if err != nil {
		return nil, fmt.Errorf("error writing admin init: %v\n", err)
	}

	//admin login with takeover
	login := []interface{}{"admin", "login", wac.session.ClientToken, wac.session.ServerToken, wac.session.ClientId, "takeover"}
	loginChan, err := wac.write(login)
	if err != nil {
		return nil, fmt.Errorf("error writing admin login: %v\n", err)
	}

	select {
	case r := <-initChan:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			return nil, fmt.Errorf("error decoding login connResp: %v\n", err)
		}

		if int(resp["status"].(float64)) != 200 {
			return nil, fmt.Errorf("init responded with %d", resp["status"])
		}
	case <-time.After(wac.msgTimeout):
		return nil, fmt.Errorf("restore session init timed out")
	}

	//wait for s1
	var connResp []interface{}
	select {
	case r1 := <-wac.listener["s1"]:
		if err := json.Unmarshal([]byte(r1), &connResp); err != nil {
			return nil, fmt.Errorf("error decoding s1 message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return nil, fmt.Errorf("restore session connection timed out")
	}

	//check if challenge is present
	if len(connResp) == 2 && connResp[0] == "Cmd" && connResp[1].(map[string]interface{})["type"] == "challenge" {
		wac.listener["s2"] = make(chan string, 1)

		if err := wac.resolveChallenge(connResp[1].(map[string]interface{})["challenge"].(string)); err != nil {
			return nil, fmt.Errorf("error resolving challenge: %v\n", err)
		}

		select {
		case r := <-wac.listener["s2"]:
			if err := json.Unmarshal([]byte(r), &connResp); err != nil {
				return nil, fmt.Errorf("error decoding s2 message: %v\n", err)
			}
		case <-time.After(wac.msgTimeout):
			return nil, fmt.Errorf("restore session challenge timed out")
		}
	}

	//check for login 200 --> login success
	select {
	case r := <-loginChan:
		var resp map[string]interface{}
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			return nil, fmt.Errorf("error decoding login connResp: %v\n", err)
		}

		if int(resp["status"].(float64)) != 200 {
			return nil, fmt.Errorf("admin login responded with %d", resp["status"])
		}
	case <-time.After(wac.msgTimeout):
		return nil, fmt.Errorf("restore session login timed out")
	}

	//set new tokens
	wac.session.ClientToken = connResp[1].(map[string]interface{})["clientToken"].(string)
	wac.session.ServerToken = connResp[1].(map[string]interface{})["serverToken"].(string)

	return wac.session, nil
}

func (wac *conn) resolveChallenge(challenge string) error {
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

func (wac *conn) Logout() error {
	login := []interface{}{"admin", "Conn", "disconnect"}
	_, err := wac.write(login)
	if err != nil {
		return fmt.Errorf("error writing logout: %v\n", err)
	}

	return nil
}
