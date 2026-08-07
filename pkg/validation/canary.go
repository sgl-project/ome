package validation

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// ReasonCanaryInvalid is the admission rejection reason for a malformed canary
// progression on a spec.rollout.groups[] entry. Operators and tests reference it.
const ReasonCanaryInvalid = "CanaryInvalid"

// ReasonCanaryRequiresOMENative is the rejection reason for a canary group whose
// Component is not OMENative. Canary relies on per-revision pod selection,
// partition staging, and the InferenceReplica object — all OMENative/IR-managed
// constructs — so other deployment modes would silently wedge at Pending.
const ReasonCanaryRequiresOMENative = "CanaryRequiresOMENative"

// ReasonMultipleCanaryGroups is the rejection reason for a spec.rollout that
// declares more than one canary group. The canary engine drives a single group
// (the dispatcher resolves it via GetCanaryGroup, which returns the first);
// additional canary groups are executed by neither engine and would roll
// ungated. Sequenced multi-group canary
// needs the cross-group sequencer that does not exist yet, so admission rejects
// the shape rather than silently mis-executing all but the first group.
const ReasonMultipleCanaryGroups = "MultipleCanaryGroups"

// ReasonAnalysisInvalid is the rejection reason for a malformed metric-gated step
// (a step's analysis). It guards the spec-internal rules a pure validator can
// check: every analysis step has a complete config — at least one metric, a
// numeric threshold, a valid operator, a positive interval, and a failure limit
// >= 1. The metrics source (GroupCanary.Prometheus) is environmental and is
// resolved controller-side, not checked here.
const ReasonAnalysisInvalid = "AnalysisInvalid"

// ValidateCanary checks the canary progression on every spec.rollout.groups[]
// entry that has one. Returns nil when no group canaries (nil-safe). The CRD CEL
// rule enforces the one-of progression (canary may span multiple Components,
// primary-driven); these are the value-level rules CEL can't express. Each
// guards a concrete
// failure:
//   - Steps non-empty — an empty plan has nothing to execute.
//   - Every Component in the group is OMENative — other modes wedge at Pending.
//   - Final step Traffic == 100 — otherwise the rollout never fully cuts over.
//   - Final step Capacity == 100% — otherwise it completes on a partial fleet.
//   - Traffic in [0,100], non-decreasing, and Traffic>0 requires Capacity>0.
//   - Each analysis step's config is complete and well-formed (delegated to
//     validateCanaryAnalysis).
func ValidateCanary(spec *v1beta1.InferenceServiceSpec) error {
	groups := spec.GetRolloutGroups()
	modeFor := omenativeDeploymentModeMap(spec)
	declared := declaredComponents(spec)
	canaryGroups := 0
	for gi := range groups {
		if groups[gi].Canary != nil {
			canaryGroups++
		}
	}
	if canaryGroups > 1 {
		return fmt.Errorf("spec.rollout.groups declares %d canary groups; only one is supported — the canary engine drives a single group and the others would roll ungated (%s)",
			canaryGroups, ReasonMultipleCanaryGroups)
	}
	for gi := range groups {
		g := &groups[gi]
		if g.Canary == nil {
			continue
		}
		c := g.Canary
		if len(c.Steps) == 0 {
			return fmt.Errorf("spec.rollout.groups[%d].canary.steps must not be empty (%s)", gi, ReasonCanaryInvalid)
		}
		if err := validateCanaryAnalysis(gi, c); err != nil {
			return err
		}
		for _, comp := range g.Components {
			// A canary group's Components must be valid and declared on the ISVC:
			// primaryComponent picks router>engine>decoder among them, so an invalid
			// or undeclared member becomes a phantom primary that wedges the roll.
			if _, ok := validComponents[comp]; !ok {
				return fmt.Errorf("spec.rollout.groups[%d]: canary component %q is not one of router/engine/decoder (%s)",
					gi, comp, ReasonCanaryInvalid)
			}
			if _, ok := declared[comp]; !ok {
				return fmt.Errorf("spec.rollout.groups[%d]: canary component %q is not declared on the InferenceService (%s)",
					gi, comp, ReasonCanaryInvalid)
			}
			if m, ok := modeFor[comp]; ok && m != string(constants.OMENative) {
				return fmt.Errorf("spec.rollout.groups[%d]: component %q has deploymentMode=%q; canary requires OMENative (%s)",
					gi, comp, m, ReasonCanaryRequiresOMENative)
			}
		}
		// A canary group must contain the ISVC's external entrypoint (router if
		// present, else engine). primaryComponent picks the entrypoint *within the
		// group* and the engine writes the stepped traffic onto its Service, but
		// external requests enter through the ISVC entrypoint. A canary group that
		// omits it shifts traffic on an internal Service only, so the operator's
		// steps never move real traffic.
		entry := canaryEntrypoint(spec)
		hasEntry := false
		for _, comp := range g.Components {
			if comp == entry {
				hasEntry = true
				break
			}
		}
		if !hasEntry {
			return fmt.Errorf("spec.rollout.groups[%d]: a canary group must include the entrypoint component %q — external traffic routes through it, so a group without it only shifts traffic on an internal Service (%s)",
				gi, entry, ReasonCanaryInvalid)
		}
		// MaintainRatio is silently ignored by the canary engine (it gates only
		// coordination-style pacing). Reject rather than no-op so an operator is not
		// misled into thinking the cross-Component ratio is guarded during the canary.
		if g.MaintainRatio != nil {
			return fmt.Errorf("spec.rollout.groups[%d]: maintainRatio is not honored on a canary group (the canary engine does not enforce it) — remove it, or use a blueGreen/rollingUpdate group for ratio-guarded rollout (%s)",
				gi, ReasonCanaryInvalid)
		}
		last := c.Steps[len(c.Steps)-1]
		if last.Traffic != 100 {
			return fmt.Errorf("spec.rollout.groups[%d].canary final step traffic must be 100, got %d (%s)", gi, last.Traffic, ReasonCanaryInvalid)
		}
		// Only the percentage form is checkable at admission (absolute counts need
		// the desired replica count); the done sentinel also forces partition 0.
		if pct, ok := percentValue(last.Capacity); ok && pct != 100 {
			return fmt.Errorf("spec.rollout.groups[%d].canary final step capacity must be 100%% (rollout completes at full capacity), got %q (%s)",
				gi, last.Capacity.String(), ReasonCanaryInvalid)
		}
		var prevWeight int32
		for i, s := range c.Steps {
			if s.Traffic < 0 || s.Traffic > 100 {
				return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d]: traffic %d must be in [0,100] (%s)", gi, i, s.Traffic, ReasonCanaryInvalid)
			}
			if s.Traffic < prevWeight {
				return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d]: traffic %d must be >= the previous step's %d (canary weights are non-decreasing) (%s)",
					gi, i, s.Traffic, prevWeight, ReasonCanaryInvalid)
			}
			prevWeight = s.Traffic
			if s.Traffic > 0 && newCapacityIsZero(s.Capacity) {
				return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d]: traffic %d>0 requires capacity>0 (%s)", gi, i, s.Traffic, ReasonCanaryInvalid)
			}
		}
	}
	return nil
}

