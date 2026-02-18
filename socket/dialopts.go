//go:build !js

package socket

import (
	"github.com/coder/websocket"
)

func (fs *FrameSocket) makeDialOptions() *websocket.DialOptions {
	return &websocket.DialOptions{
		HTTPClient: fs.HTTPClient,
		HTTPHeader: fs.HTTPHeaders,
	}
}
