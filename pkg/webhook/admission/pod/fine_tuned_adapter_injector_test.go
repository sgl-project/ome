package pod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestNewFineTunedAdapterInjector(t *testing.T) {
	tests := []struct {
		name        string
		configData  map[string]string
		expectError bool
	}{
		{
			name: "valid config",
			configData: map[string]string{
				fineTunedAdapterConfigMapKeyName: `{"image":"ome/agent:1","compartmentId":"c1","authType":"InstancePrincipal"}`,
			},
			expectError: false,
		},
		{
			name:        "missing key yields zero-value injector",
			configData:  map[string]string{},
			expectError: false,
		},
		{
			name: "malformed JSON returns error instead of panicking",
			configData: map[string]string{
				fineTunedAdapterConfigMapKeyName: `{"image": not-json`,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap := &v1.ConfigMap{Data: tt.configData}
			var injector *FineTunedAdapterInjector
			var err error
			assert.NotPanics(t, func() {
				injector, err = newFineTunedAdapterInjector(configMap, nil)
			})
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, injector)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, injector)
			}
		})
	}
}

// TestInjectFineTunedAdapter_MissingWeightReturnsError verifies that a
// FineTunedWeight lookup failure surfaces as an error from the injector
// instead of a nil-pointer panic in getModelInitEnvs.
func TestInjectFineTunedAdapter_MissingWeightReturnsError(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))
	fakeClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	injector, err := newFineTunedAdapterInjector(&v1.ConfigMap{Data: map[string]string{
		fineTunedAdapterConfigMapKeyName: `{"image":"ome/agent:1","compartmentId":"c1","authType":"InstancePrincipal","memoryRequest":"100Mi","memoryLimit":"1Gi","cpuRequest":"100m","cpuLimit":"1"}`,
	}}, fakeClient)
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Annotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: "does-not-exist",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: constants.MainContainerName}},
		},
	}

	assert.NotPanics(t, func() {
		err = injector.InjectFineTunedAdapter(pod)
	})
	assert.Error(t, err)
	assert.Empty(t, pod.Spec.InitContainers, "no init container should be injected on lookup failure")
}

// TestInjectFineTunedAdapter_MissingStorageURIReturnsError covers the
// FineTunedWeight-exists-but-has-no-storage-URI shape, which must return
// an error rather than dereference a nil pointer.
func TestInjectFineTunedAdapter_MissingStorageURIReturnsError(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))
	ftw := &v1beta1.FineTunedWeight{
		ObjectMeta: metav1.ObjectMeta{Name: "no-storage"},
	}
	fakeClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(ftw).Build()

	injector, err := newFineTunedAdapterInjector(&v1.ConfigMap{Data: map[string]string{
		fineTunedAdapterConfigMapKeyName: `{"image":"ome/agent:1","compartmentId":"c1","authType":"InstancePrincipal","memoryRequest":"100Mi","memoryLimit":"1Gi","cpuRequest":"100m","cpuLimit":"1"}`,
	}}, fakeClient)
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Annotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: "no-storage",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: constants.MainContainerName}},
		},
	}

	assert.NotPanics(t, func() {
		err = injector.InjectFineTunedAdapter(pod)
	})
	assert.Error(t, err)
	assert.Empty(t, pod.Spec.InitContainers)
}
