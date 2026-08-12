package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/factory"
	omefake "sigs.k8s.io/ome/pkg/client/clientset/versioned/fake"
	"sigs.k8s.io/ome/pkg/constants"
)

func pod(name, ns, isvc, component string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ns,
		Labels: map[string]string{
			constants.InferenceServiceLabel: isvc,
			constants.OMEComponentLabel:     component,
		},
	}}
}

func TestGatherGroupsPodsByComponent(t *testing.T) {
	f := factory.Static{
		OME: omefake.NewSimpleClientset(&v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "llama", Namespace: "team-a"},
		}),
		Kube: kubefake.NewSimpleClientset(
			pod("llama-engine-1", "team-a", "llama", "engine"),
			pod("llama-decoder-1", "team-a", "llama", "decoder"),
			pod("other-engine-1", "team-a", "other", "engine"),
		),
		NS: "team-a",
	}
	r, err := gather(context.Background(), f, "team-a", "llama")
	require.NoError(t, err)
	require.Len(t, r.Pods[v1beta1.EngineComponent], 1)
	assert.Equal(t, "llama-engine-1", r.Pods[v1beta1.EngineComponent][0].Name)
	require.Len(t, r.Pods[v1beta1.DecoderComponent], 1)
	assert.Empty(t, r.Pods[v1beta1.RouterComponent])
}

func TestGatherKeepsOnlyWarningEventsForOurObjects(t *testing.T) {
	warn := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "e1", Namespace: "team-a"},
		Type:           corev1.EventTypeWarning,
		Reason:         "FailedScheduling",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "llama-engine-1", Namespace: "team-a"},
	}
	other := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "e2", Namespace: "team-a"},
		Type:           corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "unrelated", Namespace: "team-a"},
	}
	f := factory.Static{
		OME:  omefake.NewSimpleClientset(&v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "llama", Namespace: "team-a"}}),
		Kube: kubefake.NewSimpleClientset(pod("llama-engine-1", "team-a", "llama", "engine"), warn, other),
		NS:   "team-a",
	}
	r, err := gather(context.Background(), f, "team-a", "llama")
	require.NoError(t, err)
	require.Len(t, r.Events, 1)
	assert.Equal(t, "FailedScheduling", r.Events[0].Reason)
}

