/*
HKDF is a simple key derivation function (KDF) based on
a hash-based message authentication code (HMAC). It was initially proposed by its authors as a building block in
various protocols and applications, as well as to discourage the proliferation of multiple KDF mechanisms.
The main approach HKDF follows is the "extract-then-expand" paradigm, where the KDF logically consists of two modules:
the first stage takes the input keying material and "extracts" from it a fixed-length pseudorandom key, and then the
second stage "expands" this key into several additional pseudorandom keys (the output of the KDF).
*/
package hkdf

import (
	"crypto/sha256"
	"fmt"
	"golang.org/x/crypto/hkdf"
	"io"
)

/*
Expand expands a given key with the HKDF algorithm.
*/
func Expand(key []byte, length int, info string) ([]byte, error) {
	var h io.Reader
	if info == "" {
		/*
			Only used during initial login
			Pseudorandom Key is provided by server and has not to be created
		*/
		h = hkdf.Expand(sha256.New, key, []byte(info))
	} else {
		/*
			Used every other time
			Pseudorandom Key is created during kdf.New
			This is the normal that crypto/hkdf is used
		*/
		h = hkdf.New(sha256.New, key, nil, []byte(info))
	}
	out := make([]byte, length)
	n, err := io.ReadAtLeast(h, out, length)
	if err != nil {
		return nil, err
	}
	if n != length {
		return nil, fmt.Errorf("new key to short")
	}

	return out, nil
}
