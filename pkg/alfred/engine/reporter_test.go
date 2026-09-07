package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/metrics"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
)

func newTestReporter(t *testing.T, objs ...client.Object) (*Reporter, *metrics.Metrics, *record.FakeRecorder, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	m := metrics.New(prometheus.NewRegistry())
	recorder := record.NewFakeRecorder(50)
	return &Reporter{
		Client:    fakeClient,
		Recorder:  recorder,
		Metrics:   m,
		Log:       logr.Discard(),
		Namespace: "ome",
	}, m, recorder, fakeClient
}

func recommendationsCM(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ome", Name: "alfred-recommendations"},
		Data:       data,
	}
}

func drainEvents(recorder *record.FakeRecorder) []string {
	var events []string
	for {
		select {
		case e := <-recorder.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}

func hasEvent(events []string, substr string) bool {
	for _, e := range events {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

type capturedEvent struct {
	object    runtime.Object
	eventType string
	reason    string
	message   string
}

type capturingRecorder struct {
	events []capturedEvent
}

func (r *capturingRecorder) Event(object runtime.Object, eventType, reason, message string) {
	r.events = append(r.events, capturedEvent{object: object, eventType: eventType, reason: reason, message: message})
}

func (r *capturingRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	r.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (r *capturingRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	r.Eventf(object, eventType, reason, messageFmt, args...)
}

func (r *capturingRecorder) count(reason string) int {
	count := 0
	for _, event := range r.events {
		if event.reason == reason {
			count++
		}
	}
	return count
}

func (r *capturingRecorder) event(reason string) (capturedEvent, bool) {
	for _, event := range r.events {
		if event.reason == reason {
			return event, true
		}
	}
	return capturedEvent{}, false
}

type nodeRecordView struct {
	State                  snapshot.NodeHealthState `json:"state"`
	Conditions             []nodeConditionView      `json:"conditions"`
	SuspectUntil           *time.Time               `json:"suspectUntil,omitempty"`
	Workloads              []string                 `json:"workloads"`
	OMEGPUOccupantsPresent bool                     `json:"omeGpuOccupantsPresent"`
	ObservedAt             time.Time                `json:"observedAt"`
	SignaledAt             *time.Time               `json:"signaledAt,omitempty"`
	DrainedAt              *time.Time               `json:"drainedAt,omitempty"`
}

type nodeConditionView struct {
	Type               corev1.NodeConditionType `json:"type"`
	Status             corev1.ConditionStatus   `json:"status"`
	LastTransitionTime time.Time                `json:"lastTransitionTime"`
}

func remediationCandidate(node string, health snapshot.NodeHealthObservation, workloads ...string) policy.Candidate {
	return policy.Candidate{
		Policy:     "nodehealth",
		Reason:     policy.ReasonRemediationSignal,
		Executable: false,
		FromNode:   node,
		Remediation: &policy.NodeRemediation{
			Node:                   node,
			Health:                 health,
			Workloads:              workloads,
			OMEGPUOccupantsPresent: len(workloads) > 0,
		},
	}
}

func nodeRecord(t *testing.T, cl client.Client, node string) (nodeRecordView, bool) {
	t.Helper()
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	raw, ok := cm.Data[nodeRecordKey(node)]
	if !ok {
		return nodeRecordView{}, false
	}
	var got nodeRecordView
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode node.%s: %v: %s", node, err, raw)
	}
	return got, true
}

func TestReportCycleOutcomes(t *testing.T) {
	r, m, recorder, cl := newTestReporter(t, recommendationsCM(map[string]string{"other.key": "kept"}))
	cfg := config.Default() // recommend-only

	admitted := cand("prod/a", "node1")
	rejected := cand("prod/b", "node3")
	lws := cand("prod/lws", "node1")
	lws.Executable = false
	lws.AdvisoryReason = policy.AdvisoryLWSMigrationUnsupported

	decisions := []Decision{
		{Candidate: admitted, Admitted: true, Target: "node2"},
		{Candidate: rejected, Reason: RejectCooldown},
	}
	r.ReportCycle(context.Background(), []policy.Candidate{admitted, rejected, lws}, decisions, cfg, testNow)

	get := func(vec *prometheus.CounterVec, labels ...string) float64 {
		return promtestutil.ToFloat64(vec.WithLabelValues(labels...))
	}
	if got := get(m.RecommendationsProduced, "defragmentation", "prod/a", "engine", policy.ReasonFragmentation, "true"); got != 1 {
		t.Fatalf("produced{prod/a} = %v", got)
	}
	if got := get(m.RecommendationsProduced, "defragmentation", "prod/lws", "engine", policy.ReasonFragmentation, "false"); got != 1 {
		t.Fatalf("produced{advisory} = %v", got)
	}
	if got := get(m.RecommendationsAccepted, "defragmentation", "prod/a", "engine"); got != 1 {
		t.Fatalf("accepted = %v", got)
	}
	if got := get(m.RecommendationsRejected, "defragmentation", "prod/b", "engine", RejectCooldown); got != 1 {
		t.Fatalf("rejected = %v", got)
	}
	if got := get(m.LWSRecommendations, "prod/lws", "MigrateToOMENative"); got != 1 {
		t.Fatalf("lws counter = %v", got)
	}

	events := drainEvents(recorder)
	for _, want := range []string{
		"RecommendationWithheld", "recommend-only: not dispatched",
		"RecommendationRejected", RejectCooldown,
		"FragmentationRecommendationProduced", policy.AdvisoryLWSMigrationUnsupported,
	} {
		if !hasEvent(events, want) {
			t.Fatalf("events missing %q: %v", want, events)
		}
	}

	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	doc := cm.Data[recommendationsKey]
	for _, want := range []string{
		`"outcome":"` + OutcomeWithheld + `"`,
		`"outcome":"` + OutcomeRejected + `"`,
		`"outcome":"` + OutcomeAdvisory + `"`,
		`"rejectReason":"` + RejectCooldown + `"`,
		`"mode":"recommend-only"`,
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("record missing %s: %s", want, doc)
		}
	}
	if cm.Data["other.key"] != "kept" {
		t.Fatal("sibling ConfigMap keys must be preserved")
	}
}

func TestRawAdvisoryUsesDedicatedEventReason(t *testing.T) {
	r, _, recorder, _ := newTestReporter(t)
	cfg := config.Default()
	*cfg.RecommendationsConfigMapEnabled = false

	raw := cand("prod/raw", "node1")
	raw.Executable = false
	raw.AdvisoryReason = "RawDeploymentMigrationUnsupported"
	raw.HintTargetNodes = nil
	raw.FootprintGPUs = 0
	r.ReportCycle(context.Background(), []policy.Candidate{raw}, nil, cfg, testNow)

	events := drainEvents(recorder)
	if !hasEvent(events, "RawDeploymentMigrationUnsupported") {
		t.Fatalf("Raw advisory must use its dedicated Event reason: %v", events)
	}
	if hasEvent(events, "FragmentationRecommendationProduced") {
		t.Fatalf("Raw advisory must not use the generic fragmentation Event reason: %v", events)
	}
}

// TestCrossPolicyDecisionsKeyedSeparately: node-health and defrag can both
// target the same instance in one pass; each candidate must be reported with
// its own decision, not the other policy's.
func TestCrossPolicyDecisionsKeyedSeparately(t *testing.T) {
	r, m, _, cl := newTestReporter(t, recommendationsCM(nil))
	cfg := config.Default()

	defragCand := cand("prod/a", "node1")
	healthCand := health("prod/a", "node1")
	decisions := []Decision{
		{Candidate: healthCand, Admitted: true, Target: "node2"},
		{Candidate: defragCand, Reason: RejectWorkloadBusy},
	}
	r.ReportCycle(context.Background(), []policy.Candidate{defragCand, healthCand}, decisions, cfg, testNow)

	if got := promtestutil.ToFloat64(m.RecommendationsAccepted.WithLabelValues("nodehealth", "prod/a", "engine")); got != 1 {
		t.Fatalf("health admission miscounted: %v", got)
	}
	if got := promtestutil.ToFloat64(m.RecommendationsRejected.WithLabelValues("defragmentation", "prod/a", "engine", RejectWorkloadBusy)); got != 1 {
		t.Fatalf("defrag rejection miscounted: %v", got)
	}
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	doc := cm.Data[recommendationsKey]
	for _, want := range []string{
		`"policy":"nodehealth","reason":"NodeUnhealthy","outcome":"withheld"`,
		`"policy":"defragmentation","reason":"Fragmentation","outcome":"rejected"`,
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("record missing %s: %s", want, doc)
		}
	}
}

func TestReportCycleWithholdsAdmissionUntilDispatcherExists(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantReason string
		wantNote   string
	}{
		{name: "recommend-only", mode: config.ModeRecommendOnly, wantReason: "RecommendOnly", wantNote: "recommend-only: not dispatched"},
		{name: "execute", mode: config.ModeExecute, wantReason: "DispatcherUnavailable", wantNote: "dispatcher unavailable: not dispatched"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, _, recorder, cl := newTestReporter(t, recommendationsCM(nil))
			cfg := config.Default()
			cfg.Mode = test.mode

			c := cand("prod/a", "node1")
			r.ReportCycle(context.Background(), []policy.Candidate{c},
				[]Decision{{Candidate: c, Admitted: true, Target: "node2"}}, cfg, testNow)

			events := drainEvents(recorder)
			for _, want := range []string{"RecommendationWithheld", test.wantNote} {
				if !hasEvent(events, want) {
					t.Errorf("events missing %q: %v", want, events)
				}
			}
			for _, forbidden := range []string{"RecommendationAdmitted", "will dispatch"} {
				if hasEvent(events, forbidden) {
					t.Errorf("events contain %q before Dispatcher exists: %v", forbidden, events)
				}
			}

			var cm corev1.ConfigMap
			if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
				t.Fatal(err)
			}
			var record struct {
				Recommendations []struct {
					Outcome        string `json:"outcome"`
					WithholdReason string `json:"withholdReason"`
				} `json:"recommendations"`
			}
			if err := json.Unmarshal([]byte(cm.Data[recommendationsKey]), &record); err != nil {
				t.Fatalf("decode %s: %v", recommendationsKey, err)
			}
			if len(record.Recommendations) != 1 {
				t.Fatalf("recommendations = %d, want 1", len(record.Recommendations))
			}
			got := record.Recommendations[0]
			if got.Outcome != OutcomeWithheld || got.WithholdReason != test.wantReason {
				t.Errorf("record outcome/reason = %q/%q, want %q/%q: %s",
					got.Outcome, got.WithholdReason, OutcomeWithheld, test.wantReason, cm.Data[recommendationsKey])
			}
		})
	}
}

