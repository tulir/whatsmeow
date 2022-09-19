package tickets

// local packages
import (
	models "bitaminco/support-whatsapp-bridge/src/models"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

func flowOnDepartmentSelection(cri *types.ST_G_CustomerRegistrationInfo, i *types.ST_G_IncomingMessage, cd *types.ST_DBR_Customer) string {
	// switch according to conversation state
	switch cd.ConversationStateWhatsapp {
	case 0:
		// set temporary department
		if cd.TemporaryTicketDepartment == nil {
			cd.TemporaryTicketDepartment = "technical"
		}

		// ask for customer first name OR last name OR city saved
		if !(cri.IsNameFirstSaved && cri.IsNameLastSaved && cri.IsCitySaved) {

			// set conversation state to 1
			err := models.SetConversationState(cd.CustomerID, 1)
			if err != nil {
				return err.Error()
			}
			// save temporary ticket department of customer
			err = models.SaveTemporaryTicket(cd)
			if err != nil {
				return err.Error()
			}

			// check for customer details column by column
			if !cri.IsNameFirstSaved {
				// TODO : whatsAppMessageType = "normal";
				return askFirstNameMessage
			}
			if cri.IsNameFirstSaved && !cri.IsNameLastSaved {
				// TODO : whatsAppMessageType = "normal";
				return askLastNameMessage
			}
			if cri.IsNameFirstSaved && cri.IsNameLastSaved && !cri.IsCitySaved {
				// TODO : whatsAppMessageType = "normal";
				return askCityMessage
			}

		} else {
			// create new ticket and assign employee
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

	case 1:
		// check for customer details column by column
		if !cri.IsNameFirstSaved {
			// TODO : whatsAppMessageType = "normal";
			return askFirstNameInvalidInputMessage
		}
		if cri.IsNameFirstSaved && !cri.IsNameLastSaved {
			// TODO : whatsAppMessageType = "normal";
			return askLastNameInvalidInputMessage
		}
		if cri.IsNameFirstSaved && cri.IsNameLastSaved && !cri.IsCitySaved {
			// TODO : whatsAppMessageType = "normal";
			return askCityInvalidInputMessage
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
		// send message of invalid input
		// ask for 1, 2, 3, 4 or 5 as input
		// TODO : whatsAppMessageType = "normal";
		return askRatingInvalidInputMessage
	}

	return errorMessageToCustomer
}
