package parsing

import (
	"git.willing.nrw/WhatsPoll/whatsapp-connection/whatsapp/binary"
	pb "git.willing.nrw/WhatsPoll/whatsapp-connection/whatsapp/binary/proto"
	"github.com/golang/protobuf/proto"
)

func unmarshalWebMessageInfo(msg []byte) (*pb.WebMessageInfo, error) {
	message := new(pb.WebMessageInfo)
	err := proto.Unmarshal(msg, message)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func unmarshalMessageArray(msgs []binary.Node) ([]interface{}, error) {
	ret := make([]interface{}, len(msgs))

	for i, msg := range msgs {
		if msg.Description == "message" {
			info, err := unmarshalWebMessageInfo(msg.Content.([]byte))
			if err != nil {
				return nil, err
			}
			ret[i] = info
		} else {
			ret[i] = msg
		}
	}

	return ret, nil
}

func Unmarshal(data []byte) (*binary.Node, error) {
	r := NewBinaryReader(data)
	n, err := r.readNode()
	if err != nil {
		return nil, err
	}

	if n != nil && n.Attributes != nil && n.Content != nil {
		n.Content, err = unmarshalMessageArray(n.Content.([]binary.Node))
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}
