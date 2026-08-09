package runtimeinheritance

import (
	"context"
	"errors"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return s
}

func mkClusterRuntime(name, parent string, envs ...corev1.EnvVar) *v1beta1.ClusterServingRuntime {
	annotations := map[string]string{}
	if parent != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = parent
	}
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
		Spec: v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
				Containers: []corev1.Container{{Name: "ome-container", Env: envs}},
			},
		},
	}
}

func mkNamespacedRuntime(ns, name, parent string, envs ...corev1.EnvVar) *v1beta1.ServingRuntime {
	annotations := map[string]string{}
	if parent != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = parent
	}
	return &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Annotations: annotations},
		Spec: v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
				Containers: []corev1.Container{{Name: "ome-container", Env: envs}},
			},
		},
	}
}

func TestResolveClusterRuntime_TwoLevelMerge(t *testing.T) {
	g := gomega.NewWithT(t)
	profile := mkClusterRuntime("profile", "", corev1.EnvVar{Name: "FROM_PROFILE", Value: "yes"})
	child := mkClusterRuntime("rt", "profile", corev1.EnvVar{Name: "FROM_CHILD", Value: "yes"})
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(profile, child).Build()

	spec, chain, err := ResolveClusterRuntime(context.Background(), c, "rt")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"profile", "rt"}))
	envByName := map[string]string{}
	for _, e := range spec.Containers[0].Env {
		envByName[e.Name] = e.Value
	}
	g.Expect(envByName).To(gomega.HaveKeyWithValue("FROM_PROFILE", "yes"))
	g.Expect(envByName).To(gomega.HaveKeyWithValue("FROM_CHILD", "yes"))
}

func TestResolveClusterRuntime_RuntimeMissing(t *testing.T) {
	g := gomega.NewWithT(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	_, _, err := ResolveClusterRuntime(context.Background(), c, "ghost")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(errors.Is(err, ErrParentNotFound)).To(gomega.BeTrue())
}

func TestResolveClusterRuntime_ParentMissingReturnsTypedError(t *testing.T) {
	g := gomega.NewWithT(t)
	child := mkClusterRuntime("rt", "ghost-parent")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(child).Build()

	_, _, err := ResolveClusterRuntime(context.Background(), c, "rt")
	g.Expect(err).To(gomega.HaveOccurred())
	var pnf *ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeTrue())
	g.Expect(pnf.Parent).To(gomega.Equal("ghost-parent"))
}

func TestResolveNamespacedRuntime_SameNamespaceParent(t *testing.T) {
	g := gomega.NewWithT(t)
	parent := mkNamespacedRuntime("team", "ns-parent", "", corev1.EnvVar{Name: "FROM_NS_PARENT", Value: "y"})
	child := mkNamespacedRuntime("team", "rt", "ns-parent", corev1.EnvVar{Name: "FROM_CHILD", Value: "y"})
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(parent, child).Build()

	spec, chain, err := ResolveNamespacedRuntime(context.Background(), c, "team", "rt")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"ns-parent", "rt"}))
	g.Expect(spec.Containers[0].Env).To(gomega.HaveLen(2))
}

func TestResolveNamespacedRuntime_FallsBackToCluster(t *testing.T) {
	g := gomega.NewWithT(t)
	clusterParent := mkClusterRuntime("infra", "", corev1.EnvVar{Name: "FROM_CLUSTER", Value: "y"})
	child := mkNamespacedRuntime("team", "rt", "infra")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(clusterParent, child).Build()

	spec, chain, err := ResolveNamespacedRuntime(context.Background(), c, "team", "rt")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"infra", "rt"}))
	g.Expect(spec.Containers[0].Env[0].Name).To(gomega.Equal("FROM_CLUSTER"))
}

func TestResolveNamespacedRuntime_NamespacedShadowsCluster(t *testing.T) {
	g := gomega.NewWithT(t)
	clusterParent := mkClusterRuntime("infra", "", corev1.EnvVar{Name: "FROM_CLUSTER", Value: "y"})
	nsParent := mkNamespacedRuntime("team", "infra", "", corev1.EnvVar{Name: "FROM_NS", Value: "y"})
	child := mkNamespacedRuntime("team", "rt", "infra")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(clusterParent, nsParent, child).Build()

	spec, _, err := ResolveNamespacedRuntime(context.Background(), c, "team", "rt")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(spec.Containers[0].Env[0].Name).To(gomega.Equal("FROM_NS"),
		"namespaced parent must win when both scopes carry the same name")
}

func TestResolveNamespacedRuntime_CrossNamespaceRejected(t *testing.T) {
	g := gomega.NewWithT(t)
	// Parent in different namespace; not visible from "team".
	alien := mkNamespacedRuntime("other-team", "shared", "")
	child := mkNamespacedRuntime("team", "rt", "shared")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(alien, child).Build()

	_, _, err := ResolveNamespacedRuntime(context.Background(), c, "team", "rt")
	g.Expect(err).To(gomega.HaveOccurred())
	var pnf *ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeTrue())
}
