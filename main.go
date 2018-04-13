package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"math/big"
	"encoding/json"
	"time"
	"encoding/base64"
	"net/http"
	"io"
	"log"
	"strings"
	"strconv"
	"crypto/hmac"
	"crypto/sha256"
	"github.com/rhymen/go-whatsapp/aes"
	"github.com/rhymen/go-whatsapp/ecdh"

	"golang.org/x/crypto/hkdf"
	"github.com/gorilla/websocket"
	grqcode "github.com/skip2/go-qrcode"
)

type WhatsAppConn struct {
	conn     *websocket.Conn
	clientId string
	listener map[string]chan string
	encKey   []byte
}

func NewWhatsAppConn() (*WhatsAppConn, error) {
	clientId := make([]byte, 16)
	_, err := rand.Read(clientId)
	if err != nil {
		return nil, fmt.Errorf("error creating random clientId: %v", err)
	}

	clientIdB64 := base64.StdEncoding.EncodeToString(clientId)

	nBig, err := rand.Int(rand.Reader, big.NewInt(8))
	if err != nil {
		return nil, err
	}
	address := fmt.Sprintf("wss://w%d.web.whatsapp.com/ws", nBig.Int64()+1)

	dialer := &websocket.Dialer{
		ReadBufferSize:  25 * 1024 * 1024,
		WriteBufferSize: 10 * 1024 * 1024,
	}

	headers := http.Header{}
	headers.Add("Origin", "https://web.whatsapp.com")

	conn, _, err := dialer.Dial(address, headers)
	if err != nil {
		return nil, fmt.Errorf("ws dial error: %v", err)
	}

	wac := &WhatsAppConn{conn, clientIdB64, make(map[string]chan string), nil}

	go wac.readPump()

	return wac, nil
}

func (wac *WhatsAppConn) readPump() {
	defer wac.conn.Close()

	for {
		msgType, msg, err := wac.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}

		data := strings.SplitN(string(msg), ",", 2)

		if wac.listener[data[0]] != nil {
			wac.listener[data[0]] <- data[1]
			delete(wac.listener, data[0])
			fmt.Printf("[] received msg: %v\n\n", data[1])
		} else if msgType == 2 && wac.encKey != nil {
			d, err := aes.Decrypt(wac.encKey, string([]byte(data[1])[32:]))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error decryptAes data: %v\n", err)
				return
			}
			fmt.Printf("[] binary data: %s\n", d)
		} else {
			fmt.Printf("[%v] discarded msg: %v\n\n", msgType, string(msg))
		}

	}
}

func (wac *WhatsAppConn) Write(data []interface{}) (*string, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	messageTag := strconv.Itoa(int(time.Now().Unix()))
	msg := fmt.Sprintf("%s,%v", messageTag, string(d))

	wac.listener[messageTag] = make(chan string)

	err = wac.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return nil, err
	}

	resp := <-wac.listener[messageTag]

	return &resp, nil
}

func (wac *WhatsAppConn) createQrCode(ref, pub string) (*[]byte, error) {
	qrData := fmt.Sprintf("%v,%v,%v", ref, pub, wac.clientId)
	grqcode.WriteFile(qrData, grqcode.Medium, 256, "qr.png")

	messageTag := "s1"
	wac.listener[messageTag] = make(chan string)
	r := <-wac.listener[messageTag]

	var resp []interface{}
	if err := json.Unmarshal([]byte(r), &resp); err != nil {
		return nil, fmt.Errorf("error decoding qr code resp: %v", err)
	}

	s := resp[1].(map[string]interface{})["secret"].(string)
	decodedSecret, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("error decoding secret: %v", err)
	}

	return &decodedSecret, nil
}

func main() {
	wac, err := NewWhatsAppConn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating connection: %v\n", err)
		return
	}

	login := []interface{}{"admin", "init", []int{0, 2, 8691}, []string{"Windows 10", "Chrome"}, wac.clientId, true}
	r, err := wac.Write(login)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing login: %v\n", err)
		return
	}

	resp := make(map[string]interface{})
	if err = json.Unmarshal([]byte(*r), &resp); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding login resp: %v\n", err)
		return
	}

	priv, pub, err := ecdh.GenerateCurve25519Key()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating keys: %v\n", err)
		return
	}

	secret, err := wac.createQrCode(resp["ref"].(string), base64.StdEncoding.EncodeToString(pub[:]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error qr code login: %v\n", err)
		return
	}

	var pubKey [32]byte
	copy(pubKey[:32], (*secret)[:32])

	sharedSecret := ecdh.GenerateCurve25519SharedSecret(*priv, pubKey)

	hash := sha256.New

	nullKey := make([]byte, 32)
	h := hmac.New(hash, nullKey)
	h.Write(sharedSecret)

	sharedSecretExtended := make([]byte, 80)
	hkdfReader := hkdf.New(hash, sharedSecret, nil, nil)
	_, err = io.ReadFull(hkdfReader, sharedSecretExtended)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hkdf error: %v\n", err)
		return
	}

	// doesn't work, check keys
	checkSecret := make([]byte, 112)
	copy(checkSecret[:32], (*secret)[:32])
	copy(checkSecret[32:], (*secret)[64:])
	h2 := hmac.New(hash, sharedSecretExtended[32:64])
	h2.Write(checkSecret)
	//.

	keysEncrypted := make([]byte, 96)
	copy(keysEncrypted[:16], sharedSecretExtended[64:])
	copy(keysEncrypted[16:], (*secret)[64:])

	keysDecrypted, err := aes.Decrypt(sharedSecretExtended[:32], string(keysEncrypted))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decryptAes: %v\n", err)
		return
	}

	keyDec := []byte(keysDecrypted)

	wac.encKey = (keyDec)[:32]

	<-time.After(3600 * time.Second)
}
