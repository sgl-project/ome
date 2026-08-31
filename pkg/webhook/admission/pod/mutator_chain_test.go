package pod

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// TestMutator_Handle_MissingConfigMap verifies that admission of a
// labeled pod is denied with a 500 when inferenceservice-config is
// absent — the failurePolicy=Fail contract means a missing config
// surfaces loudly instead of admitting an uninjected pod.
func TestMutator_Handle_MissingConfigMap(t *testing.T) {
	mutator := Mutator{Client: c, Clientset: clientset, Decoder: admission.NewDecoder(c.Scheme())}

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{constants.InferenceServicePodLabelKey: "isvc-pod"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: constants.MainContainerName}},
		},
	}
	raw, err := json.Marshal(pod)
	require.NoError(t, err)

	res := mutator.Handle(context.TODO(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID(uuid.NewString()),
			Namespace: "default",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: raw},
		},
	})
	assert.False(t, res.Allowed)
	require.NotNil(t, res.Result)
	assert.Equal(t, int32(500), res.Result.Code)
}

// TestMutatorChain_ReinvocationIdempotent runs the full injector chain
// twice over one pod and requires the second pass to be a no-op. The
// webhook registers with reinvocationPolicy=IfNeeded, so any other
// mutating webhook in the cluster can trigger a re-run against the
// already-mutated pod; a non-idempotent injector would stack duplicate
// env/volumes/containers.
func TestMutatorChain_ReinvocationIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	mutator := Mutator{Client: ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()}

	configMap := &v1.ConfigMap{Data: map[string]string{}}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chain-pod",
			Namespace: "default",
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: "chain-isvc",
			},
			Annotations: map[string]string{
				constants.RDMAAutoInjectAnnotationKey:              "true",
				constants.RuntimeShmProfileAnnotationKey:           "default",
				constants.RuntimeProbeProfileAnnotationKey:         "sglang-http",
				constants.RuntimeObservabilityProfileAnnotationKey: "prometheus",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: DefaultContainerName}},
		},
	}

	require.NoError(t, mutator.mutate(pod, configMap))
	// Sanity: the first pass actually injected work for every
	// annotation-gated injector in the chain.
	require.NotEmpty(t, pod.Spec.Containers[0].Env, "rdma profile env expected")
	require.NotEmpty(t, pod.Spec.Volumes, "rdma/shm volumes expected")
	require.NotNil(t, pod.Spec.Containers[0].ReadinessProbe, "probe profile expected")
	require.Equal(t, "true", pod.Annotations[constants.PrometheusScrapeAnnotationKey])

	first := pod.DeepCopy()
	require.NoError(t, mutator.mutate(pod, configMap))
	assert.Equal(t, first, pod, "second mutate pass must be a byte-for-byte no-op")
}