// TestGatherIssuesFieldSelectorEventQueriesPerObject pins the kubectl-describe
// pattern: gather() must query events per involved object (the
// InferenceService, then each pod) with a field selector, rather than
// listing every Warning event in the namespace. The fake clientset ignores
// field selectors when serving the request (so this cannot be observed via
// gather()'s output), but it still parses and records the selector actually
// sent -- capture it with a reactor and check it real-selector-matches the
// right object and rejects a wrong one.
func TestGatherIssuesFieldSelectorEventQueriesPerObject(t *testing.T) {
	kube := kubefake.NewSimpleClientset(pod("llama-engine-1", "team-a", "llama", "engine"))
	var restrictions []ktesting.ListRestrictions
	kube.PrependReactor("list", "events", func(action ktesting.Action) (bool, runtime.Object, error) {
		la := action.(ktesting.ListActionImpl)
		restrictions = append(restrictions, la.GetListRestrictions())
		return false, nil, nil // fall through to the default tracker-backed reactor
	})
	f := factory.Static{
		OME:  omefake.NewSimpleClientset(&v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "llama", Namespace: "team-a"}}),
		Kube: kube,
		NS:   "team-a",
	}

	_, err := gather(context.Background(), f, "team-a", "llama")

	require.NoError(t, err)
	require.Len(t, restrictions, 2, "one events query for the InferenceService, one for its pod")

	// Fields.Matches only checks that the selector's *required* terms hold;
	// it does not fail just because the candidate has extra keys. So
	// asserting Matches() on a full {name,kind,type} set alone would not
	// notice a selector that forgot to require "kind" at all. Nail that
	// down explicitly with RequiresExactMatch, then use Matches() with a
	// same-name-but-wrong-kind candidate (the real scenario this guards:
	// an InferenceService and a Pod that happen to share a name) as a
	// second, semantic check that kind is actually load-bearing.
	kind0, found0 := restrictions[0].Fields.RequiresExactMatch("involvedObject.kind")
	require.True(t, found0, "InferenceService query must require involvedObject.kind")
	assert.Equal(t, "InferenceService", kind0)
	name0, found0n := restrictions[0].Fields.RequiresExactMatch("involvedObject.name")
	require.True(t, found0n)
	assert.Equal(t, "llama", name0)
	assert.True(t, restrictions[0].Fields.Matches(fields.Set{
		"involvedObject.name": "llama", "involvedObject.kind": "InferenceService", "type": "Warning",
	}), "first query scopes to the InferenceService")
	assert.False(t, restrictions[0].Fields.Matches(fields.Set{
		"involvedObject.name": "llama", "involvedObject.kind": "Pod", "type": "Warning",
	}), "must not match a same-named Pod -- kind has to discriminate too")
	assert.False(t, restrictions[0].Fields.Matches(fields.Set{
		"involvedObject.name": "someone-else", "involvedObject.kind": "InferenceService", "type": "Warning",
	}), "must not match some other object's events")

	kind1, found1 := restrictions[1].Fields.RequiresExactMatch("involvedObject.kind")
	require.True(t, found1, "Pod query must require involvedObject.kind")
	assert.Equal(t, "Pod", kind1)
	assert.True(t, restrictions[1].Fields.Matches(fields.Set{
		"involvedObject.name": "llama-engine-1", "involvedObject.kind": "Pod", "type": "Warning",
	}), "second query scopes to the pod")
	assert.False(t, restrictions[1].Fields.Matches(fields.Set{
		"involvedObject.name": "llama-engine-1", "involvedObject.kind": "InferenceService", "type": "Warning",
	}), "must not match an InferenceService that happens to share the pod's name")
}

// TestMergeWarningEventsFiltersByTypeNameAndKind is the defense-in-depth
// client-side check that must survive even though the field selector
// already does this filtering server-side: a field-selector-blind source
// (the fake clientset in tests, or a misbehaving API server) must not leak
// unrelated events into the report.
func TestMergeWarningEventsFiltersByTypeNameAndKind(t *testing.T) {
	want := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{UID: "uid-1"},
		Type:           corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "llama-engine-1"},
	}
	wrongType := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{UID: "uid-2"},
		Type:           corev1.EventTypeNormal,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "llama-engine-1"},
	}
	wrongName := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{UID: "uid-3"},
		Type:           corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "unrelated"},
	}
	wrongKind := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{UID: "uid-4"},
		Type:           corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{Kind: "InferenceService", Name: "llama-engine-1"},
	}

	got := mergeWarningEvents(nil, map[types.UID]bool{},
		[]corev1.Event{want, wrongType, wrongName, wrongKind}, "llama-engine-1", "Pod")

	require.Len(t, got, 1)
	assert.Equal(t, types.UID("uid-1"), got[0].UID)
}

// TestMergeWarningEventsDedupesByUIDAcrossQueries mirrors gather()'s real
// usage: one shared `seen` map threaded across several per-object queries
// (InferenceService, then each pod). An event that would otherwise be
// counted twice must survive only once.
func TestMergeWarningEventsDedupesByUIDAcrossQueries(t *testing.T) {
	e := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{UID: "dup-uid"},
		Type:           corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "llama-engine-1"},
	}
	seen := map[types.UID]bool{}
	var acc []corev1.Event
	acc = mergeWarningEvents(acc, seen, []corev1.Event{e}, "llama-engine-1", "Pod")
	acc = mergeWarningEvents(acc, seen, []corev1.Event{e}, "llama-engine-1", "Pod")

	assert.Len(t, acc, 1, "the same event UID must not be appended twice across queries")
}
