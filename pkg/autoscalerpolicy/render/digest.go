package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const (
	portableDigestPrefix = "pv1:"
	resolvedDigestPrefix = "rv1:"
	digestHexLength      = 12
)

// PortableDigest hashes the policy spec in a canonical form: API defaults
// applied first, then deterministic JSON. Defaulting-before-hashing is
// load-bearing — the apiserver stores a defaulted object (e.g.
// enforcement: Default filled by its schema default) while a raw GitOps file
// omits the field; hashing raw bytes would make byte-identical files
// digest-mismatch their own stored objects. Every consumer of the digest
// (member controllers, the control-plane preflight, CI tooling) links this
// function, so the values are comparable with no trust chain.
func PortableDigest(spec *v1beta1.AutoscalerPolicySpec) (string, error) {
	canonical := canonicalizeSpec(spec)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encode policy spec for digest: %w", err)
	}
	return portableDigestPrefix + shortHash(encoded), nil
}

// canonicalizeSpec returns a defaulted deep copy of the spec. Keep this in
// lockstep with the CRD schema defaults: a default added to the API without
// a matching line here splits the digest between raw files and stored
// objects.
func canonicalizeSpec(spec *v1beta1.AutoscalerPolicySpec) *v1beta1.AutoscalerPolicySpec {
	canonical := spec.DeepCopy()
	if canonical.Enforcement == "" {
		canonical.Enforcement = v1beta1.PolicyEnforcementDefault
	}
	return canonical
}

// resolvedDigestFor hashes one component's rendered output together with the
// inputs that vary per cluster: the effective bounds and the bound provider
// endpoint. It differs across homes by design; its job is per-home
// provenance ("did my policy edit land here?"), not cross-cluster equality —
// that is the portable digest's job.
func resolvedDigestFor(rendered *v1beta1.ComponentAutoscaler, rctx Context, serverAddresses []string) (string, error) {
	payload := struct {
		Autoscaler      *v1beta1.ComponentAutoscaler `json:"autoscaler"`
		MinReplicas     int32                        `json:"minReplicas"`
		MaxReplicas     int32                        `json:"maxReplicas"`
		ServerAddresses []string                     `json:"serverAddresses,omitempty"`
	}{rendered, rctx.MinReplicas, rctx.MaxReplicas, serverAddresses}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode rendered autoscaler for digest: %w", err)
	}
	return resolvedDigestPrefix + shortHash(encoded), nil
}

func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:digestHexLength]
}
