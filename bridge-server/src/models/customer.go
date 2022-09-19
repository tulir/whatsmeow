package models

// local packages
import (
	db "bitaminco/support-whatsapp-bridge/src/database"
	types "bitaminco/support-whatsapp-bridge/src/types"
)

// get customer details if exists
func GetCustomerTicketDetailsConversationStateForWhatsAppTicketByPhone(phone *string, c *types.ST_DBR_Customer) error {
	err := db.Postgres.QueryRow(
		db.Context,
		`
		SELECT
			c.customer_id,
			c.organization_id,
			c.name_first,
			c.name_last,
			c.city,
			c.tag,
			c.comment,
			t.ticket_id,
			t.employee_id,
			tt.department       AS temporary_ticket_department,
			ccs.state_whatsapp  AS conversation_state_whatsapp
		FROM customers c
		LEFT OUTER JOIN customers_conversation_state ccs
		ON ccs.customer_id = c.customer_id
		LEFT OUTER JOIN (
			SELECT customer_id, department
			FROM tickets_temporary
			WHERE is_handled = FALSE
		) tt
		ON tt.customer_id = c.customer_id
		LEFT OUTER JOIN (
			SELECT customer_id, ticket_id, employee_id
			FROM tickets
			WHERE
				ticket_type = 'whatsapp' AND
				ticket_status = 'active'
		) t
		ON t.customer_id = c.customer_id
		WHERE c.phone = $1;
		`, phone,
	).Scan(
		&c.CustomerID,
		&c.OrganizationID,
		&c.NameFirst,
		&c.NameLast,
		&c.City,
		&c.Tag,
		&c.Comment,
		&c.TicketID,
		&c.EmployeeID,
		&c.TemporaryTicketDepartment,
		&c.ConversationStateWhatsapp,
	)
	if err != nil {
		return err
	}
	return nil
}

// save customer with phone
func SaveCustomerWithPhone(subdomain, phone, countryCode *string, c *types.ST_DBR_Customer) error {
	err := db.Postgres.QueryRow(
		db.Context,
		`
		WITH new_customer AS (
			INSERT INTO customers(organization_id, phone, country_id)
			VALUES (
				(
					SELECT organization_id
					FROM organizations
					WHERE subdomain = $1
				),
				$2,
				(
					SELECT country_id
					FROM countries
					WHERE country_code = $3
				)
			)
			RETURNING customer_id
		)
		INSERT INTO customers_conversation_state(customer_id)
		SELECT new_customer.customer_id
		FROM new_customer
		RETURNING customer_id;
		`, subdomain, phone, countryCode,
	).Scan(&c.CustomerID)
	if err != nil {
		return err
	}
	return nil
}

// set conversation state
func SetConversationState(customerID string, state int) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		UPDATE customers_conversation_state
		SET state_whatsapp = $2
		WHERE customer_id = $1;
		`,
		customerID, state,
	)
	if err != nil {
		return err
	}
	return nil
}

// update first name
func UpdateCustomerFirstNameByID(customerID, firstName *string) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		UPDATE customers
		SET name_first = $2
		WHERE customer_id = $1;
		`,
		customerID, firstName,
	)
	if err != nil {
		return err
	}
	return nil
}

// update last name
func UpdateCustomerLastNameByID(customerID, lastName *string) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		UPDATE customers
		SET name_last = $2
		WHERE customer_id = $1;
		`,
		customerID, lastName,
	)
	if err != nil {
		return err
	}
	return nil
}

// update city
func UpdateCustomerCityByID(customerID, city *string) error {
	_, err := db.Postgres.Exec(
		db.Context,
		`
		UPDATE customers
		SET city = $2
		WHERE customer_id = $1;
		`,
		customerID, city,
	)
	if err != nil {
		return err
	}
	return nil
}
