package get

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/factory"
	omefake "sigs.k8s.io/ome/pkg/client/clientset/versioned/fake"
)

var update = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func execute(t *testing.T, f factory.Factory, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: &out, ErrOut: &out}
	cmd := NewCmd(f, streams)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func fixtureISVC(name, ns string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: v1beta1.InferenceServiceSpec{
			Model:   &v1beta1.ModelRef{Name: "llama-3-3-70b"},
			Runtime: &v1beta1.ServingRuntimeRef{Name: "srt-llama"},
		},
	}
}

func TestGetISVCTable(t *testing.T) {
	f := factory.Static{
		OME: omefake.NewSimpleClientset(fixtureISVC("a-isvc", "team-a"), fixtureISVC("b-isvc", "team-a")),
		NS:  "team-a",
	}
	out, err := execute(t, f, "isvc")
	require.NoError(t, err)
	assertGolden(t, "isvc_list.golden", []byte(out))
}

func TestGetModelsMerged(t *testing.T) {
	arch := "LlamaForCausalLM"
	f := factory.Static{
		OME: omefake.NewSimpleClientset(
			&v1beta1.BaseModel{ObjectMeta: metav1.ObjectMeta{Name: "ns-model", Namespace: "team-a"},
				Spec: v1beta1.BaseModelSpec{ModelArchitecture: &arch}},
			&v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "cluster-model"},
				Spec: v1beta1.BaseModelSpec{ModelArchitecture: &arch}},
		),
		NS: "team-a",
	}
	out, err := execute(t, f, "models")
	require.NoError(t, err)
	assertGolden(t, "models_merged.golden", []byte(out))
}

func TestGetSingleNameJSON(t *testing.T) {
	f := factory.Static{OME: omefake.NewSimpleClientset(fixtureISVC("a-isvc", "team-a")), NS: "team-a"}
	out, err := execute(t, f, "isvc", "a-isvc", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"name": "a-isvc"`)
}

func TestGetUnknownResourceErr(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "d"}, "banana")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource")
}

func TestGetNotFoundFriendly(t *testing.T) {
	f := factory.Static{OME: omefake.NewSimpleClientset(), NS: "team-a"}
	_, err := execute(t, f, "isvc", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"missing" not found`)
}

// TestGetISVCListJSONEnvelope pins Finding 1: a multi-object -o json listing
// must be one valid document -- a v1 List envelope -- not several bare
// objects printed back-to-back.
func TestGetISVCListJSONEnvelope(t *testing.T) {
	f := factory.Static{
		OME: omefake.NewSimpleClientset(fixtureISVC("a-isvc", "team-a"), fixtureISVC("b-isvc", "team-a")),
		NS:  "team-a",
	}
	out, err := execute(t, f, "isvc", "-o", "json")
	require.NoError(t, err)
	var list struct {
		Kind  string            `json:"kind"`
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &list), "must be a single valid JSON document")
	assert.Equal(t, "List", list.Kind)
	assert.Len(t, list.Items, 2)
}

// TestGetModelsListYAMLEnvelope pins Finding 1 for the YAML path: printing
// each object separately produces a malformed multi-document stream (no ---
// separators); the fix wraps the list in a single List envelope instead.
func TestGetModelsListYAMLEnvelope(t *testing.T) {
	arch := "LlamaForCausalLM"
	f := factory.Static{
		OME: omefake.NewSimpleClientset(
			&v1beta1.BaseModel{ObjectMeta: metav1.ObjectMeta{Name: "ns-model", Namespace: "team-a"},
				Spec: v1beta1.BaseModelSpec{ModelArchitecture: &arch}},
			&v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "cluster-model"},
				Spec: v1beta1.BaseModelSpec{ModelArchitecture: &arch}},
		),
		NS: "team-a",
	}
	out, err := execute(t, f, "models", "-o", "yaml")
	require.NoError(t, err)
	assert.Contains(t, out, "kind: List")
	assert.Contains(t, out, "name: ns-model")
	assert.Contains(t, out, "name: cluster-model")

	var doc struct {
		Kind  string           `json:"kind"`
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(out), &doc), "must be a single valid YAML document")
	assert.Equal(t, "List", doc.Kind)
	assert.Len(t, doc.Items, 2)
}

// TestGetEmptyListJSONEnvelopeHasEmptyItemsArray pins the round-2 fix: an
// empty result set's -o json output must have an empty items ARRAY, never a
// null items field -- naive consumers like `jq '.items[]'` error on null.
func TestGetEmptyListJSONEnvelopeHasEmptyItemsArray(t *testing.T) {
	f := factory.Static{OME: omefake.NewSimpleClientset(), NS: "team-a"}
	out, err := execute(t, f, "workloadclusters", "-o", "json")
	require.NoError(t, err)
	assert.NotContains(t, out, "null", "no field of an empty List envelope should render as null")

	var list struct {
		Kind  string            `json:"kind"`
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &list), "must be a single valid JSON document")
	assert.Equal(t, "List", list.Kind)
	assert.NotNil(t, list.Items, "items must unmarshal as an empty array, not null")
	assert.Len(t, list.Items, 0)
}

// TestGetAllNamespacesWarnsOnClusterScoped pins Finding 2a: -A on a
// cluster-scoped resource is accepted (not an error, kubectl parity) but
// warns instead of silently doing nothing.
func TestGetAllNamespacesWarnsOnClusterScoped(t *testing.T) {
	f := factory.Static{OME: omefake.NewSimpleClientset(), NS: "team-a"}
	out, err := execute(t, f, "acceleratorclasses", "-A")
	require.NoError(t, err)
	assert.Contains(t, out, `warning: --all-namespaces is ignored for the cluster-scoped resource "acceleratorclasses"`)
}

// TestGetEmptyClusterScopedMessageHasNoNamespaceClause pins Finding 2b: the
// empty-list message for a cluster-scoped resource must not claim a
// namespace scope that doesn't apply.
func TestGetEmptyClusterScopedMessageHasNoNamespaceClause(t *testing.T) {
	f := factory.Static{OME: omefake.NewSimpleClientset(), NS: "team-a"}
	out, err := execute(t, f, "workloadclusters")
	require.NoError(t, err)
	assert.Contains(t, out, "No workloadclusters found.\n")
	assert.NotContains(t, out, "namespace")
}

func TestGetInvalidOutputFormat(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "team-a"}, "isvc", "-o", "toml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported: wide, json, yaml")
}

func TestGetNameWithAllNamespacesRejected(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "team-a"}, "isvc", "a-isvc", "-A")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be combined with --all-namespaces")
}

func TestGetNameWithSelectorRejected(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "team-a"}, "isvc", "a-isvc", "-l", "foo=bar")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be combined with --selector")
}

// TestGetSelectorSurvivesPaging pins the pagination helper's contract to
// preserve a caller-supplied --selector: chunking a list through
// Limit/Continue must never silently drop LabelSelector filtering.
func TestGetSelectorSurvivesPaging(t *testing.T) {
	tagged := fixtureISVC("gpu-isvc", "team-a")
	tagged.Labels = map[string]string{"tier": "gpu"}
	untagged := fixtureISVC("cpu-isvc", "team-a")
	f := factory.Static{
		OME: omefake.NewSimpleClientset(tagged, untagged),
		NS:  "team-a",
	}
	out, err := execute(t, f, "isvc", "-l", "tier=gpu")
	require.NoError(t, err)
	assert.Contains(t, out, "gpu-isvc")
	assert.NotContains(t, out, "cpu-isvc")
}
