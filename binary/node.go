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

func (n *Node) GetOptionalChildByTag(tags ...string) (val Node, ok bool) {
	val = *n
Outer:
	for _, tag := range tags {
		for _, child := range val.GetChildren() {
			if child.Tag == tag {
				val = child
				continue Outer
			}
		}
		// If no matching children are found, return false
		return
	}
	// All iterations of loop found a matching child, return it
	ok = true
	return
}

func (n *Node) GetChildByTag(tags ...string) Node {
	node, _ := n.GetOptionalChildByTag(tags...)
	return node
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
