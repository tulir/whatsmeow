package hkdf

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"golang.org/x/crypto/hkdf"
	"io"
)

func Expand(key []byte, length int, info string) ([]byte, error) {
	if info == "" {
		keyBlock := hmac.New(sha256.New, key)
		var out, last []byte

		var blockIndex byte = 1
		for i := 0; len(out) < length; i++ {
			keyBlock.Reset()
			//keyBlock.Write(append(append(last, []byte(info)...), blockIndex))
			keyBlock.Write(last)
			keyBlock.Write([]byte(info))
			keyBlock.Write([]byte{blockIndex})
			last = keyBlock.Sum(nil)
			blockIndex += 1
			out = append(out, last...)
		}
		return out[:length], nil
	} else {
		h := hkdf.New(sha256.New, key, nil, []byte(info))
		out := make([]byte, length)
		n, err := io.ReadAtLeast(h, out, length)
		if err != nil {
			return nil, err
		}
		if n != length {
			return nil, fmt.Errorf("new key to short")
		}

		return out[:length], nil
	}
}