func TestCooldownOverrideAudited(t *testing.T) {
	r, m, recorder, _ := newTestReporter(t, recommendationsCM(nil))
	c := health("prod/a", "node1")
	r.ReportCycle(context.Background(), []policy.Candidate{c},
		[]Decision{{Candidate: c, Admitted: true, Target: "node2", CooldownOverridden: true}},
		config.Default(), testNow)

	if got := promtestutil.ToFloat64(m.CooldownOverrides.WithLabelValues("nodehealth")); got != 1 {
		t.Fatalf("override counter = %v", got)
	}
	if events := drainEvents(recorder); !hasEvent(events, "CooldownOverriddenForEvacuation") {
		t.Fatalf("override event missing: %v", events)
	}
}

func TestConfigMapIsUpdateOnly(t *testing.T) {
	r, _, _, cl := newTestReporter(t) // no pre-created ConfigMap
	c := cand("prod/a", "node1")
	r.ReportCycle(context.Background(), []policy.Candidate{c},
		[]Decision{{Candidate: c, Admitted: true, Target: "node2"}}, config.Default(), testNow)

	var cm corev1.ConfigMap
	err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm)
	if err == nil {
		t.Fatal("the reporter must never create the ConfigMap (the Kustomize bundle pre-creates it)")
	}
}

