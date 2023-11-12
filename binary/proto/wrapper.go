package proto

import (
	"fmt"
	"strings"
)

// Bool stores v in a new bool value and returns a pointer to it.
func Bool(v bool) *bool {
	return &v
}

// Float32 stores v in a new float32 value and returns a pointer to it.
func Float32(v float32) *float32 {
	return &v
}

// Float64 stores v in a new float64 value and returns a pointer to it.
func Float64(v float64) *float64 {
	return &v
}

// Int32 stores v in a new int32 value and returns a pointer to it.
func Int32(v int32) *int32 {
	return &v
}

// Int64 stores v in a new int64 value and returns a pointer to it.
func Int64(v int64) *int64 {
	return &v
}

// String stores v in a new string value and returns a pointer to it.
func String(v string) *string {
	return &v
}

// Uint32 stores v in a new uint32 value and returns a pointer to it.
func Uint32(v uint32) *uint32 {
	return &v
}

// Uint64 stores v in a new uint64 value and returns a pointer to it.
func Uint64(v uint64) *uint64 {
	return &v
}

// HexArgb converts a rgba hex string to an argb uint32 and returns a pointer to it.
//
//	The string may be prefixed with a hash sign (#) and may contain 3, 6 or 8
//	hex digits. If the string contains 3 digits, each digit is repeated to
//	produce a 6-digit hex string. If the string contains 6 digits, the alpha
//	channel is assumed to be 255. If the string contains 8 digits, the alpha
//	channel is read from the string. #fff #ffffff #ffffff69 are all valid
//	strings.
func HexArgb(x string) *uint32 {
	var r, g, b, a, color uint32
	x = strings.TrimPrefix(x, "#")
	a = 255
	if len(x) == 3 {
		fmt.Sscanf(x, "%1x%1x%1x", &r, &g, &b)
		r |= r << 4
		g |= g << 4
		b |= b << 4
	}
	if len(x) == 6 {
		fmt.Sscanf(x, "%02x%02x%02x", &r, &g, &b)
	}
	if len(x) == 8 {
		fmt.Sscanf(x, "%02x%02x%02x%02x", &r, &g, &b, &a)
	}
	color = (a << 24) | (r << 16) | (g << 8) | (b)
	return &color
}
