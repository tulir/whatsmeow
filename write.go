package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
)

func (wac *Conn) addListener(ch chan string, messageTag string) {
	wac.listener.Lock()
	wac.listener.m[messageTag] = ch
	wac.listener.Unlock()
}

func (wac *Conn) removeListener(answerMessageTag string) {
	wac.listener.Lock()
	delete(wac.listener.m, answerMessageTag)
	wac.listener.Unlock()
}

//writeJson enqueues a json message into the writeChan
func (wac *Conn) writeJson(data []interface{}) (<-chan string, error) {
	ch := make(chan string, 1)

	wac.writerLock.Lock()
	defer wac.writerLock.Unlock()

	d, err := json.Marshal(data)
	if err != nil {
		close(ch)
		return ch, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, d))

	if wac.timeTag == "" {
		tss := fmt.Sprintf("%d", ts)
		wac.timeTag = tss[len(tss)-3:]
	}

	wac.addListener(ch, messageTag)

	err = wac.write(websocket.TextMessage, bytes)
	if err != nil {
		close(ch)
		wac.removeListener(messageTag)
		return ch, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	ch := make(chan string, 1)

	if len(messageTag) < 2 {
		close(ch)
		return ch, ErrMissingMessageTag
	}

	wac.writerLock.Lock()
	defer wac.writerLock.Unlock()

	data, err := wac.encryptBinaryMessage(node)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("encryptBinaryMessage(node) failed: %w", err)
	}

	bytes := []byte(messageTag + ",")
	bytes = append(bytes, byte(metric), byte(flag))
	bytes = append(bytes, data...)

	wac.addListener(ch, messageTag)

	err = wac.write(websocket.BinaryMessage, bytes)
	if err != nil {
		close(ch)
		wac.removeListener(messageTag)
		return ch, fmt.Errorf("failed to write message: %w", err)
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) sendKeepAlive() error {

	respChan := make(chan string, 1)
	wac.addListener(respChan, "!")

	bytes := []byte("?,,")
	err := wac.write(websocket.TextMessage, bytes)
	if err != nil {
		close(respChan)
		wac.removeListener("!")
		return fmt.Errorf("error sending keepAlive: %w", err)
	}

	select {
	case resp := <-respChan:
		msecs, err := strconv.ParseInt(resp, 10, 64)
		if err != nil {
			return fmt.Errorf("Error converting time string to uint: %w", err)
		}
		wac.ServerLastSeen = time.Unix(msecs/1000, (msecs%1000)*int64(time.Millisecond))

	case <-time.After(wac.msgTimeout):
		return ErrConnectionTimeout
	}

	return nil
}

/*
	When phone is unreachable, WhatsAppWeb sends ["admin","test"] time after time to try a successful contact.
	Tested with Airplane mode and no connection at all.
*/
func (wac *Conn) sendAdminTest() error {
	data := []interface{}{"admin", "test"}

	r, err := wac.writeJson(data)
	if err != nil {
		return fmt.Errorf("error sending admin test: %w", err)
	}

	var response []interface{}
	var resp string

	select {
	case resp = <-r:
		if err := json.Unmarshal([]byte(resp), &response); err != nil {
			return fmt.Errorf("error decoding response message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return ErrConnectionTimeout
	}

	if len(response) == 2 && response[0].(string) == "Pong" && response[1].(bool) == true {
		return nil
	} else {
		return fmt.Errorf("unexpected ping response: %s", resp)
	}
}

func (wac *Conn) write(messageType int, data []byte) error {
	if wac == nil || wac.ws == nil {
		return ErrInvalidWebsocket
	}

	wac.ws.Lock()
	err := wac.ws.conn.WriteMessage(messageType, data)
	wac.ws.Unlock()

	if err != nil {
		return fmt.Errorf("error writing to websocket: %w", err)
	}

	return nil
}

func (wac *Conn) encryptBinaryMessage(node binary.Node) (data []byte, err error) {
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("binary node marshal failed: %w", err)
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
	if err != nil {
		return nil, fmt.Errorf("encrypt failed: %w", err)
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	return data, nil
}