// TestConfigMapConflictRetried: a stale read must not drop the complete
// desired reporting record.
func TestConfigMapConflictRetried(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	conflicts := 1
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(recommendationsCM(nil)).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if conflicts > 0 {
					conflicts--
					return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, obj.GetName(), errors.New("stale"))
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
	r := &Reporter{
		Client:    cl,
		Recorder:  record.NewFakeRecorder(10),
		Metrics:   metrics.New(prometheus.NewRegistry()),
		Log:       logr.Discard(),
		Namespace: "ome",
	}

	c := cand("prod/a", "node1")
	r.ReportCycle(context.Background(), []policy.Candidate{c},
		[]Decision{{Candidate: c, Admitted: true, Target: "node2"}}, config.Default(), testNow)

	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cm.Data[recommendationsKey], `"outcome":"`+OutcomeWithheld+`"`) {
		t.Fatalf("record lost to the conflict: %v", cm.Data)
	}
	if conflicts != 0 {
		t.Fatal("the interceptor conflict was never exercised")
	}
}

func TestNodeRemediationWriteRetriesWithoutRepeatingSignals(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	failing := true
	updates := 0
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(recommendationsCM(nil)).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				updates++
				if failing {
					return apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, obj.GetName(), errors.New("rbac denied"))
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
	recorder := &capturingRecorder{}
	m := metrics.New(prometheus.NewRegistry())
	r := &Reporter{
		Client:    cl,
		Recorder:  recorder,
		Metrics:   m,
		Log:       logr.Discard(),
		Namespace: "ome",
	}
	cfg := config.Default()
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: testNow.Add(-time.Minute),
		}},
	}, "prod/a")

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow)
	afterFirst := updates
	if afterFirst == 0 {
		t.Fatal("the first signal must attempt the durable write")
	}
	if _, ok := nodeRecord(t, cl, "node7"); ok {
		t.Fatal("failed write unexpectedly persisted the node record")
	}

	failing = false
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow.Add(time.Minute))
	if updates <= afterFirst {
		t.Fatal("a failed durable write must be retried on the next cycle")
	}
	if _, ok := nodeRecord(t, cl, "node7"); !ok {
		t.Fatal("durable record still missing after recovery")
	}
	if got := recorder.count("NodeRepairNeeded"); got != 1 {
		t.Fatalf("repair-needed events = %d, want one across the retry", got)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeRepairNeeded")); got != 1 {
		t.Fatalf("repair-needed signals = %v, want one across the retry", got)
	}
}

func TestOMENativeTransitionEventOnce(t *testing.T) {
	r, _, recorder, _ := newTestReporter(t)

	r.ReportOMENativeState(false)
	r.ReportOMENativeState(false)
	events := drainEvents(recorder)
	if len(events) != 1 || !hasEvent(events, "OMENativeUnavailable") {
		t.Fatalf("transition into degraded must emit exactly once: %v", events)
	}
	if !strings.Contains(events[0], "OMENative candidates") || strings.Contains(events[0], "multi-pod") {
		t.Fatalf("degraded Event is surface-incomplete: %v", events)
	}

	r.ReportOMENativeState(true)
	if events := drainEvents(recorder); len(events) != 0 {
		t.Fatalf("recovery is a log line, not an event: %v", events)
	}

	r.ReportOMENativeState(false)
	if events := drainEvents(recorder); len(events) != 1 {
		t.Fatalf("a fresh degradation must emit again: %v", events)
	}
}

