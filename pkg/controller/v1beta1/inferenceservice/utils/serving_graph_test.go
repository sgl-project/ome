package utils

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func TestResolveServingGraphPredictorCompatibility(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				PodSpec: v1beta1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "ome-container",
							Image: "engine:latest",
						},
						{
							Name:  "sidecar",
							Image: "sidecar:latest",
						},
					},
				},
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: servingGraphTestIntPtr(0),
				},
				Model: &v1beta1.ModelSpec{
					BaseModel: servingGraphTestStringPtr("base-model"),
					Runtime:   servingGraphTestStringPtr("runtime-a"),
					PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
						StorageUri:      servingGraphTestStringPtr("oci://bucket/model"),
						ProtocolVersion: servingGraphTestProtocolPtr(constants.OpenInferenceProtocolV2),
					},
				},
			},
		},
	}

	graph, err := ResolveServingGraph(isvc)
	require.NoError(t, err)

	require.NotNil(t, graph.Model)
	assert.True(t, graph.PredictorCompatibility)
	assert.Equal(t, "base-model", graph.Model.Name)
	require.NotNil(t, graph.Runtime)
	assert.Equal(t, "runtime-a", graph.Runtime.Name)
	require.NotNil(t, graph.Engine)
	require.NotNil(t, graph.Engine.Runner)
	assert.Equal(t, "ome-container", graph.Engine.Runner.Name)
	assert.Len(t, graph.Engine.Containers, 1)
	assert.Equal(t, "sidecar", graph.Engine.Containers[0].Name)
	assert.Equal(t, v1beta1.EngineComponent, graph.EntrypointComponent)
	assert.Equal(t, v1beta1.EngineComponent, DetermineEntrypointComponent(isvc))
}

func TestResolveServingGraphWithRuntimeSelectsEntrypointAndModes(t *testing.T) {
	tests := []struct {
		name                   string
		isvc                   *v1beta1.InferenceService
		expectedEntrypoint     v1beta1.ComponentType
		expectedEntrypointMode constants.DeploymentModeType
		expectedEngineMode     constants.DeploymentModeType
		expectedDecoderMode    constants.DeploymentModeType
		expectedRouterMode     constants.DeploymentModeType
	}{
		{
			name: "engine only keeps engine as entrypoint",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: servingGraphTestIntPtr(0),
						},
					},
				},
			},
			expectedEntrypoint:     v1beta1.EngineComponent,
			expectedEntrypointMode: constants.Serverless,
			expectedEngineMode:     constants.Serverless,
			expectedDecoderMode:    constants.RawDeployment,
			expectedRouterMode:     constants.RawDeployment,
		},
		{
			name: "decoder does not replace engine as entrypoint",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: servingGraphTestIntPtr(0),
						},
					},
					Decoder: &v1beta1.DecoderSpec{},
				},
			},
			expectedEntrypoint:     v1beta1.EngineComponent,
			expectedEntrypointMode: constants.RawDeployment,
			expectedEngineMode:     constants.RawDeployment,
			expectedDecoderMode:    constants.RawDeployment,
			expectedRouterMode:     constants.RawDeployment,
		},
		{
			name: "router takes precedence over engine",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: servingGraphTestIntPtr(1),
						},
					},
					Router: &v1beta1.RouterSpec{
						ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
							MinReplicas: servingGraphTestIntPtr(0),
						},
					},
				},
			},
			expectedEntrypoint:     v1beta1.RouterComponent,
			expectedEntrypointMode: constants.Serverless,
			expectedEngineMode:     constants.RawDeployment,
			expectedDecoderMode:    constants.RawDeployment,
			expectedRouterMode:     constants.Serverless,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := ResolveServingGraph(tt.isvc)
			require.NoError(t, err)

			graph, err = ResolveServingGraphWithRuntime(graph, nil, logr.Discard())
			require.NoError(t, err)

			assert.Equal(t, tt.expectedEntrypoint, graph.EntrypointComponent)
			assert.Equal(t, tt.expectedEntrypointMode, graph.EntrypointDeploymentMode)
			assert.Equal(t, tt.expectedEngineMode, graph.EngineDeploymentMode)
			assert.Equal(t, tt.expectedDecoderMode, graph.DecoderDeploymentMode)
			assert.Equal(t, tt.expectedRouterMode, graph.RouterDeploymentMode)
		})
	}
}

func servingGraphTestIntPtr(i int) *int {
	return &i
}

func servingGraphTestStringPtr(s string) *string {
	return &s
}

func servingGraphTestProtocolPtr(p constants.InferenceServiceProtocol) *constants.InferenceServiceProtocol {
	return &p
}
