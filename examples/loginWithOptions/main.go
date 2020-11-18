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
	// set proxy
	// or you can use *url.URL directly like loginWithProxy example
	purl, err := url.Parse("socks5://127.0.0.1/")
	if err != nil {
		panic(err)
	}
	proxy := http.ProxyURL(purl)

	// or just left it empty
	proxy = nil

	wac, err := whatsapp.NewConnWithOptions(&whatsapp.Options{
		// timeout
		Timeout: 20 * time.Second,
		Proxy:   proxy,
		// set custom client name
		ShortClientName: "My-WhatsApp-Client",
		LongClientName:  "My-WhatsApp-Clientttttttttttttt",
	})
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
