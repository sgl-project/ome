package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGenerateRandomPassword_LengthAndCharacterClasses(t *testing.T) {
	length := 16
	pw := GenerateRandomPassword(length)

	if utf8.RuneCountInString(pw) != length {
		t.Fatalf("expected length %d, got %d (%q)", length, utf8.RuneCountInString(pw), pw)
	}

	// must contain at least one from each class
	if !containsAny(pw, specialChars) {
		t.Fatalf("password missing special character: %q", pw)
	}
	if !containsAny(pw, upperAlphabets) {
		t.Fatalf("password missing uppercase letter: %q", pw)
	}
	if !containsAny(pw, lowerAlphabets) {
		t.Fatalf("password missing lowercase letter: %q", pw)
	}
	if !containsAny(pw, numbers) {
		t.Fatalf("password missing digit: %q", pw)
	}
}

func TestGenerateRandomPassword_MinLengthPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for length < 4, but no panic occurred")
		}
	}()
	_ = GenerateRandomPassword(3)
}

func TestGenerateRandomPassword_RandomnessBasic(t *testing.T) {
	// Not a cryptographic test—just sanity that we’re not returning the same string every time.
	const n = 5
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		seen[GenerateRandomPassword(20)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected at least 2 distinct passwords across %d generations; got %d", n, len(seen))
	}
}

func containsAny(s, charset string) bool {
	return strings.ContainsAny(s, charset)
}
