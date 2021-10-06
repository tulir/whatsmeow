package binary

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	pb "github.com/Rhymen/go-whatsapp/binary/proto"
)

type Node struct {
	Tag     string
	Attrs   map[string]interface{}
	Content interface{}

	LegacyAttributes map[string]string
}

func (n *Node) convertLegacyAttributes() {
	n.Attrs = make(map[string]interface{}, len(n.LegacyAttributes))
	for key, attr := range n.LegacyAttributes {
		atIndex := strings.Index(attr, "@")
		if atIndex < 1 {
			n.Attrs[key] = attr
		} else {
			n.Attrs[key] = NewJID(attr[:atIndex], attr[atIndex+1:])
		}
	}
	n.LegacyAttributes = nil
}

func (n *Node) addLegacyAttributes() {
	n.LegacyAttributes = make(map[string]string, len(n.Attrs))
	for key, rawAttr := range n.Attrs {
		switch attr := rawAttr.(type) {
		case string:
			n.LegacyAttributes[key] = attr
		case *FullJID:
			n.LegacyAttributes[key] = attr.String()
		}
	}
}

func Marshal(n Node, md bool) ([]byte, error) {
	if n.LegacyAttributes != nil {
		n.convertLegacyAttributes()
	}
	if n.Attrs != nil && n.Content != nil {
		a, err := marshalMessageArray(n.Content.([]interface{}))
		if err != nil {
			return nil, err
		}
		n.Content = a
	}

	w := NewEncoder(md)
	w.WriteNode(n)
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
			ret[i] = Node{"message", nil, b, nil}
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

func Unmarshal(data []byte, md bool) (*Node, error) {
	r := NewDecoder(data, md)
	n, err := r.ReadNode()
	if err != nil {
		return nil, err
	}

	if !md {
		n.addLegacyAttributes()
	}
	if !md && n != nil && n.Attrs != nil && n.Content != nil {
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
		if msg.Tag == "message" {
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
