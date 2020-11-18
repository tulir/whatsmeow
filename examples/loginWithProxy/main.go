package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Rhymen/go-whatsapp"
)

func main() {
	proxy := http.ProxyURL(&url.URL{
		Scheme: "socks5", // or http/https depending on your proxy
		Host:   "127.0.0.1:1080",
		Path:   "/",
	})
	wac, err := whatsapp.NewConnWithProxy(5*time.Second, proxy)
	if err != nil {
		panic(err)
	}

	qr := make(chan string)
	go func() {
		terminal := qrcodeTerminal.New()
		terminal.Get(<-qr).Print()
	}()

	session, err := wac.Login(qr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error during login: %v\n", err)
		return
	}
	fmt.Printf("login successful, session: %v\n", session)
}
