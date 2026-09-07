package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/metrics"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Candidate outcomes as recorded in the alfred-recommendations ConfigMap.
const (
	OutcomeAdvisory = "advisory"
	// OutcomeAdmitted is reserved for a future Dispatcher-confirmed
	// submission. Arbiter admission alone is reported as withheld.
	OutcomeAdmitted = "admitted"
	// OutcomeWithheld is an Arbiter-admitted candidate that has not been
	// submitted by a Dispatcher.
	OutcomeWithheld = "withheld"
	OutcomeRejected = "rejected"
)

// recommendationsKey is the ConfigMap key holding the latest cycle record;
// node remediation records (Policy #2) live beside it under derived per-node keys.
const recommendationsKey = "last-cycle.json"

const (
	nodeRecordPrefix          = "node."
	hashedNodeRecordPrefix    = "node-hash."
	maxConfigMapDataKeyLength = 253
	eventNodeRepairNeeded     = "NodeRepairNeeded"
	eventNodeDrainedForRepair = "NodeDrainedForRepair"
)

// Reporter centralizes candidate, decision, and remediation-lifecycle output:
// their Events, metrics, and alfred-recommendations ConfigMap entries come
// from here, so emission is tested and deduplicated once, not per policy.
// Loop health, snapshot gauges, leader status, and config-reload output remain
// with the components that own those operational signals.
type Reporter struct {
	// Client updates the recommendations ConfigMap. Update-only: the chart
	// pre-creates it, and a missing ConfigMap disables the record with a
	// log line rather than a create (the write contract is narrow).
	Client client.Client
	// DirectReader reads the ConfigMap uncached for the read-modify-write
	// cycle (the manager's API reader): retrying a conflict against the
	// informer cache would just re-read the same stale object. Nil falls
	// back to Client.
	DirectReader client.Reader
	Recorder     record.EventRecorder
	Metrics      *metrics.Metrics
	Log          logr.Logger
	// Namespace is Alfred's own namespace — the recommendations ConfigMap
	// and the engine-level transition events live there.
	Namespace string
	// ConfigMapName is the alfred-config ConfigMap's configured name (the
	// --config-name flag): the anchor object for engine-level events.
	ConfigMapName string

	// Transition dedup: the OMENativeUnavailable event fires on the way
	// into degraded mode, never per pass.
	omenativeSeeded   bool
	omenativeDegraded bool

	// cmMissingLogged dedups the missing-ConfigMap log.
	cmMissingLogged bool

	// nodeEpisodes carries lifecycle-event phase across decision loops. The
	// durable per-node records seed it after leader failover; persistence
	// retries never roll it back, because doing so would replay Events and
	// counters on every failed ConfigMap write.
	nodeEpisodes map[string]nodeEpisode
	// initializedNodeEpisodes records every node whose phase was initialized
	// in this Reporter lifetime, from durable state when enabled or otherwise
	// from local observation. Entries outlive active episodes so a node that
	// leaves and re-enters the desired set cannot revive a stale record.
	initializedNodeEpisodes map[string]struct{}
}

// cycleRecord is the JSON document written to the recommendations ConfigMap.
type cycleRecord struct {
	Timestamp       time.Time            `json:"timestamp"`
	Mode            string               `json:"mode"`
	Recommendations []recommendationView `json:"recommendations"`
}

type recommendationView struct {
	Workload       string   `json:"workload"`
	Component      string   `json:"component"`
	Instance       int32    `json:"instance"`
	Policy         string   `json:"policy"`
	Reason         string   `json:"reason"`
	Outcome        string   `json:"outcome"`
	AdvisoryReason string   `json:"advisoryReason,omitempty"`
	RejectReason   string   `json:"rejectReason,omitempty"`
	WithholdReason string   `json:"withholdReason,omitempty"`
	FromNode       string   `json:"fromNode,omitempty"`
	Target         string   `json:"target,omitempty"`
	HintTargets    []string `json:"hintTargets,omitempty"`
	Score          float64  `json:"score"`
	Emergency      bool     `json:"emergency,omitempty"`
	CooldownOver   bool     `json:"cooldownOverridden,omitempty"`
}

