package types

////////////////////////////
//   input form structs   //
////////////////////////////

// send message form
type ST_Form_SendMessage struct {
	FromPhone   string `form:"fromPhone,omitempty"`
	ToPhone     string `form:"toPhone,omitempty"`
	MessageText string `form:"messageText,omitempty"`
}
