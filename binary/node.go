// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package binary implements encoding and decoding documents in WhatsApp's binary XML format.
package binary

import (
	"fmt"
)

// Attrs is a type alias for the attributes of an XML element (Node).
type Attrs = map[string]interface{}

// Node represents an XML element.
type Node struct {
	Tag     string      // The tag of the element.
	Attrs   Attrs       // The attributes of the element.
	Content interface{} // The content inside the element. Can be nil, a list of Nodes, or a byte array.
}

// GetChildren returns the Content of the node as a list of nodes. If the content is not a list of nodes, this returns nil.
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

// GetChildrenByTag returns the same list as GetChildren, but filters it by tag first.
func (n *Node) GetChildrenByTag(tag string) (children []Node) {
	for _, node := range n.GetChildren() {
		if node.Tag == tag {
			children = append(children, node)
		}
	}
	return
}

// GetOptionalChildByTag finds the first child with the given tag and returns it.
// Each provided tag will recurse in, so this is useful for getting a specific nested element.
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

// GetChildByTag does the same thing as GetOptionalChildByTag, but returns the Node directly without the ok boolean.
func (n *Node) GetChildByTag(tags ...string) Node {
	node, _ := n.GetOptionalChildByTag(tags...)
	return node
}

// Marshal encodes an XML element (Node) into WhatsApp's binary XML representation.
func Marshal(n Node) ([]byte, error) {
	w := newEncoder()
	w.writeNode(n)
	return w.getData(), nil
}

// Unmarshal decodes WhatsApp's binary XML representation into a Node.
func Unmarshal(data []byte) (*Node, error) {
	r := newDecoder(data)
	n, err := r.readNode()
	if err != nil {
		return nil, err
	} else if r.index != len(r.data) {
		return n, fmt.Errorf("%d leftover bytes after decoding", len(r.data)-r.index)
	}
	return n, nil
}
