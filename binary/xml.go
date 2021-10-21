// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Options to control how Node.XMLString behaves.
var (
	IndentXML            = false
	MaxBytesToPrintAsHex = 128
)

// XMLString converts the Node to its XML representation
func (n *Node) XMLString() string {
	content := n.contentString()
	if len(content) == 0 {
		return fmt.Sprintf("<%[1]s%[2]s/>", n.Tag, n.attributeString())
	}
	newline := "\n"
	if len(content) == 1 || !IndentXML {
		newline = ""
	}
	return fmt.Sprintf("<%[1]s%[2]s>%[4]s%[3]s%[4]s</%[1]s>", n.Tag, n.attributeString(), strings.Join(content, newline), newline)
}

func (n *Node) attributeString() string {
	if len(n.Attrs) == 0 {
		return ""
	}
	stringAttrs := make([]string, len(n.Attrs)+1)
	i := 1
	for key, value := range n.Attrs {
		stringAttrs[i] = fmt.Sprintf(`%s="%v"`, key, value)
		i++
	}
	sort.Strings(stringAttrs)
	return strings.Join(stringAttrs, " ")
}

func printable(data []byte) string {
	if !utf8.Valid(data) {
		return ""
	}
	str := string(data)
	for _, c := range str {
		if !unicode.IsPrint(c) {
			return ""
		}
	}
	return str
}

func (n *Node) contentString() []string {
	split := make([]string, 0)
	switch content := n.Content.(type) {
	case []Node:
		for _, item := range content {
			split = append(split, strings.Split(item.XMLString(), "\n")...)
		}
	case []byte:
		if strContent := printable(content); len(strContent) > 0 {
			if IndentXML {
				split = append(split, strings.Split(string(content), "\n")...)
			} else {
				split = append(split, strings.ReplaceAll(string(content), "\n", "\\n"))
			}
		} else if len(content) > MaxBytesToPrintAsHex {
			split = append(split, fmt.Sprintf("<!-- %d bytes -->", len(content)))
		} else if !IndentXML {
			split = append(split, hex.EncodeToString(content))
		} else {
			hexData := hex.EncodeToString(content)
			for i := 0; i < len(hexData); i += 80 {
				if len(hexData) < i+80 {
					split = append(split, hexData[i:])
				} else {
					split = append(split, hexData[i:i+80])
				}
			}
		}
	case nil:
		// don't append anything
	default:
		strContent := fmt.Sprintf("%s", content)
		if IndentXML {
			split = append(split, strings.Split(strContent, "\n")...)
		} else {
			split = append(split, strings.ReplaceAll(strContent, "\n", "\\n"))
		}
	}
	if len(split) > 1 && IndentXML {
		for i, line := range split {
			split[i] = "  " + line
		}
	}
	return split
}
