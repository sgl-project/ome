package basemodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/runtimeselector"
)

func TestValidateModelRuntimeSupport(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))

	tests := []struct {
		name      string
		model     *v1beta1.BaseModelSpec
		namespace string
		objects   []client.Object
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "matching cluster runtime passes",
			model:     modelSpec("safetensors", "LlamaForCausalLM", "7B"),
			namespace: "default",
			objects: []client.Object{
				clusterRuntime("llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", true),
			},
		},
		{
			name:      "model size outside runtime range fails",
			model:     modelSpec("safetensors", "LlamaForCausalLM", "70B"),
			namespace: "default",
			objects: []client.Object{
				clusterRuntime("small-llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", true),
			},
			wantErr: true,
			errMsg:  "no supporting runtime found for model llama-model",
		},
		{
			name:      "runtime accelerator requirements are ignored for model-only validation",
			model:     modelSpec("safetensors", "LlamaForCausalLM", "7B"),
			namespace: "default",
			objects: []client.Object{
				clusterRuntimeWithAccelerator("gpu-llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", "gpu-a10", true),
			},
		},
		{
			name:      "runtime with autoselect disabled is not a match",
			model:     modelSpec("safetensors", "LlamaForCausalLM", "7B"),
			namespace: "default",
			objects: []client.Object{
				clusterRuntime("manual-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", false),
			},
			wantErr: true,
			errMsg:  "no supporting runtime found for model llama-model",
		},
		{
			name:      "namespace base model can match namespace scoped runtime",
			model:     modelSpec("safetensors", "LlamaForCausalLM", "7B"),
			namespace: "default",
			objects: []client.Object{
				namespacedRuntime("namespace-runtime", "default", "safetensors", "LlamaForCausalLM", "1B", "10B", true),
			},
		},
		{
			name:      "missing model format skips validation",
			model:     &v1beta1.BaseModelSpec{},
			namespace: "default",
			objects: []client.Object{
				clusterRuntime("llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", true),
			},
		},
		{
			name:      "disabled model skips validation",
			model:     disabledModelSpec("unknown", "UnknownForCausalLM", "7B"),
			namespace: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				Build()

			_, err := validateModelRuntimeSupport(context.Background(), runtimeselector.New(fakeClient), "llama-model", tt.namespace, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestBaseModelValidatorValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))

	runtime := clusterRuntime("llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", true)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(runtime).
		Build()

	validator := &BaseModelValidator{
		RuntimeSelector: runtimeselector.New(fakeClient),
	}
	model := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-model",
			Namespace: "default",
		},
		Spec: *modelSpec("safetensors", "LlamaForCausalLM", "7B"),
	}

	_, err := validator.ValidateCreate(context.Background(), model)
	assert.NoError(t, err)
}

func TestClusterBaseModelValidatorValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))

	runtime := clusterRuntime("llama-runtime", "safetensors", "LlamaForCausalLM", "1B", "10B", true)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(runtime).
		Build()

	validator := &ClusterBaseModelValidator{
		RuntimeSelector: runtimeselector.New(fakeClient),
	}
	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "llama-model",
		},
		Spec: *modelSpec("safetensors", "LlamaForCausalLM", "7B"),
	}

	_, err := validator.ValidateCreate(context.Background(), model)
	assert.NoError(t, err)
}

func TestClusterBaseModelValidatorIgnoresNamespacedRuntimes(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1beta1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespacedRuntime("namespace-runtime", "default", "safetensors", "LlamaForCausalLM", "1B", "10B", true)).
		Build()

	validator := &ClusterBaseModelValidator{
		RuntimeSelector: runtimeselector.New(fakeClient),
	}
	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "llama-model",
		},
		Spec: *modelSpec("safetensors", "LlamaForCausalLM", "7B"),
	}

	_, err := validator.ValidateCreate(context.Background(), model)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no supporting runtime found for model llama-model")
}

func modelSpec(formatName, architecture, size string) *v1beta1.BaseModelSpec {
	return &v1beta1.BaseModelSpec{
		ModelFormat: v1beta1.ModelFormat{
			Name: formatName,
		},
		ModelArchitecture:  stringPtr(architecture),
		ModelParameterSize: stringPtr(size),
	}
}

func disabledModelSpec(formatName, architecture, size string) *v1beta1.BaseModelSpec {
	model := modelSpec(formatName, architecture, size)
	model.Disabled = boolPtr(true)
	return model
}

func clusterRuntime(name, formatName, architecture, minSize, maxSize string, autoSelect bool) *v1beta1.ClusterServingRuntime {
	return clusterRuntimeWithAccelerator(name, formatName, architecture, minSize, maxSize, "", autoSelect)
}

func clusterRuntimeWithAccelerator(name, formatName, architecture, minSize, maxSize, acceleratorClass string, autoSelect bool) *v1beta1.ClusterServingRuntime {
	spec := servingRuntimeSpec(formatName, architecture, minSize, maxSize, autoSelect)
	if acceleratorClass != "" {
		spec.AcceleratorRequirements = &v1beta1.AcceleratorRequirements{
			AcceleratorClasses: []string{acceleratorClass},
		}
	}
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
}

func namespacedRuntime(name, namespace, formatName, architecture, minSize, maxSize string, autoSelect bool) *v1beta1.ServingRuntime {
	return &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: servingRuntimeSpec(formatName, architecture, minSize, maxSize, autoSelect),
	}
}

func servingRuntimeSpec(formatName, architecture, minSize, maxSize string, autoSelect bool) v1beta1.ServingRuntimeSpec {
	return v1beta1.ServingRuntimeSpec{
		SupportedModelFormats: []v1beta1.SupportedModelFormat{
			{
				ModelFormat: &v1beta1.ModelFormat{
					Name: formatName,
				},
				ModelArchitecture: stringPtr(architecture),
				AutoSelect:        boolPtr(autoSelect),
				Priority:          int32Ptr(1),
			},
		},
		ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
			Min: stringPtr(minSize),
			Max: stringPtr(maxSize),
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}
