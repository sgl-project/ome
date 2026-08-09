package runtimeinheritance

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// fakeStore wires names to RuntimeRefs for the fetcher closure tests build.
type fakeStore map[string]*RuntimeRef

func (s fakeStore) fetch(_ context.Context, name string) (*RuntimeRef, error) {
	ref, ok := s[name]
	if !ok {
		return nil, fmt.Errorf("lookup %q: %w", name, ErrParentNotFound)
	}
	return ref, nil
}

func mkRef(name, parent string, spec *v1beta1.ServingRuntimeSpec) *RuntimeRef {
	return &RuntimeRef{Name: name, ParentName: parent, Spec: spec}
}

func envSpec(envs ...corev1.EnvVar) *v1beta1.ServingRuntimeSpec {
	return &v1beta1.ServingRuntimeSpec{
		ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
			Containers: []corev1.Container{{Name: "ome-container", Env: envs}},
		},
	}
}

func TestResolve_NoInheritance(t *testing.T) {
	g := gomega.NewWithT(t)
	start := mkRef("solo", "", &v1beta1.ServingRuntimeSpec{Disabled: ptr.To(true)})

	eff, chain, err := Resolve(context.Background(), start, fakeStore{}.fetch, 5)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"solo"}))
	g.Expect(*eff.Disabled).To(gomega.BeTrue())
}

func TestResolve_TwoLevelChain(t *testing.T) {
	g := gomega.NewWithT(t)
	profile := mkRef("profile", "", envSpec(corev1.EnvVar{Name: "NCCL_DEBUG", Value: "INFO"}))
	runtime := mkRef("runtime", "profile", envSpec(corev1.EnvVar{Name: "FROM_RUNTIME", Value: "yes"}))

	store := fakeStore{"profile": profile}
	eff, chain, err := Resolve(context.Background(), runtime, store.fetch, 5)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"profile", "runtime"}))
	envByName := map[string]string{}
	for _, e := range eff.Containers[0].Env {
		envByName[e.Name] = e.Value
	}
	g.Expect(envByName).To(gomega.HaveKeyWithValue("NCCL_DEBUG", "INFO"))
	g.Expect(envByName).To(gomega.HaveKeyWithValue("FROM_RUNTIME", "yes"))
}

func TestResolve_FourLevelChainBottomUp(t *testing.T) {
	g := gomega.NewWithT(t)
	// root contributes only NCCL; each descendant adds one env. Final union should have all 4.
	root := mkRef("root", "", envSpec(corev1.EnvVar{Name: "NCCL", Value: "info"}))
	mid := mkRef("mid", "root", envSpec(corev1.EnvVar{Name: "MID", Value: "1"}))
	near := mkRef("near", "mid", envSpec(corev1.EnvVar{Name: "NEAR", Value: "1"}))
	leaf := mkRef("leaf", "near", envSpec(corev1.EnvVar{Name: "LEAF", Value: "1"}))

	store := fakeStore{"root": root, "mid": mid, "near": near}
	eff, chain, err := Resolve(context.Background(), leaf, store.fetch, 5)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(chain).To(gomega.Equal([]string{"root", "mid", "near", "leaf"}))
	envByName := map[string]string{}
	for _, e := range eff.Containers[0].Env {
		envByName[e.Name] = e.Value
	}
	g.Expect(envByName).To(gomega.HaveLen(4))
	g.Expect(envByName).To(gomega.HaveKey("NCCL"))
	g.Expect(envByName).To(gomega.HaveKey("MID"))
	g.Expect(envByName).To(gomega.HaveKey("NEAR"))
	g.Expect(envByName).To(gomega.HaveKey("LEAF"))
}

func TestResolve_ChildScalarWinsOverDeepRoot(t *testing.T) {
	g := gomega.NewWithT(t)
	root := mkRef("root", "", &v1beta1.ServingRuntimeSpec{Disabled: ptr.To(true)})
	leaf := mkRef("leaf", "root", &v1beta1.ServingRuntimeSpec{Disabled: ptr.To(false)})

	eff, _, err := Resolve(context.Background(), leaf, fakeStore{"root": root}.fetch, 5)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(*eff.Disabled).To(gomega.BeFalse(), "child must win over root")
}

func TestResolve_ParentNotFound(t *testing.T) {
	g := gomega.NewWithT(t)
	runtime := mkRef("runtime", "ghost", &v1beta1.ServingRuntimeSpec{})

	_, _, err := Resolve(context.Background(), runtime, fakeStore{}.fetch, 5)
	g.Expect(err).To(gomega.HaveOccurred())
	var pnf *ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeTrue())
	g.Expect(pnf.Parent).To(gomega.Equal("ghost"))
	g.Expect(pnf.Chain).To(gomega.Equal([]string{"runtime"}))
}

func TestResolve_CycleDirect(t *testing.T) {
	g := gomega.NewWithT(t)
	// a → b → a
	a := mkRef("a", "b", &v1beta1.ServingRuntimeSpec{})
	b := mkRef("b", "a", &v1beta1.ServingRuntimeSpec{})
	store := fakeStore{"a": a, "b": b}

	_, _, err := Resolve(context.Background(), a, store.fetch, 5)
	g.Expect(err).To(gomega.HaveOccurred())
	var ce *CycleError
	g.Expect(errors.As(err, &ce)).To(gomega.BeTrue())
	g.Expect(ce.Cycle).To(gomega.Equal([]string{"a", "b", "a"}))
}

func TestResolve_CycleSelf(t *testing.T) {
	g := gomega.NewWithT(t)
	a := mkRef("a", "a", &v1beta1.ServingRuntimeSpec{})

	_, _, err := Resolve(context.Background(), a, fakeStore{"a": a}.fetch, 5)
	g.Expect(err).To(gomega.HaveOccurred())
	var ce *CycleError
	g.Expect(errors.As(err, &ce)).To(gomega.BeTrue())
}

func TestResolve_MaxDepthExceeded(t *testing.T) {
	g := gomega.NewWithT(t)
	// chain of 6: leaf → l4 → l3 → l2 → l1 → root. With maxDepth=5,
	// 5 links allowed total (leaf+4 parents); 6th link is the rejection.
	store := fakeStore{}
	prev := ""
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("n%d", i)
		store[name] = mkRef(name, prev, &v1beta1.ServingRuntimeSpec{})
		prev = name
	}
	leaf := mkRef("leaf", "n5", &v1beta1.ServingRuntimeSpec{})

	_, _, err := Resolve(context.Background(), leaf, store.fetch, 5)
	g.Expect(err).To(gomega.HaveOccurred())
	var de *MaxDepthExceededError
	g.Expect(errors.As(err, &de)).To(gomega.BeTrue())
	g.Expect(de.MaxDepth).To(gomega.Equal(5))
}

func TestResolve_TransientFetchErrorPropagates(t *testing.T) {
	g := gomega.NewWithT(t)
	transient := errors.New("apiserver unreachable")
	runtime := mkRef("runtime", "parent", &v1beta1.ServingRuntimeSpec{})
	fetch := func(_ context.Context, name string) (*RuntimeRef, error) {
		return nil, transient
	}

	_, _, err := Resolve(context.Background(), runtime, fetch, 5)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(errors.Is(err, transient)).To(gomega.BeTrue(), "non-typed errors must propagate unwrapped")
	// Should NOT be classified as ParentNotFoundError.
	var pnf *ParentNotFoundError
	g.Expect(errors.As(err, &pnf)).To(gomega.BeFalse())
}
