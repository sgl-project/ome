package runtimeprojection

import (
	"sort"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/effective"
	reportv1alpha1 "sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
)

func projectEffectiveDiagnostics(
	isvc *v1beta1.InferenceService,
	state *effective.RuntimeState,
	live *effective.LiveConfiguration,
	inheritance reportv1alpha1.RuntimeInheritance,
	active reportv1alpha1.RuntimeConfiguration,
) ([]reportv1alpha1.RuntimeIssue, []reportv1alpha1.RuntimeSourceReference, []reportv1alpha1.RuntimeWarning, bool) {
	issues := []reportv1alpha1.RuntimeIssue{}
	warnings := []reportv1alpha1.RuntimeWarning{}
	sources := projectBaseSources(isvc, live)

	if inheritance.State == reportv1alpha1.InheritanceStateUnavailable {
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueInheritanceUnavailable})
	}
	switch state.StatusFreshness {
	case effective.StatusFreshnessCurrent:
	case effective.StatusFreshnessUnknown:
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueStatusUnobserved})
	case effective.StatusFreshnessStale:
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueStatusStale})
		warnings = append(warnings, reportv1alpha1.RuntimeWarning{Code: reportv1alpha1.WarningStaleEvidence})
	case effective.StatusFreshnessInconsistent:
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueStatusInvalid})
		warnings = append(warnings, reportv1alpha1.RuntimeWarning{Code: reportv1alpha1.WarningStaleEvidence})
	default:
		return nil, nil, nil, false
	}
	if state.DriftState == effective.RuntimeDriftStateMalformed {
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueReportedDriftConflict})
	}

	if live != nil {
		for _, advisory := range live.Advisories {
			switch advisory.Code {
			case effective.RuntimeAdvisoryDeclaredCompatibilityMismatch:
				issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueDeclaredCompatibilityMismatch})
			case effective.RuntimeAdvisoryInvalidDeclaredKind:
				issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueInvalidDeclaredKind})
			default:
				return nil, nil, nil, false
			}
		}
	}

	if state.PinState == effective.RuntimePinStateAwaitingPin {
		issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueActiveRevisionUnreported})
	}
	if active.State == reportv1alpha1.ConfigurationStateUnavailable {
		switch state.PinMode {
		case effective.RuntimePinModeManagedPin, effective.RuntimePinModeExplicitPin, effective.RuntimePinModeInvalidPin:
			issues = append(issues, reportv1alpha1.RuntimeIssue{Code: reportv1alpha1.RuntimeIssueActiveRevisionUnavailable})
		}
	}

	observations := state.RevisionObservations()
	for _, observation := range observations {
		if !isEffectiveObservation(observation) {
			continue
		}
		revisionName := observation.ExpectedName()
		if revisionName == "" {
			revisionName = observation.ReturnedName()
		}
		for _, code := range observation.ConsistencyCodes() {
			if historyOnlyConsistencyCode(code) {
				continue
			}
			issueCode, ok := mapConsistencyIssue(code)
			if !ok {
				return nil, nil, nil, false
			}
			issues = append(issues, reportv1alpha1.RuntimeIssue{Code: issueCode, Revision: revisionName})
		}
		if observation.Disabled {
			issues = append(issues, reportv1alpha1.RuntimeIssue{
				Code: reportv1alpha1.RuntimeIssueRevisionDisabled, Revision: revisionName,
			})
		}
		if observation.ObjectReturned() {
			sources = append(sources, reportv1alpha1.RuntimeSourceReference{
				Kind: "ControllerRevision", Namespace: observation.ReturnedNamespace(), Name: observation.ReturnedName(),
				UID: observation.UID, Evidence: reportv1alpha1.EvidenceObserved,
			})
			continue
		}
		if reason, found := failedRevisionReason(state.SourceIssues(), observation.ExpectedName()); found {
			sources = append(sources, reportv1alpha1.RuntimeSourceReference{
				Kind: "ControllerRevision", Namespace: observation.ExpectedNamespace(), Name: observation.ExpectedName(),
				Evidence: reportv1alpha1.EvidenceUnavailable, UnavailableReason: reason,
			})
		}
	}

	for _, sourceIssue := range state.SourceIssues() {
		if sourceIssue.Code == effective.RuntimeSourceIssueRevisionListFailed {
			continue
		}
		issue, _, ok := projectSourceIssue(sourceIssue)
		if !ok {
			return nil, nil, nil, false
		}
		issues = append(issues, issue)
		warnings = append(warnings,
			reportv1alpha1.RuntimeWarning{Code: reportv1alpha1.WarningPartialData},
			reportv1alpha1.RuntimeWarning{Code: reportv1alpha1.WarningSourceUnavailable},
		)
	}

	if live == nil && state.RuntimeName != "" {
		kind := state.DeclaredSourceKind
		namespace := state.DeclaredSourceNamespace
		if kind == "" {
			kind = state.RuntimeKind
			namespace = state.RuntimeNamespace
		}
		if !safeRuntimeSourceKind(kind) {
			return nil, nil, nil, false
		}
		sources = append(sources, reportv1alpha1.RuntimeSourceReference{
			Kind: kind, Namespace: namespace, Name: state.RuntimeName,
			Evidence: reportv1alpha1.EvidenceUnavailable, UnavailableReason: liveUnavailableReason(state),
		})
	}

	issues = deduplicateIssues(issues)
	warnings = deduplicateWarnings(warnings)
	sources = deduplicateSources(sources)
	return issues, sources, warnings, true
}

func isEffectiveObservation(observation effective.RuntimeRevisionObservation) bool {
	roles := observation.Roles()
	return hasRevisionRole(roles, effective.RuntimeRevisionRoleActive) ||
		hasRevisionRole(roles, effective.RuntimeRevisionRoleRequested) ||
		hasRevisionRole(roles, effective.RuntimeRevisionRoleReported)
}

func historyOnlyConsistencyCode(code effective.RevisionConsistencyCode) bool {
	switch code {
	case effective.RevisionConsistencyDuplicateIdentity,
		effective.RevisionConsistencyConflictingIdentity,
		effective.RevisionConsistencyDuplicateContentHash,
		effective.RevisionConsistencyShortHashCollision:
		return true
	default:
		return false
	}
}

func safeRuntimeSourceKind(kind string) bool {
	_, ok := mapRuntimeKind(kind)
	return ok && kind != ""
}

func sortRuntimeIssues(values []reportv1alpha1.RuntimeIssue) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Code != values[j].Code {
			return values[i].Code < values[j].Code
		}
		return values[i].Revision < values[j].Revision
	})
}
