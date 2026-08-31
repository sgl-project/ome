package v1alpha1

import (
	"sort"
	"strings"

	"sigs.k8s.io/ome/pkg/cli/report"
)

const RuntimeEffectiveReportKind = "RuntimeEffectiveReport"

// RuntimeSelection reports how and, when observed, which runtime was selected.
type RuntimeSelection struct {
	Source  RuntimeSelectionSource  `json:"source"`
	Runtime *RuntimeObjectReference `json:"runtime,omitempty"`
}

// RuntimeInheritance reports the root-first runtime inheritance chain.
type RuntimeInheritance struct {
	State             InheritanceState         `json:"state"`
	Sources           []RuntimeObjectReference `json:"sources"`
	UnavailableReason UnavailableReason        `json:"unavailableReason,omitempty"`
}

// RuntimeStatusObservation reports bounded generation freshness evidence.
type RuntimeStatusObservation struct {
	Generation         int64           `json:"generation"`
	ObservedGeneration int64           `json:"observedGeneration"`
	Freshness          StatusFreshness `json:"freshness"`
}

// RuntimeDriftObservation reports the bounded controller drift condition.
type RuntimeDriftObservation struct {
	State DriftConditionState `json:"state"`
	Cause RuntimeDriftCause   `json:"cause,omitempty"`
}

// RuntimePin reports pin intent and bounded status relationships.
type RuntimePin struct {
	Mode              RuntimePinMode           `json:"mode"`
	State             RuntimePinState          `json:"state"`
	RequestedRevision string                   `json:"requestedRevision,omitempty"`
	ReportedRevision  string                   `json:"reportedRevision,omitempty"`
	Status            RuntimeStatusObservation `json:"status"`
	ReportedDrift     RuntimeDriftObservation  `json:"reportedDrift"`
	SyncState         RuntimeSyncState         `json:"syncState"`
}

// RuntimeConfiguration is an allowlisted runtime configuration summary.
type RuntimeConfiguration struct {
	State             ConfigurationState        `json:"state"`
	Origin            ConfigurationOrigin       `json:"origin,omitempty"`
	Source            *RuntimeObjectReference   `json:"source,omitempty"`
	Revision          *RuntimeRevisionReference `json:"revision,omitempty"`
	Hash              string                    `json:"hash,omitempty"`
	Components        []RuntimeComponent        `json:"components"`
	UnavailableReason UnavailableReason         `json:"unavailableReason,omitempty"`
}

// RuntimeEffectiveContent compares live and active runtime configuration.
type RuntimeEffectiveContent struct {
	Selection    RuntimeSelection     `json:"selection"`
	Inheritance  RuntimeInheritance   `json:"inheritance"`
	Pin          RuntimePin           `json:"pin"`
	Live         RuntimeConfiguration `json:"live"`
	Active       RuntimeConfiguration `json:"active"`
	LiveToActive RuntimeHashRelation  `json:"liveToActive"`
	Issues       []RuntimeIssue       `json:"issues"`
}

// NewRuntimeEffectiveReport creates a canonical effective report.
func NewRuntimeEffectiveReport(metadata Metadata, content RuntimeEffectiveContent, clock Clock) RuntimeEnvelope[RuntimeEffectiveContent] {
	return newRuntimeEnvelope(metadata, content, clock)
}

func (RuntimeEffectiveContent) runtimeReportKind() string {
	return RuntimeEffectiveReportKind
}

// Canonical returns a deeply copied deterministic effective report content.
func (c RuntimeEffectiveContent) Canonical() RuntimeEffectiveContent {
	result := c
	result.Selection.Runtime = copyRuntimeObjectReference(c.Selection.Runtime)
	result.Inheritance.Sources = append([]RuntimeObjectReference{}, c.Inheritance.Sources...)
	result.Live = c.Live.canonical()
	result.Active = c.Active.canonical()
	result.Issues = append([]RuntimeIssue{}, c.Issues...)
	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Revision < result.Issues[j].Revision
	})
	return result
}

