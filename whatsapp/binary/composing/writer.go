package composing

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary"
	"math"
	"strconv"
	"strings"
)

type binaryWriter struct {
	data []byte
}

func NewBinaryWriter() *binaryWriter {
	return &binaryWriter{make([]byte, 0)}
}

func (w *binaryWriter) getData() []byte {
	return w.data
}

func (w *binaryWriter) pushByte(b byte) {
	w.data = append(w.data, b)
}

func (w *binaryWriter) pushBytes(bytes []byte) {
	w.data = append(w.data, bytes...)
}

func (w *binaryWriter) pushIntN(value, n int, littleEndian bool) {
	for i := 0; i < n; i++ {
		var curShift int
		if littleEndian {
			curShift = i
		} else {
			curShift = n - i - 1
		}
		w.pushByte(byte((value >> uint(curShift*8)) & 0xFF))
	}
}

func (w *binaryWriter) pushInt20(value int) {
	w.pushBytes([]byte{byte((value >> 16) & 0x0F), byte((value >> 8) & 0xFF), byte(value & 0xFF)})
}

func (w *binaryWriter) pushInt8(value int) {
	w.pushIntN(value, 1, false)
}

func (w *binaryWriter) pushInt16(value int) {
	w.pushIntN(value, 2, false)
}

func (w *binaryWriter) pushInt32(value int) {
	w.pushIntN(value, 4, false)
}

func (w *binaryWriter) pushInt64(value int) {
	w.pushIntN(value, 8, false)
}

func (w *binaryWriter) pushString(value string) {
	w.pushBytes([]byte(value))
}

func (w *binaryWriter) writeByteLength(length int) error {
	if length >= 4294967296 {
		return fmt.Errorf("length is too large: %d", length)
	} else if length >= (1 << 20) {
		w.pushByte(binary.BINARY_32)
		w.pushInt32(length)
	} else if length >= 256 {
		w.pushByte(binary.BINARY_20)
		w.pushInt20(length)
	} else {
		w.pushByte(binary.BINARY_8)
		w.pushInt8(length)
	}

	return nil
}

func (w *binaryWriter) writeNode(n binary.Node) error {
	numAttributes := 0
	if n.Attributes != nil {
		numAttributes = len(n.Attributes)
	}

	hasContent := 0
	if n.Content != nil {
		hasContent = 1
	}

	w.writeListStart(2*numAttributes + 1 + hasContent)
	if err := w.writeString(n.Description, false); err != nil {
		return err
	}

	if err := w.writeAttributes(n.Attributes); err != nil {
		return err
	}

	if err := w.writeChildren(n.Content); err != nil {
		return err
	}

	return nil
}

