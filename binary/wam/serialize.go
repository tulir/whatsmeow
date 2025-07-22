package wam

import (
	"encoding/binary"
	"math"
)

const (
	FLAG_BYTE     = 8
	FLAG_GLOBAL   = 0
	FLAG_EVENT    = 1
	FLAG_FIELD    = 2
	FLAG_EXTENDED = 4
)

func getHeaderBitLength(key int) int {
	if key < 256 {
		return 2
	} else {
		return 3
	}
}

func serializeHeader(buf []byte, offset int, key int, flag byte) int {
	if key < 256 {
		buf[offset] = flag
		offset++
		buf[offset] = byte(key)
		offset++
	} else {
		buf[offset] = flag | FLAG_BYTE
		offset++
		binary.LittleEndian.PutUint16(buf[offset:], uint16(key))
		offset += 2
	}
	return offset
}

func serializeData(key int, value any, flag byte) []byte {
	headerLen := getHeaderBitLength(key)
	var buf []byte
	offset := 0

	switch v := value.(type) {
	case bool:
		if v {
			value = int32(1)
		} else {
			value = int32(0)
		}
	}

	switch v := value.(type) {
	case nil:
		if flag == FLAG_GLOBAL {
			buf = make([]byte, headerLen)
			offset = serializeHeader(buf, offset, key, flag)
			return buf
		}
	case int32:
		if v == 0 || v == 1 {
			buf = make([]byte, headerLen)
			offset = serializeHeader(buf, offset, key, flag|byte((v+1)<<4))
			return buf
		} else if v >= -128 && v < 128 {
			buf = make([]byte, headerLen+1)
			offset = serializeHeader(buf, offset, key, flag|(3<<4))
			buf[offset] = byte(int8(v))
			return buf
		} else if v >= -32768 && v < 32768 {
			buf = make([]byte, headerLen+2)
			offset = serializeHeader(buf, offset, key, flag|(4<<4))
			binary.LittleEndian.PutUint16(buf[offset:], uint16(int16(v)))
			return buf
		} else if v >= math.MinInt32 && v <= math.MaxInt32 {
			buf = make([]byte, headerLen+4)
			offset = serializeHeader(buf, offset, key, flag|(5<<4))
			binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(v)))
			return buf
		} else {
			buf = make([]byte, headerLen+8)
			offset = serializeHeader(buf, offset, key, flag|(7<<4))
			binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(float64(v)))
			return buf
		}
	case float64:
		buf = make([]byte, headerLen+8)
		offset = serializeHeader(buf, offset, key, flag|(7<<4))
		binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(v))
		return buf
	case string:
		strBytes := []byte(v)
		strLen := len(strBytes)
		if strLen < 256 {
			buf = make([]byte, headerLen+1+strLen)
			offset = serializeHeader(buf, offset, key, flag|(8<<4))
			buf[offset] = byte(strLen)
			offset++
		} else if strLen < 65536 {
			buf = make([]byte, headerLen+2+strLen)
			offset = serializeHeader(buf, offset, key, flag|(9<<4))
			binary.LittleEndian.PutUint16(buf[offset:], uint16(strLen))
			offset += 2
		} else {
			buf = make([]byte, headerLen+4+strLen)
			offset = serializeHeader(buf, offset, key, flag|(10<<4))
			binary.LittleEndian.PutUint32(buf[offset:], uint32(strLen))
			offset += 4
		}
		copy(buf[offset:], strBytes)
		return buf
	default:
		panic("serializeData: unsupported type")
	}

	panic("serializeData: unexpected case")
}
