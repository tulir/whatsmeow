package composing

import (
	"fmt"
	"git.willing.nrw/WhatsPoll/whatsapp-connection/whatsapp/binary"
	pb "git.willing.nrw/WhatsPoll/whatsapp-connection/whatsapp/binary/proto"
	"github.com/golang/protobuf/proto"
)

func Marshal(n binary.Node) ([]byte, error) {
	if n.Attributes != nil && n.Content != nil {
		a, err := marshalMessageArray(n.Content.([]interface{}))
		if err != nil {
			return nil, err
		}
		n.Content = a
	}

	w := NewBinaryWriter()
	if err := w.writeNode(n); err != nil {
		return nil, err
	}

	return w.getData(), nil
}
func marshalMessageArray(c []interface{}) ([]binary.Node, error) {
	ret := make([]binary.Node, len(c))

	for i, m := range c {
		if wmi, ok := m.(*pb.WebMessageInfo); ok {
			b, err := marshalWebMessageInfo(wmi)
			if err != nil {
				return nil, nil
			}
			node := new(binary.Node)
			node.Description = "message"
			node.Attributes = nil
			node.Content = b
			ret[i] = *node
		} else {
			ret[i], ok = m.(binary.Node)
			if !ok {
				return nil, fmt.Errorf("invalid Node")
			}
		}
	}
	return ret, nil
}

func marshalWebMessageInfo(p *pb.WebMessageInfo) ([]byte, error) {
	b, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}
