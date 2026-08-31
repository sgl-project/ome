package query

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestRevisionHashFromControllerRevisionName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no separator", "abcd1234", ""},
		{"single separator", "isvc-hash", "hash"},
		{"isvc-component-hash", "llama-engine-abcd1234", "abcd1234"},
		{"trailing dash returns empty suffix", "llama-engine-", ""},
		{"isvc name with dashes", "llama-70b-instruct-engine-abcd1234", "abcd1234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RevisionHashFromControllerRevisionName(tc.in); got != tc.want {
				t.Errorf("RevisionHashFromControllerRevisionName(%q): got %q want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestLabelPodGroup_MatchesUpstream pins the exact label key the
// scheduler-plugins coscheduler reads. The constant must match
// `scheduling.x-k8s.io/pod-group` byte-for-byte — drift here means
// the scheduler will silently NOT enforce gang placement.
func TestLabelPodGroup_MatchesUpstream(t *testing.T) {
	const want = "scheduling.x-k8s.io/pod-group"
	if LabelPodGroup != want {
		t.Errorf("LabelPodGroup: got %q want %q", LabelPodGroup, want)
	}
}

func TestAnnotationTopologyKey_MatchesSchedulerContract(t *testing.T) {
	const want = "ome.io/topology-key"
	if AnnotationTopologyKey != want {
		t.Errorf("AnnotationTopologyKey: got %q want %q", AnnotationTopologyKey, want)
	}
}

func TestPodName(t *testing.T) {
	cases := []struct {
		name      string
		isvc      string
		component workload.ComponentType
		idx       int32
		runner    string
		ordinal   int32
		want      string
	}{
		{
			name:      "single-pod default",
			isvc:      "llama",
			component: workload.ComponentEngine,
			idx:       0,
			runner:    "default",
			ordinal:   0,
			want:      "llama-engine-0-default-0",
		},
		{
			name:      "multi-pod leader",
			isvc:      "llama-70b",
			component: workload.ComponentEngine,
			idx:       0,
			runner:    "leader",
			ordinal:   0,
			want:      "llama-70b-engine-0-leader-0",
		},
		{
			name:      "multi-pod worker ordinal 2",
			isvc:      "llama-70b",
			component: workload.ComponentEngine,
			idx:       1,
			runner:    "worker",
			ordinal:   2,
			want:      "llama-70b-engine-1-worker-2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PodName(tc.isvc, tc.component, tc.idx, tc.runner, tc.ordinal); got != tc.want {
				t.Errorf("PodName: got %q want %q", got, tc.want)
			}
		})
	}
}

// TestPodGroupName_MatchesInstanceSubdomain pins that the PodGroup name
// is the same shape as the per-Instance DNS subdomain — `<isvc>-<comp>-<idx>`.
// Operators that find a stuck PodGroup can directly run
// `kubectl get pods -l <pod-group-label>=<name>` to find its members,
// and operators that find an unhealthy pod can derive the owning
// PodGroup from the pod's name prefix without consulting status.
func TestPodGroupName_MatchesInstanceSubdomain(t *testing.T) {
	cases := []struct {
		name      string
		isvc      string
		component workload.ComponentType
		idx       int32
		want      string
	}{
		{"engine 0", "llama", workload.ComponentEngine, 0, "llama-engine-0"},
		{"decoder 1", "llama-70b", workload.ComponentDecoder, 1, "llama-70b-decoder-1"},
		{"engine high-index", "llama-70b-instruct", workload.ComponentEngine, 12, "llama-70b-instruct-engine-12"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PodGroupName(tc.isvc, tc.component, tc.idx)
			if got != tc.want {
				t.Errorf("PodGroupName: got %q want %q", got, tc.want)
			}
			// Equivalence pin: same shape as InstanceSubdomain so the
			// operator's mental model is one prefix, not two.
			if sub := InstanceSubdomain(tc.isvc, tc.component, tc.idx); sub != got {
				t.Errorf("PodGroupName diverges from InstanceSubdomain: pg=%q sub=%q", got, sub)
			}
		})
	}
}

func TestHeadlessServiceName(t *testing.T) {
	got := HeadlessServiceName("llama-70b", workload.ComponentEngine)
	if got != "llama-70b-engine-headless" {
		t.Errorf("HeadlessServiceName: got %q want llama-70b-engine-headless", got)
	}
}

func TestStableServiceName(t *testing.T) {
	got := StableServiceName("llama-70b", workload.ComponentDecoder)
	if got != "llama-70b-decoder" {
		t.Errorf("StableServiceName: got %q want llama-70b-decoder", got)
	}
}

func newDiscoverTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1beta1: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// testPodLabels reproduces the label set Render stamps on every
// OMENative pod. Duplicated here (rather than importing render.go's
// podLabels) because that would create a cycle: render.go imports
// query/.
func testPodLabels(isvc string, component workload.ComponentType, idx int32, runner string, incarnation int64) map[string]string {
	return map[string]string{
		constants.InferenceServicePodLabelKey: isvc,
		constants.OMEComponentLabel:           string(component),
		LabelInstanceIdx:                      fmt.Sprintf("%d", idx),
		LabelInstanceIncarnation:              fmt.Sprintf("%d", incarnation),
		LabelRunner:                           runner,
		LabelManagedBy:                        ManagedByOMENative,
	}
}

func newDiscoverPod(name string, instanceIdx int32, incarnation int64, withDeletionTimestamp bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			Labels:    testPodLabels("isvc", workload.ComponentEngine, instanceIdx, "default", incarnation),
		},
	}
	if withDeletionTimestamp {
		t := metav1.Now()
		pod.DeletionTimestamp = &t
		// fake client requires finalizers on objects with DeletionTimestamp
		pod.Finalizers = []string{"keep"}
	}
	return pod
}

func TestLiveListPodsForInstance_FiltersByIndex(t *testing.T) {
	pod0 := newDiscoverPod("p-0", 0, 1, false)
	pod1 := newDiscoverPod("p-1", 1, 1, false)
	c := newDiscoverTestClient(t, pod0, pod1)

	got, err := LiveListPodsForInstance(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0)
	if err != nil {
		t.Fatalf("LiveListPodsForInstance: %v", err)
	}
	if len(got) != 1 || got[0].Name != "p-0" {
		t.Errorf("expected only p-0; got %v", podNames(got))
	}
}

// TestLiveListPodsForComponent_AllIndicesAndUnlabeled pins the teardown
// completion read: every component pod counts regardless of its
// instance-index label — including a pod whose index label is missing
// entirely (the statusless-orphan shape) — while pods of another
// component stay invisible.
func TestLiveListPodsForComponent_AllIndicesAndUnlabeled(t *testing.T) {
	pod0 := newDiscoverPod("p-0", 0, 1, false)
	pod1 := newDiscoverPod("p-1", 1, 1, false)
	orphan := newDiscoverPod("p-orphan", 9, 1, false)
	delete(orphan.Labels, LabelInstanceIdx)
	foreign := newDiscoverPod("p-foreign", 0, 1, false)
	foreign.Labels[constants.OMEComponentLabel] = string(workload.ComponentDecoder)
	c := newDiscoverTestClient(t, pod0, pod1, orphan, foreign)

	got, err := LiveListPodsForComponent(context.Background(), c, "ns", "isvc", workload.ComponentEngine)
	if err != nil {
		t.Fatalf("LiveListPodsForComponent: %v", err)
	}
	want := map[string]bool{"p-0": true, "p-1": true, "p-orphan": true}
	if len(got) != len(want) {
		t.Fatalf("expected %d pods, got %v", len(want), podNames(got))
	}
	for _, p := range got {
		if !want[p.Name] {
			t.Errorf("unexpected pod %s in component list", p.Name)
		}
	}
}

