package version

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/ome/pkg/cli/factory"
)

func run(t *testing.T, f factory.Factory, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: &out, ErrOut: &out}
	cmd := NewCmd(f, streams)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return out.String()
}

func TestOperatorVersionFromDeployment(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ome-controller-manager", Namespace: "ome"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "manager", Image: "ghcr.io/x/ome-manager:v0.9.1"}},
		}}},
	}
	out := run(t, factory.Static{Kube: kubefake.NewSimpleClientset(dep)})
	assert.Contains(t, out, "Client Version:")
	assert.Contains(t, out, "Operator Version: v0.9.1")
}

func TestOperatorVersionUnknownWhenMissing(t *testing.T) {
	out := run(t, factory.Static{Kube: kubefake.NewSimpleClientset()})
	assert.Contains(t, out, "Operator Version: unknown")
}

func TestOMENamespaceFlag(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ome-controller-manager", Namespace: "infra"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Image: "ome-manager:v0.9.2"}},
		}}},
	}
	out := run(t, factory.Static{Kube: kubefake.NewSimpleClientset(dep)}, "--ome-namespace", "infra")
	assert.Contains(t, out, "Operator Version: v0.9.2")
}
