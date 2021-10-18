package binary

type Attrs = map[string]interface{}

type Node struct {
	Tag     string
	Attrs   Attrs
	Content interface{}
}

func (n *Node) GetChildren() []Node {
	if n.Content == nil {
		return nil
	}
	children, ok := n.Content.([]Node)
	if !ok {
		return nil
	}
	return children
}

func (n *Node) GetChildrenByTag(tag string) (children []Node) {
	for _, node := range n.GetChildren() {
		if node.Tag == tag {
			children = append(children, node)
		}
	}
	return
}

func (n *Node) GetChildPtrByTag(tag string) *Node {
	val := n.GetChildByTag(tag)
	return &val
}

func (n *Node) GetChildByTag(tag string) (val Node) {
	nodes := n.GetChildrenByTag(tag)
	if len(nodes) > 0 {
		val = nodes[0]
	}
	return
}

func (n *Node) GetOptionalChildByTag(tag string) (val *Node, ok bool) {
	nodes := n.GetChildrenByTag(tag)
	if len(nodes) > 0 {
		val = &nodes[0]
		ok = true
	}
	return
}

func Marshal(n Node) ([]byte, error) {
	w := NewEncoder()
	w.WriteNode(n)
	return w.GetData(), nil
}

func Unmarshal(data []byte) (*Node, error) {
	r := NewDecoder(data)
	n, err := r.ReadNode()
	if err != nil {
		return nil, err
	}
	return n, nil
}
