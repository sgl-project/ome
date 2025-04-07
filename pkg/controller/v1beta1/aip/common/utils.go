package common

import (
	"errors"
	"math/big"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ShouldRetry returns true if the error should be retried
func ShouldRetry(err error) bool {
	if errors.Unwrap(err) != nil {
		return true
	}

	// Alternatively, check for known error types
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.Status().Reason {
		case metav1.StatusReasonConflict,
			metav1.StatusReasonTimeout,
			metav1.StatusReasonServerTimeout,
			metav1.StatusReasonTooManyRequests:
			return true
		}
	}

	return false
}

// GenerateId returns a base64 encoded UUID with a prefix.
func GenerateId(prefix string, uid types.UID) string {
	parsedUUID, err := uuid.Parse(string(uid))
	if err != nil {
		panic(err)
	}

	encoded := encodeBase62(parsedUUID[:])
	return prefix + encoded
}

const base62chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const base62Len = 22

func encodeBase62(b []byte) string {
	num := new(big.Int).SetBytes(b)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	encoded := ""

	for num.Cmp(zero) > 0 {
		mod := new(big.Int)
		num.DivMod(num, base, mod)
		encoded = string(base62chars[mod.Int64()]) + encoded
	}

	for len(encoded) < base62Len {
		encoded = "0" + encoded
	}

	return encoded
}
