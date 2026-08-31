package validation

import (
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
