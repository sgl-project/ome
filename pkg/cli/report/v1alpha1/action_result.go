package v1alpha1

import (
	"strings"
	"time"

	"sigs.k8s.io/ome/pkg/cli/report"
)

const ActionResultKind = "ActionResult"

// DryRunMode identifies whether an action was submitted to the API server.
type DryRunMode string

const (
	DryRunNone   DryRunMode = "none"
	DryRunClient DryRunMode = "client"
	DryRunServer DryRunMode = "server"
)

// ActionTarget is the exact Kubernetes object identity used by an action.
type ActionTarget struct {
	Kind            string `json:"kind"`
	Namespace       string `json:"namespace,omitempty"`
	Name            string `json:"name"`
	UID             string `json:"uid,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// ActionResult is the separate, versioned stdout contract shared by guarded
// mutating commands. Preview, confirmation, and warnings remain on stderr.
type ActionResult struct {
	APIVersion   string       `json:"apiVersion"`
	Kind         string       `json:"kind"`
	CollectedAt  time.Time    `json:"collectedAt"`
	Action       string       `json:"action"`
	Target       ActionTarget `json:"target"`
	DryRun       DryRunMode   `json:"dryRun"`
	RequestID    string       `json:"requestID,omitempty"`
	RevisionHash string       `json:"revisionHash,omitempty"`
	Accepted     bool         `json:"accepted"`
	Applied      bool         `json:"applied"`
	Message      string       `json:"message,omitempty"`
	FollowUp     string       `json:"followUp,omitempty"`
}

// NewActionResult creates an unapplied result using an injectable clock.
func NewActionResult(action string, target ActionTarget, dryRun DryRunMode, clock Clock) ActionResult {
	if clock == nil {
		clock = SystemClock{}
	}
	return (ActionResult{
		APIVersion:  APIVersion,
		Kind:        ActionResultKind,
		CollectedAt: clock.Now().UTC(),
		Action:      action,
		Target:      target,
		DryRun:      dryRun,
	}).Canonical()
}

// Canonical returns a deterministic copy and enforces dry-run invariants:
// neither dry-run mode may claim application, and client dry-run cannot claim
// API acceptance because it submits no request.
func (r ActionResult) Canonical() ActionResult {
	result := r
	result.APIVersion = APIVersion
	result.Kind = ActionResultKind
	result.CollectedAt = r.CollectedAt.UTC()
	if result.DryRun != DryRunNone {
		result.Applied = false
	}
	if result.DryRun == DryRunClient {
		result.Accepted = false
	}
	return result
}

// Table derives the concise human view from the typed action result.
func (r ActionResult) Table() report.Table {
	return report.Table{
		Headers: []string{
			"ACTION", "TARGET", "DRY-RUN", "ACCEPTED", "APPLIED",
			"REQUEST-ID", "REVISION-HASH", "MESSAGE", "FOLLOW-UP",
		},
		Rows: [][]string{{
			r.Action,
			r.Target.displayName(),
			string(r.DryRun),
			yesNo(r.Accepted),
			yesNo(r.Applied),
			orDash(r.RequestID),
			orDash(r.RevisionHash),
			orDash(r.Message),
			orDash(r.FollowUp),
		}},
	}
}

func (t ActionTarget) displayName() string {
	parts := []string{t.Kind}
	if t.Namespace != "" {
		parts = append(parts, t.Namespace)
	}
	parts = append(parts, t.Name)
	return strings.Join(parts, "/")
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
