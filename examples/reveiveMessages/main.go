package main

import (
	"encoding/gob"
	"fmt"
	"github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/Rhymen/go-whatsapp"
	"os"
	"strings"
	"time"
)

type waHandler struct{}

//HandleError needs to be implemented to be a valid WhatsApp handler
func (*waHandler) HandleError(err error) {
	fmt.Fprintf(os.Stderr, "error occoured: %v", err)
}

//Optional to be implemented. Implement HandleXXXMessage for the types you need.
func (*waHandler) HandleTextMessage(message whatsapp.TextMessage) {
	fmt.Printf("%v %v\n\t%v\n", message.Info.Timestamp, message.Info.RemoteJid, message.Text)
}

//Example for media handling. Video, Audio, Document are also possible in the same way
func (*waHandler) HandleImageMessage(message whatsapp.ImageMessage) {
	data, err := message.Download()
	if err != nil {
		return
	}
	filename := fmt.Sprintf("%v/%v.%v", os.TempDir(), message.Info.Id, strings.Split(message.Type, "/")[1])
	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		return
	}
	_, err = file.Write(data)
	if err != nil {
		return
	}
	fmt.Printf("%v %v\n\timage reveived, saved at:%v\n", message.Info.Timestamp, message.Info.RemoteJid, filename)
}

func main() {
	//create new WhatsApp connection
	wac, err := whatsapp.NewConn(5 * time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating connection: %v\n", err)
		return
	}

	//Add handler
	wac.AddHandler(&waHandler{})

	//load saved session
	session, err := readSession()
	if err == nil {
		//restore session
		session, err = wac.RestoreSession(session)
		if err != nil {
			fmt.Fprintf(os.Stderr, "restoring failed: %v\n", err)
			return
		}
	} else {
		//no saved session -> regular login
		qr := make(chan string)
		go func() {
			terminal := qrcodeTerminal.New()
			terminal.Get(<-qr).Print()
		}()
		session, err = wac.Login(qr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error during login: %v\n", err)
		}
	}

	//save session
	err = writeSession(session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error saving session: %v\n", err)
	}

	<-time.After(5 * time.Minute)
}
func readSession() (whatsapp.Session, error) {
	session := whatsapp.Session{}
	file, err := os.Open(os.TempDir() + "/whatsappSession.gob")
	if err != nil {
		return session, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)
	if err != nil {
		return session, err
	}
	return session, nil
}

func writeSession(session whatsapp.Session) error {
	file, err := os.Create(os.TempDir() + "/whatsappSession.gob")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	if err != nil {
		return err
	}
	return nil
}
