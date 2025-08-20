package whatsmeow

import (
	"net/http"

	wanet "go.mau.fi/whatsmeow/net"
	"golang.org/x/net/proxy"
)

func (cli *Client) configureWebSocketProxy(proxyVal Proxy) {
	if coderDialer, ok := cli.wsDialer.(*wanet.CoderDialer); ok {
		transport := &http.Transport{
			Proxy: proxyVal,
		}
		if coderDialer.DialOptions.HTTPClient == nil {
			coderDialer.DialOptions.HTTPClient = new(http.Client)
		}
		coderDialer.DialOptions.HTTPClient.Transport = transport
	}
}

func (cli *Client) configureWebSocketSOCKSProxy(px proxy.Dialer) {
	if coderDialer, ok := cli.wsDialer.(*wanet.CoderDialer); ok {
		transport := &http.Transport{}
		if contextDialer, ok := px.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.Dial = px.Dial
		}

		if coderDialer.DialOptions.HTTPClient == nil {
			coderDialer.DialOptions.HTTPClient = new(http.Client)
		}
		coderDialer.DialOptions.HTTPClient.Transport = transport
	}
}
