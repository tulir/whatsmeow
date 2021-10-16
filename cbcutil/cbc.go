/*
CBC describes a block cipher mode. In cryptography, a block cipher mode of operation is an algorithm that uses a
block cipher to provide an information service such as confidentiality or authenticity. A block cipher by itself
is only suitable for the secure cryptographic transformation (encryption or decryption) of one fixed-length group of
bits called a block. A mode of operation describes how to repeatedly apply a cipher's single-block operation to
securely transform amounts of data larger than a block.

This package simplifies the usage of AES-256-CBC.
*/
package cbcutil

/*
Some code is provided by the GitHub user locked (github.com/locked):
https://gist.github.com/locked/b066aa1ddeb2b28e855e
Thanks!
*/
import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

/*
Decrypt is a function that decrypts a given cipher text with a provided key and initialization vector(iv).
*/
func Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext is shorter then block size: %d / %d", len(ciphertext), aes.BlockSize)
	}

	if iv == nil {
		iv = ciphertext[:aes.BlockSize]
		ciphertext = ciphertext[aes.BlockSize:]
	}

	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(ciphertext, ciphertext)

	return unpad(ciphertext)
}

/*
Encrypt is a function that encrypts plaintext with a given key and an optional initialization vector(iv).
*/
func Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	plaintext = pad(plaintext, aes.BlockSize)

	if len(plaintext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("plaintext is not a multiple of the block size: %d / %d", len(plaintext), aes.BlockSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	var ciphertext []byte
	if iv == nil {
		ciphertext = make([]byte, aes.BlockSize+len(plaintext))
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}

		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)
	} else {
		ciphertext = make([]byte, len(plaintext))

		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(ciphertext, plaintext)
	}

	return ciphertext, nil
}

func pad(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func unpad(src []byte) ([]byte, error) {
	length := len(src)
	padLen := int(src[length-1])

	if padLen > length {
		return nil, fmt.Errorf("padding is greater then the length: %d / %d", padLen, length)
	}

	return src[:(length - padLen)], nil
}