func TestReportCycleReconcilesNodeRemediationLifecycle(t *testing.T) {
	r, m, _, cl := newTestReporter(t, recommendationsCM(nil))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	cfg := config.Default()
	transitioned := testNow.Add(-time.Minute)
	health := snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
	}

	first := remediationCandidate("node7", health, "prod/b", "prod/a", "prod/b")
	r.ReportCycle(context.Background(), []policy.Candidate{first}, nil, cfg, testNow)
	if got := recorder.count("NodeRepairNeeded"); got != 1 {
		t.Fatalf("repair-needed events = %d, want 1", got)
	}
	event, ok := recorder.event("NodeRepairNeeded")
	if !ok {
		t.Fatal("NodeRepairNeeded event missing")
	}
	node, ok := event.object.(*corev1.Node)
	if !ok {
		t.Fatalf("repair event target = %T, want *corev1.Node", event.object)
	}
	if node.Name != "node7" || event.eventType != corev1.EventTypeWarning {
		t.Fatalf("repair event target/type = %q/%q, want node7/Warning", node.Name, event.eventType)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeRepairNeeded")); got != 1 {
		t.Fatalf("repair-needed signals = %v, want 1", got)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok {
		t.Fatal("node remediation record missing")
	}
	if got.State != snapshot.NodeHealthUnhealthy || len(got.Conditions) != 1 ||
		got.Conditions[0].Type != "GpuUnhealthy" || got.Conditions[0].Status != corev1.ConditionTrue ||
		!got.Conditions[0].LastTransitionTime.Equal(transitioned) ||
		strings.Join(got.Workloads, ",") != "prod/a,prod/b" || !got.OMEGPUOccupantsPresent || !got.ObservedAt.Equal(testNow) ||
		got.SignaledAt == nil || !got.SignaledAt.Equal(testNow) || got.DrainedAt != nil {
		t.Fatalf("initial node record = %+v", got)
	}

	// Markers are node lifecycle state, not workload recommendations.
	if got := promtestutil.CollectAndCount(m.RecommendationsProduced); got != 0 {
		t.Fatalf("remediation marker created %d workload recommendation series", got)
	}
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	var cycle cycleRecord
	if err := json.Unmarshal([]byte(cm.Data[recommendationsKey]), &cycle); err != nil {
		t.Fatal(err)
	}
	if len(cycle.Recommendations) != 0 {
		t.Fatalf("remediation marker leaked into workload recommendations: %+v", cycle.Recommendations)
	}

	// The durable record follows the complete observed workload set without
	// replaying the episode-opening signal.
	secondAt := testNow.Add(time.Minute)
	second := remediationCandidate("node7", health, "prod/b")
	r.ReportCycle(context.Background(), []policy.Candidate{second}, nil, cfg, secondAt)
	got, _ = nodeRecord(t, cl, "node7")
	if strings.Join(got.Workloads, ",") != "prod/b" || !got.ObservedAt.Equal(secondAt) ||
		got.SignaledAt == nil || !got.SignaledAt.Equal(testNow) {
		t.Fatalf("updated node record = %+v", got)
	}
	if recorder.count("NodeRepairNeeded") != 1 {
		t.Fatalf("unchanged episode repeated repair event: %+v", recorder.events)
	}

	// An OME GPU occupant whose workload identity cannot currently be resolved
	// still blocks completion. Empty Workloads is not proof of an empty node.
	unresolvedAt := testNow.Add(2 * time.Minute)
	unresolved := remediationCandidate("node7", health)
	unresolved.Remediation.OMEGPUOccupantsPresent = true
	r.ReportCycle(context.Background(), []policy.Candidate{unresolved}, nil, cfg, unresolvedAt)
	got, _ = nodeRecord(t, cl, "node7")
	if got.DrainedAt != nil || !got.OMEGPUOccupantsPresent || len(got.Workloads) != 0 {
		t.Fatalf("unresolved-occupancy record = %+v, want occupied with no invented workload", got)
	}
	if got := recorder.count("NodeDrainedForRepair"); got != 0 {
		t.Fatalf("unresolved OME occupancy emitted %d drained events", got)
	}

	// Completion is based on every observed OME GPU occupant, not merely on
	// whether an executable migration candidate remains.
	drainedAt := testNow.Add(3 * time.Minute)
	empty := remediationCandidate("node7", health)
	r.ReportCycle(context.Background(), []policy.Candidate{empty}, nil, cfg, drainedAt)
	got, _ = nodeRecord(t, cl, "node7")
	if got.DrainedAt == nil || !got.DrainedAt.Equal(drainedAt) {
		t.Fatalf("drained time = %v, want %v", got.DrainedAt, drainedAt)
	}
	if got := recorder.count("NodeDrainedForRepair"); got != 1 {
		t.Fatalf("drained events = %d, want 1", got)
	}
	drainedEvent, ok := recorder.event("NodeDrainedForRepair")
	if !ok {
		t.Fatal("NodeDrainedForRepair event missing")
	}
	drainedNode, ok := drainedEvent.object.(*corev1.Node)
	if !ok {
		t.Fatalf("drained event target = %T, want *corev1.Node", drainedEvent.object)
	}
	if drainedNode.Name != "node7" || drainedEvent.eventType != corev1.EventTypeNormal {
		t.Fatalf("drained event target/type = %q/%q, want node7/Normal", drainedNode.Name, drainedEvent.eventType)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeDrainedForRepair")); got != 1 {
		t.Fatalf("drained signals = %v, want 1", got)
	}
	r.ReportCycle(context.Background(), []policy.Candidate{empty}, nil, cfg, testNow.Add(4*time.Minute))
	if got := recorder.count("NodeDrainedForRepair"); got != 1 {
		t.Fatalf("stable drained episode repeated completion: %d", got)
	}

	// Alfred does not cordon Nodes. If an OME workload lands after the drained
	// transition, the durable record must immediately withdraw stale readiness;
	// a later empty observation is a new drain transition and is signaled again.
	reoccupiedAt := testNow.Add(5 * time.Minute)
	r.ReportCycle(context.Background(), []policy.Candidate{second}, nil, cfg, reoccupiedAt)
	got, _ = nodeRecord(t, cl, "node7")
	if got.DrainedAt != nil || !got.OMEGPUOccupantsPresent {
		t.Fatalf("reoccupied node retained stale repair readiness: %+v", got)
	}
	if got := recorder.count("NodeDrainedForRepair"); got != 1 {
		t.Fatalf("reoccupation itself changed drained event count to %d", got)
	}

	redrainedAt := testNow.Add(6 * time.Minute)
	r.ReportCycle(context.Background(), []policy.Candidate{empty}, nil, cfg, redrainedAt)
	got, _ = nodeRecord(t, cl, "node7")
	if got.DrainedAt == nil || !got.DrainedAt.Equal(redrainedAt) {
		t.Fatalf("second drain transition time = %v, want %v", got.DrainedAt, redrainedAt)
	}
	if got := recorder.count("NodeDrainedForRepair"); got != 2 {
		t.Fatalf("second drain transition emitted %d total drained events, want 2", got)
	}
}

