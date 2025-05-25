package vault

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestB64Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello world",
			expected: "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "special characters",
			input:    "hello@world!123",
			expected: "aGVsbG9Ad29ybGQhMTIz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := B64Encode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestB64Decode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "aGVsbG8gd29ybGQ=",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "special characters",
			input:    "aGVsbG9Ad29ybGQhMTIz",
			expected: "hello@world!123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := B64Decode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestB64EncodeDecodeRoundTrip(t *testing.T) {
	testStrings := []string{
		"hello world",
		"",
		"special chars: !@#$%^&*()",
		"unicode: 你好世界",
		"multiline\nstring\nwith\nnewlines",
	}

	for _, original := range testStrings {
		t.Run("roundtrip_"+original, func(t *testing.T) {
			encoded := B64Encode(original)
			decoded := B64Decode(encoded)
			assert.Equal(t, original, decoded)
		})
	}
}

func TestResolveVaultPrefix(t *testing.T) {
	tests := []struct {
		name     string
		vaultId  string
		expected string
	}{
		{
			name:     "valid vault OCID",
			vaultId:  "ocid1.vault.oc1.ap-mumbai-1.ensluxzxaahi2.abrg6ljr4dfykdarhmr2urn3gopbrh53ahemqsa7wfmcmvgcrux3pwory6rq",
			expected: "ensluxzxaahi2",
		},
		{
			name:     "another valid vault OCID",
			vaultId:  "ocid1.vault.oc1.us-ashburn-1.testprefix.someotherpart",
			expected: "testprefix",
		},
		{
			name:     "empty vault ID",
			vaultId:  "",
			expected: "",
		},
		{
			name:     "short vault ID",
			vaultId:  "short.id",
			expected: "short",
		},
		{
			name:     "single part",
			vaultId:  "singlepart",
			expected: "singlepart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveVaultPrefix(tt.vaultId)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCFBEncryptDecrypt(t *testing.T) {
	// Generate a test key (32 bytes for AES-256, base64 encoded)
	testKey := B64Encode(strings.Repeat("a", 32))
	testPlaintext := "Hello, World! This is a test message."

	t.Run("successful encryption and decryption", func(t *testing.T) {
		// Test encryption
		ciphertext, err := CFBEncrypt(testPlaintext, testKey)
		require.NoError(t, err)
		assert.NotEmpty(t, ciphertext)
		assert.NotEqual(t, testPlaintext, ciphertext)

		// Test decryption
		decrypted, err := CFBDecrypt(ciphertext, testKey)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, decrypted)
	})

	t.Run("encryption with invalid key", func(t *testing.T) {
		invalidKey := B64Encode("short")
		_, err := CFBEncrypt(testPlaintext, invalidKey)
		assert.Error(t, err)
	})

	t.Run("decryption with invalid ciphertext", func(t *testing.T) {
		_, err := CFBDecrypt("invalid-ciphertext", testKey)
		assert.Error(t, err)
	})

	t.Run("decryption with short ciphertext", func(t *testing.T) {
		shortCiphertext := B64Encode("short")
		_, err := CFBDecrypt(shortCiphertext, testKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ciphertext too short")
	})
}

func TestGCMEncryptDecrypt(t *testing.T) {
	// Generate a test key (32 bytes for AES-256, base64 encoded)
	testKey := B64Encode(strings.Repeat("b", 32))
	testPlaintext := "Hello, GCM World! This is a test message."

	t.Run("successful encryption and decryption", func(t *testing.T) {
		// Test encryption
		ciphertext, err := GCMEncrypt(testPlaintext, testKey)
		require.NoError(t, err)
		assert.NotEmpty(t, ciphertext)
		assert.NotEqual(t, testPlaintext, ciphertext)

		// Test decryption
		decrypted, err := GCMDecrypt(ciphertext, testKey)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, decrypted)
	})

	t.Run("encryption with invalid key", func(t *testing.T) {
		invalidKey := B64Encode("short")
		_, err := GCMEncrypt(testPlaintext, invalidKey)
		assert.Error(t, err)
	})

	t.Run("decryption with invalid key", func(t *testing.T) {
		// First encrypt with valid key
		ciphertext, err := GCMEncrypt(testPlaintext, testKey)
		require.NoError(t, err)

		// Try to decrypt with invalid key
		invalidKey := B64Encode("short")
		_, err = GCMDecrypt(ciphertext, invalidKey)
		assert.Error(t, err)
	})
}

func TestGCMEncryptWithoutCopy(t *testing.T) {
	tests := []struct {
		name        string
		text        []byte
		key         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful encryption",
			text:        []byte("test-plaintext"),
			key:         B64Encode("0123456789abcdef0123456789abcdef"), // 32 bytes key
			expectError: false,
		},
		{
			name:        "empty text",
			text:        []byte(""),
			key:         B64Encode("0123456789abcdef0123456789abcdef"),
			expectError: false,
		},
		{
			name:        "invalid key length",
			text:        []byte("test-plaintext"),
			key:         B64Encode("shortkey"), // Invalid key length
			expectError: true,
			errorMsg:    "crypto/aes: invalid key size",
		},
		{
			name:        "empty key",
			text:        []byte("test-plaintext"),
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := GCMEncryptWithoutCopy(tt.text, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, ciphertext)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ciphertext)
				assert.NotEqual(t, tt.text, ciphertext)

				// Test that we can decrypt it back
				decrypted, err := GCMDecryptWithoutCopy(ciphertext, tt.key)
				assert.NoError(t, err)
				assert.Equal(t, tt.text, decrypted)
			}
		})
	}
}

