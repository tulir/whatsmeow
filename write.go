package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	"time"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

//writeJson enqueues a json message into the writeChan
func (wac *Conn) writeJson(data []interface{}) (<-chan string, error) {

	wac.writerLock.Lock()
	defer wac.writerLock.Unlock()

	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, d))

	if wac.timeTag == "" {
		tss := fmt.Sprintf("%d", ts)
		wac.timeTag = tss[len(tss)-3:]
	}

	ch, err := wac.write(websocket.TextMessage, messageTag, bytes)
	if err != nil {
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	if len(messageTag) < 2 {
		return nil, ErrMissingMessageTag
	}

	wac.writerLock.Lock()
	defer wac.writerLock.Unlock()

	data, err := wac.encryptBinaryMessage(node)
	if err != nil {
		return nil, errors.Wrap(err, "encryptBinaryMessage(node) failed")
	}

	bytes := []byte(messageTag + ",")
	bytes = append(bytes, byte(metric), byte(flag))
	bytes = append(bytes, data...)

	ch, err := wac.write(websocket.BinaryMessage, messageTag, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write message")
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) sendKeepAlive() error {
	bytes := []byte("?,,")
	respChan, err := wac.write(websocket.TextMessage, "!", bytes)
	if err != nil {
		return errors.Wrap(err, "error sending keepAlive")
	}

	select {
	case resp := <-respChan:
		msecs, err := strconv.ParseInt(resp, 10, 64)
		if err != nil {
			return errors.Wrap(err, "Error converting time string to uint")
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
func (wac *Conn) sendAdminTest() (bool, error) {
	data := []interface{}{"admin", "test"}

	r, err := wac.writeJson(data)
	if err != nil {
		return false, errors.Wrap(err, "error sending admin test")
	}

	var response []interface{}

	select {
	case resp := <-r:
		if err := json.Unmarshal([]byte(resp), &response); err != nil {
			return false, fmt.Errorf("error decoding response message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return false, ErrConnectionTimeout
	}

	if len(response) == 2 && response[0].(string) == "Pong" && response[1].(bool) == true {
		return true, nil
	} else {
		return false, nil
	}
}

func (wac *Conn) write(messageType int, answerMessageTag string, data []byte) (<-chan string, error) {
	var ch chan string
	if answerMessageTag != "" {
		ch = make(chan string, 1)

		wac.listener.Lock()
		wac.listener.m[answerMessageTag] = ch
		wac.listener.Unlock()
	}

	if wac == nil || wac.ws == nil {
		return nil, ErrInvalidWebsocket
	}
	wac.ws.Lock()
	err := wac.ws.conn.WriteMessage(messageType, data)
	wac.ws.Unlock()

	if err != nil {
		if answerMessageTag != "" {
			wac.listener.Lock()
			delete(wac.listener.m, answerMessageTag)
			wac.listener.Unlock()
		}
		return nil, errors.Wrap(err, "error writing to websocket")
	}
	return ch, nil
}

func (wac *Conn) encryptBinaryMessage(node binary.Node) (data []byte, err error) {
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, errors.Wrap(err, "binary node marshal failed")
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
	if err != nil {
		return nil, errors.Wrap(err, "encrypt failed")
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	return data, nil
}
