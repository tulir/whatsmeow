package binary

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary/token"
	"io"
	"strconv"
)

type binaryDecoder struct {
	data  []byte
	index int
}

func NewDecoder(data []byte) *binaryDecoder {
	return &binaryDecoder{data, 0}
}

func (r *binaryDecoder) checkEOS(length int) error {
	if r.index+length > len(r.data) {
		return io.EOF
	}

	return nil
}

func (r *binaryDecoder) readByte() (byte, error) {
	if err := r.checkEOS(1); err != nil {
		return 0, err
	}

	b := r.data[r.index]
	r.index++

	return b, nil
}

func (r *binaryDecoder) readIntN(n int, littleEndian bool) (int, error) {
	if err := r.checkEOS(n); err != nil {
		return 0, err
	}

	var ret int

	for i := 0; i < n; i++ {
		var curShift int
		if littleEndian {
			curShift = i
		} else {
			curShift = n - i - 1
		}
		ret |= int(r.data[r.index+i]) << uint(curShift*8)
	}

	r.index += n
	return ret, nil
}

func (r *binaryDecoder) readInt8(littleEndian bool) (int, error) {
	return r.readIntN(1, littleEndian)
}

func (r *binaryDecoder) readInt16(littleEndian bool) (int, error) {
	return r.readIntN(2, littleEndian)
}

func (r *binaryDecoder) readInt20() (int, error) {
	if err := r.checkEOS(3); err != nil {
		return 0, err
	}

	ret := ((int(r.data[r.index]) & 15) << 16) + (int(r.data[r.index+1]) << 8) + int(r.data[r.index+2])
	r.index += 3
	return ret, nil
}

func (r *binaryDecoder) readInt32(littleEndian bool) (int, error) {
	return r.readIntN(4, littleEndian)
}

func (r *binaryDecoder) readInt64(littleEndian bool) (int, error) {
	return r.readIntN(8, littleEndian)
}

func (r *binaryDecoder) readPacked8(tag int) (string, error) {
	startByte, err := r.readByte()
	if err != nil {
		return "", err
	}

	ret := ""

	for i := 0; i < int(startByte&127); i++ {
		currByte, err := r.readByte()
		if err != nil {
			return "", err
		}

		lower, err := unpackByte(tag, currByte&0xF0>>4)
		if err != nil {
			return "", err
		}

		upper, err := unpackByte(tag, currByte&0x0F)
		if err != nil {
			return "", err
		}

		ret += lower + upper
	}

	if startByte>>7 != 0 {
		ret = ret[:len(ret)-1]
	}
	return ret, nil
}

func unpackByte(tag int, value byte) (string, error) {
	switch tag {
	case token.NIBBLE_8:
		return unpackNibble(value)
	case token.HEX_8:
		return unpackHex(value)
	default:
		return "", fmt.Errorf("unpackByte with unknown tag %d", tag)
	}
}

func unpackNibble(value byte) (string, error) {
	switch {
	case value < 0 || value > 15:
		return "", fmt.Errorf("unpackNibble with value %d", value)
	case value == 10:
		return "-", nil
	case value == 11:
		return ".", nil
	case value == 15:
		return "\x00", nil
	default:
		return strconv.Itoa(int(value)), nil
	}
}

func unpackHex(value byte) (string, error) {
	switch {
	case value < 0 || value > 15:
		return "", fmt.Errorf("unpackHex with value %d", value)
	case value < 10:
		return strconv.Itoa(int(value)), nil
	default:
		return string('A' + value - 10), nil
	}
}

func (r *binaryDecoder) readListSize(tag int) (int, error) {
	switch tag {
	case token.LIST_EMPTY:
		return 0, nil
	case token.LIST_8:
		return r.readInt8(false)
	case token.LIST_16:
		return r.readInt16(false)
	default:
		return 0, fmt.Errorf("readListSize with unknown tag %d at position %d", tag, r.index)
	}
}

