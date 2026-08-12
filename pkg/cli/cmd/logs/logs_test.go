package logs

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/constants"
)

func pod(name, isvc, component string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "team-a",
		Labels: map[string]string{
			constants.InferenceServiceLabel: isvc,
			constants.OMEComponentLabel:     component,
		},
	}}
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

func TestLogsPrefixesMultiplePods(t *testing.T) {
	f := factory.Static{
		Kube: kubefake.NewSimpleClientset(
			pod("llama-engine-1", "llama", "engine"),
			pod("llama-decoder-1", "llama", "decoder"),
		),
		NS: "team-a",
	}
	out, err := execute(t, f, "llama")
	require.NoError(t, err)
	assert.Contains(t, out, "[engine/llama-engine-1] fake logs")
	assert.Contains(t, out, "[decoder/llama-decoder-1] fake logs")
}

func TestLogsComponentFilterSinglePodNoPrefix(t *testing.T) {
	f := factory.Static{
		Kube: kubefake.NewSimpleClientset(
			pod("llama-engine-1", "llama", "engine"),
			pod("llama-decoder-1", "llama", "decoder"),
		),
		NS: "team-a",
	}
	out, err := execute(t, f, "llama", "-c", "engine")
	require.NoError(t, err)
	assert.Equal(t, "fake logs\n", out)
}

func TestLogsNoPodsIsError(t *testing.T) {
	f := factory.Static{Kube: kubefake.NewSimpleClientset(), NS: "team-a"}
	_, err := execute(t, f, "llama")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

func TestLogsRejectsBadComponent(t *testing.T) {
	_, err := execute(t, factory.Static{NS: "d"}, "llama", "-c", "gpu")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid component")
}
