//go:build js && wasm

package socket

import (
	"net/http"
	"github.com/coder/websocket"
)

func setDialOptions(opts *websocket.DialOptions, client *http.Client, headers http.Header) {
	// HTTPClient and HTTPHeader are not available on js/wasm
}
