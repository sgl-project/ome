package utils

import (
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func TestShouldInferServerless(t *testing.T) {
	tests := []struct {
		name           string
		minReplicas    *int
		globalMode     constants.DeploymentModeType
		expectedResult bool
	}{
		{
			name:           "minReplicas=0 with empty global mode returns true",
			minReplicas:    intPtr(0),
			globalMode:     "",
			expectedResult: true,
		},
		{
			name:           "minReplicas=0 with Serverless global mode returns true",
			minReplicas:    intPtr(0),
			globalMode:     constants.Serverless,
			expectedResult: true,
		},
		{
			name:           "minReplicas=0 with RawDeployment global mode returns false",
			minReplicas:    intPtr(0),
			globalMode:     constants.RawDeployment,
			expectedResult: false,
		},
		{
			name:           "minReplicas=1 with empty global mode returns false",
			minReplicas:    intPtr(1),
			globalMode:     "",
			expectedResult: false,
		},
		{
			name:           "nil minReplicas with empty global mode returns false",
			minReplicas:    nil,
			globalMode:     "",
			expectedResult: false,
		},
		{
			name:           "nil minReplicas with RawDeployment global mode returns false",
			minReplicas:    nil,
			globalMode:     constants.RawDeployment,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldInferServerless(tt.minReplicas, tt.globalMode)
			if result != tt.expectedResult {
				t.Errorf("shouldInferServerless(%v, %s) = %v, expected %v",
					tt.minReplicas, tt.globalMode, result, tt.expectedResult)
			}
		})
	}
}

func TestDetermineDeploymentModes_RespectsGlobalMode(t *testing.T) {
	tests := []struct {
		name               string
		engineSpec         *v1beta1.EngineSpec
		routerSpec         *v1beta1.RouterSpec
		globalMode         constants.DeploymentModeType
		expectedEngineMode constants.DeploymentModeType
		expectedRouterMode constants.DeploymentModeType
	}{
		{
			name: "minReplicas=0 triggers Serverless when global mode is empty",
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			routerSpec:         nil,
			globalMode:         "", // No global mode set
			expectedEngineMode: constants.Serverless,
			expectedRouterMode: constants.RawDeployment,
		},
		{
			name: "minReplicas=0 triggers Serverless when global mode is Serverless",
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			routerSpec:         nil,
			globalMode:         constants.Serverless,
			expectedEngineMode: constants.Serverless,
			expectedRouterMode: constants.RawDeployment,
		},
		{
			name: "minReplicas=0 stays RawDeployment when global mode is RawDeployment",
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			routerSpec:         nil,
			globalMode:         constants.RawDeployment,
			expectedEngineMode: constants.RawDeployment,
			expectedRouterMode: constants.RawDeployment,
		},
		{
			name: "Router minReplicas=0 stays RawDeployment when global mode is RawDeployment",
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			routerSpec: &v1beta1.RouterSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			globalMode:         constants.RawDeployment,
			expectedEngineMode: constants.RawDeployment,
			expectedRouterMode: constants.RawDeployment,
		},
		{
			name: "Router minReplicas=0 triggers Serverless when global mode is empty",
			engineSpec: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			routerSpec: &v1beta1.RouterSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			globalMode:         "",
			expectedEngineMode: constants.RawDeployment,
			expectedRouterMode: constants.Serverless,
		},
		{
			name: "MultiNode mode is not affected by global RawDeployment",
			engineSpec: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			routerSpec:         nil,
			globalMode:         constants.RawDeployment,
			expectedEngineMode: constants.MultiNode,
			expectedRouterMode: constants.RawDeployment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineMode, _, routerMode, err := DetermineDeploymentModes(
				tt.engineSpec,
				nil, // decoder
				tt.routerSpec,
				nil, // runtime
				tt.globalMode,
			)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if engineMode != tt.expectedEngineMode {
				t.Errorf("Engine mode: expected %s, got %s", tt.expectedEngineMode, engineMode)
			}

			if routerMode != tt.expectedRouterMode {
				t.Errorf("Router mode: expected %s, got %s", tt.expectedRouterMode, routerMode)
			}
		})
	}
}
