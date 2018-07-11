package binary

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary/token"
	"math"
	"strconv"
	"strings"
)

type binaryEncoder struct {
	data []byte
}

func NewEncoder() *binaryEncoder {
	return &binaryEncoder{make([]byte, 0)}
}

func (w *binaryEncoder) GetData() []byte {
	return w.data
}

func (w *binaryEncoder) pushByte(b byte) {
	w.data = append(w.data, b)
}

func (w *binaryEncoder) pushBytes(bytes []byte) {
	w.data = append(w.data, bytes...)
}

func (w *binaryEncoder) pushIntN(value, n int, littleEndian bool) {
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

func (w *binaryEncoder) pushInt20(value int) {
	w.pushBytes([]byte{byte((value >> 16) & 0x0F), byte((value >> 8) & 0xFF), byte(value & 0xFF)})
}

func (w *binaryEncoder) pushInt8(value int) {
	w.pushIntN(value, 1, false)
}

func (w *binaryEncoder) pushInt16(value int) {
	w.pushIntN(value, 2, false)
}

func (w *binaryEncoder) pushInt32(value int) {
	w.pushIntN(value, 4, false)
}

func (w *binaryEncoder) pushInt64(value int) {
	w.pushIntN(value, 8, false)
}

func (w *binaryEncoder) pushString(value string) {
	w.pushBytes([]byte(value))
}

func (w *binaryEncoder) writeByteLength(length int) error {
	if length > math.MaxInt32 {
		return fmt.Errorf("length is too large: %d", length)
	} else if length >= (1 << 20) {
		w.pushByte(token.BINARY_32)
		w.pushInt32(length)
	} else if length >= 256 {
		w.pushByte(token.BINARY_20)
		w.pushInt20(length)
	} else {
		w.pushByte(token.BINARY_8)
		w.pushInt8(length)
	}

	return nil
}

func (w *binaryEncoder) WriteNode(n Node) error {
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

func (w *binaryEncoder) writeString(tok string, i bool) error {
	if !i && tok == "c.us" {
		if err := w.writeToken(token.IndexOfSingleToken("s.whatsapp.net")); err != nil {
			return err
		}
		return nil
	}

	tokenIndex := token.IndexOfSingleToken(tok)
	if tokenIndex == -1 {
		jidSepIndex := strings.Index(tok, "@")
		if jidSepIndex < 1 {
			w.writeStringRaw(tok)
		} else {
			w.writeJid(tok[:jidSepIndex], tok[jidSepIndex+1:])
		}
	} else {
		if tokenIndex < token.SINGLE_BYTE_MAX {
			if err := w.writeToken(tokenIndex); err != nil {
				return err
			}
		} else {
			singleByteOverflow := tokenIndex - token.SINGLE_BYTE_MAX
			dictionaryIndex := singleByteOverflow >> 8
			if dictionaryIndex < 0 || dictionaryIndex > 3 {
				return fmt.Errorf("double byte dictionary token out of range: %v", tok)
			}
			if err := w.writeToken(token.DICTIONARY_0 + dictionaryIndex); err != nil {
				return err
			}
			if err := w.writeToken(singleByteOverflow % 256); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *binaryEncoder) writeStringRaw(value string) error {
	if err := w.writeByteLength(len(value)); err != nil {
		return err
	}

	w.pushString(value)

	return nil
}

func (w *binaryEncoder) writeJid(jidLeft, jidRight string) error {
	w.pushByte(token.JID_PAIR)

	if jidLeft != "" {
		if err := w.writePackedBytes(jidLeft); err != nil {
			return err
		}
	} else {
		if err := w.writeToken(token.LIST_EMPTY); err != nil {
			return err
		}
	}

	if err := w.writeString(jidRight, false); err != nil {
		return err
	}

	return nil
}

func (w *binaryEncoder) writeToken(tok int) error {
	if tok < len(token.SingleByteTokens) {
		w.pushByte(byte(tok))
	} else if tok <= 500 {
		return fmt.Errorf("invalid token: %d", tok)
	}

	return nil
}

func (w *binaryEncoder) writeAttributes(attributes map[string]string) error {
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

func (w *binaryEncoder) writeChildren(children interface{}) error {
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
	case []Node:
		w.writeListStart(len(childs))
		for _, n := range childs {
			if err := w.WriteNode(n); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cannot write child of type: %T", children)
	}

	return nil
}

func (w *binaryEncoder) writeListStart(listSize int) {
	if listSize == 0 {
		w.pushByte(byte(token.LIST_EMPTY))
	} else if listSize < 256 {
		w.pushByte(byte(token.LIST_8))
		w.pushInt8(listSize)
	} else {
		w.pushByte(byte(token.LIST_16))
		w.pushInt16(listSize)
	}
}

func (w *binaryEncoder) writePackedBytes(value string) error {
	if err := w.writePackedBytesImpl(value, token.NIBBLE_8); err != nil {
		if err := w.writePackedBytesImpl(value, token.HEX_8); err != nil {
			return err
		}
	}

	return nil
}

func (w *binaryEncoder) writePackedBytesImpl(value string, dataType int) error {
	numBytes := len(value)
	if numBytes > token.PACKED_MAX {
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

func (w *binaryEncoder) packBytePair(packType int, part1, part2 string) (int, error) {
	if packType == token.NIBBLE_8 {
		n1, err := packNibble(part1)
		if err != nil {
			return 0, err
		}

		n2, err := packNibble(part2)
		if err != nil {
			return 0, err
		}

		return (n1 << 4) | n2, nil
	} else if packType == token.HEX_8 {
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
