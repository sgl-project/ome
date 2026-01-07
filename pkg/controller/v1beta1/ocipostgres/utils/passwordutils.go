package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const (
	upperAlphabets = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerAlphabets = "abcdefghijklmnopqrstuvwxyz"
	specialChars   = "-_."
	numbers        = "0123456789"
)

// GenerateRandomPassword generates a random password with the specified length
// and at least one special character, one uppercase letter, one lowercase letter, and one number.
func GenerateRandomPassword(length int) string {
	if length < 4 {
		panic("Password length must be at least 4")
	}
	// Initialize the password slice with the required characters
	password := make([]byte, length)
	password[0] = getRandomChar(specialChars)
	password[1] = getRandomChar(upperAlphabets)
	password[2] = getRandomChar(lowerAlphabets)
	password[3] = getRandomChar(numbers)

	// Fill the rest of the password with random characters from all sets
	charset := specialChars + upperAlphabets + lowerAlphabets + numbers
	for i := 4; i < length; i++ {
		password[i] = getRandomChar(charset)
	}

	// Shuffle the password securely
	secureShuffle(password)
	return string(password)
}

// getRandomChar returns a random character from the given charset using crypto/rand
func getRandomChar(charset string) byte {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		panic(err)
	}
	return charset[nBig.Int64()]
}

// secureShuffle shuffles a byte slice securely using crypto/rand
func secureShuffle(slice []byte) {
	for i := len(slice) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			panic(err)
		}
		j := jBig.Int64()
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func HashPasswordSHA256Hex(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}
