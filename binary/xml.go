package binary

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// XMLString converts the Node to its XML representation
func (n *Node) XMLString() string {
	content := n.contentString()
	if len(content) == 0 {
		return fmt.Sprintf("<%[1]s%[2]s/>", n.Description, n.attributeString())
	} else if len(content) == 1 {
		return fmt.Sprintf("<%[1]s%[2]s>%[3]s</%[1]s>", n.Description, n.attributeString(), content[0])
	} else {
		return fmt.Sprintf("<%[1]s%[2]s>\n%[3]s\n</%[1]s>", n.Description, n.attributeString(), strings.Join(content, "\n"))
	}
}

func (n *Node) attributeString() string {
	if len(n.Attributes) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteRune(' ')
	count := 0
	for key, value := range n.Attributes {
		count += 1
		_, _ = fmt.Fprintf(&builder, `%s="%s"`, key, value)
		if count < len(n.Attributes) {
			builder.WriteRune(' ')
		}
	}
	return builder.String()
}

func (n *Node) contentString() []string {
	split := make([]string, 0)
	switch content := n.Content.(type) {
	case []Node:
		for _, item := range content {
			split = append(split, strings.Split(item.XMLString(), "\n")...)
		}
	case []byte:
		if utf8.Valid(content) {
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