type nodeEpisode struct {
	signaledAt *time.Time
	drainedAt  *time.Time
}

type nodeRemediationRecord struct {
	State                  snapshot.NodeHealthState `json:"state"`
	Conditions             []nodeConditionRecord    `json:"conditions"`
	SuspectUntil           *time.Time               `json:"suspectUntil,omitempty"`
	Workloads              []string                 `json:"workloads"`
	OMEGPUOccupantsPresent bool                     `json:"omeGpuOccupantsPresent"`
	ObservedAt             time.Time                `json:"observedAt"`
	SignaledAt             *time.Time               `json:"signaledAt,omitempty"`
	DrainedAt              *time.Time               `json:"drainedAt,omitempty"`
}

type nodeConditionRecord struct {
	Type               corev1.NodeConditionType `json:"type"`
	Status             corev1.ConditionStatus   `json:"status"`
	LastTransitionTime time.Time                `json:"lastTransitionTime"`
}

// ReportCycle publishes one decision pass: produced/accepted/rejected
// counters, Events on the target InferenceServices, and (when enabled) the
// recommendations ConfigMap record. decisions covers the executable
// candidates; advisories appear only in candidates.
func (r *Reporter) ReportCycle(ctx context.Context, candidates []policy.Candidate, decisions []Decision, cfg *config.Config, now time.Time) {
	record := cycleRecord{Timestamp: now, Mode: cfg.Mode}
	workloadCandidates := make([]policy.Candidate, 0, len(candidates))
	remediations := make([]*policy.NodeRemediation, 0)
	for i := range candidates {
		if candidates[i].Remediation != nil {
			remediations = append(remediations, candidates[i].Remediation)
			continue
		}
		workloadCandidates = append(workloadCandidates, candidates[i])
	}
	nodeRecords, reconcileNodeRecords := r.reconcileNodeRemediations(ctx, remediations, cfg, now)

	decided := map[string]Decision{}
	for _, d := range decisions {
		decided[candidateKey(d.Candidate)] = d
	}

	for _, c := range workloadCandidates {
		executable := "false"
		if c.Executable {
			executable = "true"
		}
		r.Metrics.RecommendationsProduced.WithLabelValues(
			c.Policy, c.Workload.String(), string(c.Component), c.Reason, executable).Inc()

		view := recommendationView{
			Workload:       c.Workload.String(),
			Component:      string(c.Component),
			Instance:       c.Instance,
			Policy:         c.Policy,
			Reason:         c.Reason,
			AdvisoryReason: c.AdvisoryReason,
			FromNode:       c.FromNode,
			HintTargets:    c.HintTargetNodes,
			Score:          c.Score,
			Emergency:      c.Emergency,
		}

		if !c.Executable {
			view.Outcome = OutcomeAdvisory
			r.reportAdvisory(c)
			record.Recommendations = append(record.Recommendations, view)
			continue
		}

		d, ok := decided[candidateKey(c)]
		if !ok {
			// An executable candidate without a decision (the arbiter
			// never saw it) is a wiring bug worth surfacing loudly.
			r.Log.Error(nil, "executable candidate has no arbitration decision",
				"workload", c.Workload.String(), "component", c.Component)
			continue
		}
		if d.Admitted {
			view.Outcome = OutcomeWithheld
			view.WithholdReason, _ = withholdDetails(cfg.Mode)
			view.Target = d.Target
			view.CooldownOver = d.CooldownOverridden
			r.reportAdmitted(c, d, cfg)
		} else {
			view.Outcome = OutcomeRejected
			view.RejectReason = d.Reason
			r.Metrics.RecommendationsRejected.WithLabelValues(
				c.Policy, c.Workload.String(), string(c.Component), d.Reason).Inc()
			r.event(c, corev1.EventTypeNormal, "RecommendationRejected",
				"%s recommendation for %s/%s rejected: %s",
				c.Policy, c.Workload.String(), c.Component, d.Reason)
		}
		record.Recommendations = append(record.Recommendations, view)
	}

	if *cfg.RecommendationsConfigMapEnabled {
		r.writeRecord(ctx, cfg.RecommendationsConfigMapName, record, nodeRecords, reconcileNodeRecords)
	}
}

