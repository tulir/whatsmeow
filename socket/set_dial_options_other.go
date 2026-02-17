//go:build !js || !wasm

package socket

import (
	"net/http"
	"github.com/coder/websocket"
)

func setDialOptions(opts *websocket.DialOptions, client *http.Client, headers http.Header) {
	opts.HTTPClient = client
	opts.HTTPHeader = headers
}