// validateCanaryAnalysis enforces the spec-internal rules for each metric-gated
// step: a positive interval, failureLimit >= 1, and at least one
// well-formed metric. The metrics source (GroupCanary.Prometheus) is not checked
// here — an unreachable source is surfaced controller-side as an inconclusive
// sample, not an admission error.
func validateCanaryAnalysis(gi int, c *v1beta1.GroupCanary) error {
	for si := range c.Steps {
		a := c.Steps[si].Analysis
		if a == nil {
			continue
		}
		if a.Interval.Duration <= 0 {
			return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d].analysis: interval must be > 0 (%s)",
				gi, si, ReasonAnalysisInvalid)
		}
		if a.FailureLimit < 1 {
			return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d].analysis: failureLimit must be >= 1 (%s)",
				gi, si, ReasonAnalysisInvalid)
		}
		if len(a.Metrics) == 0 {
			return fmt.Errorf("spec.rollout.groups[%d].canary.steps[%d].analysis: at least one metric is required (%s)",
				gi, si, ReasonAnalysisInvalid)
		}
		if err := validateAnalysisMetrics(gi, si, a.Metrics); err != nil {
			return err
		}
	}
	return nil
}

// validateAnalysisMetrics checks each metric is well-formed: unique non-empty
// name, non-empty query, numeric threshold, valid operator. si < 0 addresses the
// canary-level defaults; si >= 0 a step's effective metrics.
func validateAnalysisMetrics(gi, si int, metrics []v1beta1.AnalysisMetric) error {
	where := fmt.Sprintf("spec.rollout.groups[%d].canary.steps[%d].analysis", gi, si)
	seen := make(map[string]struct{}, len(metrics))
	for mi := range metrics {
		m := &metrics[mi]
		if m.Name == "" {
			return fmt.Errorf("%s.metrics[%d].name must not be empty (%s)", where, mi, ReasonAnalysisInvalid)
		}
		if _, dup := seen[m.Name]; dup {
			return fmt.Errorf("%s.metrics[%d]: duplicate metric name %q (%s)", where, mi, m.Name, ReasonAnalysisInvalid)
		}
		seen[m.Name] = struct{}{}
		if strings.TrimSpace(m.Query) == "" {
			return fmt.Errorf("%s.metrics[%d] (%q): query must not be empty (%s)", where, mi, m.Name, ReasonAnalysisInvalid)
		}
		if _, err := strconv.ParseFloat(m.Threshold, 64); err != nil {
			return fmt.Errorf("%s.metrics[%d] (%q): threshold %q must be numeric (%s)", where, mi, m.Name, m.Threshold, ReasonAnalysisInvalid)
		}
		switch m.Operator {
		case v1beta1.ComparisonLT, v1beta1.ComparisonLTE, v1beta1.ComparisonGT, v1beta1.ComparisonGTE:
		default:
			return fmt.Errorf("%s.metrics[%d] (%q): operator %q must be one of LT/LTE/GT/GTE (%s)", where, mi, m.Name, m.Operator, ReasonAnalysisInvalid)
		}
	}
	return nil
}

// percentValue extracts the integer N from an IntOrString "<N>%" form.
func percentValue(v intstr.IntOrString) (int, bool) {
	if v.Type != intstr.String || !strings.HasSuffix(v.StrVal, "%") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v.StrVal, "%"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// newCapacityIsZero reports whether a step's Capacity resolves to zero pods
// regardless of how it is expressed (integer 0, "0", or "0%").
func newCapacityIsZero(c intstr.IntOrString) bool {
	if c.Type == intstr.Int {
		return c.IntValue() == 0
	}
	return c.StrVal == "0" || c.StrVal == "0%"
}

// canaryEntrypoint returns the ISVC's external entrypoint Component, mirroring
// DetermineEntrypointComponent: the router when declared, else the engine. The
// canary group must contain it (see ValidateCanary) so the stepped traffic lands
// on the Service that actually fronts external requests.
func canaryEntrypoint(spec *v1beta1.InferenceServiceSpec) v1beta1.ComponentType {
	if spec != nil && spec.Router != nil {
		return v1beta1.RouterComponent
	}
	return v1beta1.EngineComponent
}
