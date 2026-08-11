package get

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"

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
