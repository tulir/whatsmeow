package bridge

import (
	// internal packages
	strconv "strconv"
	// external packages
	phonenumbers "github.com/nyaruka/phonenumbers"
	events "go.mau.fi/whatsmeow/types/events"
	// local packages
	tickets "bitaminco/support-whatsapp-bridge/src/tickets"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

///////////////////////
//   handle events   //
///////////////////////

// send whatsapp message
func receiveMessageEventHandler(m *events.Message, eventReceivedPhone string) {
	// only personal message
	if !m.Info.IsGroup {
		// parse country code
		userPhone, _ := phonenumbers.Parse("+"+m.Info.Sender.User, "")
		userCountryCode := phonenumbers.GetCountryCodeForRegion(phonenumbers.GetRegionCodeForNumber(userPhone))

		// TODO : add subdomain
		// extract incoming message
		var im types.ST_G_IncomingMessage
		im.Subdomain = "imb"
		im.CountryCode = strconv.Itoa(userCountryCode)
		im.PushName = m.Info.PushName
		im.JID = m.Info.Sender

		// normal message
		if m.Message.Conversation != nil {
			im.IncomingMessageText = *m.Message.Conversation
		}

		// image message
		if m.Message.ImageMessage != nil {
			im.IncomingMessageText = *m.Message.ImageMessage.Caption
			im.IncomingAttachmentMimeType = *m.Message.ImageMessage.Mimetype
			im.IncomingAttachmentURL = *m.Message.ImageMessage.Url
		}

		/////////////////////////////////
		//   handle incoming message   //
		/////////////////////////////////

		outgoingMessageText := tickets.HandleIncomingMessage(&im)
		SendWhatsappMessage(&eventReceivedPhone, &m.Info.Sender.User, outgoingMessageText)
	}
}
