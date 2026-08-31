// Package v1alpha1 defines the alpha diagnostic output contract owned by the
// kubectl-ome CLI. These objects are reports, not Kubernetes resources for
// apply, and intentionally have no spec or arbitrary extension fields.
package v1alpha1

import (
	"sort"
	"time"

	"sigs.k8s.io/ome/pkg/cli/report"
)

const (
	// APIVersion is the versioned schema identifier for CLI-owned reports.
	APIVersion = "cli.ome.io/v1alpha1"
)

// Clock makes report collection timestamps deterministic in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function into a Clock.
type ClockFunc func() time.Time

// Now implements Clock.
func (f ClockFunc) Now() time.Time {
	return f()
}

// SystemClock reads the process wall clock.
type SystemClock struct{}

// Now implements Clock.
func (SystemClock) Now() time.Time {
	return time.Now()
}

// Metadata identifies the primary object described by a report.
type Metadata struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// EvidenceLevel identifies how the CLI obtained a reported value.
type EvidenceLevel string

const (
	EvidenceDeclared    EvidenceLevel = "Declared"
	EvidenceReported    EvidenceLevel = "Reported"
	EvidenceObserved    EvidenceLevel = "Observed"
	EvidenceComputed    EvidenceLevel = "Computed"
	EvidenceUnavailable EvidenceLevel = "Unavailable"
)

// UnavailableReason is a bounded reason that source evidence is absent.
type UnavailableReason string

const (
	UnavailableNotFound         UnavailableReason = "NotFound"
	UnavailableForbidden        UnavailableReason = "Forbidden"
	UnavailableUnsupportedAPI   UnavailableReason = "UnsupportedAPI"
	UnavailableStaleGeneration  UnavailableReason = "StaleGeneration"
	UnavailableMalformedPayload UnavailableReason = "MalformedPayload"
	UnavailableNotConfigured    UnavailableReason = "NotConfigured"
	UnavailableUnreadable       UnavailableReason = "Unreadable"
	UnavailableCycle            UnavailableReason = "Cycle"
	UnavailableMaxDepthExceeded UnavailableReason = "MaxDepthExceeded"
	UnavailableDisabled         UnavailableReason = "Disabled"
)

// SourceReference identifies one typed source used to build a report.
type SourceReference struct {
	Kind              string            `json:"kind"`
	Namespace         string            `json:"namespace,omitempty"`
	Name              string            `json:"name"`
	UID               string            `json:"uid,omitempty"`
	Generation        int64             `json:"generation,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	Evidence          EvidenceLevel     `json:"evidence"`
	CollectedAt       time.Time         `json:"collectedAt"`
	UnavailableReason UnavailableReason `json:"unavailableReason,omitempty"`
}

// WarningCode gives consumers a stable category without exposing arbitrary
// structured data.
type WarningCode string

const (
	WarningPartialData       WarningCode = "PartialData"
	WarningSourceUnavailable WarningCode = "SourceUnavailable"
	WarningStaleEvidence     WarningCode = "StaleEvidence"
	WarningTruncated         WarningCode = "Truncated"
)

// Warning is a typed, ordered diagnostic warning.
type Warning struct {
	Code    WarningCode `json:"code"`
	Message string      `json:"message,omitempty"`
}

// Content is implemented by the typed content owned by each report kind.
// Canonical must return a sorted copy with every slice normalized to a
// non-nil value. Table derives the human view from that same typed content.
type Content[T any] interface {
	Canonical() T
	Table() report.Table
}

// Envelope carries the fields shared by every v1alpha1 diagnostic report.
// T remains a concrete compile-time type; arbitrary maps and raw extensions
// are intentionally not part of this contract.
type Envelope[T Content[T]] struct {
	APIVersion  string            `json:"apiVersion"`
	Kind        string            `json:"kind"`
	Metadata    Metadata          `json:"metadata"`
	CollectedAt time.Time         `json:"collectedAt"`
	Sources     []SourceReference `json:"sources"`
	Content     T                 `json:"content"`
	Warnings    []Warning         `json:"warnings"`
}

// NewEnvelope creates a report using an injectable collection clock.
func NewEnvelope[T Content[T]](kind string, metadata Metadata, content T, clock Clock) Envelope[T] {
	if clock == nil {
		clock = SystemClock{}
	}
	return (Envelope[T]{
		APIVersion:  APIVersion,
		Kind:        kind,
		Metadata:    metadata,
		CollectedAt: clock.Now().UTC(),
		Sources:     []SourceReference{},
		Content:     content,
		Warnings:    []Warning{},
	}).Canonical()
}

// Canonical returns a deterministic copy suitable for table or machine
// rendering. It never reorders caller-owned slices.
func (e Envelope[T]) Canonical() Envelope[T] {
	result := e
	result.APIVersion = APIVersion
	result.CollectedAt = e.CollectedAt.UTC()
	result.Sources = append([]SourceReference{}, e.Sources...)
	for i := range result.Sources {
		if result.Sources[i].CollectedAt.IsZero() {
			result.Sources[i].CollectedAt = result.CollectedAt
		} else {
			result.Sources[i].CollectedAt = result.Sources[i].CollectedAt.UTC()
		}
	}
	sort.SliceStable(result.Sources, func(i, j int) bool {
		return sourceLess(result.Sources[i], result.Sources[j])
	})
	result.Warnings = append([]Warning{}, e.Warnings...)
	sort.SliceStable(result.Warnings, func(i, j int) bool {
		if result.Warnings[i].Code != result.Warnings[j].Code {
			return result.Warnings[i].Code < result.Warnings[j].Code
		}
		return result.Warnings[i].Message < result.Warnings[j].Message
	})
	result.Content = e.Content.Canonical()
	return result
}

// Table returns the human-readable view of the canonical typed content.
func (e Envelope[T]) Table() report.Table {
	return e.Content.Table()
}

func sourceLess(a, b SourceReference) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.UID != b.UID {
		return a.UID < b.UID
	}
	if a.Generation != b.Generation {
		return a.Generation < b.Generation
	}
	if a.ResourceVersion != b.ResourceVersion {
		return a.ResourceVersion < b.ResourceVersion
	}
	if a.Evidence != b.Evidence {
		return a.Evidence < b.Evidence
	}
	if !a.CollectedAt.Equal(b.CollectedAt) {
		return a.CollectedAt.Before(b.CollectedAt)
	}
	return a.UnavailableReason < b.UnavailableReason
}
