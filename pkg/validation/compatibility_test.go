package validation

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestCompareModelToFormat_AttributePairs_MatchOnlyWhenEveryAttributeAgrees(t *testing.T) {
	tests := []struct {
		name   string
		model  *v1beta1.BaseModelSpec
		format v1beta1.SupportedModelFormat
		opts   []CompatibilityOption
		want   bool
	}{
		{
			name: "exact match",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			want: true,
		},
		{
			name: "format name mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "pytorch"},
			},
			want: false,
		},
		{
			name: "nil format.ModelFormat",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{},
			want:   false,
		},
		{
			name: "architecture mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("MistralForCausalLM"),
			},
			want: false,
		},
		{
			name: "model has architecture but runtime does not (asymmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "both nil architecture matches",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: true,
		},
		{
			name: "quantization mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationFP8),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:  &v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationINT4),
			},
			want: false,
		},
		{
			name: "framework mismatch",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "tensorrtllm"},
			},
			want: false,
		},
		{
			name: "model has framework but runtime does not (asymmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "format version mismatch (symmetric nil)",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: false,
		},
		{
			name: "format version match",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0")},
			},
			want: true,
		},
		{
			name: "sharded model without cache provider option skips cache check",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Distribution: ptr.To(v1beta1.DistributionSharded),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			want: true,
		},
		{
			name: "sharded model with cache provider option and no support",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Distribution: ptr.To(v1beta1.DistributionSharded),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			opts: []CompatibilityOption{WithModelCacheProvider("alluxio")},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CompareModelToFormat(tc.model, tc.format, tc.opts...)
			if got != tc.want {
				t.Fatalf("CompareModelToFormat() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetFormatMismatchReason_OneAttributeOff_NamesThatAttribute(t *testing.T) {
	tests := []struct {
		name     string
		model    *v1beta1.BaseModelSpec
		format   v1beta1.SupportedModelFormat
		contains string
	}{
		{
			name: "architecture mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:       v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("LlamaForCausalLM"),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors"},
				ModelArchitecture: ptr.To("MistralForCausalLM"),
			},
			contains: "architecture mismatch",
		},
		{
			name: "format name mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat: v1beta1.ModelFormat{Name: "pytorch"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"},
			},
			contains: "format name mismatch",
		},
		{
			name: "quantization mismatch reason",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationFP8),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:  &v1beta1.ModelFormat{Name: "safetensors"},
				Quantization: ptr.To(v1beta1.ModelQuantizationINT4),
			},
			contains: "quantization mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason := GetFormatMismatchReason(tc.model, tc.format)
			if reason == "" {
				t.Fatal("expected non-empty reason")
			}
			if !strings.Contains(reason, tc.contains) {
				t.Fatalf("reason %q does not contain %q", reason, tc.contains)
			}
		})
	}
}

