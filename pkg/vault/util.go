package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
)

func B64Encode(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func B64Decode(data string) string {
	decoded, _ := base64.StdEncoding.DecodeString(data)
	return string(decoded)
}

/*
ResolveVaultPrefix resolve vault prefix from vault ocid

	e.g. vault ocid: "ocid1.vault.oc1.ap-mumbai-1.ensluxzxaahi2.abrg6ljr4dfykdarhmr2urn3gopbrh53ahemqsa7wfmcmvgcrux3pwory6rq"
	     vault prefix: "ensluxzxaahi2"
*/
func ResolveVaultPrefix(vaultId string) string {
	if len(vaultId) <= 0 {
		return ""
	}
	vaultIdChunks := strings.Split(vaultId, ".")
	if len(vaultIdChunks) < 2 {
		return vaultIdChunks[0]
	}
	return vaultIdChunks[len(vaultIdChunks)-2]
}

func CFBEncrypt(text string, key string) (string, error) {
	// Use GCM encryption instead of deprecated CFB
	return GCMEncrypt(text, key)
}

func CFBDecrypt(text string, key string) (string, error) {
	// Use GCM decryption instead of deprecated CFB
	return GCMDecrypt(text, key)
}

func GCMEncrypt(text string, key string) (string, error) {
	decodedKey := B64Decode(key)

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
	decodedKey := B64Decode(key)
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
	decodedKey := B64Decode(key)

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
	decodedKey := B64Decode(key)

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