func (r *Reporter) reportAdvisory(c policy.Candidate) {
	r.event(c, corev1.EventTypeNormal, producedEventReason(c),
		"%s advisory for %s/%s: %s (from %s, footprint %d GPUs)",
		c.Policy, c.Workload.String(), c.Component, c.AdvisoryReason, c.FromNode, c.FootprintGPUs)
	if c.AdvisoryReason == policy.AdvisoryLWSMigrationUnsupported {
		r.Metrics.LWSRecommendations.WithLabelValues(c.Workload.String(), "MigrateToOMENative").Inc()
	}
}

func (r *Reporter) reportAdmitted(c policy.Candidate, d Decision, cfg *config.Config) {
	r.Metrics.RecommendationsAccepted.WithLabelValues(
		c.Policy, c.Workload.String(), string(c.Component)).Inc()

	_, note := withholdDetails(cfg.Mode)
	target := d.Target
	if target == "" {
		target = "scheduler's choice"
	}
	r.event(c, corev1.EventTypeNormal, "RecommendationWithheld",
		"%s recommends migrating %s/%s instance %d off %s (target %s, score %.3f) — %s",
		c.Policy, c.Workload.String(), c.Component, c.Instance, c.FromNode, target, c.Score, note)

	if d.CooldownOverridden {
		r.Metrics.CooldownOverrides.WithLabelValues(c.Policy).Inc()
		r.event(c, corev1.EventTypeNormal, "CooldownOverriddenForEvacuation",
			"health evacuation of %s/%s admitted under the cooldown floor while the standard window was still running",
			c.Workload.String(), c.Component)
	}
}

func withholdDetails(mode string) (reason, note string) {
	if mode == config.ModeRecommendOnly {
		return "RecommendOnly", "recommend-only: not dispatched"
	}
	return "DispatcherUnavailable", "dispatcher unavailable: not dispatched"
}

// ReportOMENativeState emits the degraded-mode transition event: once on the
// way in, never per pass (the alfred_omenative_unavailable gauge — published
// by the observation loop every pass — is the per-pass signal; the event
// stream must not become noise).
func (r *Reporter) ReportOMENativeState(available bool) {
	degraded := !available
	enteringDegraded := degraded && (!r.omenativeSeeded || !r.omenativeDegraded)
	if enteringDegraded {
		r.Recorder.Eventf(r.namespaceRef(), corev1.EventTypeWarning, "OMENativeUnavailable",
			"no OMENative executor is available; OMENative candidates degrade to advisory")
	}
	if r.omenativeSeeded && r.omenativeDegraded && available {
		r.Log.Info("OMENative executor available again; degraded mode cleared")
	}
	r.omenativeSeeded, r.omenativeDegraded = true, degraded
}

