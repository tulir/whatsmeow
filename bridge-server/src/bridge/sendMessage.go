package bridge

import (
	// external packages
	waProto "go.mau.fi/whatsmeow/binary/proto"
	types "go.mau.fi/whatsmeow/types"
	proto "google.golang.org/protobuf/proto"
	// local packages
	env "bitaminco/support-whatsapp-bridge/src/environment"
)

// Send Message
func SendWhatsappMessage(fromPhone, toPhone, message *string) *string {
	// get client
	meowClient, found := mapAllClients[*fromPhone]
	if !found {
		invalidResponse := "Invalid from phone number."
		return &invalidResponse
	}

	// encode the data
	recipient, _ := types.ParseJID(*toPhone + "@s.whatsapp.net")
	messageText := &waProto.Message{Conversation: proto.String(*message)}

	//
	// send message
	//
	go func() { _, err = meowClient.SendMessage(recipient, "", messageText) }()
	go func() {
		env.InfoLogger.Println("Message Sent: @@@", fromPhone, ">>>>>> @@@", toPhone, "with message: ", message)
	}()
	if err != nil {
		tryAgainResponse := "Something went wrong! Try Again."
		return &tryAgainResponse
	}
	emptyResponse := ""
	return &emptyResponse
}
