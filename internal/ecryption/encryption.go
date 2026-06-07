package ecryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
)

type Algorithm int

const (
	PlainText Algorithm = iota
	AES
)

var Cyphers = make(map[Algorithm]cipher.Block)

func (a *Algorithm) UnmarshalText(text []byte) error {
	name := string(text)
	switch strings.ToUpper(name) {
	case "PLAIN_TEXT":
		*a = PlainText
	case "AES":
		*a = AES
	default:
		return fmt.Errorf("Could not parse %s as algorithm", text)
	}

	return nil
}

func (a Algorithm) String() string {
	return [...]string{"PLAIN_TEXT", "AES"}[a]
}

func Encrypt(algorithm Algorithm, plaintext []byte) ([]byte, error) {
	var err error = nil
	var data []byte
	switch algorithm {
	case PlainText:
		data = plaintext
	case AES:
		data, err = encryptAES(plaintext)
	}

	return data, err
}

func Decrypt(algorithm Algorithm, ciphertext []byte) ([]byte, error) {
	var err error = nil
	var data []byte
	switch algorithm {
	case PlainText:
		data = ciphertext
	case AES:
		data, err = decryptAES(ciphertext)
	}

	return data, err
}

// pkcs7Pad adds padding to the plaintext to make it a multiple of the block size.
func pkcs7Pad(b []byte, blockSize int) []byte {
	padding := blockSize - (len(b) % blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(b, padtext...)
}

// pkcs7Unpad removes the padding from the decrypted plaintext.
func pkcs7Unpad(b []byte) ([]byte, error) {
	length := len(b)
	if length == 0 {
		return nil, fmt.Errorf("decrypted data is empty")
	}
	padding := int(b[length-1])
	if padding < 1 || padding > aes.BlockSize || length < padding {
		return nil, fmt.Errorf("invalid padding")
	}
	return b[:length-padding], nil
}

func encryptAES(plaintext []byte) ([]byte, error) {
	block, ok := Cyphers[AES]
	if !ok {
		return nil, fmt.Errorf("Could not find block cypher to AES")
	}

	// CBC requires padding
	paddedPlaintext := pkcs7Pad(plaintext, aes.BlockSize)

	// The ciphertext needs space for the IV + the padded plaintext
	ciphertext := make([]byte, aes.BlockSize+len(paddedPlaintext))
	iv := ciphertext[:aes.BlockSize]

	// Generate a random Initialization Vector (IV)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], paddedPlaintext)

	return ciphertext, nil
}

func decryptAES(ciphertext []byte) ([]byte, error) {
	block, ok := Cyphers[AES]
	if !ok {
		return nil, fmt.Errorf("Could not find block cypher to AES")
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	actualCiphertext := ciphertext[aes.BlockSize:]

	if len(actualCiphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(actualCiphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, actualCiphertext)

	return pkcs7Unpad(plaintext)
}