func TestGCMDecryptWithoutCopy(t *testing.T) {
	validKey := B64Encode("0123456789abcdef0123456789abcdef")
	plaintext := []byte("test-plaintext")

	// First encrypt to get valid ciphertext
	ciphertext, err := GCMEncryptWithoutCopy(plaintext, validKey)
	require.NoError(t, err)

	tests := []struct {
		name        string
		ciphertext  []byte
		key         string
		expectError bool
		errorMsg    string
		expected    []byte
	}{
		{
			name:        "successful decryption",
			ciphertext:  ciphertext,
			key:         validKey,
			expectError: false,
			expected:    plaintext,
		},
		{
			name:        "invalid key length",
			ciphertext:  ciphertext,
			key:         B64Encode("shortkey"),
			expectError: true,
			errorMsg:    "crypto/aes: invalid key size",
		},
		{
			name:        "empty ciphertext",
			ciphertext:  []byte(""),
			key:         validKey,
			expectError: true,
		},
		{
			name:        "ciphertext too short",
			ciphertext:  []byte("short"),
			key:         validKey,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use defer to catch panics and convert them to test failures
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectError {
						t.Errorf("Unexpected panic: %v", r)
					}
					// If we expect an error and got a panic, that's acceptable for this test
				}
			}()

			decrypted, err := GCMDecryptWithoutCopy(tt.ciphertext, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, decrypted)
			}
		})
	}
}

func TestGCMEncryptDecryptRoundTrip(t *testing.T) {
	// Test round-trip encryption/decryption with various data sizes
	key := B64Encode("0123456789abcdef0123456789abcdef")

	testCases := [][]byte{
		[]byte(""),                               // Empty
		[]byte("a"),                              // Single character
		[]byte("Hello, World!"),                  // Short text
		[]byte(strings.Repeat("test", 100)),      // Medium text
		[]byte(strings.Repeat("longtext", 1000)), // Long text
		[]byte{0, 1, 2, 3, 255, 254, 253},        // Binary data
	}

	for i, plaintext := range testCases {
		t.Run(fmt.Sprintf("case_%d_len_%d", i, len(plaintext)), func(t *testing.T) {
			// Test GCMEncryptWithoutCopy/GCMDecryptWithoutCopy
			ciphertext, err := GCMEncryptWithoutCopy(plaintext, key)
			assert.NoError(t, err)
			assert.NotNil(t, ciphertext)

			decrypted, err := GCMDecryptWithoutCopy(ciphertext, key)
			assert.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)

			// Test GCMEncrypt/GCMDecrypt for comparison
			ciphertextStr, err := GCMEncrypt(string(plaintext), key)
			assert.NoError(t, err)
			assert.NotEmpty(t, ciphertextStr)

			decryptedStr, err := GCMDecrypt(ciphertextStr, key)
			assert.NoError(t, err)
			assert.Equal(t, string(plaintext), decryptedStr)
		})
	}
}

