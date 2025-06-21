package servingruntime

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create integer pointers
func intPointer(i int) *int {
	return &i
}

func TestValidateServingRuntimePriority(t *testing.T) {
	scenarios := map[string]struct {
		name                   string
		newServingRuntime      *v1beta1.ServingRuntime
		existingServingRuntime *v1beta1.ServingRuntime
		expected               gomega.OmegaMatcher
	}{
		"When existing serving runtime is disabled it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-runtime",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-runtime",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},
					Disabled: proto.Bool(true),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When new serving runtime and existing runtime are same it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When model format is different it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "lightgbm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When autoselect is false in the new serving runtime it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(false),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When autoselect is not specified in the new serving runtime it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:     "vllm",
							Version:  proto.String("1"),
							Priority: proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When autoselect is false in the existing serving runtime it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(false),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When model version is nil in both serving runtime and priority is same then it should return error": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.Equal(fmt.Errorf(InvalidPriorityError, "vllm")),
		},
		"When model version is nil in both serving runtime and priority is not same then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(2),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When model version is nil in new serving runtime and priority is same then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When model version is nil in existing serving runtime and priority is same then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When two serving runtime has the same supported model format then it should return error": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("CohereForCausalLM"),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("CohereForCausalLM"),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.Equal(fmt.Errorf(InvalidPriorityError, "vllm")),
		},
		"When two serving runtime has the same supported model format but different architecture then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("CohereForCausalLM"),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("Cohere2ForCasualLM"),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When two serving runtime has the same supported model format but different framework then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("LlamaForCasualLM"),
							ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: proto.String("1.0.0")},
							ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "Transformers", Version: proto.String("1.0.0")},
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "vllm",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("LlamaForCasualLM"),
							ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: proto.String("1.0.0")},
							ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "Transformers", Version: proto.String("1.2.0")},
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When two serving runtime has the same supported model format but different size then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "safetensors",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("LlamaForCasualLM"),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
						Min: proto.String("100B"),
						Max: proto.String("200B"),
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:              "safetensors",
							Version:           proto.String("1"),
							AutoSelect:        proto.Bool(true),
							Priority:          proto.Int32(1),
							ModelArchitecture: proto.String("LlamaForCasualLM"),
						},
					},
					Disabled: proto.Bool(false),
					ModelSizeRange: &v1beta1.ModelSizeRangeSpec{
						Min: proto.String("300B"),
						Max: proto.String("600B"),
					},
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When model version is same in both serving runtime and priority is same then it should return error": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.Equal(fmt.Errorf(InvalidPriorityError, "vllm")),
		},
		"When model version is different but priority is same then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1.3"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1.0"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When priority is nil in both serving runtime then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When priority is nil in new serving runtime and priority is specified in existing serving runtime then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
		"When priority is nil in existing serving runtime and priority is specified in new serving runtime then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			existingServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-2",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							Version:    proto.String("1"),
							AutoSelect: proto.Bool(true),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "seldonio/mlserver:1.2.0",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			err := validateServingRuntimePriority(&scenario.newServingRuntime.Spec, &scenario.existingServingRuntime.Spec,
				scenario.newServingRuntime.Name, scenario.existingServingRuntime.Name)
			g.Expect(err).To(scenario.expected)
		})
	}
}

func TestValidateServingRuntimeAnnotations(t *testing.T) {
	scenarios := map[string]struct {
		spec    v1beta1.ServingRuntimeSpec
		matcher gomega.OmegaMatcher
	}{
		"When chainsaw inject annotation is not set then it should return nil": {
			spec: v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{},
			},
			matcher: gomega.BeNil(),
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			err := validateServingRuntimeAnnotations(&scenario.spec)
			g.Expect(err).To(scenario.matcher)
		})
	}
}

func TestValidateServingRuntimeConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		spec          *v1beta1.ServingRuntimeSpec
		expectSuccess bool
	}{
		{
			name: "both engineConfig and decoderConfig - PDDisaggregated mode",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig:  &v1beta1.EngineSpec{},
				DecoderConfig: &v1beta1.DecoderSpec{},
			},
			expectSuccess: true,
		},
		{
			name: "only decoderConfig - not explicitly covered in validation but valid",
			spec: &v1beta1.ServingRuntimeSpec{
				DecoderConfig: &v1beta1.DecoderSpec{},
			},
			expectSuccess: true,
		},
		{
			name: "only engineConfig - MultiNode with worker size > 0",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(2),
				},
			},
			expectSuccess: true,
		},
		{
			name: "only engineConfig - MultiNode with worker size == 0 (invalid)",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(0),
				},
			},
			expectSuccess: false,
		},
		{
			name: "only engineConfig - MultiNode with worker size < 0 (invalid)",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(-1),
				},
			},
			expectSuccess: false,
		},
		{
			name: "only engineConfig - RawDeployment without worker",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: "RawDeployment",
								},
							},
						},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name: "only engineConfig - pure RawDeployment (no env vars, no workers)",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
						},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name: "only engineConfig - RawDeployment with workers (invalid configuration)",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(2),
				},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: "RawDeployment",
								},
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name: "only engineConfig - MultiNode via env var",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(2),
				},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: "MultiNode",
								},
							},
						},
					},
				},
			},
			expectSuccess: true,
		},
		{
			name: "only engineConfig - MultiNode via env var but missing worker config (invalid)",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: "MultiNode",
								},
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name:          "no configs - valid empty config",
			spec:          &v1beta1.ServingRuntimeSpec{},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServingRuntimeConfiguration(tt.spec)
			if tt.expectSuccess {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected an error but got nil")
				}
			}
		})
	}
}

