package main

import (
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/whatsapp"
	"os"
	"time"
)

func main() {
	wac, err := whatsapp.NewConn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating connection: %v\n", err)
		os.Exit(1)
	}

	sess := &whatsapp.Session{}
	err = readStruct("./savedSession.json", sess)
	if err == nil {
		sess, err = wac.RestoreSession(sess)
	} else {
		sess, err = wac.Login()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error logging in: %v\n", err)
		os.Exit(1)
	}
	writeStruct("./savedSession.json", sess)

	<-time.After(3 * time.Second)
	wac.AddHandler(h{})

	//text := whatsapp.TextMessage{
	//	Info: whatsapp.MessageInfo{
	//		RemoteJid: "jid",
	//	},
	//	Text: "I am Goland.",
	//}
	//err = wac.Send(text)
	//
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "%v", err)
	//}

	<-time.After(1 * time.Hour)
}

type h struct {
}

func (h) HandleError(err error) {
	fmt.Fprintf(os.Stderr, "%v", err)
}

func (h) HandleTextMessage(message whatsapp.TextMessage) {
	fmt.Println(message)
}

func (h) HandleImageMessage(message whatsapp.ImageMessage) {
	message.Download()
}

func writeStruct(filePath string, object interface{}) error {
	file, err := os.Create(filePath)
	if err == nil {
		encoder := json.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

func readStruct(filePath string, object interface{}) error {
	file, err := os.Open(filePath)
	if err == nil {
		decoder := json.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}
