package models

// local packages
import (
	db "bitaminco/support-whatsapp-bridge/src/database"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

// change ticket status
func ChangeTicketStatus(ticketID, status string) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		SELECT *
        FROM function_handle_ticket_status_change($1, $2);
		`,
		ticketID, status,
	)
	if err != nil {
		return err
	}
	return nil
}

// create new ticket and assign employee
func CreateNewTicketAndAssignEmployee(ticketType string, c *types.ST_DBR_Customer) (*types.ST_DBR_NewTicket, error) {
	var nt types.ST_DBR_NewTicket
	err := db.Postgres.QueryRow(
		db.Context,
		`
		SELECT *
        FROM function_create_and_assign_ticket($1, $2, $3, $4);
		`, ticketType, c.TemporaryTicketDepartment, c.CustomerID, c.OrganizationID,
	).Scan(
		&nt.GeneratedTicketID,
		&nt.ChosenEmployeeID,
		&nt.TicketStartingMessage,
		&nt.OrganizationTicketCount,
	)
	if err != nil {
		return &nt, err
	}
	return &nt, nil
}

// save temporary ticket
func SaveTemporaryTicket(c *types.ST_DBR_Customer) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		INSERT INTO tickets_temporary(organization_id, customer_id, department)
		VALUES ($1, $2, $3);
		`, c.OrganizationID, c.CustomerID, c.TemporaryTicketDepartment,
	)
	if err != nil {
		return err
	}
	return nil
}

// save new message and media attachment
func SaveMessageAndTemporaryURLOfAttachment(
	messageSenderID,
	messageType,
	ticketID,
	messageText,
	temporaryURL,
	mimeType *string,
	nm *types.ST_DBR_NewMessage,
) error {
	err := db.Postgres.QueryRow(
		db.Context,
		`
		WITH new_attachment AS (
			INSERT INTO ticket_attachments(ticket_attachment_type, url_temporary, url_download, mime_type)
			VALUES ('media', $5, $5, $6)
			RETURNING ticket_attachment_id
		)
		INSERT INTO ticket_messages(
			message_sender_id,
			message_type,
			ticket_id,
			message_text,
			ticket_attachment_id
		)
		SELECT $1, $2, $3, $4, new_attachment.ticket_attachment_id
		FROM new_attachment
		RETURNING ticket_attachment_id::TEXT;
		`, messageSenderID, messageType, ticketID, messageText, temporaryURL, mimeType,
	).Scan(&nm.TicketAttachmentID)
	if err != nil {
		return err
	}
	return nil
}

// save new message
func SaveMessageWithoutAttachment(messageSenderID, messageType, ticketID, messageText *string, nm *types.ST_DBR_NewMessage) error {
	err := db.Postgres.QueryRow(
		db.Context,
		`
		INSERT INTO ticket_messages(message_sender_id, message_type, ticket_id, message_text)
		VALUES ($1, $2, $3, $4)
		RETURNING message_id, time_stamp;
		`, messageSenderID, messageType, ticketID, messageText,
	).Scan(&nm.MessageID, &nm.TimeStamp)
	if err != nil {
		return err
	}
	return nil
}

// get current ticket details with only latest message
func GetTicketDetailsWithLatestMessage(ticketID *string, td *map[string]interface{}) error {
	err := db.Postgres.QueryRow(
		db.Context,
		`
		SELECT
			JSON_BUILD_OBJECT(
				'ticketID', ticket_id,
				'employeeID', employee_id,
				'organizationID', organization_id,
				'organizationTicketCount', organization_ticket_count,
				'ticketType', ticket_type,
				'ticketDepartment', department,
				'ticketStatus', ticket_status,
				'timeStampTicketGenerated', time_stamp_ticket_generated,
				'timeStampTicketResolved', time_stamp_ticket_resolved,
				'ticketTags', tags,
				'customer', JSON_BUILD_OBJECT(
					'customerID', customer_id,
					'customerNameFirst', customer_name_first,
					'customerNameLast', customer_name_last,
					'customerCity', customer_city,
					'customerPhone', customer_phone,
					'customerCountryCode', customer_country_code,
					'customerEmail', customer_email,
					'customerComment', customer_comment,
					'customerTag', customer_tag,
					'customerTelegramUsername', customer_telegram_username,
					'timeStampCustomerCreated', time_stamp_customer_created
				),
				'messages', ARRAY[
					JSON_BUILD_OBJECT(
						'nameMessageSender', (
							CASE
								WHEN message_type IN('outgoing', 'note', 'assignment', 'catalogue-outgoing')
									THEN CONCAT(employee_name_first, ' ', employee_name_last)
								WHEN message_type IN('incoming', 'catalogue-incoming')
									THEN CONCAT(customer_name_first, ' ', customer_name_last)
							END
						),
						'messageID', message_id,
						'messageType', message_type,
						'messageText', message_text,
						'timeStamp', time_stamp,
						'ticketAttachmentID', ticket_attachment_id,
						'ticketAttachmentType', ticket_attachment_type,
						'productArray', product_array,
						'quantityArray', quantity_array,
						'amountArray', amount_array,
						'totalAmount', total_amount,
						'URLDownload', url_download,
						'mimeType', mime_type
					)
				]
			) AS ticket
		FROM view_ticket_list_with_latest_message
		WHERE ticket_id = $1;
		`, ticketID,
	).Scan(&td)
	if err != nil {
		return err
	}
	return nil
}

/////////////////////////
//	      rating       //
/////////////////////////

// save rating
func SaveTicketRating(customerID, rating *string) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		INSERT INTO ticket_ratings(ticket_id, rating)
		SELECT ticket_id, $2
		FROM tickets
		WHERE
			customer_id = $1 AND
			ticket_status = 'closed'
		ORDER BY time_stamp_ticket_resolved DESC
		LIMIT 1;
		`,
		customerID, rating,
	)
	if err != nil {
		return err
	}
	return nil
}
