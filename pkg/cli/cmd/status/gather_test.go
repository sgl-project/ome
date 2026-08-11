package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

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
