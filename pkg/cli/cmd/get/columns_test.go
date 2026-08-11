package get

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestISVCColumns(t *testing.T) {
	e, err := resolve("isvc")
	require.NoError(t, err)
	u, _ := apis.ParseURL("https://llama.team-a.example.com")
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b"},
		Spec: v1beta1.InferenceServiceSpec{
			Model:   &v1beta1.ModelRef{Name: "llama-3-3-70b"},
			Runtime: &v1beta1.ServingRuntimeRef{Name: "srt-llama"},
		},
		Status: v1beta1.InferenceServiceStatus{
			URL: u,
			Status: duckv1.Status{Conditions: duckv1.Conditions{{
				Type: apis.ConditionReady, Status: "True",
			}}},
		},
	}
	got := map[string]string{}
	for _, c := range e.Columns {
		got[c.Name] = c.Extract(isvc)
	}
	assert.Equal(t, "llama-70b", got["NAME"])
	assert.Equal(t, "llama-3-3-70b", got["MODEL"])
	assert.Equal(t, "srt-llama", got["RUNTIME"])
	assert.Equal(t, "True", got["READY"])
	assert.Equal(t, "https://llama.team-a.example.com", got["URL"])
}

func TestISVCColumnsNilRefs(t *testing.T) {
	e, _ := resolve("isvc")
	isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "bare"}}
	for _, c := range e.Columns {
		assert.NotPanics(t, func() { c.Extract(isvc) }, c.Name)
	}
}

func TestModelColumnsMergedScope(t *testing.T) {
	e, err := resolve("models")
	require.NoError(t, err)
	arch, params := "LlamaForCausalLM", "70B"
	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "team-a"},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat:        v1beta1.ModelFormat{Name: "safetensors"},
			ModelArchitecture:  &arch,
			ModelParameterSize: &params,
		},
		Status: v1beta1.ModelStatusSpec{State: v1beta1.LifeCycleStateReady},
	}
	got := map[string]string{}
	for _, c := range e.Columns {
		got[c.Name] = c.Extract(bm)
	}
	assert.Equal(t, "Namespaced", got["SCOPE"])
	assert.Equal(t, "LlamaForCausalLM", got["ARCH"])
	assert.Equal(t, "70B", got["PARAMS"])
	assert.Equal(t, "safetensors", got["FORMAT"])
	assert.Equal(t, "Ready", got["STATE"])

	cbm := &v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "m2"}}
	for _, c := range e.Columns {
		if c.Name == "SCOPE" {
			assert.Equal(t, "Cluster", c.Extract(cbm))
		}
	}
}

// TestModelAndRuntimeColumnsWrongType pins a fix for a nil-pointer-deref
// panic: baseModelColumns'/runtimeColumns' internal spec()/status()
// type-switch helpers used to return a nil pointer for any concrete type
// outside {BaseModel,ClusterBaseModel} / {ServingRuntime,ClusterServingRuntime},
// and several extractors dereferenced that nil unconditionally. Every
// extractor must be nil-safe -- including against an object of the wrong
// resource kind entirely, which should never reach these columns in
// practice but must never crash the CLI if it does.
//
// Every column must survive (NotPanics). Columns that guard on the object's
// *domain* type (a BaseModel/ClusterBaseModel or ServingRuntime/
// ClusterServingRuntime type-switch, directly or via spec()/status()) report
// "?" for a foreign type. AGE and runtimes' NAME instead guard on the
// broader `metav1.Object` interface, which any real Kubernetes object
// satisfies -- including the InferenceService used here, since it embeds
// ObjectMeta -- so that guard defends against non-Kubernetes runtime.Objects
// rather than a wrong domain kind, and legitimately renders the object's
// real (harmless) name / a zero-value age instead of "?".
func TestModelAndRuntimeColumnsWrongType(t *testing.T) {
	wrong := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "not-a-model-or-runtime"}}
	domainTypedColumn := map[string]bool{
		"NAME": true, "SCOPE": true, "ARCH": true, "PARAMS": true,
		"FORMAT": true, "STATE": true, "DISABLED": true, "FORMATS": true,
	}
	for _, resource := range []string{"models", "runtimes"} {
		e, err := resolve(resource)
		require.NoError(t, err, resource)
		for _, c := range e.Columns {
			var got string
			assert.NotPanics(t, func() { got = c.Extract(wrong) }, "%s/%s", resource, c.Name)
			switch {
			case c.Name == "AGE":
				assert.Equal(t, "-", got, "%s/%s", resource, c.Name)
			case resource == "runtimes" && c.Name == "NAME":
				assert.Equal(t, "not-a-model-or-runtime", got, "%s/%s", resource, c.Name)
			case domainTypedColumn[c.Name]:
				assert.Equal(t, "?", got, "%s/%s", resource, c.Name)
			}
		}
	}
}