func TestLiveOldPodsClearedForRecreate_NoPods(t *testing.T) {
	c := newDiscoverTestClient(t)
	clear, err := LiveOldPodsClearedForRecreate(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clear {
		t.Errorf("expected clear=true with no pods, got false")
	}
}

func TestLiveOldPodsClearedForRecreate_OnlyNewIncarnation(t *testing.T) {
	// Phase B reconcile after the controller previously created the
	// new-incarnation pods but crashed before Ready. Old pods are
	// long gone, new pods exist. Should be clear.
	c := newDiscoverTestClient(t,
		newDiscoverPod("p-new-0", 0, 2, false),
		newDiscoverPod("p-new-1", 0, 2, false),
	)
	clear, err := LiveOldPodsClearedForRecreate(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clear {
		t.Errorf("expected clear=true when only new-incarnation pods present, got false")
	}
}

func TestLiveOldPodsClearedForRecreate_OldStillRunning(t *testing.T) {
	// Phase B reconcile fired but Phase A deletes haven't propagated
	// — old pod still present, no DeletionTimestamp. Must NOT proceed.
	c := newDiscoverTestClient(t,
		newDiscoverPod("p-old", 0, 1, false),
	)
	clear, err := LiveOldPodsClearedForRecreate(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clear {
		t.Errorf("expected clear=false when old pod still running, got true")
	}
}

func TestLiveOldPodsClearedForRecreate_OldTerminatingTolerated(t *testing.T) {
	// Old pod has DeletionTimestamp (foreground propagation in
	// flight) — Phase B can proceed because the stable name will
	// land on a different pod once GC completes.
	c := newDiscoverTestClient(t,
		newDiscoverPod("p-old", 0, 1, true),
	)
	clear, err := LiveOldPodsClearedForRecreate(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clear {
		t.Errorf("expected clear=true for terminating old pod, got false")
	}
}

func TestLiveOldPodsClearedForRecreate_MissingLabelDoesNotBlockPhaseB(t *testing.T) {
	// Unknown (label-missing) pods are not OLD —
	// LiveOldPodsClearedForRecreate only gates on OLD. In production an
	// orphan short-circuits Restart via FoundOrphan before this helper
	// runs; this test pins the helper's narrower contract: "old pods,
	// are they all terminating?".
	pod := newDiscoverPod("p-orphan", 0, 0, false)
	delete(pod.Labels, LabelInstanceIncarnation)
	c := newDiscoverTestClient(t, pod)

	clear, err := LiveOldPodsClearedForRecreate(context.Background(), c, "ns", "isvc", workload.ComponentEngine, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clear {
		t.Errorf("expected clear=true: unknown bucket pods don't count as old, so the old set is empty")
	}
}

func TestAllTerminating(t *testing.T) {
	if !AllTerminating(nil) {
		t.Errorf("nil slice should be trivially terminating")
	}
	if !AllTerminating([]*corev1.Pod{}) {
		t.Errorf("empty slice should be trivially terminating")
	}
	terminating := newDiscoverPod("t", 0, 1, true)
	if !AllTerminating([]*corev1.Pod{terminating}) {
		t.Errorf("pod with DeletionTimestamp should be terminating")
	}
	running := newDiscoverPod("r", 0, 1, false)
	if AllTerminating([]*corev1.Pod{terminating, running}) {
		t.Errorf("mixed slice should NOT be all-terminating")
	}
}

func podNames(pods []*corev1.Pod) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Name)
	}
	return out
}

// newIndexedDiscoverTestClient mirrors newDiscoverTestClient but also
// registers the OMENativePodIndexField on Pods — the same index
// cmd/manager installs on the manager cache. Lets the test exercise the
// MatchingFields fast path in ListOMENativePodsByName.
func newIndexedDiscoverTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1beta1: %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&corev1.Pod{}, OMENativePodIndexField, OMENativePodIndexExtractor).
		WithObjects(objs...).
		Build()
}

// pod with arbitrary isvc/component labels — used to prove the index
// excludes pods from a different (isvc, component) tuple.
func newDiscoverPodFor(name, isvc string, component workload.ComponentType) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			Labels:    testPodLabels(isvc, component, 0, "default", 1),
		},
	}
}

// TestListOMENativePodsByName_IndexAndFallbackAgree pins the core
// invariant of the field-index optimization: the MatchingFields fast path
// (index registered) and the label-selector fallback (no index) return
// the identical set, and the index excludes other (isvc, component)
// tuples. Without the index the existing tests above already cover the
// fallback; this adds the indexed path and the cross-tuple exclusion.
func TestListOMENativePodsByName_IndexAndFallbackAgree(t *testing.T) {
	objs := []client.Object{
		newDiscoverPodFor("eng-0", "isvc", workload.ComponentEngine),
		newDiscoverPodFor("eng-1", "isvc", workload.ComponentEngine),
		// Different component — must be excluded.
		newDiscoverPodFor("dec-0", "isvc", workload.ComponentDecoder),
		// Different isvc — must be excluded.
		newDiscoverPodFor("other-eng-0", "other", workload.ComponentEngine),
	}

	indexed := newIndexedDiscoverTestClient(t, objs...)
	fallback := newDiscoverTestClient(t, objs...)

	// useIndex=true on both: the indexed client takes the MatchingFields
	// fast path; the index-less client falls back to the label selector.
	// Both must return the identical set.
	gotIndexed, err := ListOMENativePodsByName(context.Background(), indexed, "ns", "isvc", workload.ComponentEngine, true)
	if err != nil {
		t.Fatalf("indexed list: %v", err)
	}
	gotFallback, err := ListOMENativePodsByName(context.Background(), fallback, "ns", "isvc", workload.ComponentEngine, true)
	if err != nil {
		t.Fatalf("fallback list: %v", err)
	}

	want := map[string]bool{"eng-0": true, "eng-1": true}
	assertPodSet(t, "indexed", gotIndexed, want)
	assertPodSet(t, "fallback", gotFallback, want)
}

