package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/ome/pkg/alfred/config"
	alfredstatus "sigs.k8s.io/ome/pkg/alfred/irstatus"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/policy/defrag"
	"sigs.k8s.io/ome/pkg/alfred/scheduling"
	"sigs.k8s.io/ome/pkg/alfred/scheduling/input"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	v1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

type dispatchEvidence struct {
	candidate     policy.Candidate
	owner         *v1beta1.InferenceService
	ir            *v1beta1.InferenceReplica
	fingerprint   string
	targets       []string
	historyWindow time.Duration
}

func (d *Dispatcher) freshObservation(ctx context.Context, cfg *config.Config) (*snapshot.ClusterSnapshot, error) {
	return d.observationFrom(ctx, d.Reader, cfg)
}

// sliceObservation is the observation for replaying a move onto a new TPU
// slice. The slice scheduler places the replacement, so its free-partition
// check is advisory; the source identity is fenced by CaptureSource's lossless
// reads instead.
func (d *Dispatcher) sliceObservation(ctx context.Context, cfg *config.Config) (*snapshot.ClusterSnapshot, error) {
	if d.CacheReader == nil {
		return d.freshObservation(ctx, cfg)
	}
	return d.observationFrom(ctx, d.CacheReader, cfg)
}

// capturedOwners returns the candidate's InferenceService and InferenceReplica
// from a lossless capture, or nils. predictionOwnersMatch has already tied
// them to the observation by UID, generation and status.
func capturedOwners(s *input.Snapshot, c policy.Candidate) (*v1beta1.InferenceService, *v1beta1.InferenceReplica) {
	var owner *v1beta1.InferenceService
	for i := range s.InferenceServices {
		if x := &s.InferenceServices[i]; x.Namespace == c.Workload.Namespace && x.Name == c.Workload.Name {
			owner = x
		}
	}
	if owner == nil {
		return nil, nil
	}
	for i := range s.InferenceReplicas {
		x := &s.InferenceReplicas[i]
		if x.Namespace == owner.Namespace && x.Spec.ParentRef != nil && x.Spec.ParentRef.Name == owner.Name && x.Spec.Component == c.Component {
			return owner, x
		}
	}
	return owner, nil
}

// captureFor returns the lossless capture a move needs: only the source for a
// move onto a new TPU slice, the whole scheduling input otherwise.
func (d *Dispatcher) captureFor(ctx context.Context, sliceMove bool, source input.Source) (*input.Snapshot, error) {
	if sliceMove {
		return input.CaptureSource(ctx, d.Reader, source, d.now)
	}
	return input.Capture(ctx, d.Reader, d.now)
}

func (d *Dispatcher) observationFrom(ctx context.Context, reader client.Reader, cfg *config.Config) (*snapshot.ClusterSnapshot, error) {
	return snapshot.Build(ctx, reader, snapshot.Options{Now: d.now, DefaultMovable: cfg.DefaultMovable,
		TriggerConditions: cfg.Policies.NodeHealth.TriggerConditions, PreemptibleLabels: cfg.SpotPolicy.PreemptibleLabels,
		NodeSuspicionWindow: cfg.NodeSuspicionWindow(), MaintenanceTriggers: cfg.Policies.NodeHealth.Maintenance.Triggers,
		OMENativeExecutor: snapshot.OMENativeExecutorState{Available: true, WireVersion: "v1", Reason: "OperatorConfigured"}})
}

// Fresh policy replay is an additional execution authorization. Merely having
// a feasible scheduling diagnostic never changes an advisory into an action.
func (d *Dispatcher) freshCandidate(s *snapshot.ClusterSnapshot, c policy.Candidate, cfg *config.Config) (policy.Candidate, bool) {
	for _, p := range d.Policies {
		for _, fresh := range p.Evaluate(s, cfg) {
			if fresh.Executable && fresh.Mode == constants.OMENative && sameDispatchSource(fresh, c) && sameDispatchCause(fresh, c) {
				return fresh, true
			}
		}
	}
	return policy.Candidate{}, false
}

