package servingruntime

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func TestValidateModelFormatPrioritySame(t *testing.T) {
	scenarios := map[string]struct {
		name              string
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
