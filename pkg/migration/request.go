// Package migration defines the OEP-0008 migration-request wire contract:
// the UUID-keyed `ome.io/migration-request-v1-<uuid>` InferenceService
// annotation and its JSON payload.
//
// The contract is shared by three parties:
//   - Alfred (the caretaker): its dispatcher writes requests, and its
//     snapshot builder parses in-flight ones.
//   - The InferenceService controller: executes requests addressed to
//     RawDeployment components and acks by clearing the annotation and
//     appending a Status.MigrationHistory entry.
//   - A future OMENative controller: executes requests addressed to
//     OMENative Instances through the same annotation.
//
// Requests are idempotent by UUID: re-writing the same key with the same
// payload is a no-op for a consumer already processing it, and a consumer
// finding the UUID terminal in MigrationHistory clears the key silently.
package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

// SchemaVersionV1 is the only request schema version defined today.
// Consumers must reject unknown versions explicitly (additive unknown fields
// within a version are ignored; behavior-changing fields require a new
// version).
const SchemaVersionV1 = "v1"

// ErrUnsupportedSchemaVersion is wrapped by Validate for requests carrying a
// schema version the consumer does not implement, so executors can branch on
// it (errors.Is) and surface `UnsupportedSchemaVersion` outcomes.
var ErrUnsupportedSchemaVersion = errors.New("unsupported migration-request schema version")

// componentValues mirrors the v1beta1 ComponentType enum. Kept as strings so
// the wire package stays free of API-type dependencies.
var componentValues = map[string]struct{}{
	"engine":  {},
	"decoder": {},
	"router":  {},
}

// Request is the JSON payload stored as the value of a
// `ome.io/migration-request-v1-<uuid>` annotation.
type Request struct {
	// SchemaVersion is the wire-contract version; always "v1" today.
	SchemaVersion string `json:"schemaVersion"`
	// Component addresses one component of the InferenceService
	// (engine, decoder, or router).
	Component string `json:"component"`
	// Instance is the Instance index the request addresses. For a
	// RawDeployment component the executor treats it as advisory (the
	// pod is located by FromNode, not by index).
	Instance int32 `json:"instance"`
	// Reason records why the requester wants the move
	// (e.g. "fragmentation", "nodeUnhealthy").
	Reason string `json:"reason,omitempty"`
	// FromNode is the node the addressed Instance should leave.
	FromNode string `json:"fromNode"`
	// HintTargetNodes is a ranked, advisory list of preferred
	// destinations; the scheduler makes the final placement.
	HintTargetNodes []string `json:"hintTargetNodes,omitempty"`
	// RequestedAt is when the requester created this request; consumers
	// auto-clear requests that stay unacknowledged past the stale window.
	RequestedAt metav1.Time `json:"requestedAt"`
	// RequestedBy identifies the writer (e.g. "alfred-controller").
	RequestedBy string `json:"requestedBy,omitempty"`
}

// Parsed is one well-formed request extracted from an annotation map.
type Parsed struct {
	// UUID is the request identity, taken from the annotation-key suffix.
	UUID    string
	Request Request
}

// Malformed is one request annotation that could not be parsed or failed
// validation. Consumers ack-reject these (clear the key, record the error)
// rather than ignoring them, so a bad write cannot linger forever.
type Malformed struct {
	Key  string
	UUID string
	Err  error
}

// AnnotationKey returns the annotation key for a request UUID.
func AnnotationKey(uuid string) string {
	return constants.MigrationRequestAnnotationPrefix + uuid
}

// IsAnnotationKey reports whether key is a migration-request annotation.
//
// The suffix is deliberately an opaque request identity, not a
// format-validated UUID. Consumers must see every prefixed key so their
// parse/GC step can ack-reject garbage by clearing it — a key-level format
// filter would make a bad write invisible and leave it lingering on the
// InferenceService forever. Correctness is gated by payload validation
// (Validate), and idempotency only needs suffix equality, not RFC-4122 form.
func IsAnnotationKey(key string) bool {
	return strings.HasPrefix(key, constants.MigrationRequestAnnotationPrefix) &&
		len(key) > len(constants.MigrationRequestAnnotationPrefix)
}

// UUIDFromAnnotationKey extracts the request UUID from an annotation key.
func UUIDFromAnnotationKey(key string) (string, bool) {
	if !IsAnnotationKey(key) {
		return "", false
	}
	return key[len(constants.MigrationRequestAnnotationPrefix):], true
}

// Marshal renders the request as its annotation value.
func (r *Request) Marshal() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Validate checks the request against the v1 contract.
func (r *Request) Validate() error {
	if r.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: %q", ErrUnsupportedSchemaVersion, r.SchemaVersion)
	}
	if _, ok := componentValues[r.Component]; !ok {
		return fmt.Errorf("unknown component %q (want engine, decoder, or router)", r.Component)
	}
	if r.FromNode == "" {
		return errors.New("fromNode must be set")
	}
	if r.Instance < 0 {
		return fmt.Errorf("instance must be >= 0, got %d", r.Instance)
	}
	if r.RequestedAt.IsZero() {
		return errors.New("requestedAt must be set")
	}
	return nil
}

// ExtractRequests scans an annotation map for migration-request keys and
// splits them into well-formed and malformed requests. Well-formed requests
// are returned oldest-first (by RequestedAt, then UUID) so consumers process
// deterministically; malformed ones are returned sorted by key.
func ExtractRequests(annotations map[string]string) ([]Parsed, []Malformed) {
	var valid []Parsed
	var malformed []Malformed
	for key, value := range annotations {
		uuid, ok := UUIDFromAnnotationKey(key)
		if !ok {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(value), &req); err != nil {
			malformed = append(malformed, Malformed{Key: key, UUID: uuid, Err: fmt.Errorf("invalid JSON payload: %w", err)})
			continue
		}
		if err := req.Validate(); err != nil {
			malformed = append(malformed, Malformed{Key: key, UUID: uuid, Err: err})
			continue
		}
		valid = append(valid, Parsed{UUID: uuid, Request: req})
	}
	sort.Slice(valid, func(i, j int) bool {
		ti, tj := valid[i].Request.RequestedAt.Time, valid[j].Request.RequestedAt.Time
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return valid[i].UUID < valid[j].UUID
	})
	sort.Slice(malformed, func(i, j int) bool { return malformed[i].Key < malformed[j].Key })
	return valid, malformed
}
