package curve25519

import (
	"crypto/rand"
	"golang.org/x/crypto/curve25519"
	"io"
)

func GenerateKey() (*[32]byte, *[32]byte, error) {
	var pub, priv [32]byte
	var err error

	_, err = io.ReadFull(rand.Reader, priv[:])
	if err != nil {
		return nil, nil, err
	}

	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	curve25519.ScalarBaseMult(&pub, &priv)

	return &priv, &pub, nil
}

func GenerateSharedSecret(priv, pub [32]byte) []byte {
	var secret [32]byte

	curve25519.ScalarMult(&secret, &priv, &pub)

	return secret[:]
}