// TestGetFormatMismatchReason_ManyAttributesOff_ListsEachInCompareOrder asserts the reason texts the
// RuntimeCompatibilityAdvisory event and the admission refusal print, byte for byte, in the
// order the attributes are compared.
func TestGetFormatMismatchReason_ManyAttributesOff_ListsEachInCompareOrder(t *testing.T) {
	fullModel := &v1beta1.BaseModelSpec{
		ModelFormat:       v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0.0")},
		ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("4.0.0")},
		ModelArchitecture: ptr.To("ArchA"),
		Quantization:      ptr.To(v1beta1.ModelQuantizationFP8),
	}
	bareModel := &v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: "safetensors"}}

	tests := []struct {
		name           string
		model          *v1beta1.BaseModelSpec
		format         v1beta1.SupportedModelFormat
		opts           []CompatibilityOption
		wantCompatible bool
		wantReason     string
	}{
		{
			name:  "every attribute matches",
			model: fullModel,
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0.0")},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("4.0.0")},
				ModelArchitecture: ptr.To("ArchA"),
				Quantization:      ptr.To(v1beta1.ModelQuantizationFP8),
			},
			wantCompatible: true,
			wantReason:     "unknown mismatch",
		},
		{
			name:  "every attribute set and different; versions are not compared under different names",
			model: fullModel,
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "pytorch", Version: ptr.To("2.0.0")},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "tensorrtllm", Version: ptr.To("5.0.0")},
				ModelArchitecture: ptr.To("ArchB"),
				Quantization:      ptr.To(v1beta1.ModelQuantizationINT4),
			},
			wantReason: "architecture mismatch (model=ArchA, runtime=ArchB), " +
				"quantization mismatch (model=fp8, runtime=int4), " +
				"format name mismatch (model=safetensors, runtime=pytorch), " +
				"framework name mismatch (model=transformers, runtime=tensorrtllm)",
		},
		{
			name:  "the names match and the versions differ",
			model: fullModel,
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("2.0.0")},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("5.0.0")},
				ModelArchitecture: ptr.To("ArchA"),
				Quantization:      ptr.To(v1beta1.ModelQuantizationFP8),
			},
			wantReason: "format version mismatch (model=1.0.0, runtime=2.0.0), " +
				"framework version mismatch (model=4.0.0, runtime=5.0.0)",
		},
		{
			name:   "the runtime leaves every attribute unset",
			model:  fullModel,
			format: v1beta1.SupportedModelFormat{ModelFormat: &v1beta1.ModelFormat{Name: "safetensors"}},
			wantReason: "model has architecture ArchA but runtime has no architecture requirement, " +
				"model has quantization fp8 but runtime has no quantization requirement, " +
				"model has format version but runtime has no version requirement, " +
				"model has framework transformers but runtime has no framework requirement",
		},
		{
			name:  "the model leaves every attribute unset",
			model: bareModel,
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "safetensors", Version: ptr.To("1.0.0")},
				ModelFramework:    &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("4.0.0")},
				ModelArchitecture: ptr.To("ArchB"),
				Quantization:      ptr.To(v1beta1.ModelQuantizationINT4),
			},
			wantReason: "model has no architecture but runtime requires ArchB, " +
				"model has no quantization but runtime requires int4, " +
				"model has no format version but runtime requires 1.0.0, " +
				"model has no framework but runtime requires transformers",
		},
		{
			name: "only the runtime sets the framework version",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("4.0.0")},
			},
			wantReason: "model has no framework version but runtime requires 4.0.0",
		},
		{
			name: "only the model sets the framework version",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:    v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: ptr.To("4.0.0")},
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
				ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
			},
			wantReason: "model has framework version but runtime has no version requirement",
		},
		{
			name:       "a format entry that names no model format fits no model and names no reason",
			model:      bareModel,
			format:     v1beta1.SupportedModelFormat{ModelArchitecture: ptr.To("ArchA")},
			wantReason: "model has no architecture but runtime requires ArchA",
		},
		{
			name:       "a format entry with nothing set reads unknown mismatch",
			model:      bareModel,
			format:     v1beta1.SupportedModelFormat{},
			wantReason: "unknown mismatch",
		},
		{
			name: "the cache provider and the pipeline are named first",
			model: &v1beta1.BaseModelSpec{
				ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
				Distribution: ptr.To(v1beta1.DistributionSharded),
			},
			format: v1beta1.SupportedModelFormat{
				ModelFormat:       &v1beta1.ModelFormat{Name: "pytorch"},
				DiffusionPipeline: &v1beta1.DiffusionPipelineSpec{ClassName: ptr.To("Pipeline")},
			},
			opts: []CompatibilityOption{WithModelCacheProvider("cache-a")},
			wantReason: `runtime format does not support model cache provider "cache-a", ` +
				"diffusion pipeline required by runtime but not specified in model, " +
				"format name mismatch (model=safetensors, runtime=pytorch)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareModelToFormat(tc.model, tc.format, tc.opts...); got != tc.wantCompatible {
				t.Fatalf("CompareModelToFormat() = %v, want %v", got, tc.wantCompatible)
			}
			if got := GetFormatMismatchReason(tc.model, tc.format, tc.opts...); got != tc.wantReason {
				t.Fatalf("GetFormatMismatchReason() = %q, want %q", got, tc.wantReason)
			}
		})
	}
}

