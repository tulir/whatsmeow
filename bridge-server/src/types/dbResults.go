package types

// internal packages
import time "time"

///////////////////////////
//   db result structs   //
///////////////////////////

// customer details
type ST_DBR_Customer struct {
	Phone                     int
	CountryID                 int
	CustomerID                string
	OrganizationID            string
	NameFirst                 interface{}
	NameLast                  interface{}
	City                      interface{}
	Tag                       interface{}
	Comment                   interface{}
	TicketID                  interface{}
	EmployeeID                interface{}
	TemporaryTicketDepartment interface{}
	ConversationStateWhatsapp int
}

// new ticket details
type ST_DBR_NewTicket struct {
	GeneratedTicketID       string
	ChosenEmployeeID        string
	TicketStartingMessage   string
	OrganizationTicketCount int
}

// new message details
type ST_DBR_NewMessage struct {
	MessageID          string
	TicketAttachmentID string
	TimeStamp          time.Time
}