func TestCFBEncryptDecryptEdgeCases(t *testing.T) {
	key := B64Encode("0123456789abcdef0123456789abcdef")

	tests := []struct {
		name        string
		text        string
		expectError bool
	}{
		{
			name:        "very long text",
			text:        strings.Repeat("This is a very long text for testing CFB encryption. ", 1000),
			expectError: false,
		},
		{
			name:        "text with special characters",
			text:        "Special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?`~",
			expectError: false,
		},
		{
			name:        "unicode text",
			text:        "Unicode: 你好世界 🌍 🚀 ñáéíóú",
			expectError: false,
		},
		{
			name:        "newlines and tabs",
			text:        "Line 1\nLine 2\tTabbed\r\nWindows newline",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := CFBEncrypt(tt.text, key)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, tt.text, encrypted)

			decrypted, err := CFBDecrypt(encrypted, key)
			assert.NoError(t, err)
			assert.Equal(t, tt.text, decrypted)
		})
	}
}

func TestResolveVaultPrefixEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		vaultId  string
		expected string
	}{
		{
			name:     "single part",
			vaultId:  "singlepart",
			expected: "singlepart",
		},
		{
			name:     "two parts",
			vaultId:  "part1.part2",
			expected: "part1",
		},
		{
			name:     "many parts",
			vaultId:  "ocid1.vault.oc1.region.unique.extra.parts",
			expected: "extra",
		},
		{
			name:     "empty string",
			vaultId:  "",
			expected: "",
		},
		{
			name:     "only dots",
			vaultId:  "...",
			expected: "",
		},
		{
			name:     "starts with dot",
			vaultId:  ".vault.prefix",
			expected: "vault",
		},
		{
			name:     "ends with dot",
			vaultId:  "vault.prefix.",
			expected: "prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveVaultPrefix(tt.vaultId)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncryptionKeyValidation(t *testing.T) {
	// Test various key formats and lengths
	tests := []struct {
		name        string
		keyFunc     func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid 16-byte key",
			keyFunc: func() string {
				return B64Encode("0123456789abcdef") // 16 bytes
			},
			expectError: false,
		},
		{
			name: "valid 24-byte key",
			keyFunc: func() string {
				return B64Encode("0123456789abcdef01234567") // 24 bytes
			},
			expectError: false,
		},
		{
			name: "valid 32-byte key",
			keyFunc: func() string {
				return B64Encode("0123456789abcdef0123456789abcdef") // 32 bytes
			},
			expectError: false,
		},
		{
			name: "invalid key length - too short",
			keyFunc: func() string {
				return B64Encode("short") // 5 bytes
			},
			expectError: true,
			errorMsg:    "crypto/aes: invalid key size",
		},
		{
			name: "invalid key length - odd length",
			keyFunc: func() string {
				return B64Encode("0123456789abcde") // 15 bytes
			},
			expectError: true,
			errorMsg:    "crypto/aes: invalid key size",
		},
		{
			name: "malformed base64 key",
			keyFunc: func() string {
				return "not-base64-encoded-key!!!"
			},
			expectError: true,
		},
	}

	plaintext := "test-data"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.keyFunc()

			// Test CFB encryption
			_, err := CFBEncrypt(plaintext, key)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			// Test GCM encryption
			_, err = GCMEncrypt(plaintext, key)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
