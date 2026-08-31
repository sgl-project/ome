package servingruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return s
}

func mkProfile(name string, disabled bool) *v1beta1.ClusterServingRuntime {
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{constants.RuntimeProfileAnnotationKey: "true"},
		},
		Spec: v1beta1.ServingRuntimeSpec{Disabled: ptr.To(disabled)},
	}
}

func mkCSR(name, inheritFrom string) *v1beta1.ClusterServingRuntime {
	annotations := map[string]string{}
	if inheritFrom != "" {
		annotations[constants.RuntimeInheritFromAnnotationKey] = inheritFrom
	}
	return &v1beta1.ClusterServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
	}
}

func TestValidateProfileMarker(t *testing.T) {
	g := gomega.NewWithT(t)

	// No marker → always OK.
	g.Expect(validateProfileMarker(nil, nil)).To(gomega.Succeed())
	g.Expect(validateProfileMarker(map[string]string{}, ptr.To(false))).To(gomega.Succeed())

	// Marker set + disabled=true → OK.
	g.Expect(validateProfileMarker(
		map[string]string{constants.RuntimeProfileAnnotationKey: "true"},
		ptr.To(true),
	)).To(gomega.Succeed())

	// Marker set + disabled=nil → reject.
	err := validateProfileMarker(map[string]string{constants.RuntimeProfileAnnotationKey: "true"}, nil)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("disabled: true"))

	// Marker set + disabled=false → reject.
	err = validateProfileMarker(
		map[string]string{constants.RuntimeProfileAnnotationKey: "true"},
		ptr.To(false),
	)
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestValidateClusterRuntimeInheritance_NoAnnotation(t *testing.T) {
	g := gomega.NewWithT(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	csr := mkCSR("solo", "")
	g.Expect(validateClusterRuntimeInheritance(context.Background(), c, csr)).To(gomega.Succeed())
}

func TestValidateClusterRuntimeInheritance_ParentExists(t *testing.T) {
	g := gomega.NewWithT(t)
	profile := mkProfile("profile-infra", true)
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(profile).Build()

	csr := mkCSR("rt", "profile-infra")
	g.Expect(validateClusterRuntimeInheritance(context.Background(), c, csr)).To(gomega.Succeed())
}

func TestValidateClusterRuntimeInheritance_ParentMissing(t *testing.T) {
	g := gomega.NewWithT(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).Build()

	csr := mkCSR("rt", "ghost")
	err := validateClusterRuntimeInheritance(context.Background(), c, csr)
	g.Expect(err).To(gomega.HaveOccurred())
	var pnf *runtimeinheritance.ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeTrue())
	g.Expect(pnf.Parent).To(gomega.Equal("ghost"))
}

func TestValidateClusterRuntimeInheritance_Cycle(t *testing.T) {
	g := gomega.NewWithT(t)
	// b → a; we then create a → b which forms the cycle.
	b := mkCSR("b", "a")
	a := mkCSR("a", "b")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(a, b).Build()

	err := validateClusterRuntimeInheritance(context.Background(), c, a)
	g.Expect(err).To(gomega.HaveOccurred())
	var ce *runtimeinheritance.CycleError
	g.Expect(errors.As(err, &ce)).To(gomega.BeTrue())
}

func TestValidateClusterRuntimeInheritance_DepthExceeded(t *testing.T) {
	g := gomega.NewWithT(t)
	// 6-deep chain: n0 (root) ← n1 ← … ← n5; test the runtime that points to n5 (overall depth = 7).
	objects := []client.Object{}
	prev := ""
	for i := 0; i < 6; i++ {
		name := "n" + string(rune('0'+i))
		objects = append(objects, mkCSR(name, prev))
		prev = name
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objects...).Build()

	leaf := mkCSR("leaf", "n5")
	err := validateClusterRuntimeInheritance(context.Background(), c, leaf)
	g.Expect(err).To(gomega.HaveOccurred())
	var de *runtimeinheritance.MaxDepthExceededError
	g.Expect(errors.As(err, &de)).To(gomega.BeTrue())
}

func TestValidateNamespacedRuntimeInheritance_SameNamespace(t *testing.T) {
	g := gomega.NewWithT(t)
	parent := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-profile", Namespace: "team"},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(parent).Build()

	sr := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rt",
			Namespace:   "team",
			Annotations: map[string]string{constants.RuntimeInheritFromAnnotationKey: "ns-profile"},
		},
	}
	g.Expect(validateNamespacedRuntimeInheritance(context.Background(), c, sr)).To(gomega.Succeed())
}

func TestValidateNamespacedRuntimeInheritance_FallsBackToCluster(t *testing.T) {
	g := gomega.NewWithT(t)
	parent := mkCSR("cluster-profile", "")
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(parent).Build()

	sr := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rt",
			Namespace: "team",
			Annotations: map[string]string{
				constants.RuntimeInheritFromAnnotationKey: "cluster-profile",
			},
		},
	}
	g.Expect(validateNamespacedRuntimeInheritance(context.Background(), c, sr)).To(gomega.Succeed())
}

func TestValidateNamespacedRuntimeInheritance_CrossNamespaceRejected(t *testing.T) {
	g := gomega.NewWithT(t)
	// Parent is a ServingRuntime in a different namespace. The
	// fetcher only checks the consumer's namespace + cluster scope,
	// so the cross-ns name resolves as ParentNotFound.
	otherNS := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "other-team"},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(otherNS).Build()

	sr := &v1beta1.ServingRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rt",
			Namespace:   "team",
			Annotations: map[string]string{constants.RuntimeInheritFromAnnotationKey: "shared"},
		},
	}
	err := validateNamespacedRuntimeInheritance(context.Background(), c, sr)
	g.Expect(err).To(gomega.HaveOccurred())
	var pnf *runtimeinheritance.ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeTrue())
}