// preflight re-checks a candidate against fresh API reads before any write:
// policy replay, arbitration, budgets, source identity and, unless the move
// targets a new TPU slice, a feasible scheduler simulation. It returns the
// evidence for the request, or the reason it is withheld.
func (d *Dispatcher) preflight(ctx context.Context, observed *snapshot.ClusterSnapshot, c policy.Candidate, cfg *config.Config, arbiter *Arbiter, j *dispatchJournal, existing *dispatchEntry) (dispatchEvidence, string) {
	var empty dispatchEvidence
	// A move onto a new OME-provisioned TPU slice is placed by the slice
	// scheduler, not by a scheduler profile Alfred can simulate. It keeps every
	// other check below; the policy replay re-checks capacity. It also skips
	// the cluster-wide API reads, which do not fit a large cluster. A journaled
	// retry carries no plan, but only a slice move records no targets.
	sliceMove := c.TPUSlice != nil || (existing != nil && len(existing.Targets) == 0)
	observe := d.freshObservation
	if sliceMove {
		observe = d.sliceObservation
	}
	fresh, err := observe(ctx, cfg)
	if err != nil {
		return empty, "ObservationUnavailable"
	}
	current, ok := d.freshCandidate(fresh, c, cfg)
	if !ok || (current.TPUSlice != nil) != sliceMove {
		return empty, "PolicyNoLongerEligible"
	}
	if !sliceMove && d.Simulator == nil {
		return empty, "SimulationUnavailable"
	}
	decisions := arbiter.Admit(fresh, []policy.Candidate{current}, cfg, d.now())
	if len(decisions) != 1 || !decisions[0].Admitted {
		if len(decisions) == 1 {
			return empty, decisions[0].Reason
		}
		return empty, "NotAdmitted"
	}
	source := input.Source{Namespace: current.Workload.Namespace, InferenceService: current.Workload.Name, Component: current.Component, Instance: current.Instance, FromNode: current.FromNode}
	captured, err := d.captureFor(ctx, sliceMove, source)
	if err != nil {
		return empty, "SnapshotUnavailable"
	}
	if reason := dispatchBudget(captured, j, current, cfg, d.now(), existing); reason != "" {
		return empty, reason
	}
	requestID := "preflight"
	if existing != nil {
		requestID = existing.UUID
	}
	hints := current.HintTargetNodes
	if existing != nil {
		req, err := requestForEntry(*existing)
		if err != nil {
			return empty, "JournalPayloadInvalid"
		}
		hints = req.HintTargetNodes
	}
	var request scheduling.Request
	var sourcePods []corev1.Pod
	if sliceMove {
		if len(hints) != 0 {
			return empty, "SourceUnsupported"
		}
		sourcePods, err = input.ExecutionSourcePods(captured, source, d.now(), predictionMaxAge)
	} else {
		request, err = input.BuildExecutionRequest(captured, source, cfg.Scheduling, requestID, hints, d.now(), predictionMaxAge)
		sourcePods = request.SourcePods
	}
	if err != nil {
		return empty, "SourceUnsupported"
	}
	if !predictionOwnersMatch(fresh, captured, current) || !predictionMembersMatch(fresh, current, sourcePods) {
		return empty, "SourceChanged"
	}
	if observed != nil && (!predictionOwnersMatch(observed, captured, current) || !predictionMembersMatch(observed, current, sourcePods)) {
		return empty, "SourceChanged"
	}
	owner := fresh.Workloads[current.Workload].ISVC
	ir := fresh.Workloads[current.Workload].Components[current.Component].IR
	if sliceMove {
		// The slice observation may lag; the submit-time fence compares the
		// owner's ResourceVersion with a direct read, so use the lossless copies.
		if owner, ir = capturedOwners(captured, current); owner == nil || ir == nil {
			return empty, "SourceChanged"
		}
	}
	// Select once so the pre/post-simulation comparison and any durable retry
	// use the same algorithm. Never upgrade an existing intent's source fence.
	fingerprintVersion := dispatchFingerprintSemantic
	if existing != nil && !strings.HasPrefix(existing.SourceFingerprint, semanticFingerprintPrefix) {
		legacy, err := hex.DecodeString(existing.SourceFingerprint)
		if err != nil || len(legacy) != sha256.Size {
			return empty, "SourceChanged"
		}
		fingerprintVersion = dispatchFingerprintLegacy
	}
	fingerprint, err := dispatchSourceFingerprint(owner, ir, sourcePods, current.Instance, fingerprintVersion)
	if err != nil {
		return empty, "SourceChanged"
	}
	if existing != nil && (fingerprint != existing.SourceFingerprint || owner.UID != existing.WorkloadUID || ir.UID != existing.IRUID || ir.Name != existing.IRName) {
		return empty, "SourceChanged"
	}
	var result scheduling.Result
	if !sliceMove {
		result, err = d.Simulator.Evaluate(ctx, request)
		if err != nil || scheduling.ValidateResult(request, result) != nil || result.Decision != scheduling.DecisionFeasible {
			return empty, "SimulationNotFeasible"
		}
	}
	if !observationFresh(fresh, d.now()) || captured.Validate(d.now(), predictionMaxAge) != nil {
		return empty, "SimulationStale"
	}
	// Re-read after simulation: source identity and scheduling objects must still
	// agree. Lists are not a transaction; the final owner CAS fences only the
	// submission, not future consumer acceptance (v1 has no incarnation fence).
	final, err := observe(ctx, cfg)
	if err != nil {
		return empty, "ObservationUnavailable"
	}
	finalCandidate, ok := d.freshCandidate(final, current, cfg)
	if !ok || (finalCandidate.TPUSlice != nil) != sliceMove {
		return empty, "PolicyNoLongerEligible"
	}
	finalCapture, err := d.captureFor(ctx, sliceMove, source)
	if err != nil {
		return empty, "SnapshotUnavailable"
	}
	var finalSourcePods []corev1.Pod
	if sliceMove {
		finalSourcePods, err = input.ExecutionSourcePods(finalCapture, source, d.now(), predictionMaxAge)
	} else {
		var finalRequest scheduling.Request
		finalRequest, err = input.BuildExecutionRequest(finalCapture, source, cfg.Scheduling, requestID, hints, d.now(), predictionMaxAge)
		finalSourcePods = finalRequest.SourcePods
	}
	if err != nil {
		return empty, "SourceChanged"
	}
	if !predictionOwnersMatch(final, finalCapture, current) || !predictionMembersMatch(final, current, finalSourcePods) {
		return empty, "SourceChanged"
	}
	finalOwner := final.Workloads[current.Workload].ISVC
	finalIR := final.Workloads[current.Workload].Components[current.Component].IR
	if sliceMove {
		if finalOwner, finalIR = capturedOwners(finalCapture, current); finalOwner == nil || finalIR == nil {
			return empty, "SourceChanged"
		}
	}
	// A durable retry may cross a fully observed pause/release, but a single
	// simulation attempt still requires stable owner/replica generations.
	if finalOwner.Generation != owner.Generation || finalIR.Generation != ir.Generation {
		return empty, "SourceChanged"
	}
	finalFingerprint, err := dispatchSourceFingerprint(finalOwner, finalIR, finalSourcePods, current.Instance, fingerprintVersion)
	if err != nil || finalFingerprint != fingerprint {
		return empty, "SourceChanged"
	}
	if !input.SameSchedulingState(captured, finalCapture) {
		return empty, "SchedulingStateChanged"
	}
	if finalCandidate.Policy == defrag.PolicyName {
		// Build both sides of the capacity calculation from the accepted
		// capture, never the independently listed policy observation. The
		// result still refers to the original request's synthetic identities.
		scoringSnapshot, err := d.observationFrom(ctx, &dispatchCaptureReader{capture: finalCapture, metadata: d.Reader}, cfg)
		if err != nil {
			return empty, "ObservationUnavailable"
		}
		placements, err := input.SourcePlacementTargets(request, result)
		if err != nil {
			return empty, "SourceChanged"
		}
		finalCandidate, ok = defrag.RevalidatePlacement(scoringSnapshot, cfg, finalCandidate, placements)
		if !ok {
			return empty, "PolicyNoLongerEligible"
		}
	}
	if reason := dispatchBudget(finalCapture, j, finalCandidate, cfg, d.now(), existing); reason != "" {
		return empty, reason
	}
	decisions = arbiter.Admit(final, []policy.Candidate{finalCandidate}, cfg, d.now())
	if len(decisions) != 1 || !decisions[0].Admitted {
		return empty, "SafetyStateChanged"
	}
	allowed := map[string]bool{}
	for _, node := range finalCandidate.PlacementTargetNodes {
		allowed[node] = true
	}
	if len(allowed) == 0 {
		for _, node := range finalCandidate.HintTargetNodes {
			allowed[node] = true
		}
	}
	targets := map[string]bool{}
	for _, placement := range result.Placements {
		node := final.Nodes[placement.NodeName]
		if !allowed[placement.NodeName] || node.UnavailableAsTarget() || !capturedNodeReady(finalCapture, placement.NodeName) {
			return empty, "PredictedTargetUnsafe"
		}
		if journalNodeCooling(j, placement.NodeName, cfg.PerNodeCooldown(), d.now(), existing) {
			return empty, "TargetCooldown"
		}
		targets[placement.NodeName] = true
	}
	if err := d.executionEnabled(ctx, cfg); err != nil {
		return empty, err.Error()
	}
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	historyWindow, valid := dispatchHistoryWindow(finalCapture, cfg)
	if !valid {
		return empty, "InvalidCooldown"
	}
	finalCandidate.HintTargetNodes = append([]string(nil), hints...)
	return dispatchEvidence{candidate: finalCandidate, owner: finalOwner, ir: finalIR, fingerprint: fingerprint, targets: names,
		historyWindow: historyWindow}, ""
}

