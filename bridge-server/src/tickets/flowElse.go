package tickets

// local packages
import (
	models "bitaminco/support-whatsapp-bridge/src/models"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

func flowOnElse(cri *types.ST_G_CustomerRegistrationInfo, i *types.ST_G_IncomingMessage, cd *types.ST_DBR_Customer) string {
	// switch according to conversation state
	switch cd.ConversationStateWhatsapp {
	case 0:
		// set temporary department
		// set message text
		if cri.IsRegistered {
			return chatWelcomeMessage
		} else {
			// TODO : whatsAppMessageType = "quickReply";
			return chatWelcomeAfterRegistrationMessage
			// TODO : add departments
		}

	case 1:
		// check for customer details column by column
		if !cri.IsNameFirstSaved {
			// save first name
			models.UpdateCustomerFirstNameByID(&cd.CustomerID, &i.IncomingMessageText)
			// TODO : whatsAppMessageType = "normal";
			return askLastNameMessage
		}
		if cri.IsNameFirstSaved && !cri.IsNameLastSaved {
			// save last name
			models.UpdateCustomerLastNameByID(&cd.CustomerID, &i.IncomingMessageText)
			// TODO : whatsAppMessageType = "normal";
			return askCityMessage
		}
		if cri.IsNameFirstSaved && cri.IsNameLastSaved && !cri.IsCitySaved {
			// save city
			models.UpdateCustomerCityByID(&cd.CustomerID, &i.IncomingMessageText)

			//
			// create new ticket and assign employee
			//
			newGeneratedTicket, err := models.CreateNewTicketAndAssignEmployee("whatsapp", cd)
			if err != nil {
				return err.Error()
			}

			cd.EmployeeID = newGeneratedTicket.ChosenEmployeeID
			cd.TicketID = newGeneratedTicket.GeneratedTicketID

			// store message object
			_, err = saveMessageInTicket(
				"outgoing",
				&newGeneratedTicket.TicketStartingMessage,
				&i.IncomingAttachmentURL,
				&i.IncomingAttachmentMimeType,
				cd,
			)
			if err != nil {
				return err.Error()
			}

			return newGeneratedTicket.TicketStartingMessage
		}

	case 2:
		// store message object
		_, err := saveMessageInTicket(
			"incoming",
			&i.IncomingMessageText,
			&i.IncomingAttachmentURL,
			&i.IncomingAttachmentMimeType,
			cd,
		)
		if err != nil {
			return err.Error()
		}

	case 3:
		checkUserInputRatingValidity := i.IncomingMessageText == "1" || i.IncomingMessageText == "2" ||
			i.IncomingMessageText == "3" || i.IncomingMessageText == "4" || i.IncomingMessageText == "5"

		if checkUserInputRatingValidity {
			// save rating to last closed ticket
			models.SaveTicketRating(&cd.CustomerID, &i.IncomingMessageText)

			// set conversation state to 0
			models.SetConversationState(cd.CustomerID, 0)

			// thanking message after rating
			// TODO : whatsAppMessageType = "normal";
			return thankYouAfterRatingMessage
		} else {
			// send message of invalid input
			// ask for 1, 2, 3, 4 or 5 as input
			// TODO : whatsAppMessageType = "normal";
			return askRatingInvalidInputMessage
		}
	}

	return errorMessageToCustomer
}
