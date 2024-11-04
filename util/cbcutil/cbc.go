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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

/*
Decrypt is a function that decrypts a given cipher text with a provided key and initialization vector(iv).
*/
func Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	} else if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext is shorter then block size: %d / %d", len(ciphertext), aes.BlockSize)
	}

	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(ciphertext, ciphertext)

	return unpad(ciphertext)
}

type File interface {
	io.Reader
	io.WriterAt
	Truncate(size int64) error
	Stat() (os.FileInfo, error)
}

func DecryptFile(key, iv []byte, file File) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	cbc := cipher.NewCBCDecrypter(block, iv)
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()
	if fileSize%aes.BlockSize != 0 {
		return fmt.Errorf("file size is not a multiple of the block size: %d / %d", fileSize, aes.BlockSize)
	}

	var bufSize int64 = 32 * 1024
	if fileSize < bufSize {
		bufSize = fileSize
	}
	buf := make([]byte, bufSize)
	var writePtr int64
	var lastByte byte
	for writePtr < fileSize {
		if writePtr+bufSize > fileSize {
			buf = buf[:fileSize-writePtr]
		}
		var n int
		n, err = io.ReadFull(file, buf)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		} else if n != len(buf) {
			return fmt.Errorf("failed to read full buffer: %d / %d", n, len(buf))
		}
		cbc.CryptBlocks(buf, buf)
		n, err = file.WriteAt(buf, writePtr)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		} else if n != len(buf) {
			return fmt.Errorf("failed to write full buffer: %d / %d", n, len(buf))
		}
		writePtr += int64(len(buf))
		lastByte = buf[len(buf)-1]
	}
	if int64(lastByte) > fileSize {
		return fmt.Errorf("padding is greater then the length: %d / %d", lastByte, fileSize)
	}
	err = file.Truncate(fileSize - int64(lastByte))
	if err != nil {
		return fmt.Errorf("failed to truncate file to remove padding: %w", err)
	}
	return nil
}

/*
Encrypt is a function that encrypts plaintext with a given key and an optional initialization vector(iv).
*/
func Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	sizeOfLastBlock := len(plaintext) % aes.BlockSize
	paddingLen := aes.BlockSize - sizeOfLastBlock
	plaintextStart := plaintext[:len(plaintext)-sizeOfLastBlock]
	lastBlock := append(plaintext[len(plaintext)-sizeOfLastBlock:], bytes.Repeat([]byte{byte(paddingLen)}, paddingLen)...)

	if len(plaintextStart)%aes.BlockSize != 0 {
		panic(fmt.Errorf("plaintext is not the correct size: %d %% %d != 0", len(plaintextStart), aes.BlockSize))
	}
	if len(lastBlock) != aes.BlockSize {
		panic(fmt.Errorf("last block is not the correct size: %d != %d", len(lastBlock), aes.BlockSize))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	var ciphertext []byte
	if iv == nil {
		ciphertext = make([]byte, aes.BlockSize+len(plaintext)+paddingLen)
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}

		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(ciphertext[aes.BlockSize:], plaintextStart)
		cbc.CryptBlocks(ciphertext[aes.BlockSize+len(plaintextStart):], lastBlock)
	} else {
		ciphertext = make([]byte, len(plaintext)+paddingLen, len(plaintext)+paddingLen+10)

		cbc := cipher.NewCBCEncrypter(block, iv)
		cbc.CryptBlocks(ciphertext, plaintextStart)
		cbc.CryptBlocks(ciphertext[len(plaintextStart):], lastBlock)
	}

	return ciphertext, nil
}

func unpad(src []byte) ([]byte, error) {
	length := len(src)
	padLen := int(src[length-1])

	if padLen > length {
		return nil, fmt.Errorf("padding is greater then the length: %d / %d", padLen, length)
	}

	return src[:(length - padLen)], nil
}

func EncryptStream(key, iv, macKey []byte, plaintext io.Reader, ciphertext io.Writer) ([]byte, []byte, uint64, uint64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to create cipher: %w", err)
	}
	cbc := cipher.NewCBCEncrypter(block, iv)

	plainHasher := sha256.New()
	cipherHasher := sha256.New()
	cipherMAC := hmac.New(sha256.New, macKey)
	cipherMAC.Write(iv)

	writerAt, hasWriterAt := ciphertext.(io.WriterAt)

	buf := make([]byte, 32*1024)
	var size, extraSize int
	var writePtr int64
	hasMore := true
	for hasMore {
		var n int
		n, err = io.ReadFull(plaintext, buf)
		plainHasher.Write(buf[:n])
		size += n
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			padding := aes.BlockSize - size%aes.BlockSize
			buf = append(buf[:n], bytes.Repeat([]byte{byte(padding)}, padding)...)
			extraSize = padding
			hasMore = false
		} else if err != nil {
			return nil, nil, 0, 0, fmt.Errorf("failed to read file: %w", err)
		}
		cbc.CryptBlocks(buf, buf)
		cipherMAC.Write(buf)
		cipherHasher.Write(buf)
		if hasWriterAt {
			_, err = writerAt.WriteAt(buf, writePtr)
			writePtr += int64(len(buf))
		} else {
			_, err = ciphertext.Write(buf)
		}
		if err != nil {
			return nil, nil, 0, 0, fmt.Errorf("failed to write file: %w", err)
		}
	}
	mac := cipherMAC.Sum(nil)[:10]
	extraSize += 10
	cipherHasher.Write(mac)
	if hasWriterAt {
		_, err = writerAt.WriteAt(mac, writePtr)
	} else {
		_, err = ciphertext.Write(mac)
	}
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to write checksum to file: %w", err)
	}
	return plainHasher.Sum(nil), cipherHasher.Sum(nil), uint64(size), uint64(size + extraSize), nil
}
