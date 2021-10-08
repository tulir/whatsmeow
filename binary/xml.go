package binary

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// XMLString converts the Node to its XML representation
func (n *Node) XMLString() string {
	content := n.contentString()
	if len(content) == 0 {
		return fmt.Sprintf("<%[1]s%[2]s/>", n.Tag, n.attributeString())
	} else if len(content) == 1 {
		return fmt.Sprintf("<%[1]s%[2]s>%[3]s</%[1]s>", n.Tag, n.attributeString(), content[0])
	} else {
		return fmt.Sprintf("<%[1]s%[2]s>\n%[3]s\n</%[1]s>", n.Tag, n.attributeString(), strings.Join(content, "\n"))
	}
}

func (n *Node) attributeString() string {
	if len(n.Attrs) == 0 {
		return ""
	}
	stringAttrs := make([]string, len(n.Attrs) + 1)
	i := 1
	for key, value := range n.Attrs {
		stringAttrs[i] = fmt.Sprintf(`%s="%v"`, key, value)
		i += 1
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
			split = append(split, strings.Split(string(content), "\n")...)
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
		split = append(split, strings.Split(fmt.Sprintf("%s", content), "\n")...)
	}
	if len(split) > 1 {
		for i, line := range split {
			split[i] = "  " + line
		}
	}
	return split
}