func (r *binaryDecoder) readString(tag int) (string, error) {
	switch {
	case tag >= 3 && tag <= len(token.SingleByteTokens):
		tok, err := token.GetSingleToken(tag)
		if err != nil {
			return "", err
		}

		if tok == "s.whatsapp.net" {
			tok = "c.us"
		}

		return tok, nil
	case tag == token.DICTIONARY_0 || tag == token.DICTIONARY_1 || tag == token.DICTIONARY_2 || tag == token.DICTIONARY_3:
		i, err := r.readInt8(false)
		if err != nil {
			return "", err
		}

		return token.GetDoubleToken(tag-token.DICTIONARY_0, i)
	case tag == token.LIST_EMPTY:
		return "", nil
	case tag == token.BINARY_8:
		length, err := r.readInt8(false)
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == token.BINARY_20:
		length, err := r.readInt20()
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == token.BINARY_32:
		length, err := r.readInt32(false)
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == token.JID_PAIR:
		b, err := r.readByte()
		if err != nil {
			return "", err
		}
		i, err := r.readString(int(b))
		if err != nil {
			return "", err
		}

		b, err = r.readByte()
		if err != nil {
			return "", err
		}
		j, err := r.readString(int(b))
		if err != nil {
			return "", err
		}

		if i == "" || j == "" {
			return "", fmt.Errorf("invalid jid pair: %s - %s", i, j)
		}

		return i + "@" + j, nil
	case tag == token.NIBBLE_8 || tag == token.HEX_8:
		return r.readPacked8(tag)
	default:
		return "", fmt.Errorf("invalid string with tag %d", tag)
	}
}

func (r *binaryDecoder) readStringFromChars(length int) (string, error) {
	if err := r.checkEOS(length); err != nil {
		return "", err
	}

	ret := r.data[r.index : r.index+length]
	r.index += length

	return string(ret), nil
}

func (r *binaryDecoder) readAttributes(n int) (map[string]string, error) {
	if n == 0 {
		return nil, nil
	}

	ret := make(map[string]string)
	for i := 0; i < n; i++ {
		idx, err := r.readInt8(false)
		if err != nil {
			return nil, err
		}

		index, err := r.readString(idx)
		if err != nil {
			return nil, err
		}

		idx, err = r.readInt8(false)
		if err != nil {
			return nil, err
		}

		ret[index], err = r.readString(idx)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (r *binaryDecoder) readList(tag int) ([]Node, error) {
	size, err := r.readListSize(tag)
	if err != nil {
		return nil, err
	}

	ret := make([]Node, size)
	for i := 0; i < size; i++ {
		n, err := r.ReadNode()

		if err != nil {
			return nil, err
		}

		ret[i] = *n
	}

	return ret, nil
}

func (r *binaryDecoder) ReadNode() (*Node, error) {
	ret := &Node{}

	size, err := r.readInt8(false)
	if err != nil {
		return nil, err
	}
	listSize, err := r.readListSize(size)
	if err != nil {
		return nil, err
	}

	descrTag, err := r.readInt8(false)
	if descrTag == token.STREAM_END {
		return nil, fmt.Errorf("unexpected stream end")
	}
	ret.Description, err = r.readString(descrTag)
	if err != nil {
		return nil, err
	}
	if listSize == 0 || ret.Description == "" {
		return nil, fmt.Errorf("invalid Node")
	}

	ret.Attributes, err = r.readAttributes((listSize - 1) >> 1)
	if err != nil {
		return nil, err
	}

	if listSize%2 == 1 {
		return ret, nil
	}

	tag, err := r.readInt8(false)
	if err != nil {
		return nil, err
	}

	switch tag {
	case token.LIST_EMPTY, token.LIST_8, token.LIST_16:
		ret.Content, err = r.readList(tag)
	case token.BINARY_8:
		size, err = r.readInt8(false)
		if err != nil {
			return nil, err
		}

		ret.Content, err = r.readBytes(size)
	case token.BINARY_20:
		size, err = r.readInt20()
		if err != nil {
			return nil, err
		}

		ret.Content, err = r.readBytes(size)
	case token.BINARY_32:
		size, err = r.readInt32(false)
		if err != nil {
			return nil, err
		}

		ret.Content, err = r.readBytes(size)
	default:
		ret.Content, err = r.readString(tag)
	}

	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *binaryDecoder) readBytes(n int) ([]byte, error) {
	ret := make([]byte, n)
	var err error

	for i := range ret {
		ret[i], err = r.readByte()
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}
