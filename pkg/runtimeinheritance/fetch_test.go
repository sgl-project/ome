package runtimeinheritance

import (
	"context"
	"errors"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestRuntimeRefsReadTheParentFromTheAnnotation(t *testing.T) {
	g := gomega.NewWithT(t)
	csr := mkClusterRuntime("child", "parent", corev1.EnvVar{Name: "FROM_CHILD", Value: "yes"})

	ref := ClusterRuntimeRef(csr)
	g.Expect(ref.Name).To(gomega.Equal("child"))
	g.Expect(ref.ParentName).To(gomega.Equal("parent"))
	g.Expect(ref.Spec).To(gomega.BeIdenticalTo(&csr.Spec), "the resolver reads the object's own spec, not a copy")

	solo := NamespacedRuntimeRef(mkNamespacedRuntime("team", "solo", ""))
	g.Expect(solo.Name).To(gomega.Equal("solo"))
	g.Expect(solo.ParentName).To(gomega.BeEmpty())
}

func TestClusterRuntimeFetcherReadsClusterScopeOnly(t *testing.T) {
	g := gomega.NewWithT(t)
	cluster := mkClusterRuntime("infra", "cluster-base")
	sameNameInANamespace := mkNamespacedRuntime("team", "infra", "team-base")
	c := fakeclient.New(t, fakeclient.WithObjects(cluster, sameNameInANamespace))

	ref, err := ClusterRuntimeFetcher(c)(context.Background(), "infra")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(ref.ParentName).To(gomega.Equal("cluster-base"))

	_, err = ClusterRuntimeFetcher(c)(context.Background(), "missing")
	g.Expect(errors.Is(err, ErrParentNotFound)).To(gomega.BeTrue())
}

func TestNamespacedRuntimeFetcherPrefersTheSameNamespace(t *testing.T) {
	g := gomega.NewWithT(t)
	cluster := mkClusterRuntime("infra", "cluster-base")
	sameNamespace := mkNamespacedRuntime("team", "infra", "team-base")
	c := fakeclient.New(t, fakeclient.WithObjects(cluster, sameNamespace))

	ref, err := NamespacedRuntimeFetcher(c, "team")(context.Background(), "infra")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(ref.ParentName).To(gomega.Equal("team-base"))
}

func TestNamespacedRuntimeFetcherFallsBackToClusterScope(t *testing.T) {
	g := gomega.NewWithT(t)
	cluster := mkClusterRuntime("infra", "cluster-base")
	c := fakeclient.New(t, fakeclient.WithObjects(cluster))

	ref, err := NamespacedRuntimeFetcher(c, "team")(context.Background(), "infra")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(ref.ParentName).To(gomega.Equal("cluster-base"))
}

func TestNamespacedRuntimeFetcherNeverReadsAnotherNamespace(t *testing.T) {
	g := gomega.NewWithT(t)
	otherNamespace := mkNamespacedRuntime("other-team", "infra", "")
	c := fakeclient.New(t, fakeclient.WithObjects(otherNamespace))

	_, err := NamespacedRuntimeFetcher(c, "team")(context.Background(), "infra")
	g.Expect(errors.Is(err, ErrParentNotFound)).To(gomega.BeTrue())
}

func TestFetchersPassAReadErrorThroughUnchanged(t *testing.T) {
	g := gomega.NewWithT(t)
	readFailed := errors.New("the API server did not answer")
	c := fakeclient.New(t, fakeclient.FailingVerb(fakeclient.Get, fakeclient.EveryCall, readFailed).OnKind(&v1beta1.ClusterServingRuntime{}))

	_, err := ClusterRuntimeFetcher(c)(context.Background(), "infra")
	g.Expect(err).To(gomega.BeIdenticalTo(readFailed))

	_, err = NamespacedRuntimeFetcher(c, "team")(context.Background(), "infra")
	g.Expect(err).To(gomega.BeIdenticalTo(readFailed), "the cluster-scope fallback adds no wrapper of its own")
	g.Expect(errors.Is(err, ErrParentNotFound)).To(gomega.BeFalse())
}
