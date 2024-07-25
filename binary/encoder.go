package binary

import (
	"fmt"
	"math"
	"strconv"

	"go.mau.fi/whatsmeow/binary/token"
	"go.mau.fi/whatsmeow/types"
)

type binaryEncoder struct {
	data []byte
}

func newEncoder() *binaryEncoder {
	return &binaryEncoder{[]byte{0}}
}

func (w *binaryEncoder) getData() []byte {
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

func (w *binaryEncoder) pushString(value string) {
	w.pushBytes([]byte(value))
}

func (w *binaryEncoder) writeByteLength(length int) {
	if length < 256 {
		w.pushByte(token.Binary8)
		w.pushInt8(length)
	} else if length < (1 << 20) {
		w.pushByte(token.Binary20)
		w.pushInt20(length)
	} else if length < math.MaxInt32 {
		w.pushByte(token.Binary32)
		w.pushInt32(length)
	} else {
		panic(fmt.Errorf("length is too large: %d", length))
	}
}

const tagSize = 1

func (w *binaryEncoder) writeNode(n Node) {
	if n.Tag == "0" {
		w.pushByte(token.List8)
		w.pushByte(token.ListEmpty)
		return
	}

	hasContent := 0
	if n.Content != nil {
		hasContent = 1
	}

	w.writeListStart(2*len(n.Attrs) + tagSize + hasContent)
	w.writeString(n.Tag)
	w.writeAttributes(n.Attrs)
	if n.Content != nil {
		w.write(n.Content)
	}
}

func (w *binaryEncoder) write(data interface{}) {
	switch typedData := data.(type) {
	case nil:
		w.pushByte(token.ListEmpty)
	case types.JID:
		w.writeJID(typedData)
	case string:
		w.writeString(typedData)
	case int:
		w.writeString(strconv.Itoa(typedData))
	case int32:
		w.writeString(strconv.FormatInt(int64(typedData), 10))
	case uint:
		w.writeString(strconv.FormatUint(uint64(typedData), 10))
	case uint32:
		w.writeString(strconv.FormatUint(uint64(typedData), 10))
	case int64:
		w.writeString(strconv.FormatInt(typedData, 10))
	case uint64:
		w.writeString(strconv.FormatUint(typedData, 10))
	case bool:
		w.writeString(strconv.FormatBool(typedData))
	case []byte:
		w.writeBytes(typedData)
	case []Node:
		w.writeListStart(len(typedData))
		for _, n := range typedData {
			w.writeNode(n)
		}
	default:
		panic(fmt.Errorf("%w: %T", ErrInvalidType, typedData))
	}
}

func (w *binaryEncoder) writeString(data string) {
	var dictIndex byte
	if tokenIndex, ok := token.IndexOfSingleToken(data); ok {
		w.pushByte(tokenIndex)
	} else if dictIndex, tokenIndex, ok = token.IndexOfDoubleByteToken(data); ok {
		w.pushByte(token.Dictionary0 + dictIndex)
		w.pushByte(tokenIndex)
	} else if validateNibble(data) {
		w.writePackedBytes(data, token.Nibble8)
	} else if validateHex(data) {
		w.writePackedBytes(data, token.Hex8)
	} else {
		w.writeStringRaw(data)
	}
}

func (w *binaryEncoder) writeBytes(value []byte) {
	w.writeByteLength(len(value))
	w.pushBytes(value)
}

func (w *binaryEncoder) writeStringRaw(value string) {
	w.writeByteLength(len(value))
	w.pushString(value)
}

func (w *binaryEncoder) writeJID(jid types.JID) {
	if (jid.Server == types.DefaultUserServer && jid.Device > 0) || jid.Server == types.HiddenUserServer || jid.Server == types.HostedServer {
		w.pushByte(token.ADJID)
		w.pushByte(jid.ActualAgent())
		w.pushByte(uint8(jid.Device))
		w.writeString(jid.User)
	} else if jid.Server == types.MessengerServer {
		w.pushByte(token.FBJID)
		w.write(jid.User)
		w.pushInt16(int(jid.Device))
		w.write(jid.Server)
	} else if jid.Server == types.InteropServer {
		w.pushByte(token.InteropJID)
		w.write(jid.User)
		w.pushInt16(int(jid.Device))
		w.pushInt16(int(jid.Integrator))
		w.write(jid.Server)
	} else {
		w.pushByte(token.JIDPair)
		if len(jid.User) == 0 {
			w.pushByte(token.ListEmpty)
		} else {
			w.write(jid.User)
		}
		w.write(jid.Server)
	}
}

func (w *binaryEncoder) writeAttributes(attributes Attrs) {
	if attributes == nil {
		return
	}

	for key, val := range attributes {
		if val == "" || val == nil {
			continue
		}

		w.writeString(key)
		w.write(val)
	}
}

func (w *binaryEncoder) writeListStart(listSize int) {
	if listSize == 0 {
		w.pushByte(byte(token.ListEmpty))
	} else if listSize < 256 {
		w.pushByte(byte(token.List8))
		w.pushInt8(listSize)
	} else {
		w.pushByte(byte(token.List16))
		w.pushInt16(listSize)
	}
}

func (w *binaryEncoder) writePackedBytes(value string, dataType int) {
	if len(value) > token.PackedMax {
		panic(fmt.Errorf("too many bytes to pack: %d", len(value)))
	}

	w.pushByte(byte(dataType))

	roundedLength := byte(math.Ceil(float64(len(value)) / 2.0))
	if len(value)%2 != 0 {
		roundedLength |= 128
	}
	w.pushByte(roundedLength)
	var packer func(byte) byte
	if dataType == token.Nibble8 {
		packer = packNibble
	} else if dataType == token.Hex8 {
		packer = packHex
	} else {
		// This should only be called with the correct values
		panic(fmt.Errorf("invalid packed byte data type %v", dataType))
	}
	for i, l := 0, len(value)/2; i < l; i++ {
		w.pushByte(w.packBytePair(packer, value[2*i], value[2*i+1]))
	}
	if len(value)%2 != 0 {
		w.pushByte(w.packBytePair(packer, value[len(value)-1], '\x00'))
	}
}

func (w *binaryEncoder) packBytePair(packer func(byte) byte, part1, part2 byte) byte {
	return (packer(part1) << 4) | packer(part2)
}

func validateNibble(value string) bool {
	if len(value) > token.PackedMax {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9') && char != '-' && char != '.' {
			return false
		}
	}
	return true
}

func packNibble(value byte) byte {
	switch value {
	case '-':
		return 10
	case '.':
		return 11
	case 0:
		return 15
	default:
		if value >= '0' && value <= '9' {
			return value - '0'
		}
		// This should be validated beforehand
		panic(fmt.Errorf("invalid string to pack as nibble: %d / '%s'", value, string(value)))
	}
}

func validateHex(value string) bool {
	if len(value) > token.PackedMax {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9') && !(char >= 'A' && char <= 'F') && !(char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func packHex(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'A' && value <= 'F':
		return 10 + value - 'A'
	case value >= 'a' && value <= 'f':
		return 10 + value - 'a'
	case value == 0:
		return 15
	default:
		// This should be validated beforehand
		panic(fmt.Errorf("invalid string to pack as hex: %d / '%s'", value, string(value)))
	}
}