// reconcileNodeRemediations treats the current policy markers as the complete
// desired node-record set. It advances lifecycle Events independently of the
// ConfigMap write: a failed write is retried next cycle without replaying an
// Event or incrementing its counter again.
func (r *Reporter) reconcileNodeRemediations(
	ctx context.Context,
	markers []*policy.NodeRemediation,
	cfg *config.Config,
	now time.Time,
) (map[string]nodeRemediationRecord, bool) {
	if r.nodeEpisodes == nil {
		r.nodeEpisodes = map[string]nodeEpisode{}
	}
	desired := make(map[string]*policy.NodeRemediation, len(markers))
	for _, marker := range markers {
		if marker == nil || marker.Node == "" || marker.Health.State == "" ||
			marker.Health.State == snapshot.NodeHealthClear {
			continue
		}
		desired[marker.Node] = marker
	}
	if *cfg.RecommendationsConfigMapEnabled &&
		!r.seedNodeEpisodes(ctx, cfg.RecommendationsConfigMapName, desired) {
		// Until durable event phase is known, fail closed: neither emit a
		// lifecycle signal nor replace node records. The normal cycle record
		// may still be written, and seeding is retried on the next pass.
		return nil, false
	}

	for node := range r.nodeEpisodes {
		if _, ok := desired[node]; !ok {
			delete(r.nodeEpisodes, node)
		}
	}

	nodes := make([]string, 0, len(desired))
	for node := range desired {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	records := make(map[string]nodeRemediationRecord, len(nodes))
	for _, node := range nodes {
		marker := desired[node]
		episode := r.nodeEpisodes[node]
		workloads := sortedUniqueStrings(marker.Workloads)
		occupantsPresent := marker.OMEGPUOccupantsPresent || len(workloads) > 0
		wasSignaled := episode.signaledAt != nil
		// Alfred does not cordon Nodes. A workload can land after a drained
		// transition, so persisted repair readiness describes only the current
		// continuous empty interval. Withdraw it on reoccupation; if the node
		// empties again, the later transition is reported again.
		if occupantsPresent && episode.drainedAt != nil {
			episode.drainedAt = nil
		}

		if !wasSignaled &&
			(marker.Health.State == snapshot.NodeHealthUnhealthy || marker.Health.State == snapshot.NodeHealthUnknown) {
			signaledAt := now
			episode.signaledAt = &signaledAt
			r.Metrics.NodeHealthSignals.WithLabelValues(node, eventNodeRepairNeeded).Inc()
			r.Recorder.Eventf(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: node}},
				corev1.EventTypeWarning, eventNodeRepairNeeded,
				"node %s requires repair; Alfred observed %s health with %d identified OME workload(s)",
				node, marker.Health.State, len(workloads))
		}
		if wasSignaled && episode.drainedAt == nil && !occupantsPresent {
			drainedAt := now
			episode.drainedAt = &drainedAt
			r.Metrics.NodeHealthSignals.WithLabelValues(node, eventNodeDrainedForRepair).Inc()
			r.Recorder.Eventf(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: node}},
				corev1.EventTypeNormal, eventNodeDrainedForRepair,
				"node %s has no remaining OME GPU workloads and is ready for repair", node)
		}
		r.nodeEpisodes[node] = episode
		r.markNodeEpisodeInitialized(node)
		records[node] = remediationRecord(marker, workloads, occupantsPresent, episode, now)
	}
	return records, true
}

// seedNodeEpisodes restores event phase from Alfred's durable node records so
// a newly elected Reporter does not replay repair/drained signals. A failed
// each newly encountered node is compared once, including nodes that first
// appear after the initial pass. A failed read is retried on the next cycle;
// locally observed phases always win. It reports whether durable phase is
// known and lifecycle reconciliation is safe.
func (r *Reporter) seedNodeEpisodes(
	ctx context.Context,
	name string,
	desired map[string]*policy.NodeRemediation,
) bool {
	pending := make([]string, 0, len(desired))
	for node := range desired {
		if _, initialized := r.initializedNodeEpisodes[node]; !initialized {
			pending = append(pending, node)
		}
	}
	if len(pending) == 0 {
		return true
	}
	sort.Strings(pending)
	var cm corev1.ConfigMap
	err := r.reader().Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: name}, &cm)
	if apierrors.IsNotFound(err) {
		for _, node := range pending {
			r.markNodeEpisodeInitialized(node)
		}
		return true
	}
	if err != nil {
		r.Log.Error(err, "seed node remediation episodes")
		return false
	}
	for _, node := range pending {
		marker := desired[node]
		raw, ok := cm.Data[nodeRecordKey(node)]
		if ok {
			var record nodeRemediationRecord
			if json.Unmarshal([]byte(raw), &record) == nil {
				if _, local := r.nodeEpisodes[node]; !local &&
					record.SignaledAt != nil && recordCoversHealth(record, marker.Health) {
					r.nodeEpisodes[node] = nodeEpisode{
						signaledAt: cloneTime(record.SignaledAt),
						drainedAt:  cloneTime(record.DrainedAt),
					}
				}
			}
		}
		r.markNodeEpisodeInitialized(node)
	}
	return true
}

func (r *Reporter) markNodeEpisodeInitialized(node string) {
	if r.initializedNodeEpisodes == nil {
		r.initializedNodeEpisodes = map[string]struct{}{}
	}
	r.initializedNodeEpisodes[node] = struct{}{}
}

