package whatsapp

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary/proto"
	"strconv"
)

func (wac *conn) Send(msg interface{}) error {
	switch m := msg.(type) {
	case TextMessage:
		return wac.sendProto(getTextProto(m))
	default:
		return fmt.Errorf("cannot match type %T, use messagetypes declared in the package", msg)
	}
}
func (wac *conn) sendProto(p *proto.WebMessageInfo) error {
	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "relay",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{p},
	}
	wac.msgCount++
	return wac.writeBinary(n, binary.MESSAGE, binary.IGNORE, p.Key.GetId())
}
