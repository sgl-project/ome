// Package digest holds the short content hash that the policy digests, the
// rollout run identities and the chart generator's schema partial names share.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
)

// Short returns the first n hex characters of the SHA-256 of data, so n is at
// most 64. Each caller fixes n once: a stored digest or a partial name stays
// comparable only while its length stays the same.
func Short(data []byte, n int) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:n]
}