func TestReportCycleDoesNotDrainNewEpisodeInSameObservation(t *testing.T) {
	r, m, _, cl := newTestReporter(t, recommendationsCM(nil))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	cfg := config.Default()
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
	})

	// Even when detection finds no occupants, repair-needed is the opening
	// transition. Drained-for-repair requires a later observation of the
	// already-signaled episode.
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow)
	if got := recorder.count("NodeRepairNeeded"); got != 1 {
		t.Fatalf("repair-needed events = %d, want 1", got)
	}
	if got := recorder.count("NodeDrainedForRepair"); got != 0 {
		t.Fatalf("new episode emitted %d same-observation drained events, want 0", got)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt == nil || got.DrainedAt != nil {
		t.Fatalf("new empty episode record = %+v, present=%t", got, ok)
	}

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow.Add(time.Minute))
	if got := recorder.count("NodeDrainedForRepair"); got != 1 {
		t.Fatalf("later empty observation emitted %d drained events, want 1", got)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeDrainedForRepair")); got != 1 {
		t.Fatalf("drained signals = %v, want 1", got)
	}
}

func TestReportCycleSuspectPreservesButDoesNotOpenEpisode(t *testing.T) {
	r, m, _, cl := newTestReporter(t, recommendationsCM(nil))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	cfg := config.Default()
	until := testNow.Add(20 * time.Minute)
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthSuspect,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: testNow.Add(-10 * time.Minute),
		}},
		SuspectUntil: &until,
	}, "prod/a")

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow)
	if len(recorder.events) != 0 || promtestutil.CollectAndCount(m.NodeHealthSignals) != 0 {
		t.Fatalf("suspect-only marker opened an episode: events=%+v", recorder.events)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.State != snapshot.NodeHealthSuspect || got.SuspectUntil == nil ||
		!got.SuspectUntil.Equal(until) || got.SignaledAt != nil || got.DrainedAt != nil {
		t.Fatalf("suspect record = %+v, present=%t", got, ok)
	}

	unknownAt := testNow.Add(time.Minute)
	unknown := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnknown,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: unknownAt,
		}},
	}, "prod/a")
	r.ReportCycle(context.Background(), []policy.Candidate{unknown}, nil, cfg, unknownAt)
	if recorder.count("NodeRepairNeeded") != 1 {
		t.Fatalf("Unknown did not open the suspect-only episode: %+v", recorder.events)
	}

	// Recovery suspicion remains the same active episode, preserving its
	// detection timestamp without opening another incident.
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, cfg, testNow.Add(2*time.Minute))
	got, _ = nodeRecord(t, cl, "node7")
	if got.State != snapshot.NodeHealthSuspect || got.SignaledAt == nil || !got.SignaledAt.Equal(unknownAt) ||
		recorder.count("NodeRepairNeeded") != 1 {
		t.Fatalf("recovery-suspect episode = %+v, events=%+v", got, recorder.events)
	}
}

