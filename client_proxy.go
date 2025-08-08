//go:build !wasm

package whatsmeow

import (
	wanet "go.mau.fi/whatsmeow/net"
	"golang.org/x/net/proxy"
)

func (cli *Client) configureWebSocketProxy(proxyVal Proxy) {
	if gorillaDialer, ok := cli.wsDialer.(*wanet.GorillaDialer); ok {
		gorillaDialer.Proxy = proxyVal
		gorillaDialer.NetDial = nil
	}
}

func (cli *Client) configureWebSocketSOCKSProxy(px proxy.Dialer) {
	if gorillaDialer, ok := cli.wsDialer.(*wanet.GorillaDialer); ok {
		gorillaDialer.Proxy = nil
		gorillaDialer.NetDial = px.Dial
		if contextDialer, ok := px.(proxy.ContextDialer); ok {
			gorillaDialer.NetDialContext = contextDialer.DialContext
		} else {
			gorillaDialer.NetDialContext = nil
		}
	}
}
