package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestDetermineDeploymentModes_PerComponentAnnotations(t *testing.T) {
	omeAnnot := map[string]string{
		constants.DeploymentMode: string(constants.OMENative),
	}
	rawAnnot := map[string]string{
		constants.DeploymentMode: string(constants.RawDeployment),
	}

	tests := []struct {
		name           string
		engine         *v1beta1.EngineSpec
		decoder        *v1beta1.DecoderSpec
		router         *v1beta1.RouterSpec
		wantEngineMode constants.DeploymentModeType
		wantDecoder    constants.DeploymentModeType
		wantRouter     constants.DeploymentModeType
	}{
		{
			name: "Engine-only OMENative; absent Decoder/Router default to Raw",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "Decoder annotation = OMENative is respected (foundation for PD)",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "Router annotation = OMENative is respected",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			router: &v1beta1.RouterSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.OMENative,
		},
		{
			name: "Three-way PD: Engine + Decoder + Router all opt into OMENative",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			router: &v1beta1.RouterSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.OMENative,
		},
		{
			name: "Mixed: Engine OMENative, Decoder Raw (explicit)",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: rawAnnot},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "Decoder with Leader/Worker but no annotation falls back to OMENative",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: rawAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			wantEngineMode: constants.RawDeployment,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "Decoder annotation takes precedence over Leader/Worker presence",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
				Leader:                 &v1beta1.LeaderSpec{},
				Worker:                 &v1beta1.WorkerSpec{},
			},
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.RawDeployment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineMode, decoderMode, routerMode, err := DetermineDeploymentModes(tt.engine, tt.decoder, tt.router, nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantEngineMode, engineMode, "engine mode mismatch")
			assert.Equal(t, tt.wantDecoder, decoderMode, "decoder mode mismatch")
			assert.Equal(t, tt.wantRouter, routerMode, "router mode mismatch")
		})
	}
}

func TestDetermineDeploymentModes_NilEngineRejected(t *testing.T) {
	_, _, _, err := DetermineDeploymentModes(nil, nil, nil, nil, nil)
	assert.Error(t, err)
}

// TestDetermineDeploymentModes_SpecDeploymentModeField covers the
// precedence chain that includes the new typed spec.deploymentMode
// field: per-Component annotation > spec.deploymentMode > Leader/Worker
// shape (OMENative) > RawDeployment default.
func TestDetermineDeploymentModes_SpecDeploymentModeField(t *testing.T) {
	omeNativePtr := func() *constants.DeploymentModeType {
		m := constants.OMENative
		return &m
	}
	rawDeploymentPtr := func() *constants.DeploymentModeType {
		m := constants.RawDeployment
		return &m
	}
	omeAnnot := map[string]string{
		constants.DeploymentMode: string(constants.OMENative),
	}
	rawAnnot := map[string]string{
		constants.DeploymentMode: string(constants.RawDeployment),
	}

	tests := []struct {
		name           string
		engine         *v1beta1.EngineSpec
		decoder        *v1beta1.DecoderSpec
		router         *v1beta1.RouterSpec
		specMode       *constants.DeploymentModeType
		wantEngineMode constants.DeploymentModeType
		wantDecoder    constants.DeploymentModeType
		wantRouter     constants.DeploymentModeType
	}{
		{
			name:           "spec.deploymentMode=OMENative propagates to bare Engine",
			engine:         &v1beta1.EngineSpec{},
			specMode:       omeNativePtr(),
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name:           "spec.deploymentMode=OMENative propagates to PD shape (Engine + Decoder)",
			engine:         &v1beta1.EngineSpec{},
			decoder:        &v1beta1.DecoderSpec{},
			specMode:       omeNativePtr(),
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.RawDeployment,
		},
		{
			name:           "spec.deploymentMode=OMENative propagates to all three Components",
			engine:         &v1beta1.EngineSpec{},
			decoder:        &v1beta1.DecoderSpec{},
			router:         &v1beta1.RouterSpec{},
			specMode:       omeNativePtr(),
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.OMENative,
			wantRouter:     constants.OMENative,
		},
		{
			name: "per-Component annotation overrides spec.deploymentMode (Decoder pinned Raw)",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeAnnot},
			},
			decoder: &v1beta1.DecoderSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: rawAnnot},
			},
			specMode:       omeNativePtr(),
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "spec.deploymentMode wins over Leader/Worker shape",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			specMode:       omeNativePtr(),
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name: "spec.deploymentMode=RawDeployment overrides Leader/Worker shape (operator opt-out)",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			specMode:       rawDeploymentPtr(),
			wantEngineMode: constants.RawDeployment,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
		{
			name:           "nil spec.deploymentMode → Leader/Worker shape still wins (OMENative)",
			engine:         &v1beta1.EngineSpec{Leader: &v1beta1.LeaderSpec{}},
			specMode:       nil,
			wantEngineMode: constants.OMENative,
			wantDecoder:    constants.RawDeployment,
			wantRouter:     constants.RawDeployment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engineMode, decoderMode, routerMode, err := DetermineDeploymentModes(tt.engine, tt.decoder, tt.router, nil, tt.specMode)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantEngineMode, engineMode, "engine mode mismatch")
			assert.Equal(t, tt.wantDecoder, decoderMode, "decoder mode mismatch")
			assert.Equal(t, tt.wantRouter, routerMode, "router mode mismatch")
		})
	}
}

func TestIsMultiPodComponent(t *testing.T) {
	ptrInt := func(i int) *int { return &i }
	tests := []struct {
		name      string
		isvc      *v1beta1.InferenceService
		component v1beta1.ComponentType
		want      bool
	}{
		{
			name:      "nil ISVC → false",
			isvc:      nil,
			component: v1beta1.EngineComponent,
			want:      false,
		},
		{
			name: "Engine without Worker → single-pod",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}},
			},
			component: v1beta1.EngineComponent,
			want:      false,
		},
		{
			name: "Engine.Worker present but Size nil → single-pod (defaulter not yet run)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{
					Worker: &v1beta1.WorkerSpec{},
				}},
			},
			component: v1beta1.EngineComponent,
			want:      false,
		},
		{
			name: "Engine.Worker.Size = 0 → single-pod (validator should reject before reconcile)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(0)},
				}},
			},
			component: v1beta1.EngineComponent,
			want:      false,
		},
		{
			name: "Engine.Worker.Size = 1 → multi-pod (Leader + 1 worker = 2 pods)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(1)},
				}},
			},
			component: v1beta1.EngineComponent,
			want:      true,
		},
		{
			name: "Engine.Worker.Size = 2 → multi-pod (3 pods)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
				}},
			},
			component: v1beta1.EngineComponent,
			want:      true,
		},
		{
			name: "Decoder.Worker.Size = 3 → multi-pod decoder",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Decoder: &v1beta1.DecoderSpec{
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(3)},
				}},
			},
			component: v1beta1.DecoderComponent,
			want:      true,
		},
		{
			name: "Router is always single-pod regardless of Engine.Worker.Size",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Engine: &v1beta1.EngineSpec{Worker: &v1beta1.WorkerSpec{Size: ptrInt(4)}},
					Router: &v1beta1.RouterSpec{},
				},
			},
			component: v1beta1.RouterComponent,
			want:      false,
		},
		{
			name: "Decoder absent → false (component asked is Decoder)",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
				}},
			},
			component: v1beta1.DecoderComponent,
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsMultiPodComponent(tt.isvc, tt.component))
		})
	}
}