// TestWithModelCacheProvider_EmptyName_FitsNoCacheLoadedModel: without the option the cache
// check is skipped; with an empty name, as a cluster that runs no provider passes, a model that
// loads through an external model cache fits nothing.
func TestWithModelCacheProvider_EmptyName_FitsNoCacheLoadedModel(t *testing.T) {
	model := &v1beta1.BaseModelSpec{
		ModelFormat:  v1beta1.ModelFormat{Name: "safetensors"},
		Distribution: ptr.To(v1beta1.DistributionSharded),
	}
	format := v1beta1.SupportedModelFormat{
		ModelFormat:         &v1beta1.ModelFormat{Name: "safetensors"},
		ModelCacheProviders: []v1beta1.ModelCacheProvider{v1beta1.DragonFly},
	}

	if !CompareModelToFormat(model, format) {
		t.Fatal("without the option the model cache check must be skipped")
	}
	if CompareModelToFormat(model, format, WithModelCacheProvider("")) {
		t.Fatal("an empty provider name must fit no model that needs a model cache provider")
	}
	// The reason is the provider rule's own text for a cluster that runs no provider.
	want := ModelCacheProviderMismatchReason("", format)
	if got := GetFormatMismatchReason(model, format, WithModelCacheProvider("")); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if !CompareModelToFormat(model, format, WithModelCacheProvider(string(v1beta1.DragonFly))) {
		t.Fatal("the listed provider must fit")
	}
	want = `runtime format supports model cache providers [model_cache], not "other"`
	if got := GetFormatMismatchReason(model, format, WithModelCacheProvider("other")); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}

func TestVersionsMatch_EachOperator_OrdersOnlyComparableVersions(t *testing.T) {
	operator := func(value v1beta1.RuntimeSelectorOperator) *v1beta1.RuntimeSelectorOperator { return &value }
	tests := []struct {
		name                           string
		modelVersion, supportedVersion string
		operator                       *v1beta1.RuntimeSelectorOperator
		want                           bool
	}{
		{name: "equal versions under Equal", modelVersion: "1.0.0", supportedVersion: "1.0.0", operator: operator(v1beta1.RuntimeSelectorOpEqual), want: true},
		{name: "equal versions under GreaterThan", modelVersion: "1.0.0", supportedVersion: "1.0.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: false},
		{name: "GreaterThan with the supported version greater", modelVersion: "1.7.0", supportedVersion: "1.8.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: true},
		{name: "GreaterThan with the supported version smaller", modelVersion: "1.8.0", supportedVersion: "1.7.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: false},
		{name: "GreaterThanOrEqual with equal versions", modelVersion: "1.8.0", supportedVersion: "1.8.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThanOrEqual), want: true},
		{name: "GreaterThanOrEqual with the supported version greater", modelVersion: "1.8.0", supportedVersion: "1.9.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThanOrEqual), want: true},
		{name: "GreaterThanOrEqual with the supported version smaller", modelVersion: "1.8.0", supportedVersion: "1.7.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThanOrEqual), want: false},
		{name: "an unofficial version must be equal whatever the operator", modelVersion: "1.8.0-dev", supportedVersion: "1.8.0-dev", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: true},
		{name: "two different unofficial versions never match", modelVersion: "1.8.0-alpha", supportedVersion: "1.8.0-dev", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: false},
		{name: "one-part versions under the nil operator", modelVersion: "1", supportedVersion: "1", want: true},
		{name: "different precision never orders", modelVersion: "1.0.0", supportedVersion: "1.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: false},
		{name: "different major prefix never orders", modelVersion: "1.0.0", supportedVersion: "v1.0.0", operator: operator(v1beta1.RuntimeSelectorOpGreaterThan), want: false},
		{name: "the nil operator means Equal", modelVersion: "v2.0.0", supportedVersion: "v2.0.0", want: true},
		{name: "a version that does not parse never matches", modelVersion: "not a version", supportedVersion: "not a version", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := VersionsMatch(tc.modelVersion, tc.supportedVersion, tc.operator); got != tc.want {
				t.Fatalf("VersionsMatch(%q, %q) = %v, want %v", tc.modelVersion, tc.supportedVersion, got, tc.want)
			}
		})
	}
}
