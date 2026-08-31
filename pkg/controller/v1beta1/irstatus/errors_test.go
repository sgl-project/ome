package irstatus

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCodecErrorCatalog(t *testing.T) {
	t.Parallel()
	const catalogMessageLimit = 96

	catalog := []struct {
		reason  ErrorReason
		message string
	}{
		{ErrorReasonUnknownEncoding, "instance status encoding is unknown"},
		{ErrorReasonRepresentationUnion, "instance status representation is invalid"},
		{ErrorReasonRangeSyntax, "instance index set syntax is invalid"},
		{ErrorReasonRangeOrder, "instance index set order is invalid"},
		{ErrorReasonRangeOverflow, "instance index set exceeds the supported index domain"},
		{ErrorReasonCardinalityLimit, "instance status cardinality limit is invalid"},
		{ErrorReasonCoverage, "instance status column coverage is invalid"},
		{ErrorReasonCanonicalOrder, "instance status column order is not canonical"},
		{ErrorReasonValueDomain, "instance status value is outside its supported domain"},
	}

	seenReasons := make(map[ErrorReason]struct{}, len(catalog))
	seenMessages := make(map[string]struct{}, len(catalog))
	maximumMessageLength := 0
	for _, test := range catalog {
		t.Run(string(test.reason), func(t *testing.T) {
			err := newCodecError(test.reason)
			if got := err.Error(); got != test.message {
				t.Fatalf("Error() = %q, want %q", got, test.message)
			}
			gotReason, ok := ErrorReasonOf(fmt.Errorf("wrapped: %w", err))
			if !ok || gotReason != test.reason {
				t.Fatalf("ErrorReasonOf() = (%q, %t), want (%q, true)", gotReason, ok, test.reason)
			}
			if strings.Contains(err.Error(), "0-2147483647") {
				t.Fatal("fixed error text included status data")
			}
		})
		if _, exists := seenReasons[test.reason]; exists {
			t.Fatalf("duplicate reason %q", test.reason)
		}
		if _, exists := seenMessages[test.message]; exists {
			t.Fatalf("duplicate message %q", test.message)
		}
		seenReasons[test.reason] = struct{}{}
		seenMessages[test.message] = struct{}{}
		if len(test.message) > maximumMessageLength {
			maximumMessageLength = len(test.message)
		}
	}
	if maximumMessageLength > catalogMessageLimit {
		t.Fatalf("maximum catalog message length = %d, want at most %d", maximumMessageLength, catalogMessageLimit)
	}
	t.Logf("maximum codec error catalog message length: %d bytes", maximumMessageLength)

	const payload = "0-2147483647-sensitive-revision"
	unknown := newCodecError(ErrorReason("unexpected-" + payload))
	if unknown.Error() != "instance status codec failed" || strings.Contains(unknown.Error(), payload) {
		t.Fatalf("unknown reason exposed payload: %q", unknown.Error())
	}
	if reason, ok := ErrorReasonOf(unknown); ok || reason != "" {
		t.Fatalf("ErrorReasonOf(unknown) = (%q, %t), want (empty, false)", reason, ok)
	}
}

func TestCodecErrorExtractionRejectsOtherErrors(t *testing.T) {
	t.Parallel()

	if reason, ok := ErrorReasonOf(errors.New("unrelated")); ok || reason != "" {
		t.Fatalf("ErrorReasonOf() = (%q, %t), want (empty, false)", reason, ok)
	}
	var nilCodecError *codecError
	if got := nilCodecError.Error(); got != "instance status codec failed" {
		t.Fatalf("nil codecError message = %q", got)
	}
}

func assertCodecReason(t *testing.T, err error, want ErrorReason) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want reason %q", want)
	}
	got, ok := ErrorReasonOf(err)
	if !ok || got != want {
		t.Fatalf("ErrorReasonOf(%v) = (%q, %t), want (%q, true)", err, got, ok, want)
	}
}
