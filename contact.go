package whatsapp_connection

//TODO: functions probably shouldn't return a string, maybe build a struct / return json
//TODO: check for further queries
func (wac *conn) GetProfilePicThumb(jid string) (<-chan string, error) {
	return wac.query("ProfilePicThumb", jid)
}

func (wac *conn) GetStatus(jid string) (<-chan string, error) {
	return wac.query("Status", jid)
}

func (wac *conn) GetGroupMetaData(jid string) (<-chan string, error) {
	return wac.query("GroupMetadata", jid)
}

func (wac *conn) query(t string, jid string) (<-chan string, error) {
	data := []interface{}{"query", t, jid}
	ch, err := wac.write(data)
	if err != nil {
		return nil, err
	}
	return ch, nil
}
