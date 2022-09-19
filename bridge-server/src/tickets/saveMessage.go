package tickets

// local packages
import (
	models "bitaminco/support-whatsapp-bridge/src/models"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

///////////////////////////
//  save ticket message  //
///////////////////////////

func saveMessageInTicket(messageType string, messageToStore, attachmentURL, attachmentMimeType *string, cd *types.ST_DBR_Customer) (*types.ST_DBR_NewMessage, error) {
	//
	// save message + check if attachment
	//
	var messageSenderID string
	if messageType == "incoming" {
		messageSenderID = cd.CustomerID
	} else {
		messageSenderID = cd.EmployeeID.(string)
	}

	// set message details from db
	var ticketDetails map[string]interface{}
	var newStoredMessage types.ST_DBR_NewMessage
	ticketID, _ := cd.TicketID.(string)

	if attachmentURL != nil {
		err := models.SaveMessageAndTemporaryURLOfAttachment(
			&messageSenderID,
			&messageType,
			&ticketID,
			messageToStore,
			attachmentURL,
			attachmentMimeType,
			&newStoredMessage,
		)
		if err != nil {
			return nil, err
		}

		// TODO
		// // send to pipe
		// await serviceRabbitMQ.instances.pipe.publishMessage({
		//     ticketType: "whatsapp",
		//     attachmentID: attachmentID,
		//     attachmentMimeType: attachmentMimeType,
		//     attachmentURL: attachmentURL
		// });

	} else {
		err := models.SaveMessageWithoutAttachment(
			&messageSenderID,
			&messageType,
			&ticketID,
			messageToStore,
			&newStoredMessage,
		)
		if err != nil {
			return nil, err
		}
	}

	// get ticket details
	models.GetTicketDetailsWithLatestMessage(&ticketID, &ticketDetails)

	// TODO
	// // send message via websocket
	// for (let applicationType of ['web', 'android']) {
	//     await serviceRabbitMQ.instances.websocketSend.publishMessage({
	//         organizationID: organizationID,
	//         userType: 'employee',
	//         applicationType: applicationType,
	//         userID: employeeID,
	//         // ticket details with the only latest message
	//         message: ticketDetails
	//     });
	// };

	return &newStoredMessage, nil
}