func TestValidateModelFormatPrioritySame(t *testing.T) {
	scenarios := map[string]struct {
		newServingRuntime *v1beta1.ServingRuntime
		expected          gomega.OmegaMatcher
	}{
		"When different priority assigned for the same model format in the runtime then it should return error": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(1),
						},
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(2),
						},
					},

					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.Equal(fmt.Errorf(PriorityIsNotSameError, "vllm")),
		},
		"When same priority assigned for the same model format in the runtime then it should return nil": {
			newServingRuntime: &v1beta1.ServingRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-runtime-1",
					Namespace: "test",
				},
				Spec: v1beta1.ServingRuntimeSpec{
					SupportedModelFormats: []v1beta1.SupportedModelFormat{
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(2),
						},
						{
							Name:       "vllm",
							AutoSelect: proto.Bool(true),
							Priority:   proto.Int32(2),
						},
					},
					Disabled: proto.Bool(false),
					ProtocolVersions: []constants.InferenceServiceProtocol{
						constants.OpenAIProtocol,
						constants.OpenAIProtocol,
					},
					ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
						Containers: []corev1.Container{
							{
								Name:  constants.MainContainerName,
								Image: "ome/vllm:latest",
								Args: []string{
									"--model_name={{.Name}}",
									"--model_dir=/mnt/models",
									"--http_port=8080",
								},
							},
						},
					},
				},
			},
			expected: gomega.BeNil(),
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			err := validateModelFormatPrioritySame(&scenario.newServingRuntime.Spec)
			g.Expect(err).To(scenario.expected)
		})
	}
}

func TestAreModelSizeRangesEqual(t *testing.T) {
	testcases := []struct {
		name     string
		range1   *v1beta1.ModelSizeRangeSpec
		range2   *v1beta1.ModelSizeRangeSpec
		expected bool
	}{
		{
			name:     "Both nil",
			range1:   nil,
			range2:   nil,
			expected: true,
		},
		{
			name:     "First nil, second not nil",
			range1:   nil,
			range2:   &v1beta1.ModelSizeRangeSpec{},
			expected: false,
		},
		{
			name:     "First not nil, second nil",
			range1:   &v1beta1.ModelSizeRangeSpec{},
			range2:   nil,
			expected: false,
		},
		{
			name:     "Both empty",
			range1:   &v1beta1.ModelSizeRangeSpec{},
			range2:   &v1beta1.ModelSizeRangeSpec{},
			expected: true,
		},
		{
			name: "Different Min values",
			range1: &v1beta1.ModelSizeRangeSpec{
				Min: stringPointer("10"),
			},
			range2: &v1beta1.ModelSizeRangeSpec{
				Min: stringPointer("20"),
			},
			expected: false,
		},
		{
			name: "Different Max values",
			range1: &v1beta1.ModelSizeRangeSpec{
				Max: stringPointer("100"),
			},
			range2: &v1beta1.ModelSizeRangeSpec{
				Max: stringPointer("200"),
			},
			expected: false,
		},
		{
			name:   "First Min nil, second Min not nil",
			range1: &v1beta1.ModelSizeRangeSpec{},
			range2: &v1beta1.ModelSizeRangeSpec{
				Min: stringPointer("10"),
			},
			expected: false,
		},
		{
			name:   "First Max nil, second Max not nil",
			range1: &v1beta1.ModelSizeRangeSpec{},
			range2: &v1beta1.ModelSizeRangeSpec{
				Max: stringPointer("100"),
			},
			expected: false,
		},
		{
			name: "Equal Min and Max values",
			range1: &v1beta1.ModelSizeRangeSpec{
				Min: stringPointer("10"),
				Max: stringPointer("100"),
			},
			range2: &v1beta1.ModelSizeRangeSpec{
				Min: stringPointer("10"),
				Max: stringPointer("100"),
			},
			expected: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := areModelSizeRangesEqual(tc.range1, tc.range2)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

// TestSRValidateAnnotations tests the validateServingRuntimeAnnotations function
func TestSRValidateAnnotations(t *testing.T) {
	testcases := []struct {
		name        string
		annotations map[string]string
		expected    gomega.OmegaMatcher
	}{
		{
			name:        "No annotations",
			annotations: nil,
			expected:    gomega.BeNil(),
		},
		{
			name:        "Empty annotations",
			annotations: map[string]string{},
			expected:    gomega.BeNil(),
		},
		{
			name: "Valid annotations",
			annotations: map[string]string{
				"some-key": "some-value",
			},
			expected: gomega.BeNil(),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			servingRuntime := &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Annotations: tc.annotations,
				},
			}

			err := validateServingRuntimeAnnotations(servingRuntime)
			g.Expect(err).To(tc.expected)
		})
	}
}