func (w *binaryWriter) writeString(token string, i bool) error {
	//TODO check why he checks for i here.
	if !i && token == "c.us" {
		if err := w.writeToken(binary.IndexOfToken("s.whatsapp.net")); err != nil {
			return err
		}
		return nil
	}

	tokenIndex := binary.IndexOfToken(token)
	if tokenIndex == -1 {
		jidSepIndex := strings.Index(token, "@")
		if jidSepIndex < 1 {
			w.writeStringRaw(token)
		} else {
			//TODO right side should be token[jidSepIndex+1:], but then it cant be parsed back. leads to @@ in jids
			w.writeJid(token[:jidSepIndex], token[jidSepIndex+1:])
		}
	} else {
		if tokenIndex < binary.SINGLE_BYTE_MAX {
			if err := w.writeToken(tokenIndex); err != nil {
				return err
			}
		} else {
			singleByteOverflow := tokenIndex - binary.SINGLE_BYTE_MAX
			dictionaryIndex := singleByteOverflow >> 8
			if dictionaryIndex < 0 || dictionaryIndex > 3 {
				return fmt.Errorf("double byte dictionary token out of range: %v", token)
			}
			if err := w.writeToken(binary.DICTIONARY_0 + dictionaryIndex); err != nil {
				return err
			}
			if err := w.writeToken(singleByteOverflow % 256); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *binaryWriter) writeStringRaw(value string) error {
	if err := w.writeByteLength(len(value)); err != nil {
		return err
	}

	w.pushString(value)

	return nil
}

func (w *binaryWriter) writeJid(jidLeft, jidRight string) error {
	w.pushByte(binary.JID_PAIR)

	if jidLeft != "" {
		if err := w.writePackedBytes(jidLeft); err != nil {
			return err
		}
	} else {
		if err := w.writeToken(binary.LIST_EMPTY); err != nil {
			return err
		}
	}

	if err := w.writeString(jidRight, false); err != nil {
		return err
	}

	return nil
}

func (w *binaryWriter) writeToken(token int) error {
	if token < len(binary.SingleTokens) {
		w.pushByte(byte(token))
	} else if token <= 500 {
		return fmt.Errorf("invalid token: %d", token)
	}

	return nil
}

func (w *binaryWriter) writeAttributes(attributes map[string]string) error {
	if attributes == nil {
		return nil
	}

	for key, val := range attributes {
		if val == "" {
			continue
		}

		if err := w.writeString(key, false); err != nil {
			return err
		}

		if err := w.writeString(val, false); err != nil {
			return err
		}
	}

	return nil
}

func (w *binaryWriter) writeChildren(children interface{}) error {
	if children == nil {
		return nil
	}

	switch childs := children.(type) {
	case string:
		if err := w.writeString(childs, true); err != nil {
			return err
		}
	case []byte:
		if err := w.writeByteLength(len(childs)); err != nil {
			return err
		}

		w.pushBytes(childs)
	case []binary.Node:
		w.writeListStart(len(childs))
		for _, n := range childs {
			if err := w.writeNode(n); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cannot write child of type: %T", children)
	}

	return nil
}

func (w *binaryWriter) writeListStart(listSize int) {
	if listSize == 0 {
		w.pushByte(byte(binary.LIST_EMPTY))
	} else if listSize < 256 {
		w.pushByte(byte(binary.LIST_8))
		w.pushInt8(listSize)
	} else {
		w.pushByte(byte(binary.LIST_16))
		w.pushInt16(listSize)
	}
}

func (w *binaryWriter) writePackedBytes(value string) error {
	if err := w.writePackedBytesImpl(value, binary.NIBBLE_8); err != nil {
		if err := w.writePackedBytesImpl(value, binary.HEX_8); err != nil {
			return err
		}
	}

	return nil
}

func (w *binaryWriter) writePackedBytesImpl(value string, dataType int) error {
	numBytes := len(value)
	if numBytes > binary.PACKED_MAX {
		return fmt.Errorf("too many bytes to pack: %d", numBytes)
	}

	w.pushByte(byte(dataType))

	x := 0
	if numBytes%2 != 0 {
		x = 128
	}
	w.pushByte(byte(x | int(math.Ceil(float64(numBytes)/2.0))))
	for i, l := 0, numBytes/2; i < l; i++ {
		b, err := w.packBytePair(dataType, value[2*i:2*i+1], value[2*i+1:2*i+2])
		if err != nil {
			return err
		}

		w.pushByte(byte(b))
	}

	if (numBytes % 2) != 0 {
		b, err := w.packBytePair(dataType, value[numBytes-1:], "\x00")
		if err != nil {
			return err
		}

		w.pushByte(byte(b))
	}

	return nil
}

func (w *binaryWriter) packBytePair(packType int, part1, part2 string) (int, error) {
	if packType == binary.NIBBLE_8 {
		n1, err := packNibble(part1)
		if err != nil {
			return 0, err
		}

		n2, err := packNibble(part2)
		if err != nil {
			return 0, err
		}

		return (n1 << 4) | n2, nil
	} else if packType == binary.HEX_8 {
		n1, err := packHex(part1)
		if err != nil {
			return 0, err
		}

		n2, err := packHex(part2)
		if err != nil {
			return 0, err
		}

		return (n1 << 4) | n2, nil
	} else {
		return 0, fmt.Errorf("invalid pack type (%d) for byte pair: %s / %s", packType, part1, part2)
	}
}

func packNibble(value string) (int, error) {
	if value >= "0" && value <= "9" {
		return strconv.Atoi(value)
	} else if value == "-" {
		return 10, nil
	} else if value == "." {
		return 11, nil
	} else if value == "\x00" {
		return 15, nil
	}

	return 0, fmt.Errorf("invalid string to pack as nibble: %v", value)
}

func packHex(value string) (int, error) {
	if (value >= "0" && value <= "9") || (value >= "A" && value <= "F") || (value >= "a" && value <= "f") {
		d, err := strconv.ParseInt(value, 16, 0)
		return int(d), err
	} else if value == "\x00" {
		return 15, nil
	}

	return 0, fmt.Errorf("invalid string to pack as hex: %v", value)
}