// recordCoversHealth is deliberately conservative about failover dedup. A
// failed desired-set deletion can leave an old node record behind, so node
// name alone never proves episode continuity. Durable phase is reused only
// when its observation covered the exact current condition evidence. Newer or
// identity-free evidence drops durable phase: a bad state signals at least
// once, while Suspect remains phase-free instead of inheriting stale drained
// state.
func recordCoversHealth(record nodeRemediationRecord, current snapshot.NodeHealthObservation) bool {
	if record.ObservedAt.IsZero() || record.State != current.State || len(record.Conditions) == 0 ||
		len(record.Conditions) != len(current.Conditions) || record.SignaledAt == nil ||
		record.SignaledAt.After(record.ObservedAt) {
		return false
	}
	if record.DrainedAt != nil &&
		(record.DrainedAt.Before(*record.SignaledAt) || record.DrainedAt.After(record.ObservedAt)) {
		return false
	}

	recorded := make(map[corev1.NodeConditionType]nodeConditionRecord, len(record.Conditions))
	for _, condition := range record.Conditions {
		if !usableConditionEvidence(condition.Type, condition.Status, condition.LastTransitionTime) ||
			condition.LastTransitionTime.After(record.ObservedAt) {
			return false
		}
		if _, duplicate := recorded[condition.Type]; duplicate {
			return false
		}
		recorded[condition.Type] = condition
	}
	for _, condition := range current.Conditions {
		if !usableConditionEvidence(condition.Type, condition.Status, condition.LastTransitionTime) ||
			condition.LastTransitionTime.After(record.ObservedAt) {
			return false
		}
		previous, ok := recorded[condition.Type]
		if !ok || previous.Status != condition.Status ||
			!previous.LastTransitionTime.Equal(condition.LastTransitionTime) {
			return false
		}
		delete(recorded, condition.Type)
	}
	return len(recorded) == 0
}

func usableConditionEvidence(
	conditionType corev1.NodeConditionType,
	status corev1.ConditionStatus,
	transitioned time.Time,
) bool {
	if conditionType == "" || transitioned.IsZero() {
		return false
	}
	switch status {
	case corev1.ConditionTrue, corev1.ConditionFalse, corev1.ConditionUnknown:
		return true
	default:
		return false
	}
}

