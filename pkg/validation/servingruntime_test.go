package validation

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestValidateSupportedModelFormats(t *testing.T) {
	tests := []struct {
		name    string
		formats []v1beta1.SupportedModelFormat
		wantErr bool
	}{
		{
			name:    "nil formats",
			formats: nil,
			wantErr: false,
		},
		{
			name:    "empty formats",
			formats: []v1beta1.SupportedModelFormat{},
			wantErr: false,
		},
		{
			name: "valid single format",
			formats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
					ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				},
			},
		},
		{
			name: "missing modelFormat.name",
			formats: []v1beta1.SupportedModelFormat{
				{
					ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing modelFramework.name",
			formats: []v1beta1.SupportedModelFormat{
				{
					ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSupportedModelFormats(tc.formats)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateModelFormatPrioritySame(t *testing.T) {
	tests := []struct {
		name    string
		spec    *v1beta1.ServingRuntimeSpec
		wantErr bool
	}{
		{
			name: "no formats",
			spec: &v1beta1.ServingRuntimeSpec{},
		},
		{
			name: "consistent priority",
			spec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{Name: "fmt1", Priority: ptr.To[int32](1), AutoSelect: ptr.To(true)},
					{Name: "fmt1", Priority: ptr.To[int32](1), AutoSelect: ptr.To(true)},
				},
			},
		},
		{
			name: "different priority for same format name",
			spec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{Name: "fmt1", Priority: ptr.To[int32](1), AutoSelect: ptr.To(true)},
					{Name: "fmt1", Priority: ptr.To[int32](2), AutoSelect: ptr.To(true)},
				},
			},
			wantErr: true,
		},
		{
			name: "different priority but autoselect disabled",
			spec: &v1beta1.ServingRuntimeSpec{
				SupportedModelFormats: []v1beta1.SupportedModelFormat{
					{Name: "fmt1", Priority: ptr.To[int32](1), AutoSelect: ptr.To(true)},
					{Name: "fmt1", Priority: ptr.To[int32](2), AutoSelect: ptr.To(false)},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateModelFormatPrioritySame(tc.spec)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRuntimeAutoscalerPolicyRefs(t *testing.T) {
	ref := &v1beta1.AutoscalerPolicyRef{Name: "request-activity-v1"}
	tests := []struct {
		name      string
		spec      *v1beta1.ServingRuntimeSpec
		wantField string
	}{
		{name: "nil spec", spec: nil},
		{name: "no component configs", spec: &v1beta1.ServingRuntimeSpec{}},
		{
			name: "component configs without refs",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig:  &v1beta1.EngineSpec{},
				DecoderConfig: &v1beta1.DecoderSpec{},
				RouterConfig:  &v1beta1.RouterSpec{},
			},
		},
		{
			name: "engine config ref rejected",
			spec: &v1beta1.ServingRuntimeSpec{
				EngineConfig: &v1beta1.EngineSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
				},
			},
			wantField: "engineConfig",
		},
		{
			name: "decoder config ref rejected",
			spec: &v1beta1.ServingRuntimeSpec{
				DecoderConfig: &v1beta1.DecoderSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
				},
			},
			wantField: "decoderConfig",
		},
		{
			name: "router config ref rejected",
			spec: &v1beta1.ServingRuntimeSpec{
				RouterConfig: &v1beta1.RouterSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{AutoscalerPolicyRef: ref},
				},
			},
			wantField: "routerConfig",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRuntimeAutoscalerPolicyRefs(tc.spec)
			if tc.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Fatalf("error %q does not name field %q", err.Error(), tc.wantField)
			}
			if !strings.Contains(err.Error(), "policy refs attach on the InferenceService only") {
				t.Fatalf("error %q lacks the operator guidance", err.Error())
			}
		})
	}
}
