package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/gorilla/websocket"
	"time"
)

//writeJson enqueues a json message into the writeChan
func (wac *Conn) writeJson(data []interface{}) (<-chan string, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, d))

	ch, err := wac.write(websocket.TextMessage, messageTag, bytes)
	if err != nil {
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	if len(messageTag) < 2 {
		return nil, fmt.Errorf("no messageTag specified or to short")
	}

	data, err := wac.encryptBinaryMessage(node)
	if err != nil {
		return nil, err
	}

	bytes := []byte(messageTag + ",")
	bytes = append(bytes, byte(metric), byte(flag))
	bytes = append(bytes, data...)

	ch, err := wac.write(websocket.BinaryMessage, messageTag, bytes)
	if err != nil {
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) sendKeepAlive() error {
	bytes := []byte("?,,")
	_, err := wac.write(websocket.TextMessage, "", bytes)
	return err
}

func (wac *Conn) write(messageType int, answerMessageTag string, data []byte) (<-chan string, error) {
	wac.wsWriteMutex.Lock()
	defer wac.wsWriteMutex.Unlock()

	var ch chan string
	if answerMessageTag != "" {
		ch = make(chan string, 1)

		wac.listenerMutex.Lock()
		wac.listener[answerMessageTag] = ch
		wac.listenerMutex.Unlock()
	}

	if err := wac.wsConn.WriteMessage(messageType, data); err != nil {
		if answerMessageTag != "" {
			wac.listenerMutex.Lock()
			delete(wac.listener, answerMessageTag)
			wac.listenerMutex.Unlock()
		}
		return nil, fmt.Errorf("error writing to socket: %v\n", err)
	}
	return ch, nil
}

func (wac *Conn) encryptBinaryMessage(node binary.Node) (data []byte, err error) {
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, err
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
	if err != nil {
		return nil, err
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	return data, nil
}
