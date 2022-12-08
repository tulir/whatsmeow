package tickets

import (
	// internal packages
	strings "strings"
	// local packages
	env "bitaminco/support-whatsapp-bridge/src/environment"
	models "bitaminco/support-whatsapp-bridge/src/models"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

func HandleIncomingMessage(i *types.ST_G_IncomingMessage) *string {
	var (
		outgoingMessageText      string
		customerDetails          types.ST_DBR_Customer
		customerRegistrationInfo types.ST_G_CustomerRegistrationInfo
	)

	//
	// check if customer is already registered
	//
	err := models.GetCustomerTicketDetailsConversationStateForWhatsAppTicketByPhone(&i.JID.User, &customerDetails)
	if err != nil {
		if err.Error() == env.ErrorEmptyDBResponse {
			customerRegistrationInfo.IsRegistered = false
		} else {
			outgoingMessageText = errorMessageToCustomer
			return &outgoingMessageText
		}

	} else {
		customerRegistrationInfo.IsRegistered = true
	}

	// if customer not exist add new customer
	if !customerRegistrationInfo.IsRegistered {
		err := models.SaveCustomerWithPhone(&i.Subdomain, &i.JID.User, &i.CountryCode, &customerDetails)
		if err != nil {
			outgoingMessageText = errorMessageToCustomer
			return &outgoingMessageText
		}
	}

	if customerDetails.NameFirst != nil {
		customerRegistrationInfo.IsNameFirstSaved = true
	}
	if customerDetails.NameLast != nil {
		customerRegistrationInfo.IsNameLastSaved = true
	}
	if customerDetails.City != nil {
		customerRegistrationInfo.IsCitySaved = true
	}

	//
	// decision according to customer reply
	//
	if strings.ToLower(i.IncomingMessageText) == "reset" {

		/////////////////////////
		//        reset        //
		/////////////////////////

		outgoingMessageText = flowOnReset(&customerRegistrationInfo.IsRegistered, &customerDetails)

	} else if strings.ToLower(i.IncomingMessageText) == "sales" || strings.ToLower(i.IncomingMessageText) == "technical" {

		//////////////////////////
		//   ticket selection   //
		//////////////////////////

		outgoingMessageText = flowOnDepartmentSelection(&customerRegistrationInfo, i, &customerDetails)

	} else {

		/////////////////////////
		//    anything else    //
		/////////////////////////

		outgoingMessageText = flowOnElse(&customerRegistrationInfo, i, &customerDetails)
	}

	// return final message
	return &outgoingMessageText
}
