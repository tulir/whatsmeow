# go-whatsapp
Package rhymen/go-whatsapp implements the WhatsApp Web API to provide a clean interface for developers. Big thanks to all contributors of the [sigalor/whatsapp-web-reveng](https://github.com/sigalor/whatsapp-web-reveng) project. The official WhatsApp Business API was released in August 2018. You can check it out [here](https://www.whatsapp.com/business/api).

## Installation
```sh
go get github.com/Rhymen/go-whatsapp
```

## Usage
### Creating a connection
```go
import (
    whatsapp "github.com/Rhymen/go-whatsapp"
)

wac, err := whatsapp.NewConn(20 * time.Second)
```
The duration passed to the NewConn function is used to timeout login requests. If you have a bad internet connection use a higher timeout value. This function only creates a websocket connection, it does not handle authentication.

### Login
```go
qrChan := make(chan string)
go func() {
    fmt.Printf("qr code: %v\n", <-qrChan)
    //show qr code or save it somewhere to scan
}
sess, err := wac.Login(qrChan)
```
The authentication process requires you to scan the qr code, that is send through the channel, with the device you are using whatsapp on. The session struct that is returned can be saved and used to restore the login without scanning the qr code again. The qr code has a ttl of 20 seconds and the login function throws a timeout err if the time has passed or any other request fails.

### Restore
```go
newSess, err := wac.RestoreWithSession(sess)
```
The restore function needs a valid session and returns the new session that was created.

### Add message handlers
```go
type myHandler struct{}

func (myHandler) HandleError(err error) {
	fmt.Fprintf(os.Stderr, "%v", err)
}

func (myHandler) HandleTextMessage(message whatsapp.TextMessage) {
	fmt.Println(message)
}

func (myHandler) HandleImageMessage(message whatsapp.ImageMessage) {
	fmt.Println(message)
}

func (myHandler) HandleVideoMessage(message whatsapp.VideoMessage) {
	fmt.Println(message)
}

func (myHandler) HandleJsonMessage(message string) {
	fmt.Println(message)
}

wac.AddHandler(myHandler{})
```
The message handlers are all optional, you don't need to implement anything but the error handler to implement the interface. The ImageMessage and VideoMessage provide a Download function to get the media data.

### Sending text messages
```go
text := whatsapp.TextMessage{
    Info: whatsapp.MessageInfo{
        RemoteJid: "0123456789@s.whatsapp.net",
    },
    Text: "Hello Whatsapp",
}

err := wac.Send(text)
```
The message will be send over the websocket. The attributes seen above are the required ones. All other relevant attributes (id, timestamp, fromMe, status) are set if they are missing in the struct. For the time being we only support text messages, but other types are planned for the near future.

## Legal
This code is in no way affiliated with, authorized, maintained, sponsored or endorsed by WhatsApp or any of its
affiliates or subsidiaries. This is an independent and unofficial software. Use at your own risk.

## License

The MIT License (MIT)

Copyright (c) 2018

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