// Normalize independent logical source copies at the existing audited boundary.
// Both versions exclude the same non-persisted Pod observations. The legacy
// branch retains its exact byte-level format for pre-upgrade prepared intents.
func dispatchSourceFingerprint(owner *v1beta1.InferenceService, ir *v1beta1.InferenceReplica, pods []corev1.Pod, instance int32, version dispatchFingerprintVersion) (string, error) {
	if owner == nil || ir == nil {
		return "", fmt.Errorf("source owner or replica is missing")
	}
	normalized := make([]corev1.Pod, len(pods))
	for i := range pods {
		normalized[i] = *pods[i].DeepCopy()
		normalized[i].ResourceVersion = ""
		normalized[i].ManagedFields = nil
		normalized[i].Status = corev1.PodStatus{}
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Name < normalized[j].Name })
	rows, err := alfredstatus.Rows(&ir.Status)
	if err != nil {
		return "", fmt.Errorf("source status: %w", err)
	}
	var row *v1beta1.OMENativeInstanceStatus
	for i := range rows {
		if rows[i].Index == instance {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		return "", fmt.Errorf("source instance %d has no status row", instance)
	}
	// These Pod-derived observations are not part of either stored encoding.
	row.ReadyPodCount = 0
	row.ScheduledPodCount = 0
	row.NodesOccupied = nil
	if version == dispatchFingerprintSemantic {
		return dispatchSemanticSourceFingerprint(owner, ir, row, normalized)
	}
	if version != dispatchFingerprintLegacy {
		return "", fmt.Errorf("unsupported source fingerprint version")
	}
	raw, err := json.Marshal(struct {
		OwnerUID        string
		OwnerGeneration int64
		IRUID           string
		IRGeneration    int64
		Revision        string
		Row             *v1beta1.OMENativeInstanceStatus
		Pods            []corev1.Pod
	}{string(owner.UID), owner.Generation, string(ir.UID), ir.Generation, ir.Status.CurrentRevision, row, normalized})
	if err != nil {
		return "", fmt.Errorf("source fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func capturedNodeReady(s *input.Snapshot, name string) bool {
	for _, object := range s.Objects {
		var node corev1.Node
		if json.Unmarshal(object.Raw, &node) != nil || node.Kind != "Node" || node.APIVersion != "v1" || node.Name != name {
			continue
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				return condition.Status == corev1.ConditionTrue
			}
		}
	}
	return false
}
