package pdb

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// countingClient wraps a controller-runtime client to count and optionally fail operations
type countingClient struct {
	client.Client
	createCalls int
	updateCalls int
	getCalls    int

	failGet    error
	failCreate error
	failUpdate error
}

func (c *countingClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	c.getCalls++
	if c.failGet != nil {
		return c.failGet
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *countingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c.createCalls++
	if c.failCreate != nil {
		return c.failCreate
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *countingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	if c.failUpdate != nil {
		return c.failUpdate
	}
	return c.Client.Update(ctx, obj, opts...)
}

func intOrStringPtrFromInt(i int) *intstr.IntOrString {
	v := intstr.FromInt(i)
	return &v
}

func buildScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, policyv1.AddToScheme(scheme))
	return scheme
}

func TestPDBReconciler_Create_DefaultMaxUnavailable(t *testing.T) {
	scheme := buildScheme(t)

	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cc := &countingClient{Client: baseClient}

	meta := metav1.ObjectMeta{
		Name:      "test-svc",
		Namespace: "test-ns",
	}

	// No MinAvailable/MaxUnavailable provided -> defaults to MaxUnavailable=1
	ext := &v1beta1.ComponentExtensionSpec{}

	r := NewPDBReconciler(cc, scheme, meta, ext)

	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify created object in cluster
	got := &policyv1.PodDisruptionBudget{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)

	assert.Equal(t, meta.Name, got.Name)
	assert.Equal(t, meta.Namespace, got.Namespace)
	if assert.NotNil(t, got.Spec.MaxUnavailable) {
		assert.Equal(t, intstr.Int, got.Spec.MaxUnavailable.Type)
		assert.Equal(t, int32(1), got.Spec.MaxUnavailable.IntVal)
	}
	assert.Nil(t, got.Spec.MinAvailable)
	// Selector should target app label for raw service
	expectedLabel := constants.GetRawServiceLabel(meta.Name)
	require.NotNil(t, got.Spec.Selector)
	assert.Equal(t, map[string]string{"app": expectedLabel}, got.Spec.Selector.MatchLabels)

	assert.Equal(t, 1, cc.createCalls)
	assert.Equal(t, 0, cc.updateCalls)
}

func TestPDBReconciler_Update_WhenSpecDiffers(t *testing.T) {
	scheme := buildScheme(t)

	meta := metav1.ObjectMeta{
		Name:      "svc-update",
		Namespace: "ns-update",
	}

	// Pre-create an existing PDB with MaxUnavailable=2 (desired will be 1)
	existing := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            meta.Name,
			Namespace:       meta.Namespace,
			ResourceVersion: "123",
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: intOrStringPtrFromInt(2),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app": constants.GetRawServiceLabel(meta.Name),
			}},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cc := &countingClient{Client: baseClient}

	ext := &v1beta1.ComponentExtensionSpec{} // default -> MaxUnavailable=1
	r := NewPDBReconciler(cc, scheme, meta, ext)

	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify it was updated to MaxUnavailable=1
	got := &policyv1.PodDisruptionBudget{}
	err = baseClient.Get(context.Background(), types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, got)
	require.NoError(t, err)
	if assert.NotNil(t, got.Spec.MaxUnavailable) {
		assert.Equal(t, int32(1), got.Spec.MaxUnavailable.IntVal)
	}
	assert.Equal(t, 1, cc.updateCalls)
}

func TestPDBReconciler_NoOp_WhenEqual(t *testing.T) {
	scheme := buildScheme(t)

	meta := metav1.ObjectMeta{
		Name:      "svc-eq",
		Namespace: "ns-eq",
	}

	// Existing matches desired: MaxUnavailable=1 and same selector
	existing := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: intOrStringPtrFromInt(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app": constants.GetRawServiceLabel(meta.Name),
			}},
		},
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cc := &countingClient{Client: baseClient}

	ext := &v1beta1.ComponentExtensionSpec{} // default -> MaxUnavailable=1
	r := NewPDBReconciler(cc, scheme, meta, ext)

	obj, err := r.Reconcile()
	require.NoError(t, err)
	require.NotNil(t, obj)

	assert.Equal(t, 0, cc.updateCalls)
	assert.Equal(t, 0, cc.createCalls)
}

func Test_semanticPDBEquals(t *testing.T) {
	meta := metav1.ObjectMeta{Name: "svc", Namespace: "ns"}
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": constants.GetRawServiceLabel(meta.Name)}}

	// Equal MaxUnavailable=1
	a := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MaxUnavailable: intOrStringPtrFromInt(1), Selector: selector}}
	b := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MaxUnavailable: intOrStringPtrFromInt(1), Selector: selector}}
	assert.True(t, semanticPDBEquals(a, b))

	// Different MaxUnavailable
	b2 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MaxUnavailable: intOrStringPtrFromInt(2), Selector: selector}}
	assert.False(t, semanticPDBEquals(a, b2))

	// One has MinAvailable, other nil
	a2 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MinAvailable: intOrStringPtrFromInt(1), Selector: selector}}
	b3 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{Selector: selector}}
	assert.False(t, semanticPDBEquals(a2, b3))

	// Both MinAvailable equal
	b4 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MinAvailable: intOrStringPtrFromInt(1), Selector: selector}}
	assert.True(t, semanticPDBEquals(a2, b4))

	// Different selectors
	sel2 := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "different"}}
	a3 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MaxUnavailable: intOrStringPtrFromInt(1), Selector: selector}}
	b5 := &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{MaxUnavailable: intOrStringPtrFromInt(1), Selector: sel2}}
	assert.False(t, semanticPDBEquals(a3, b5))
}

func TestPDBReconciler_ErrorPaths(t *testing.T) {
	scheme := buildScheme(t)
	meta := metav1.ObjectMeta{Name: "svc-err", Namespace: "ns-err"}
	ext := &v1beta1.ComponentExtensionSpec{}

	// Get error
	{
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		cc := &countingClient{Client: baseClient, failGet: errors.New("get failure")}
		r := NewPDBReconciler(cc, scheme, meta, ext)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
	}

	// Create error (Get returns NotFound)
	{
		baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		cc := &countingClient{Client: baseClient, failCreate: errors.New("create failure")}
		r := NewPDBReconciler(cc, scheme, meta, ext)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
		assert.Equal(t, 1, cc.createCalls)
	}

	// Update error (object exists but differs)
	{
		existing := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{Name: meta.Name, Namespace: meta.Namespace},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: intOrStringPtrFromInt(2),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app": constants.GetRawServiceLabel(meta.Name),
				}},
			},
		}
		baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
		cc := &countingClient{Client: baseClient, failUpdate: errors.New("update failure")}
		r := NewPDBReconciler(cc, scheme, meta, ext)
		obj, err := r.Reconcile()
		assert.Error(t, err)
		assert.Nil(t, obj)
		assert.Equal(t, 1, cc.updateCalls)
	}
}
