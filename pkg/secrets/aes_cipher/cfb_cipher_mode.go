package aes_cipher

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/secrets"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

/* Use AES CFB cipher mode to do encryption/decryption using DEK */

func CFBEncrypt(text string, key string) (string, error) {
	decodedKey := secrets.B64Decode(key)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len([]byte(text)))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(text))

	// convert to base64
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func CFBDecrypt(text string, key string) (string, error) {
	decodedKey := secrets.B64Decode(key)
	ciphertext, _ := base64.URLEncoding.DecodeString(text)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return "", err
	}

	// The IV needs to be unique, but not secure. Therefore, it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}
