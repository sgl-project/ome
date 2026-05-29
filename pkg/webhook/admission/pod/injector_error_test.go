package pod

import (
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestInjectorConstructorsReturnJSONErrors(t *testing.T) {
	tests := map[string]struct {
		configKey string
		new       func(*v1.ConfigMap) error
	}{
		"model init": {
			configKey: modelInitConfigMapKeyName,
			new: func(configMap *v1.ConfigMap) error {
				_, err := newModelInitInjector(configMap)
				return err
			},
		},
		"fine-tuned adapter": {
			configKey: fineTunedAdapterConfigMapKeyName,
			new: func(configMap *v1.ConfigMap) error {
				_, err := newFineTunedAdapterInjector(configMap, nil)
				return err
			},
		},
		"serving sidecar": {
			configKey: servingSidecarConfigMapKeyName,
			new: func(configMap *v1.ConfigMap) error {
				_, err := newServingSidecarInjector(configMap)
				return err
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
				Data: map[string]string{
					tt.configKey: `{"image":`,
				},
			}
			if err := tt.new(configMap); err == nil {
				t.Fatalf("expected invalid JSON error")
			}
		})
	}
}

func TestInjectorContainerBuildersReturnResourceErrors(t *testing.T) {
	tests := map[string]func() error{
		"model init": func() error {
			_, err := (&ModelInitInjector{
				CpuLimit:      "bad-cpu",
				MemoryLimit:   "1Gi",
				CpuRequest:    "1",
				MemoryRequest: "1Gi",
			}).createInitContainer(nil, nil, nil)
			return err
		},
		"fine-tuned adapter": func() error {
			_, err := (&FineTunedAdapterInjector{
				CpuLimit:      "bad-cpu",
				MemoryLimit:   "1Gi",
				CpuRequest:    "1",
				MemoryRequest: "1Gi",
			}).createInitContainer(nil, nil, nil)
			return err
		},
		"serving sidecar": func() error {
			_, err := (&ServingSidecarInjector{
				CpuLimit:      "bad-cpu",
				MemoryLimit:   "1Gi",
				CpuRequest:    "1",
				MemoryRequest: "1Gi",
			}).createServingSidecarContainer(nil, nil, nil)
			return err
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test(); err == nil {
				t.Fatalf("expected invalid resource quantity error")
			}
		})
	}
}

func TestFineTunedAdapterReturnsWeightLookupError(t *testing.T) {
	injector := newTestFineTunedAdapterInjector(t, nil)
	pod := newFineTunedAdapterPod("missing-weight")

	err := injector.InjectFineTunedAdapter(pod)
	if err == nil {
		t.Fatalf("expected fine-tuned weight lookup error")
	}
}

func TestFineTunedAdapterRequiresStorageURI(t *testing.T) {
	tests := map[string]*v1beta1.StorageSpec{
		"missing storage": nil,
		"missing uri":     {},
		"empty uri":       {StorageUri: stringPtr("")},
	}

	for name, storageSpec := range tests {
		t.Run(name, func(t *testing.T) {
			injector := newTestFineTunedAdapterInjector(t, &v1beta1.FineTunedWeight{
				ObjectMeta: metav1.ObjectMeta{Name: "ft-weight"},
				Spec: v1beta1.FineTunedWeightSpec{
					Storage: storageSpec,
				},
			})
			pod := newFineTunedAdapterPod("ft-weight")

			err := injector.InjectFineTunedAdapter(pod)
			if err == nil {
				t.Fatalf("expected storage URI validation error")
			}
			if !strings.Contains(err.Error(), "storage.storageUri is required") {
				t.Fatalf("expected storage URI validation error, got %v", err)
			}
		})
	}
}

func newTestFineTunedAdapterInjector(t *testing.T, objects ...*v1beta1.FineTunedWeight) *FineTunedAdapterInjector {
	t.Helper()

	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add v1beta1 to scheme: %v", err)
	}

	builder := fake.NewClientBuilder().WithScheme(s)
	for _, obj := range objects {
		if obj != nil {
			builder.WithObjects(obj)
		}
	}

	return &FineTunedAdapterInjector{
		Image:         "fine-tuned-adapter:latest",
		CompartmentId: "ocid1.compartment.oc1..test",
		AuthType:      "InstancePrincipal",
		client:        builder.Build(),
	}
}

func newFineTunedAdapterPod(weightName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.FineTunedAdapterInjectionKey: weightName,
			},
		},
	}
}

func stringPtr(value string) *string {
	return &value
}