func TestReportCycleSeedsNodeEpisodeAcrossReporterFailover(t *testing.T) {
	signaledAt := testNow.Add(-10 * time.Minute)
	transitioned := testNow.Add(-time.Hour)
	raw, err := json.Marshal(nodeRecordView{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []nodeConditionView{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
		Workloads:  []string{"prod/a"},
		ObservedAt: signaledAt,
		SignaledAt: &signaledAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, m, _, cl := newTestReporter(t, recommendationsCM(map[string]string{"node.node7": string(raw)}))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
	})

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	if recorder.count("NodeRepairNeeded") != 0 {
		t.Fatalf("new reporter repeated durable repair signal: %+v", recorder.events)
	}
	if recorder.count("NodeDrainedForRepair") != 1 {
		t.Fatalf("new reporter did not complete the durable episode: %+v", recorder.events)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeDrainedForRepair")); got != 1 {
		t.Fatalf("drained signals = %v, want 1", got)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt == nil || !got.SignaledAt.Equal(signaledAt) ||
		got.DrainedAt == nil || !got.DrainedAt.Equal(testNow) {
		t.Fatalf("failover-updated record = %+v, present=%t", got, ok)
	}

	// A second leader seeds both phases and emits neither event again.
	secondRecorder := &capturingRecorder{}
	secondMetrics := metrics.New(prometheus.NewRegistry())
	second := &Reporter{
		Client:    cl,
		Recorder:  secondRecorder,
		Metrics:   secondMetrics,
		Log:       logr.Discard(),
		Namespace: "ome",
	}
	second.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow.Add(time.Minute))
	if len(secondRecorder.events) != 0 || promtestutil.CollectAndCount(secondMetrics.NodeHealthSignals) != 0 {
		t.Fatalf("second leader repeated lifecycle signals: events=%+v", secondRecorder.events)
	}
}

func TestReportCycleUsesValidNodeRecordKeyForMaximumNodeName(t *testing.T) {
	node := strings.Join([]string{
		strings.Repeat("n", 63),
		strings.Repeat("n", 63),
		strings.Repeat("n", 63),
		strings.Repeat("n", 61),
	}, ".")
	if len(node) != 253 {
		t.Fatalf("test node length = %d, want the DNS-subdomain maximum 253", len(node))
	}
	transitioned := testNow.Add(-time.Hour)
	marker := remediationCandidate(node, snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
	}, "prod/a")
	r, _, _, cl := newTestReporter(t, recommendationsCM(nil))
	firstRecorder := &capturingRecorder{}
	r.Recorder = firstRecorder

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	if firstRecorder.count("NodeRepairNeeded") != 1 {
		t.Fatalf("first reporter emitted %d repair signals, want 1", firstRecorder.count("NodeRepairNeeded"))
	}
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, 1)
	for key := range cm.Data {
		if isNodeRecordKey(key) {
			keys = append(keys, key)
		}
	}
	if len(keys) != 1 || len(keys[0]) > 253 {
		t.Fatalf("node record keys = %q, want one ConfigMap-safe key no longer than 253 bytes", keys)
	}
	if !strings.HasPrefix(keys[0], hashedNodeRecordPrefix) {
		t.Fatalf("maximum node record key = %q, want collision-disjoint %q prefix", keys[0], hashedNodeRecordPrefix)
	}
	if problems := utilvalidation.IsConfigMapKey(keys[0]); len(problems) > 0 {
		t.Fatalf("node record key %q is invalid: %v", keys[0], problems)
	}

	secondRecorder := &capturingRecorder{}
	second := &Reporter{
		Client:    cl,
		Recorder:  secondRecorder,
		Metrics:   metrics.New(prometheus.NewRegistry()),
		Log:       logr.Discard(),
		Namespace: "ome",
	}
	second.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow.Add(time.Minute))
	if secondRecorder.count("NodeRepairNeeded") != 0 {
		t.Fatalf("second reporter replayed durable repair signal: %+v", secondRecorder.events)
	}
	second.ReportCycle(context.Background(), nil, nil, config.Default(), testNow.Add(2*time.Minute))
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	for key := range cm.Data {
		if isNodeRecordKey(key) {
			t.Fatalf("cleared maximum-length node left stale record key %q", key)
		}
	}
}

