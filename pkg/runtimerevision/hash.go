package runtimerevision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Hash returns sha256(JSON-marshaled spec) in two forms: full hex
// (64 chars) for collision auditing and the 8-char short prefix the
// revision name uses.
//
// Determinism: ServingRuntimeSpec contains only maps with string keys
// (Annotations, Labels, NodeSelector, ...); Go's encoding/json sorts
// string-keyed maps and serializes struct fields in declaration order.
// Same spec → same bytes → same hash across controller restarts.
func Hash(spec *v1beta1.ServingRuntimeSpec) (full, short string, err error) {
	if spec == nil {
		return "", "", fmt.Errorf("runtimerevision.Hash: nil spec")
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return "", "", fmt.Errorf("marshal spec: %w", err)
	}
	sum := sha256.Sum256(b)
	full = hex.EncodeToString(sum[:])
	return full, full[:8], nil
}
