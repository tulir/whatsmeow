package binary

import (
	"fmt"
	pb "github.com/Rhymen/go-whatsapp/binary/proto"
	"github.com/golang/protobuf/proto"
)

type Node struct {
	Description string
	Attributes  map[string]string
	Content     interface{}
}

func Marshal(n Node) ([]byte, error) {
	if n.Attributes != nil && n.Content != nil {
		a, err := marshalMessageArray(n.Content.([]interface{}))
		if err != nil {
			return nil, err
		}
		n.Content = a
	}

	w := NewEncoder()
	if err := w.WriteNode(n); err != nil {
		return nil, err
	}

	return w.GetData(), nil
}

func marshalMessageArray(messages []interface{}) ([]Node, error) {
	ret := make([]Node, len(messages))

	for i, m := range messages {
		if wmi, ok := m.(*pb.WebMessageInfo); ok {
			b, err := marshalWebMessageInfo(wmi)
			if err != nil {
				return nil, nil
			}
			ret[i] = Node{"message", nil, b}
		} else {
			ret[i], ok = m.(Node)
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

func Unmarshal(data []byte) (*Node, error) {
	r := NewDecoder(data)
	n, err := r.ReadNode()
	if err != nil {
		return nil, err
	}

	if n != nil && n.Attributes != nil && n.Content != nil {
		nContent, ok := n.Content.([]Node)
		if ok {
			n.Content, err = unmarshalMessageArray(nContent)
			if err != nil {
				return nil, err
			}
		}
	}

	return n, nil
}

func unmarshalMessageArray(messages []Node) ([]interface{}, error) {
	ret := make([]interface{}, len(messages))

	for i, msg := range messages {
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

func unmarshalWebMessageInfo(msg []byte) (*pb.WebMessageInfo, error) {
	message := &pb.WebMessageInfo{}
	err := proto.Unmarshal(msg, message)
	if err != nil {
		return nil, err
	}
	return message, nil
}
