package ecdh

import (
	"io"

	"golang.org/x/crypto/curve25519"
	"crypto/rand"
)

func GenerateCurve25519Key() (*[32]byte, *[32]byte, error) {
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

func GenerateCurve25519SharedSecret(priv, pub [32]byte) []byte {
	secret := new([32]byte)

	curve25519.ScalarMult(secret, &priv, &pub)

	return secret[:]
}