// Table returns the deterministic human-readable effective configuration view.
func (c RuntimeEffectiveContent) Table() report.Table {
	canonical := c.Canonical()
	table := report.Table{
		Headers: []string{
			"VIEW", "STATE", "REASON", "RUNTIME", "REVISION", "HASH",
			"COMPONENT", "MODE", "MODE-SOURCE", "PIN", "PIN-STATE", "SYNC",
			"STATUS", "DRIFT", "LIVE-RELATION", "ISSUES",
		},
		Rows: [][]string{},
	}
	table.Rows = appendConfigurationRows(
		table.Rows, "Live", canonical.Live, canonical.Pin, canonical.LiveToActive, canonical.Issues,
	)
	table.Rows = appendConfigurationRows(
		table.Rows, "Active", canonical.Active, canonical.Pin, canonical.LiveToActive, canonical.Issues,
	)
	return table
}

func (c RuntimeConfiguration) canonical() RuntimeConfiguration {
	result := c
	result.Source = copyRuntimeObjectReference(c.Source)
	if c.Revision != nil {
		revision := *c.Revision
		revision.CreatedAt = revision.CreatedAt.UTC()
		result.Revision = &revision
	}
	result.Components = append([]RuntimeComponent{}, c.Components...)
	sort.Slice(result.Components, func(i, j int) bool {
		a, b := result.Components[i], result.Components[j]
		if componentOrder(a.Type) != componentOrder(b.Type) {
			return componentOrder(a.Type) < componentOrder(b.Type)
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.DeploymentMode != b.DeploymentMode {
			return a.DeploymentMode < b.DeploymentMode
		}
		return a.DeploymentModeSource < b.DeploymentModeSource
	})
	return result
}

func copyRuntimeObjectReference(source *RuntimeObjectReference) *RuntimeObjectReference {
	if source == nil {
		return nil
	}
	result := *source
	return &result
}

func componentOrder(component RuntimeComponentType) int {
	switch component {
	case RuntimeComponentEngine:
		return 0
	case RuntimeComponentDecoder:
		return 1
	case RuntimeComponentRouter:
		return 2
	default:
		return 3
	}
}

func appendConfigurationRows(
	rows [][]string,
	view string,
	configuration RuntimeConfiguration,
	pin RuntimePin,
	relation RuntimeHashRelation,
	issues []RuntimeIssue,
) [][]string {
	components := configuration.Components
	if len(components) == 0 {
		components = []RuntimeComponent{{}}
	}
	for _, component := range components {
		rows = append(rows, []string{
			view,
			orDash(string(configuration.State)),
			orDash(string(configuration.UnavailableReason)),
			runtimeReferenceDisplay(configuration.Source),
			revisionReferenceDisplay(configuration.Revision),
			orDash(configuration.Hash),
			orDash(string(component.Type)),
			orDash(string(component.DeploymentMode)),
			orDash(string(component.DeploymentModeSource)),
			orDash(string(pin.Mode)),
			orDash(string(pin.State)),
			orDash(string(pin.SyncState)),
			orDash(string(pin.Status.Freshness)),
			runtimeDriftDisplay(pin.ReportedDrift),
			orDash(string(relation)),
			joinRuntimeIssues(issues),
		})
	}
	return rows
}

func runtimeDriftDisplay(drift RuntimeDriftObservation) string {
	if drift.State == "" {
		return "-"
	}
	if drift.Cause == "" {
		return string(drift.State)
	}
	return string(drift.State) + "/" + string(drift.Cause)
}

func joinRuntimeIssues(issues []RuntimeIssue) string {
	values := make([]string, len(issues))
	for i, issue := range issues {
		values[i] = string(issue.Code)
		if issue.Revision != "" {
			values[i] += "(" + issue.Revision + ")"
		}
	}
	return orDash(strings.Join(values, ","))
}

func runtimeReferenceDisplay(reference *RuntimeObjectReference) string {
	if reference == nil {
		return "-"
	}
	parts := []string{string(reference.Kind)}
	if reference.Namespace != "" {
		parts = append(parts, reference.Namespace)
	}
	parts = append(parts, reference.Name)
	return strings.Join(parts, "/")
}

func revisionReferenceDisplay(reference *RuntimeRevisionReference) string {
	if reference == nil {
		return "-"
	}
	return orDash(reference.Name)
}
