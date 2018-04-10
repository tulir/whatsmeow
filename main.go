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

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/curve25519"
	grqcode "github.com/skip2/go-qrcode"
	"log"
)

type WhatsAppConn struct {
	conn     *websocket.Conn
	clientId string
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

	wac := &WhatsAppConn{conn, clientIdB64}

	go wac.readPump()

	return wac, nil
}

func (wac *WhatsAppConn) Write(data []interface{}) (int64, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}

	messageTag := time.Now().Unix()

	msg := fmt.Sprintf("%d,%v", messageTag, string(d))

	err = wac.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return 0, err
	}



	return messageTag, nil
}

func (wac *WhatsAppConn) readPump() {
	defer wac.conn.Close()

	for {
		_, msg, err := wac.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}

		//TODO response

		fmt.Printf("msg: %v\n", string(msg))
	}
}

func generateCurve25519Key() (*[32]byte, *[32]byte, error) {
	var pub, priv [32]byte
	var err error

	_, err = io.ReadFull(rand.Reader, priv[:])
	if err != nil {
		return nil, nil, err
	}

	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	curve25519.ScalarBaseMult(&pub, &priv)

	return &priv, &pub, nil
}

func (wac *WhatsAppConn) createQrCode(ref, pub string) {
	qrData := fmt.Sprintf("%v,%v,%v", ref, pub, wac.clientId)
	grqcode.WriteFile(qrData, grqcode.Medium, 256, "qr.png")
}

func main() {
	wac, err := NewWhatsAppConn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	_, pub, err := generateCurve25519Key()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	wac.createQrCode("1@8CYHF+kz6eV18RbVy5taLqEtEIXmg7T5sFSEZJ8BKpj0lnkC5zYHBuQP", base64.StdEncoding.EncodeToString(pub[:]))

	login := []interface{}{"admin", "init", []int{0, 2, 8691}, []string{"Windows 10", "Chrome"}, wac.clientId, true}
	wac.Write(login)

	<-time.After(10000 * time.Millisecond)
}
