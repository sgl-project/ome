package pdb

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func Test_createPDB_SpecAndSelector(t *testing.T) {
	g := NewWithT(t)

	name := "test-svc"
	namespace := "default"
	meta := metav1.ObjectMeta{Name: name, Namespace: namespace}
	min := 2
	max := 3
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &min, MaxReplicas: max}

	p := createPDB(meta, ext)
	g.Expect(p.Name).To(Equal(name))
	g.Expect(p.Namespace).To(Equal(namespace))
	// MinAvailable should be MinReplicas (int) coerced to IntOrString
	g.Expect(p.Spec.MinAvailable).NotTo(BeNil())
	g.Expect(p.Spec.MinAvailable.Type).To(Equal(intstr.Int))
	g.Expect(p.Spec.MinAvailable.IntVal).To(Equal(int32(min)))
	// MaxUnavailable should be equal to max (not less than min)
	g.Expect(p.Spec.MaxUnavailable).NotTo(BeNil())
	g.Expect(p.Spec.MaxUnavailable.Type).To(Equal(intstr.Int))
	g.Expect(p.Spec.MaxUnavailable.IntVal).To(Equal(int32(max)))
	// Selector should match app label with raw service label (identity)
	g.Expect(p.Spec.Selector).NotTo(BeNil())
	g.Expect(p.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", constants.GetRawServiceLabel(name)))
}

func Test_calculateMinReplicas(t *testing.T) {
	g := NewWithT(t)

	// nil or < default -> default
	var nilMin *int
	ext1 := &v1beta1.ComponentExtensionSpec{MinReplicas: nilMin}
	m := calculateMinReplicas(ext1)
	g.Expect(m.IntVal).To(Equal(int32(constants.DefaultMinReplicas)))

	val := constants.DefaultMinReplicas - 1
	ext2 := &v1beta1.ComponentExtensionSpec{MinReplicas: &val}
	m = calculateMinReplicas(ext2)
	g.Expect(m.IntVal).To(Equal(int32(constants.DefaultMinReplicas)))

	valOk := constants.DefaultMinReplicas + 1
	ext3 := &v1beta1.ComponentExtensionSpec{MinReplicas: &valOk}
	m = calculateMinReplicas(ext3)
	g.Expect(m.IntVal).To(Equal(int32(valOk)))
}

func Test_calculateMaxReplicas(t *testing.T) {
	g := NewWithT(t)

	min := 3
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &min, MaxReplicas: 2}
	mx := calculateMaxReplicas(ext)
	// max < min -> coerced to min
	g.Expect(mx.IntVal).To(Equal(int32(*ext.MinReplicas)))

	ext.MaxReplicas = 5
	mx = calculateMaxReplicas(ext)
	g.Expect(mx.IntVal).To(Equal(int32(5)))
}

func Test_semanticPDBEquals(t *testing.T) {
	g := NewWithT(t)

	meta := metav1.ObjectMeta{Name: "svc", Namespace: "ns"}
	min := 1
	max := 2
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &min, MaxReplicas: max}
	a := createPDB(meta, ext)
	b := createPDB(meta, ext)
	g.Expect(semanticPDBEquals(a, b)).To(BeTrue())

	// Change max
	b.Spec.MaxUnavailable = &intstr.IntOrString{IntVal: 3}
	g.Expect(semanticPDBEquals(a, b)).To(BeFalse())

	// Restore max, change selector
	b = createPDB(meta, ext)
	b.Spec.Selector.MatchLabels["app"] = "different"
	g.Expect(semanticPDBEquals(a, b)).To(BeFalse())
}

func Test_PDBReconciler_Reconcile_Create(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = policyv1.AddToScheme(scheme)

	name := "svc-create"
	ns := "default"
	min := 1
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &min, MaxReplicas: 2}
	rec := NewPDBReconciler(
		ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build(),
		scheme,
		metav1.ObjectMeta{Name: name, Namespace: ns},
		ext,
	)

	obj, err := rec.Reconcile()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj).NotTo(BeNil())

	// Verify created in cluster
	client := rec.client
	stored := &policyv1.PodDisruptionBudget{}
	g.Expect(client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, stored)).To(Succeed())
	g.Expect(stored.Spec.MinAvailable.IntVal).To(Equal(int32(*ext.MinReplicas)))
}

func Test_PDBReconciler_Reconcile_Update(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = policyv1.AddToScheme(scheme)

	name := "svc-update"
	ns := "default"
	minOld := 1
	minNew := 2

	// Existing PDB with old min
	existing := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{IntVal: int32(minOld)},
			Selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": constants.GetRawServiceLabel(name)}},
		},
	}

	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &minNew, MaxReplicas: 3}
	rec := NewPDBReconciler(client, scheme, metav1.ObjectMeta{Name: name, Namespace: ns}, ext)

	obj, err := rec.Reconcile()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj).NotTo(BeNil())

	// Verify updated in cluster
	stored := &policyv1.PodDisruptionBudget{}
	g.Expect(client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, stored)).To(Succeed())
	g.Expect(stored.Spec.MinAvailable.IntVal).To(Equal(int32(*ext.MinReplicas)))
}

func Test_PDBReconciler_Reconcile_Existed(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = policyv1.AddToScheme(scheme)

	name := "svc-existed"
	ns := "default"
	min := 2
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: &min, MaxReplicas: 3}
	desired := createPDB(metav1.ObjectMeta{Name: name, Namespace: ns}, ext)

	client := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(desired.DeepCopy()).Build()
	rec := NewPDBReconciler(client, scheme, metav1.ObjectMeta{Name: name, Namespace: ns}, ext)

	obj, err := rec.Reconcile()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj).NotTo(BeNil())
	// Since existed, reconciler should return the existing object rather than desired pointer
	g.Expect(obj.Spec.MinAvailable.IntVal).To(Equal(desired.Spec.MinAvailable.IntVal))
}
