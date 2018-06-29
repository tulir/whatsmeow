package whatsapp_connection

//TODO: filename? WhatsApp uses Store.Contacts for these functions
//TODO: functions probably shouldn't return a string, maybe build a struct / return json
//TODO: check for further queries
func (wac *conn) GetProfilePicThumb(jid string) (<-chan string, error) {
	data := []interface{}{"query", "ProfilePicThumb", jid}
	return wac.write(data)
}

func (wac *conn) GetStatus(jid string) (<-chan string, error) {
	data := []interface{}{"query", "Status", jid}
	return wac.write(data)
}

func (wac *conn) GetGroupMetaData(jid string) (<-chan string, error) {
	data := []interface{}{"query", "GroupMetadata", jid}
	return wac.write(data)
}

func (wac *conn) SubscribePresence(jid string) (<-chan string, error) {
	data := []interface{}{"action", "presence", "subscribe", jid}
	return wac.write(data)
}

