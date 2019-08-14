package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Rhymen/go-whatsapp"
)

type requestBody struct {
	Text string `json:"text"`
}

type version struct {
	major int
	minor int
	patch int
}

func main() {
	log.SetFlags(0)

	// Place here your slack webhook url
	var webhookUrl = ""

	wac, err := whatsapp.NewConn(5 * time.Second)
	if err != nil {
		panic(err)
	}

	v := wac.GetClientVersion()
	clientVersion := &version{major: v[0], minor: v[1], patch: v[2]}

	fmt.Printf("Client has version %d.%d.%d\n", clientVersion.major, clientVersion.minor, clientVersion.patch)

	v, err = whatsapp.CheckCurrentServerVersion()
	serverVersion := &version{major: v[0], minor: v[1], patch: v[2]}
	if err != nil {
		panic(err)
	}
	fmt.Printf("Server has version %d.%d.%d\n", serverVersion.major, serverVersion.minor, serverVersion.patch)

	var report = ""

	if serverVersion.major == clientVersion.major {
		if serverVersion.minor == clientVersion.minor {
			if serverVersion.patch == clientVersion.patch {
				report = "Versions are equal"
			} else if serverVersion.patch > clientVersion.patch {
				report = fmt.Sprintf("New patch detected %d", serverVersion.patch)
			}
		} else if serverVersion.minor > clientVersion.minor {
			report = fmt.Sprintf("New minor detected %d", serverVersion.minor)
		}
	} else if serverVersion.major > clientVersion.major {
		report = fmt.Sprintf("New major detected %d", serverVersion.major)
	}

	fmt.Println(report)
	if err := sendWebHookNotification(webhookUrl, report); err != nil {
		log.Fatalln(err)
	}
}

func sendWebHookNotification(webhookUrl string, msg string) error {
	if webhookUrl == "" {
		return errors.New("You must provide webhookurl to send the notification")
	}

	slackBody, _ := json.Marshal(requestBody{Text: msg})
	req, err := http.NewRequest(http.MethodPost, webhookUrl, bytes.NewBuffer(slackBody))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "ok" {
		return errors.New("Non-ok response returned from Slack")
	}
	return nil
}
