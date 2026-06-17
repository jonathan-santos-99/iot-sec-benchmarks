package ecryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"strings"

	"golang.org/x/crypto/chacha20"
)

type Algorithm int

const (
	PlainText Algorithm = iota
	AES
	ChaCha20
)

var Cyphers = make(map[Algorithm]cipher.Block)

func (a *Algorithm) UnmarshalText(text []byte) error {
	name := string(text)
	switch strings.ToUpper(name) {
	case "PLAIN_TEXT":
		*a = PlainText
	case "AES":
		*a = AES
	case "CHACHA20":
		*a = ChaCha20
	default:
		return fmt.Errorf("Could not parse %s as algorithm", text)
	}

	return nil
}

func marshalText(a Algorithm) ([]byte, error) {
	switch a {
	case PlainText:
		return []byte("PLAIN_TEXT"), nil
	case AES:
		return []byte("AES"), nil
	case ChaCha20:
		return []byte("CHACHA20"), nil
	}

	return nil, fmt.Errorf("Could not parse %s as algorithm", a)
}

func (a Algorithm) String() string {
	return [...]string{"PLAIN_TEXT", "AES", "CHACHA20"}[a]
}

func Encrypt(algorithm Algorithm, key, plaintext []byte) ([]byte, error) {
	var err error = nil
	var data []byte
	switch algorithm {
	case PlainText:
		data = plaintext
	case AES:
		data, err = encryptAES(plaintext)
	case ChaCha20:
		data, err = encryptChaCha20(plaintext, key)
	}

	return data, err
}

func Decrypt(algorithm Algorithm, key, ciphertext []byte) ([]byte, error) {
	var err error = nil
	var data []byte
	switch algorithm {
	case PlainText:
		data = ciphertext
	case AES:
		data, err = decryptAES(ciphertext)
	case ChaCha20:
		data, err = decryptChaCha20(ciphertext, key)
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

func encryptChaCha20(plaintext, key []byte) ([]byte, error) {
	nonce := make([]byte, chacha20.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		log.Fatal(err)
	}

	cipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		log.Fatal(err)
	}

	// Encrypt plaintext into ciphertext buffer
	ciphertext := make([]byte, len(plaintext))
	cipher.XORKeyStream(ciphertext, plaintext)

	return append(nonce, ciphertext...), nil
}

func decryptChaCha20(ciphertext, key []byte) ([]byte, error) {
	// To decrypt, you must recreate the cipher with the exact same Key and Nonce
	if len(ciphertext) <= chacha20.NonceSize {
		return nil, fmt.Errorf("Invalid ciphertext size")
	}
	nonce := ciphertext[:chacha20.NonceSize]
	ciphertext = ciphertext[chacha20.NonceSize:]
	decipher, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		log.Fatal(err)
	}

	// Decrypt ciphertext back into original plaintext
	decrypted := make([]byte, len(ciphertext))
	decipher.XORKeyStream(decrypted, ciphertext)
	return decrypted, nil
}