// TestContains tests the contains function
func TestContains(t *testing.T) {
	testcases := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "test",
			expected: false,
		},
		{
			name:     "Element not in slice",
			slice:    []string{"a", "b", "c"},
			element:  "d",
			expected: false,
		},
		{
			name:     "Element in slice",
			slice:    []string{"a", "b", "c"},
			element:  "b",
			expected: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := contains(tc.slice, tc.element)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

// TestValidateServingRuntimeConfigurationComplete tests all edge cases of validateServingRuntimeConfiguration
func TestValidateServingRuntimeConfigurationComplete(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *v1beta1.ServingRuntimeSpec
		expectSuccess bool
	}{
		{
			name: "RawDeployment with explicit mode and workers - conflict case",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(1),
				},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: string(constants.RawDeployment),
								},
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name: "MultiNode and RawDeployment mode conflict",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(2),
				},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: string(constants.RawDeployment),
								},
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name: "Explicit MultiNode but missing workers",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Containers: []corev1.Container{
						{
							Name: "engine",
							Env: []corev1.EnvVar{
								{
									Name:  "DEPLOYMENT_MODE",
									Value: string(constants.MultiNode),
								},
							},
						},
					},
				},
			},
			expectSuccess: false,
		},
		{
			name: "Edge case - DecoderConfig only without explicit deployment mode",
			spec: &v1beta1.ServingRuntimeSpec{
				DecoderConfig: &v1beta1.DecoderSpec{},
			},
			expectSuccess: true,
		},
		{
			name: "Edge case - Worker size = 0",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{},
				WorkerPodSpec: &v1beta1.WorkerPodSpec{
					Size: intPointer(0),
				},
			},
			expectSuccess: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			err := validateServingRuntimeConfiguration(tc.spec)

			if tc.expectSuccess {
				g.Expect(err).To(gomega.BeNil())
			} else {
				g.Expect(err).NotTo(gomega.BeNil())
			}
		})
	}
}

// TestValidateServingRuntimeAnnotationsComplete tests edge cases of validateServingRuntimeAnnotations
func TestValidateServingRuntimeAnnotationsComplete(t *testing.T) {
	testcases := []struct {
		name        string
		annotations map[string]string
		expected    gomega.OmegaMatcher
	}{
		{
			name:        "No annotations",
			annotations: nil,
			expected:    gomega.BeNil(),
		},
		{
			name:        "Empty annotations",
			annotations: map[string]string{},
			expected:    gomega.BeNil(),
		},
		{
			name: "Valid annotations",
			annotations: map[string]string{
				"some-key": "some-value",
			},
			expected: gomega.BeNil(),
		},
		{
			name: "Chainsaw annotation - not implemented yet",
			annotations: map[string]string{
				"chainsaw.inject": "true",
			},
			expected: gomega.BeNil(), // Currently this returns nil since the functionality is not implemented
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			servingRuntime := &v1beta1.ServingRuntimeSpec{
				ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
					Annotations: tc.annotations,
				},
			}

			err := validateServingRuntimeAnnotations(servingRuntime)
			g.Expect(err).To(tc.expected)
		})
	}
}

// TestContainsComplete tests the contains function thoroughly
func TestContainsComplete(t *testing.T) {
	testcases := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "test",
			expected: false,
		},
		{
			name:     "Element not in slice",
			slice:    []string{"a", "b", "c"},
			element:  "d",
			expected: false,
		},
		{
			name:     "Element in slice",
			slice:    []string{"a", "b", "c"},
			element:  "b",
			expected: true,
		},
		{
			name:     "Edge case - nil slice",
			slice:    nil,
			element:  "test",
			expected: false,
		},
		{
			name:     "Edge case - empty string",
			slice:    []string{"a", "", "c"},
			element:  "",
			expected: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := contains(tc.slice, tc.element)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

// StringPointer returns a pointer to the given string value
func stringPointer(s string) *string {
	return &s
}