func assertPodSet(t *testing.T, label string, pods []*corev1.Pod, want map[string]bool) {
	t.Helper()
	if len(pods) != len(want) {
		t.Fatalf("%s: got %v, want keys %v", label, podNames(pods), want)
	}
	for _, p := range pods {
		if !want[p.Name] {
			t.Errorf("%s: unexpected pod %q in result set %v", label, p.Name, podNames(pods))
		}
	}
}

// probeCountingReader wraps a client.Reader and records, per PodList List
// call, whether the caller passed a MatchingFields option (the index
// probe). Lets a test prove the live-reader path skips the doomed probe.
type probeCountingReader struct {
	client.Reader
	podListCalls       int
	matchingFieldsHits int
}

func (r *probeCountingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*corev1.PodList); ok {
		r.podListCalls++
		for _, o := range opts {
			if _, ok := o.(client.MatchingFields); ok {
				r.matchingFieldsHits++
			}
		}
	}
	return r.Reader.List(ctx, list, opts...)
}

// TestListOMENativePodsByName_LiveReaderSkipsProbe pins the live-reader
// contract: with useIndex=false (the live-reader / index-less path), the
// helper goes STRAIGHT to the label selector — exactly ONE PodList List,
// and ZERO MatchingFields probes. A MatchingFields probe against the live
// API reader always fails and forces a second fallback List. Asserting one
// List + zero probes proves the live path never attempts the doomed probe
// while the returned set is still correct.
func TestListOMENativePodsByName_LiveReaderSkipsProbe(t *testing.T) {
	objs := []client.Object{
		newDiscoverPodFor("eng-0", "isvc", workload.ComponentEngine),
		newDiscoverPodFor("eng-1", "isvc", workload.ComponentEngine),
		newDiscoverPodFor("dec-0", "isvc", workload.ComponentDecoder),
	}
	// Index-less client (the real APIReader has no Pod field index either).
	live := newDiscoverTestClient(t, objs...)
	counter := &probeCountingReader{Reader: live}

	got, err := ListOMENativePodsByName(context.Background(), counter, "ns", "isvc", workload.ComponentEngine, false)
	if err != nil {
		t.Fatalf("live list: %v", err)
	}
	assertPodSet(t, "live", got, map[string]bool{"eng-0": true, "eng-1": true})

	if counter.matchingFieldsHits != 0 {
		t.Errorf("live reader path must NOT issue a MatchingFields probe, got %d", counter.matchingFieldsHits)
	}
	if counter.podListCalls != 1 {
		t.Errorf("live reader path must issue exactly 1 PodList List, got %d", counter.podListCalls)
	}
}

// TestListOMENativePodsByName_CachedReaderUsesIndex is the companion to the
// live-path test: with useIndex=true against an index-backed client, the
// helper takes the MatchingFields fast path in a SINGLE List (the index
// resolves, no fallback). Together the two tests prove cached callers keep
// the index fast path while live callers skip the probe.
func TestListOMENativePodsByName_CachedReaderUsesIndex(t *testing.T) {
	objs := []client.Object{
		newDiscoverPodFor("eng-0", "isvc", workload.ComponentEngine),
		newDiscoverPodFor("eng-1", "isvc", workload.ComponentEngine),
		newDiscoverPodFor("dec-0", "isvc", workload.ComponentDecoder),
	}
	indexed := newIndexedDiscoverTestClient(t, objs...)
	counter := &probeCountingReader{Reader: indexed}

	got, err := ListOMENativePodsByName(context.Background(), counter, "ns", "isvc", workload.ComponentEngine, true)
	if err != nil {
		t.Fatalf("cached list: %v", err)
	}
	assertPodSet(t, "cached", got, map[string]bool{"eng-0": true, "eng-1": true})

	if counter.matchingFieldsHits != 1 {
		t.Errorf("cached reader path must take the MatchingFields fast path exactly once, got %d", counter.matchingFieldsHits)
	}
	if counter.podListCalls != 1 {
		t.Errorf("cached reader path must resolve via the index in 1 List (no fallback), got %d", counter.podListCalls)
	}
}
