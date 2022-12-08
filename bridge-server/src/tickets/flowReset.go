package tickets

// local packages
import (
	models "bitaminco/support-whatsapp-bridge/src/models"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

func flowOnReset(isCustomerRegistered *bool, customerDetails *types.ST_DBR_Customer) string {
	// check if current ticket open
	if customerDetails.TicketID != nil {
		// delete ticket and set the conversation state to 0
		err := models.ChangeTicketStatus(customerDetails.TicketID.(string), "deleted")
		if err != nil {
			return err.Error()
		}

	} else {
		// set the conversation state to 0
		err := models.SetConversationState(customerDetails.CustomerID, 0)
		if err != nil {
			return err.Error()
		}
	}

	// set message text
	if *isCustomerRegistered {
		return chatWelcomeMessage
	} else {
		// TODO : whatsAppMessageType = "quickReply";
		return chatWelcomeAfterRegistrationMessage
		// TODO : add departments
	}
}
