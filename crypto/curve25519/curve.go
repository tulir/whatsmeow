/*
In cryptography, Curve25519 is an elliptic curve offering 128 bits of security and designed for use with the elliptic
curve Diffie–Hellman (ECDH) key agreement scheme. It is one of the fastest ECC curves and is not covered by any known
patents. The reference implementation is public domain software. The original Curve25519 paper defined it
as a Diffie–Hellman (DH) function.
*/
package curve25519

import (
	"crypto/rand"
	"golang.org/x/crypto/curve25519"
	"io"
)

/*
GenerateKey generates a public private key pair using Curve25519.
*/
func GenerateKey() (privateKey *[32]byte, publicKey *[32]byte, err error) {
	var pub, priv [32]byte

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

/*
GenerateSharedSecret generates the shared secret with a given public private key pair.
*/
func GenerateSharedSecret(priv, pub [32]byte) []byte {
	var secret [32]byte

	curve25519.ScalarMult(&secret, &priv, &pub)

	return secret[:]
}
