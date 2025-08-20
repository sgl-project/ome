package v1beta1

import (
	"testing"

	"github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	"github.com/sgl-project/ome/pkg/constants"
)

func TestSupportedModelFormat_GetAcceleratorConfig(t *testing.T) {
	testCases := []struct {
		name             string
		format           *SupportedModelFormat
		acceleratorClass string
		expected         *AcceleratorModelConfig
	}{
		{
			name: "returns config when accelerator class exists",
			format: &SupportedModelFormat{
				Name: "test-format",
				AcceleratorConfig: map[string]*AcceleratorModelConfig{
					"nvidia-a100": {
						MinMemoryPerBillionParams: proto.Int64(1000),
						RuntimeArgsOverride:       []string{"--arg1", "--arg2"},
						EnvironmentOverride: map[string]string{
							"ENV_VAR": "value",
						},
					},
					"nvidia-h100": {
						MinMemoryPerBillionParams: proto.Int64(800),
					},
				},
			},
			acceleratorClass: "nvidia-a100",
			expected: &AcceleratorModelConfig{
				MinMemoryPerBillionParams: proto.Int64(1000),
				RuntimeArgsOverride:       []string{"--arg1", "--arg2"},
				EnvironmentOverride: map[string]string{
					"ENV_VAR": "value",
				},
			},
		},
		{
			name: "returns nil when accelerator class does not exist",
			format: &SupportedModelFormat{
				Name: "test-format",
				AcceleratorConfig: map[string]*AcceleratorModelConfig{
					"nvidia-a100": {
						MinMemoryPerBillionParams: proto.Int64(1000),
					},
				},
			},
			acceleratorClass: "nvidia-v100",
			expected:         nil,
		},
		{
			name: "returns nil when AcceleratorConfig is nil",
			format: &SupportedModelFormat{
				Name:              "test-format",
				AcceleratorConfig: nil,
			},
			acceleratorClass: "nvidia-a100",
			expected:         nil,
		},
		{
			name: "returns nil when AcceleratorConfig is empty",
			format: &SupportedModelFormat{
				Name:              "test-format",
				AcceleratorConfig: map[string]*AcceleratorModelConfig{},
			},
			acceleratorClass: "nvidia-a100",
			expected:         nil,
		},
		{
			name: "returns config with complex tensor parallelism settings",
			format: &SupportedModelFormat{
				Name: "test-format",
				AcceleratorConfig: map[string]*AcceleratorModelConfig{
					"nvidia-h100": {
						MinMemoryPerBillionParams: proto.Int64(500),
						TensorParallelismOverride: &TensorParallelismConfig{
							TensorParallelSize:   proto.Int64(4),
							PipelineParallelSize: proto.Int64(2),
							DataParallelSize:     proto.Int64(1),
						},
						RuntimeArgsOverride: []string{"--tensor-parallel-size=4"},
						EnvironmentOverride: map[string]string{
							"TP_SIZE": "4",
							"PP_SIZE": "2",
						},
					},
				},
			},
			acceleratorClass: "nvidia-h100",
			expected: &AcceleratorModelConfig{
				MinMemoryPerBillionParams: proto.Int64(500),
				TensorParallelismOverride: &TensorParallelismConfig{
					TensorParallelSize:   proto.Int64(4),
					PipelineParallelSize: proto.Int64(2),
					DataParallelSize:     proto.Int64(1),
				},
				RuntimeArgsOverride: []string{"--tensor-parallel-size=4"},
				EnvironmentOverride: map[string]string{
					"TP_SIZE": "4",
					"PP_SIZE": "2",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.format.GetAcceleratorConfig(tc.acceleratorClass)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

func TestServingRuntimeSpec_IsDisabled(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *ServingRuntimeSpec
		expected bool
	}{
		{
			name: "returns true when disabled is explicitly true",
			spec: &ServingRuntimeSpec{
				Disabled: proto.Bool(true),
			},
			expected: true,
		},
		{
			name: "returns false when disabled is explicitly false",
			spec: &ServingRuntimeSpec{
				Disabled: proto.Bool(false),
			},
			expected: false,
		},
		{
			name: "returns false when disabled is nil",
			spec: &ServingRuntimeSpec{
				Disabled: nil,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.spec.IsDisabled()
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

func TestServingRuntimeSpec_IsProtocolVersionSupported(t *testing.T) {
	testCases := []struct {
		name                 string
		spec                 *ServingRuntimeSpec
		modelProtocolVersion constants.InferenceServiceProtocol
		expected             bool
	}{
		{
			name: "returns true when protocol is supported",
			spec: &ServingRuntimeSpec{
				ProtocolVersions: []constants.InferenceServiceProtocol{
					constants.OpenAIProtocol,
					constants.OpenInferenceProtocolV1,
				},
			},
			modelProtocolVersion: constants.OpenAIProtocol,
			expected:             true,
		},
		{
			name: "returns false when protocol is not supported",
			spec: &ServingRuntimeSpec{
				ProtocolVersions: []constants.InferenceServiceProtocol{
					constants.OpenAIProtocol,
				},
			},
			modelProtocolVersion: constants.OpenInferenceProtocolV2,
			expected:             false,
		},
		{
			name: "returns true when protocol versions is nil",
			spec: &ServingRuntimeSpec{
				ProtocolVersions: nil,
			},
			modelProtocolVersion: constants.OpenAIProtocol,
			expected:             true,
		},
		{
			name: "returns true when protocol versions is empty",
			spec: &ServingRuntimeSpec{
				ProtocolVersions: []constants.InferenceServiceProtocol{},
			},
			modelProtocolVersion: constants.OpenAIProtocol,
			expected:             true,
		},
		{
			name: "returns true when model protocol version is empty",
			spec: &ServingRuntimeSpec{
				ProtocolVersions: []constants.InferenceServiceProtocol{
					constants.OpenAIProtocol,
				},
			},
			modelProtocolVersion: "",
			expected:             true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.spec.IsProtocolVersionSupported(tc.modelProtocolVersion)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

func TestServingRuntimeSpec_GetPriority(t *testing.T) {
	testCases := []struct {
		name      string
		spec      *ServingRuntimeSpec
		modelName string
		expected  *int32
	}{
		{
			name: "returns priority when model exists",
			spec: &ServingRuntimeSpec{
				SupportedModelFormats: []SupportedModelFormat{
					{
						Name:     "pytorch",
						Priority: proto.Int32(1),
					},
					{
						Name:     "tensorflow",
						Priority: proto.Int32(2),
					},
				},
			},
			modelName: "pytorch",
			expected:  proto.Int32(1),
		},
		{
			name: "returns nil when model does not exist",
			spec: &ServingRuntimeSpec{
				SupportedModelFormats: []SupportedModelFormat{
					{
						Name:     "pytorch",
						Priority: proto.Int32(1),
					},
				},
			},
			modelName: "onnx",
			expected:  nil,
		},
		{
			name: "returns nil when priority is not set",
			spec: &ServingRuntimeSpec{
				SupportedModelFormats: []SupportedModelFormat{
					{
						Name:     "pytorch",
						Priority: nil,
					},
				},
			},
			modelName: "pytorch",
			expected:  nil,
		},
		{
			name: "returns nil when no supported model formats",
			spec: &ServingRuntimeSpec{
				SupportedModelFormats: []SupportedModelFormat{},
			},
			modelName: "pytorch",
			expected:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.spec.GetPriority(tc.modelName)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

func TestSupportedModelFormat_IsAutoSelectEnabled(t *testing.T) {
	testCases := []struct {
		name     string
		format   *SupportedModelFormat
		expected bool
	}{
		{
			name: "returns true when auto select is explicitly true",
			format: &SupportedModelFormat{
				Name:       "pytorch",
				AutoSelect: proto.Bool(true),
			},
			expected: true,
		},
		{
			name: "returns false when auto select is explicitly false",
			format: &SupportedModelFormat{
				Name:       "pytorch",
				AutoSelect: proto.Bool(false),
			},
			expected: false,
		},
		{
			name: "returns false when auto select is nil",
			format: &SupportedModelFormat{
				Name:       "pytorch",
				AutoSelect: nil,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.format.IsAutoSelectEnabled()
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}

func TestServingRuntimeSpec_SupportsAcceleratorClass(t *testing.T) {
	testCases := []struct {
		name             string
		spec             *ServingRuntimeSpec
		acceleratorClass string
		expected         bool
	}{
		{
			name: "returns true when accelerator class is supported",
			spec: &ServingRuntimeSpec{
				AcceleratorRequirements: &AcceleratorRequirements{
					AcceleratorClasses: []string{"nvidia-a100", "nvidia-h100"},
				},
			},
			acceleratorClass: "nvidia-a100",
			expected:         true,
		},
		{
			name: "returns false when accelerator class is not supported",
			spec: &ServingRuntimeSpec{
				AcceleratorRequirements: &AcceleratorRequirements{
					AcceleratorClasses: []string{"nvidia-a100"},
				},
			},
			acceleratorClass: "nvidia-v100",
			expected:         false,
		},
		{
			name: "returns true when accelerator requirements is nil",
			spec: &ServingRuntimeSpec{
				AcceleratorRequirements: nil,
			},
			acceleratorClass: "nvidia-a100",
			expected:         true,
		},
		{
			name: "returns true when accelerator classes list is empty",
			spec: &ServingRuntimeSpec{
				AcceleratorRequirements: &AcceleratorRequirements{
					AcceleratorClasses: []string{},
				},
			},
			acceleratorClass: "nvidia-a100",
			expected:         true,
		},
		{
			name: "returns true when accelerator classes list is nil",
			spec: &ServingRuntimeSpec{
				AcceleratorRequirements: &AcceleratorRequirements{
					AcceleratorClasses: nil,
				},
			},
			acceleratorClass: "nvidia-a100",
			expected:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			result := tc.spec.SupportsAcceleratorClass(tc.acceleratorClass)
			g.Expect(result).To(gomega.Equal(tc.expected))
		})
	}
}
