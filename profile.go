package whatsapp

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"strconv"
	"time"
)

// Pictures must be JPG 640x640 and 96x96, respectively
func (wac *Conn) UploadProfilePic(ownJID JID, image, preview []byte) (<-chan string, error) {
	tag := fmt.Sprintf("%d.--%d", time.Now().Unix(), wac.msgCount*19)
	n := binary.Node{
		Description: "action",
		LegacyAttributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{
			binary.Node{
				Description: "picture",
				LegacyAttributes: map[string]string{
					"id":   tag,
					"jid":  ownJID,
					"type": "set",
				},
				Content: []binary.Node{
					{
						Description: "image",
						LegacyAttributes:  nil,
						Content:     image,
					},
					{
						Description: "preview",
						LegacyAttributes:  nil,
						Content:     preview,
					},
				},
			},
		},
	}
	return wac.writeBinary(n, profile, 136, tag)
}
