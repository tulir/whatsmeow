package whatsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	ws := wac.ws
	defer func() {
		wac.log.Debugln("Websocket keepalive loop exiting")
		ws.Done()
	}()
	for {
		if ws.pingInKeepalive > 0 {
			go wac.keepAliveAdminTest()
		}
		err := wac.sendKeepAlive(ws)
		if err != nil {
			wac.log.Errorln("keepAlive failed:", err)
			if errors.Is(err, ErrConnectionTimeout) {
				continue
			}
			// TODO consequences?
		}
		interval := rand.Intn(maxIntervalMs-minIntervalMs) + minIntervalMs
		select {
		case <-time.After(time.Duration(interval) * time.Millisecond):
		case <-ws.ctx.Done():
			return
		}
	}
}

func (wac *Conn) keepAliveAdminTest() {
	err := wac.AdminTest()
	if err != nil {
		wac.log.Warnln("Keepalive admin test failed: %v", err)
	} else {
		if wac.ws.pingInKeepalive <= 0 {
			wac.log.Infoln("Keepalive admin test successful, not pinging anymore")
		} else {
			wac.ws.pingInKeepalive--
			wac.log.Infofln("Keepalive admin test successful, stopping pings after %d more successes", wac.ws.pingInKeepalive)
		}

	}
}

func (wac *Conn) sendKeepAlive(ws *websocketWrapper) error {
	respChan := make(chan string, 1)
	wac.listener.add(respChan, "!")

	bytes := []byte("?,,")
	err := ws.write(websocket.TextMessage, bytes)
	if err != nil {
		close(respChan)
		wac.listener.remove("!")
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

func (wac *Conn) AdminTest() error {
	if !wac.connected {
		return ErrNotConnected
	}

	if !wac.loggedIn {
		return ErrNotLoggedIn
	}

	return wac.sendAdminTest()
}

/*
	When phone is unreachable, WhatsAppWeb sends ["admin","test"] time after time to try a successful contact.
	Tested with Airplane mode and no connection at all.
*/
func (wac *Conn) sendAdminTest() error {
	data := []interface{}{"admin", "test"}

	wac.log.Debugln("Sending admin test request")
	r, err := wac.writeJson(data)
	if err != nil {
		return fmt.Errorf("error sending admin test: %w", err)
	}

	var response interface{}
	var resp string

	select {
	case resp = <-r:
		if err = json.Unmarshal([]byte(resp), &response); err != nil {
			return fmt.Errorf("error decoding response message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return ErrConnectionTimeout
	}

	if respArr, ok := response.([]interface{}); ok {
		if len(respArr) == 2 && respArr[0].(string) == "Pong" && respArr[1].(bool) == true {
			return nil
		}
	}
	return fmt.Errorf("unexpected ping response: %s", resp)
}
