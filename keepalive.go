package whatsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func (wac *Conn) keepAlive(ws *websocketWrapper, minIntervalMs int, maxIntervalMs int) {
	wac.log.Debugfln("Websocket keepalive loop starting %p", ws)
	defer func() {
		wac.log.Debugfln("Websocket keepalive loop exiting %p", ws)
		ws.Done()
	}()
	for {
		if ws.pingInKeepalive > 0 {
			go wac.keepAliveAdminTest(ws)
		}
		err := wac.sendKeepAlive(ws)
		if err != nil {
			wac.log.Errorfln("Websocket keepalive for %p failed: %v", ws, err)
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

func (wac *Conn) keepAliveAdminTest(ws *websocketWrapper) {
	if wac.ws != ws {
		wac.log.Warnln("keepAliveAdminTest was called with wrong websocket wrapper (got %p, current is %p)", ws, wac.ws)
		return
	}
	err := wac.AdminTest()
	if err != nil {
		wac.log.Warnln("Keepalive admin test failed:", err)
		if errors.Is(err, ErrPingFalse) {
			wac.dispatch(err)
		}
	} else {
		if wac.ws.pingInKeepalive <= 0 {
			wac.log.Infoln("Keepalive admin test successful, not pinging anymore")
		} else {
			wac.log.Infofln("Keepalive admin test successful, stopping pings after %d more successes", wac.ws.pingInKeepalive)
			wac.ws.pingInKeepalive--
		}
	}
}

func (wac *Conn) sendKeepAlive(ws *websocketWrapper) error {
	respChan := make(chan string, 1)
	wac.listener.add(respChan, nil, false,"!")

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

	case <-ws.ctx.Done():
		return nil
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

type adminTestWait struct {
	sync.Mutex
	input chan<- string
	output []chan error
	done   bool
	result error
}

func newAdminTestWait() *adminTestWait {
	input := make(chan string, 1)
	atw := &adminTestWait{
		output: make([]chan error, 0),
		input: input,
	}
	go atw.wait(input)
	return atw
}

func (atw *adminTestWait) wait(input <-chan string) {
	atw.result = atw.handleResp(<-input)
	atw.done = true
	atw.Lock()
	for _, ch := range atw.output {
		ch <- atw.result
	}
	atw.output = nil
	atw.Unlock()
}

func (atw *adminTestWait) handleResp(resp string) error {
	var response interface{}
	if err := json.Unmarshal([]byte(resp), &response); err != nil {
		return fmt.Errorf("error decoding response message: %w", err)
	}

	if respArr, ok := response.([]interface{}); ok && len(respArr) == 2 && respArr[0].(string) == "Pong" {
		if respArr[1].(bool) == true {
			return nil
		} else {
			return ErrPingFalse
		}
	}
	return fmt.Errorf("unexpected ping response: %s", resp)
}

func (atw *adminTestWait) Listen() <-chan error {
	atw.Lock()
	ch := make(chan error, 1)
	if atw.done {
		ch <- atw.result
	} else {
		atw.output = append(atw.output, ch)
	}
	atw.Unlock()
	return ch
}

func (wac *Conn) CountTimeout() {
	if wac.ws != nil {
		wac.ws.countTimeout()
	}
}

const adminTest = `["admin","test"]`

func (wac *Conn) sendAdminTest() error {
	wac.atwLock.Lock()
	if wac.atw == nil || wac.atw.done {
		wac.atw = newAdminTestWait()
	}
	atw := wac.atw
	wac.atwLock.Unlock()

	messageTag := fmt.Sprintf("%d.--%d", time.Now().Unix(), wac.msgCount)
	// TODO clean up listeners when there are multiple admin test?
	wac.listener.add(atw.input, nil, false, messageTag)
	wac.log.Debugln("Sending admin test request with tag", messageTag)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, adminTest))
	err := wac.ws.write(websocket.TextMessage, bytes)
	if err != nil {
		return fmt.Errorf("error sending admin test: %w", err)
	}
	wac.msgCount++

	select {
	case err = <- atw.Listen():
		return err
	case <-time.After(wac.msgTimeout):
		return ErrConnectionTimeout
	}
}
