package parsing

import (
	"fmt"
	"git.willing.nrw/WhatsPoll/whatsapp-connection/whatsapp/binary"
	"io"
	"strconv"
)

type binaryReader struct {
	data  []byte
	index int
}

func NewBinaryReader(data []byte) *binaryReader {
	return &binaryReader{data, 0}
}

func (r *binaryReader) checkEOS(length int) error {
	if r.index+length > len(r.data) {
		return io.EOF
	}

	return nil
}

func (r *binaryReader) readByte() (byte, error) {
	if err := r.checkEOS(1); err != nil {
		return 0, err
	}

	b := r.data[r.index]
	r.index++

	return b, nil
}

func (r *binaryReader) readIntN(n int, littleEndian bool) (int, error) {
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

func (r *binaryReader) readInt8(littleEndian bool) (int, error) {
	return r.readIntN(1, littleEndian)
}

func (r *binaryReader) readInt16(littleEndian bool) (int, error) {
	return r.readIntN(2, littleEndian)
}

func (r *binaryReader) readInt20() (int, error) {
	if err := r.checkEOS(3); err != nil {
		return 0, err
	}

	ret := ((int(r.data[r.index]) & 15) << 16) + (int(r.data[r.index+1]) << 8) + int(r.data[r.index+2])
	r.index += 3
	return ret, nil
}

func (r *binaryReader) readInt32(littleEndian bool) (int, error) {
	return r.readIntN(4, littleEndian)
}

func (r *binaryReader) readInt64(littleEndian bool) (int, error) {
	return r.readIntN(8, littleEndian)
}

func (r *binaryReader) readPacked8(tag int) (string, error) {
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
	case binary.NIBBLE_8:
		return unpackNibble(value)
	case binary.HEX_8:
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

func (r *binaryReader) readListSize(tag int) (int, error) {
	switch tag {
	case binary.LIST_EMPTY:
		return 0, nil
	case binary.LIST_8:
		return r.readInt8(false)
	case binary.LIST_16:
		return r.readInt16(false)
	default:
		return 0, fmt.Errorf("readListSize with unknown tag %d at position %d", tag, r.index)
	}
}

func (r *binaryReader) readString(tag int) (string, error) {
	switch {
	case tag >= 3 && tag <= len(binary.SingleTokens):
		token, err := binary.GetToken(tag)
		if err != nil {
			return "", err
		}

		if token == "s.whatsapp.net" {
			token = "c.us"
		}

		return token, nil
	case tag == binary.DICTIONARY_0 || tag == binary.DICTIONARY_1 || tag == binary.DICTIONARY_2 || tag == binary.DICTIONARY_3:
		i, err := r.readInt8(false)
		if err != nil {
			return "", err
		}

		return binary.GetTokenDouble(tag-binary.DICTIONARY_0, i)
	case tag == binary.LIST_EMPTY:
		return "", nil
	case tag == binary.BINARY_8:
		length, err := r.readInt8(false)
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == binary.BINARY_20:
		length, err := r.readInt20()
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == binary.BINARY_32:
		length, err := r.readInt32(false)
		if err != nil {
			return "", err
		}

		return r.readStringFromChars(length)
	case tag == binary.JID_PAIR:
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
	case tag == binary.NIBBLE_8 || tag == binary.HEX_8:
		return r.readPacked8(tag)
	default:
		return "", fmt.Errorf("invalid string with tag %d", tag)
	}
}

func (r *binaryReader) readStringFromChars(length int) (string, error) {
	if err := r.checkEOS(length); err != nil {
		return "", err
	}

	ret := r.data[r.index : r.index+length]
	r.index += length

	return string(ret), nil
}

func (r *binaryReader) readAttributes(n int) (map[string]string, error) {
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

func (r *binaryReader) readList(tag int) ([]binary.Node, error) {
	size, err := r.readListSize(tag)
	if err != nil {
		return nil, err
	}

	ret := make([]binary.Node, size)
	for i := 0; i < size; i++ {
		n, err := r.readNode()

		if err != nil {
			return nil, err
		}

		ret[i] = *n
	}

	return ret, nil
}

func (r *binaryReader) readNode() (*binary.Node, error) {
	ret := new(binary.Node)

	size, err := r.readInt8(false)
	if err != nil {
		return nil, err
	}
	listSize, err := r.readListSize(size)
	if err != nil {
		return nil, err
	}

	descrTag, err := r.readInt8(false)
	if descrTag == binary.STREAM_END {
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
	case binary.LIST_EMPTY, binary.LIST_8, binary.LIST_16:
		ret.Content, err = r.readList(tag)
	case binary.BINARY_8:
		size, err = r.readInt8(false)
		if err != nil {
			return nil, err
		}

		ret.Content, err = r.readBytes(size)
	case binary.BINARY_20:
		size, err = r.readInt20()
		if err != nil {
			return nil, err
		}

		ret.Content, err = r.readBytes(size)
	case binary.BINARY_32:
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

func (r *binaryReader) readBytes(n int) ([]byte, error) {
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
