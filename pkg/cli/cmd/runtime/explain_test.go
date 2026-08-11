package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/factory"
	omefake "sigs.k8s.io/ome/pkg/client/clientset/versioned/fake"
)

func scheme(t *testing.T) *k8sruntime.Scheme {
	t.Helper()
	s := k8sruntime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(v1beta1.AddToScheme(s))
	return s
}

func ptr[T any](v T) *T { return &v }

func execute(t *testing.T, f factory.Factory, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: &out, ErrOut: &out}
	cmd := NewCmd(f, streams)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// row returns the whitespace-split fields of the output line whose first
// column is name, so tests can assert specific columns (e.g. COMPATIBLE)
// instead of substring-searching the whole table.
func row(t *testing.T, out, name string) []string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return fields
		}
	}
	t.Fatalf("no output row for %q in:\n%s", name, out)
	return nil
}

// autoSelectFormat builds a SupportedModelFormat that compareSupportedModelFormats
// (pkg/runtimeselector/matcher.go) will match against a BaseModel with the
// given ModelFormat.Name, and that CalculateScore will treat as auto-select
// eligible -- both required for the runtime to appear in GetCompatibleRuntimes'
// output at all. See pkg/runtimeselector/selector_test.go for the reference
// fixture shape.
func autoSelectFormat(format string) v1beta1.SupportedModelFormat {
	return v1beta1.SupportedModelFormat{
		ModelFormat: &v1beta1.ModelFormat{Name: format},
		AutoSelect:  ptr(true),
	}
}

func TestExplainRequiresExactlyOneTarget(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "d"}, "explain")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one of --model or --isvc")

	_, err = execute(t, factory.Static{NS: "d"}, "explain", "--model", "m", "--isvc", "s")
	require.Error(t, err)
}

func TestExplainRanksRuntimes(t *testing.T) {
	format := "safetensors"
	size := "70B"
	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b"},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat:        v1beta1.ModelFormat{Name: format},
			ModelParameterSize: &size,
		},
	}
	compatible := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "srt-llama"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{autoSelectFormat(format)},
		},
	}
	ctrl := ctrlfake.NewClientBuilder().WithScheme(scheme(t)).
		WithObjects(model, compatible).Build()
	f := factory.Static{
		OME:     omefake.NewSimpleClientset(model),
		Runtime: ctrl,
		NS:      "team-a",
	}
	out, err := execute(t, f, "explain", "--model", "llama-70b")
	require.NoError(t, err)
	assert.Contains(t, out, "RUNTIME")

	got := row(t, out, "srt-llama")
	require.Len(t, got, 6, "row: %v", got)
	assert.Equal(t, "Cluster", got[1])
	assert.Equal(t, "Yes", got[2], "srt-llama supports the model's format and has autoSelect enabled, so GetCompatibleRuntimes must return it")
}

// TestExplainListsIncompatibleRuntimeWithReason pins down the semantics
// documented in explain.go: GetCompatibleRuntimes only ever returns matches
// (see pkg/runtimeselector/selector.go evaluateRuntime), so a runtime it
// silently drops must still show up here, marked incompatible, with a reason.
func TestExplainListsIncompatibleRuntimeWithReason(t *testing.T) {
	format := "safetensors"
	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b"},
		Spec:       v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: format}},
	}
	compatible := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "srt-llama"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{autoSelectFormat(format)},
		},
	}
	// srt-onnx supports a different format entirely: GetCompatibleRuntimes
	// drops it silently, but explain must still list it as incompatible.
	incompatible := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "srt-onnx"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{autoSelectFormat("onnx")},
		},
	}
	ctrl := ctrlfake.NewClientBuilder().WithScheme(scheme(t)).
		WithObjects(model, compatible, incompatible).Build()
	f := factory.Static{
		OME:     omefake.NewSimpleClientset(model),
		Runtime: ctrl,
		NS:      "team-a",
	}
	out, err := execute(t, f, "explain", "--model", "llama-70b")
	require.NoError(t, err)

	rejected := row(t, out, "srt-onnx")
	assert.Equal(t, "No", rejected[2])
	assert.Contains(t, out, "not in supported formats", "rejection reason should explain the format mismatch")

	accepted := row(t, out, "srt-llama")
	assert.Equal(t, "Yes", accepted[2])
}

// TestExplainPrefersNamespacedBaseModel exercises resolveTarget's precedence:
// a namespaced BaseModel must win over a ClusterBaseModel of the same name.
func TestExplainPrefersNamespacedBaseModel(t *testing.T) {
	format := "safetensors"
	nsModel := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b", Namespace: "team-a"},
		Spec:       v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: format}},
	}
	// Same name, different (unsupported-by-any-runtime) format: if
	// resolveTarget picked this one by mistake, srt-llama would never match.
	clusterModel := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b"},
		Spec:       v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: "onnx"}},
	}
	rt := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "srt-llama"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{autoSelectFormat(format)},
		},
	}
	ctrl := ctrlfake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(rt).Build()
	f := factory.Static{
		OME:     omefake.NewSimpleClientset(nsModel, clusterModel),
		Runtime: ctrl,
		NS:      "team-a",
	}
	out, err := execute(t, f, "explain", "--model", "llama-70b")
	require.NoError(t, err)
	got := row(t, out, "srt-llama")
	assert.Equal(t, "Yes", got[2])
}

// TestExplainISVCPath covers --isvc resolution (model looked up via
// spec.model) and the spec.runtime pin note.
func TestExplainISVCPath(t *testing.T) {
	format := "safetensors"
	model := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b"},
		Spec:       v1beta1.BaseModelSpec{ModelFormat: v1beta1.ModelFormat{Name: format}},
	}
	rt := &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "srt-llama"},
		Spec: v1beta1.ServingRuntimeSpec{
			SupportedModelFormats: []v1beta1.SupportedModelFormat{autoSelectFormat(format)},
		},
	}
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc", Namespace: "team-a"},
		Spec: v1beta1.InferenceServiceSpec{
			Model:   &v1beta1.ModelRef{Name: "llama-70b"},
			Runtime: &v1beta1.ServingRuntimeRef{Name: "srt-llama"},
		},
	}
	ctrl := ctrlfake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(rt).Build()
	f := factory.Static{
		OME:     omefake.NewSimpleClientset(model, isvc),
		Runtime: ctrl,
		NS:      "team-a",
	}
	out, err := execute(t, f, "explain", "--isvc", "my-isvc")
	require.NoError(t, err)
	got := row(t, out, "srt-llama")
	assert.Equal(t, "Yes", got[2])
	assert.Contains(t, out, `pins spec.runtime="srt-llama"`)
}

func TestExplainISVCWithoutModelErrors(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "my-isvc", Namespace: "team-a"},
	}
	f := factory.Static{
		OME: omefake.NewSimpleClientset(isvc),
		NS:  "team-a",
	}
	_, err := execute(t, f, "explain", "--isvc", "my-isvc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no spec.model")
}

func TestExplainModelNotFound(t *testing.T) {
	f := factory.Static{
		OME: omefake.NewSimpleClientset(),
		NS:  "team-a",
	}
	_, err := execute(t, f, "explain", "--model", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
