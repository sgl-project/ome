package pod

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