func TestNodeRemediationSeedReadFailureFailsClosed(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	signaledAt := testNow.Add(-10 * time.Minute)
	transitioned := signaledAt.Add(-time.Minute)
	raw, err := json.Marshal(nodeRecordView{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []nodeConditionView{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
		Workloads:  []string{"prod/a"},
		ObservedAt: signaledAt,
		SignaledAt: &signaledAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(recommendationsCM(map[string]string{"node.node7": string(raw)})).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				reads++
				if reads == 1 {
					return apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, key.Name, errors.New("transient read failure"))
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).Build()
	m := metrics.New(prometheus.NewRegistry())
	recorder := &capturingRecorder{}
	r := &Reporter{Client: cl, Recorder: recorder, Metrics: m, Log: logr.Discard(), Namespace: "ome"}
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: transitioned,
		}},
	}, "prod/a")

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	if len(recorder.events) != 0 || promtestutil.CollectAndCount(m.NodeHealthSignals) != 0 {
		t.Fatalf("uncertain failover seed emitted lifecycle signals: events=%+v", recorder.events)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt == nil || !got.SignaledAt.Equal(signaledAt) {
		t.Fatalf("uncertain seed overwrote durable episode: %+v, present=%t", got, ok)
	}
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	var cycle cycleRecord
	if err := json.Unmarshal([]byte(cm.Data[recommendationsKey]), &cycle); err != nil {
		t.Fatalf("normal cycle record was not written while node seeding was gated: %v", err)
	}
	if !cycle.Timestamp.Equal(testNow) {
		t.Fatalf("normal cycle timestamp = %v, want %v", cycle.Timestamp, testNow)
	}

	// The next pass seeds successfully from the preserved record and still
	// does not replay its opening signal.
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow.Add(time.Minute))
	if len(recorder.events) != 0 || promtestutil.CollectAndCount(m.NodeHealthSignals) != 0 {
		t.Fatalf("successful seed replayed durable lifecycle signals: events=%+v", recorder.events)
	}
}

func TestFailedClearDoesNotSuppressFreshEpisodeAfterFailover(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	failUpdates := false
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(recommendationsCM(nil)).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if failUpdates {
					return apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, obj.GetName(), errors.New("transient write failure"))
				}
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
	firstRecorder := &capturingRecorder{}
	first := &Reporter{
		Client:    cl,
		Recorder:  firstRecorder,
		Metrics:   metrics.New(prometheus.NewRegistry()),
		Log:       logr.Discard(),
		Namespace: "ome",
	}
	oldTransition := testNow.Add(-time.Minute)
	oldMarker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: oldTransition,
		}},
	}, "prod/a")

	first.ReportCycle(context.Background(), []policy.Candidate{oldMarker}, nil, config.Default(), testNow)
	oldRecord, ok := nodeRecord(t, cl, "node7")
	if !ok || oldRecord.SignaledAt == nil || !oldRecord.SignaledAt.Equal(testNow) {
		t.Fatalf("initial episode record = %+v, present=%t", oldRecord, ok)
	}

	// Clearing drops the local phase, but the failed desired-state update
	// leaves the old episode durable before this leader exits.
	failUpdates = true
	first.ReportCycle(context.Background(), nil, nil, config.Default(), testNow.Add(time.Minute))
	staleRecord, ok := nodeRecord(t, cl, "node7")
	if !ok || staleRecord.SignaledAt == nil || !staleRecord.SignaledAt.Equal(testNow) {
		t.Fatalf("failed clear did not leave the expected stale record: %+v, present=%t", staleRecord, ok)
	}

	// A later condition transition proves this is a distinct incident. The
	// new leader must not seed the stale signaled phase merely by node name.
	failUpdates = false
	freshAt := testNow.Add(3 * time.Minute)
	freshMarker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: testNow.Add(2 * time.Minute),
		}},
	}, "prod/a")
	secondRecorder := &capturingRecorder{}
	secondMetrics := metrics.New(prometheus.NewRegistry())
	second := &Reporter{
		Client:    cl,
		Recorder:  secondRecorder,
		Metrics:   secondMetrics,
		Log:       logr.Discard(),
		Namespace: "ome",
	}
	second.ReportCycle(context.Background(), []policy.Candidate{freshMarker}, nil, config.Default(), freshAt)
	if got := secondRecorder.count("NodeRepairNeeded"); got != 1 {
		t.Fatalf("fresh incident repair-needed events = %d, want 1: %+v", got, secondRecorder.events)
	}
	if got := promtestutil.ToFloat64(secondMetrics.NodeHealthSignals.WithLabelValues("node7", "NodeRepairNeeded")); got != 1 {
		t.Fatalf("fresh incident repair-needed signals = %v, want 1", got)
	}
	freshRecord, ok := nodeRecord(t, cl, "node7")
	if !ok || freshRecord.SignaledAt == nil || !freshRecord.SignaledAt.Equal(freshAt) {
		t.Fatalf("fresh episode record = %+v, present=%t", freshRecord, ok)
	}
}

func TestFailoverDoesNotSeedPhaseWithoutConditionIdentity(t *testing.T) {
	signaledAt := testNow.Add(-10 * time.Minute)
	raw, err := json.Marshal(nodeRecordView{
		State: snapshot.NodeHealthUnhealthy,
		Conditions: []nodeConditionView{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: signaledAt.Add(-time.Minute),
		}},
		Workloads:  []string{"prod/a"},
		ObservedAt: signaledAt,
		SignaledAt: &signaledAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, m, _, cl := newTestReporter(t, recommendationsCM(map[string]string{"node.node7": string(raw)}))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnhealthy,
	}, "prod/a")

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	if got := recorder.count("NodeRepairNeeded"); got != 1 {
		t.Fatalf("identity-free current evidence emitted %d repair-needed events, want 1", got)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeRepairNeeded")); got != 1 {
		t.Fatalf("identity-free repair-needed signals = %v, want 1", got)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt == nil || !got.SignaledAt.Equal(testNow) {
		t.Fatalf("identity-free evidence retained stale phase: %+v, present=%t", got, ok)
	}
}

