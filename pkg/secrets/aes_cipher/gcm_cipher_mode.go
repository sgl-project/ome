package aes_cipher

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

/* Use AES GCM cipher mode to do encryption/decryption using DEK */

func GCMEncrypt(text string, key string) (string, error) {
	decodedKey := secrets.B64Decode(key)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// creates a new byte array the size of the nonce which must be passed to Seal
	nonce := make([]byte, gcm.NonceSize())

	// populates our nonce with a cryptographically secure random sequence
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(text), nil)
	return string(cipherText), nil
}

func GCMDecrypt(text string, key string) (string, error) {
	decodedKey := secrets.B64Decode(key)
	cipherText := []byte(text)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := cipherText[:gcm.NonceSize()]
	cipherText = cipherText[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	return string(plainText), err
}

func GCMEncryptWithoutCopy(text []byte, key string) ([]byte, error) {
	decodedKey := secrets.B64Decode(key)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// creates a new byte array the size of the nonce which must be passed to Seal
	nonce := make([]byte, gcm.NonceSize())

	// populates our nonce with a cryptographically secure random sequence
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherText := gcm.Seal(nonce, nonce, text, nil)
	return cipherText, nil
}

func GCMDecryptWithoutCopy(cipherText []byte, key string) ([]byte, error) {
	decodedKey := secrets.B64Decode(key)

	block, err := aes.NewCipher([]byte(decodedKey))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := cipherText[:gcm.NonceSize()]
	cipherText = cipherText[gcm.NonceSize():]
	// add cipherText[:0] as dst to reuse the cipherText's memory
	plainText, err := gcm.Open(cipherText[:0], nonce, cipherText, nil)
	return plainText, err
}