func remediationRecord(
	marker *policy.NodeRemediation,
	workloads []string,
	occupantsPresent bool,
	episode nodeEpisode,
	now time.Time,
) nodeRemediationRecord {
	conditions := make([]nodeConditionRecord, len(marker.Health.Conditions))
	for i, condition := range marker.Health.Conditions {
		conditions[i] = nodeConditionRecord{
			Type:               condition.Type,
			Status:             condition.Status,
			LastTransitionTime: condition.LastTransitionTime,
		}
	}
	return nodeRemediationRecord{
		State:                  marker.Health.State,
		Conditions:             conditions,
		SuspectUntil:           cloneTime(marker.Health.SuspectUntil),
		Workloads:              workloads,
		OMEGPUOccupantsPresent: occupantsPresent,
		ObservedAt:             now,
		SignaledAt:             cloneTime(episode.signaledAt),
		DrainedAt:              cloneTime(episode.drainedAt),
	}
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// nodeRecordKey keeps the readable node.<name> form whenever it fits a
// ConfigMap data key. Kubernetes node names may themselves use all 253 bytes,
// so longer derived keys use a disjoint prefix, a readable name prefix, and a
// collision-resistant digest. The disjoint prefix prevents a hashed long name
// from colliding with an otherwise valid short name. Seeding computes the key
// from the desired Node rather than trying to recover a name from the truncated
// form.
func nodeRecordKey(node string) string {
	key := nodeRecordPrefix + node
	if len(key) <= maxConfigMapDataKeyLength {
		return key
	}
	digest := sha256.Sum256([]byte(node))
	suffix := fmt.Sprintf(".%x", digest[:])
	prefixLength := maxConfigMapDataKeyLength - len(hashedNodeRecordPrefix) - len(suffix)
	return hashedNodeRecordPrefix + node[:prefixLength] + suffix
}

func isNodeRecordKey(key string) bool {
	return strings.HasPrefix(key, nodeRecordPrefix) || strings.HasPrefix(key, hashedNodeRecordPrefix)
}

func (r *Reporter) writeRecord(
	ctx context.Context,
	name string,
	rec cycleRecord,
	nodeRecords map[string]nodeRemediationRecord,
	reconcileNodeRecords bool,
) {
	raw, err := json.Marshal(rec)
	if err != nil {
		r.Log.Error(err, "marshal recommendations record")
		return
	}
	nodeRaw := map[string]string(nil)
	if reconcileNodeRecords {
		nodeRaw = make(map[string]string, len(nodeRecords))
		for node, record := range nodeRecords {
			entry, err := json.Marshal(record)
			if err != nil {
				r.Log.Error(err, "marshal node remediation record", "node", node)
				return
			}
			nodeRaw[node] = string(entry)
		}
	}
	// A failed cycle record needs no unwinding: the next pass rewrites it.
	_ = r.updateConfigMap(ctx, name, func(cm *corev1.ConfigMap) {
		cm.Data[recommendationsKey] = string(raw)
		if !reconcileNodeRecords {
			return
		}
		for key := range cm.Data {
			if isNodeRecordKey(key) {
				delete(cm.Data, key)
			}
		}
		for node, entry := range nodeRaw {
			cm.Data[nodeRecordKey(node)] = entry
		}
	})
}

// updateConfigMap applies a mutation to the recommendations ConfigMap,
// update-only: other keys (node records) are preserved, and a missing
// ConfigMap logs once instead of being created. The read-modify-write cycle
// retries on conflict with an uncached read so the complete desired record is
// not lost to a stale update. A later reporting pass retries other failures
// while retaining lifecycle phase, so Events and counters are not replayed.
func (r *Reporter) updateConfigMap(ctx context.Context, name string, mutate func(*corev1.ConfigMap)) error {
	key := types.NamespacedName{Namespace: r.Namespace, Name: name}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var cm corev1.ConfigMap
		if err := r.reader().Get(ctx, key, &cm); err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		mutate(&cm)
		return r.Client.Update(ctx, &cm)
	})
	switch {
	case err == nil:
		r.cmMissingLogged = false
	case apierrors.IsNotFound(err):
		if !r.cmMissingLogged {
			r.cmMissingLogged = true
			r.Log.Info("recommendations ConfigMap missing; record disabled (the Kustomize bundle pre-creates it)",
				"configmap", key.String())
		}
	default:
		r.Log.Error(err, "update recommendations ConfigMap", "configmap", key.String())
	}
	return err
}

func (r *Reporter) reader() client.Reader {
	if r.DirectReader != nil {
		return r.DirectReader
	}
	return r.Client
}

// event emits a candidate-scoped Event on its InferenceService. A minimal
// object reference is enough for the recorder, and candidates without a
// workload (node-scoped signals) skip the ISVC event.
func (r *Reporter) event(c policy.Candidate, eventType, reason, format string, args ...interface{}) {
	if c.Workload.Name == "" {
		return
	}
	ref := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{
		Namespace: c.Workload.Namespace,
		Name:      c.Workload.Name,
	}}
	r.Recorder.Eventf(ref, eventType, reason, format, args...)
}

func (r *Reporter) namespaceRef() *corev1.ConfigMap {
	name := r.ConfigMapName
	if name == "" {
		name = "alfred-config"
	}
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: r.Namespace,
		Name:      name,
	}}
}

func producedEventReason(c policy.Candidate) string {
	if c.AdvisoryReason == policy.AdvisoryRawDeploymentMigrationUnsupported {
		return policy.AdvisoryRawDeploymentMigrationUnsupported
	}
	switch c.Reason {
	case policy.ReasonNodeUnhealthy:
		return "EvacuationRecommendationProduced"
	case policy.ReasonRemediationSignal:
		return "RemediationSignalProduced"
	default:
		return "FragmentationRecommendationProduced"
	}
}

// candidateKey identifies one candidate within a cycle. The policy is part
// of the key: node-health evacuation and defragmentation can both target the
// same instance in one pass, and their decisions must not overwrite each
// other in the report.
func candidateKey(c policy.Candidate) string {
	return fmt.Sprintf("%s|%s|%s|%d", c.Policy, c.Workload.String(), c.Component, c.Instance)
}