func TestUnprovenSuspectDoesNotInheritStalePhase(t *testing.T) {
	observedAt := testNow.Add(-10 * time.Minute)
	signaledAt := observedAt.Add(-time.Hour)
	drainedAt := observedAt.Add(-time.Minute)
	raw, err := json.Marshal(nodeRecordView{
		State: snapshot.NodeHealthSuspect,
		Conditions: []nodeConditionView{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: observedAt.Add(-time.Minute),
		}},
		ObservedAt: observedAt,
		SignaledAt: &signaledAt,
		DrainedAt:  &drainedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, m, _, cl := newTestReporter(t, recommendationsCM(map[string]string{"node.node7": string(raw)}))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	newRecovery := observedAt.Add(time.Minute)
	until := newRecovery.Add(30 * time.Minute)
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthSuspect,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionFalse,
			LastTransitionTime: newRecovery,
		}},
		SuspectUntil: &until,
	})

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	if len(recorder.events) != 0 || promtestutil.CollectAndCount(m.NodeHealthSignals) != 0 {
		t.Fatalf("unproven Suspect emitted lifecycle signals: events=%+v", recorder.events)
	}
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt != nil || got.DrainedAt != nil {
		t.Fatalf("unproven Suspect inherited stale phase: %+v, present=%t", got, ok)
	}
}

func TestReportCycleReconcilesCompleteNodeMarkerSet(t *testing.T) {
	signaledAt := testNow.Add(-10 * time.Minute)
	oldRaw, err := json.Marshal(nodeRecordView{
		State:      snapshot.NodeHealthUnknown,
		Workloads:  []string{"prod/old"},
		ObservedAt: signaledAt,
		SignaledAt: &signaledAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, m, _, cl := newTestReporter(t, recommendationsCM(map[string]string{
		"node.old":  string(oldRaw),
		"unrelated": "preserved",
	}))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	marker := remediationCandidate("live", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnknown,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: testNow,
		}},
	}, "prod/live")

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow)
	var cm corev1.ConfigMap
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "ome", Name: "alfred-recommendations"}, &cm); err != nil {
		t.Fatal(err)
	}
	if _, ok := cm.Data["node.old"]; ok {
		t.Fatalf("stale node marker survived desired-state reconciliation: %v", cm.Data)
	}
	if _, ok := cm.Data["node.live"]; !ok || cm.Data["unrelated"] != "preserved" {
		t.Fatalf("live/unrelated records were not reconciled safely: %v", cm.Data)
	}

	// No markers represents Clear, disabled policy, or a disappeared Node:
	// all remove durable and in-memory episode state.
	r.ReportCycle(context.Background(), nil, nil, config.Default(), testNow.Add(time.Minute))
	if _, ok := nodeRecord(t, cl, "live"); ok {
		t.Fatal("cleared/disabled marker survived in the ConfigMap")
	}
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow.Add(2*time.Minute))
	if got := recorder.count("NodeRepairNeeded"); got != 2 {
		t.Fatalf("fresh incident after clear emitted %d repair signals, want 2 total", got)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("live", "NodeRepairNeeded")); got != 2 {
		t.Fatalf("fresh-incident metric = %v, want 2", got)
	}
}

func TestReportCyclePersistsActiveEpisodeWhenConfigMapIsEnabled(t *testing.T) {
	r, m, _, cl := newTestReporter(t, recommendationsCM(nil))
	recorder := &capturingRecorder{}
	r.Recorder = recorder
	marker := remediationCandidate("node7", snapshot.NodeHealthObservation{
		State: snapshot.NodeHealthUnknown,
		Conditions: []snapshot.NodeConditionObservation{{
			Type:               "GpuUnhealthy",
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: testNow,
		}},
	}, "prod/a")
	disabled := config.Default()
	*disabled.RecommendationsConfigMapEnabled = false

	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, disabled, testNow)
	if _, ok := nodeRecord(t, cl, "node7"); ok {
		t.Fatal("disabled ConfigMap output unexpectedly wrote a node record")
	}
	r.ReportCycle(context.Background(), []policy.Candidate{marker}, nil, config.Default(), testNow.Add(time.Minute))
	got, ok := nodeRecord(t, cl, "node7")
	if !ok || got.SignaledAt == nil || !got.SignaledAt.Equal(testNow) || !got.ObservedAt.Equal(testNow.Add(time.Minute)) {
		t.Fatalf("record after enabling ConfigMap = %+v, present=%t", got, ok)
	}
	if recorder.count("NodeRepairNeeded") != 1 {
		t.Fatalf("enabling persistence repeated the repair event: %+v", recorder.events)
	}
	if got := promtestutil.ToFloat64(m.NodeHealthSignals.WithLabelValues("node7", "NodeRepairNeeded")); got != 1 {
		t.Fatalf("repair-needed signals = %v, want 1", got)
	}
}
