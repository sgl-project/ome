package pdb

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestResolveBudget(t *testing.T) {
	tests := []struct {
		name      string
		component *v1beta1.ComponentExtensionSpec
		fallback  *controllerconfig.PodDisruptionBudgetPolicy
		want      *Budget
		wantError string
	}{
		{
			name: "both sources absent",
		},
		{
			name: "fallback minimum",
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MinAvailable: intOrStringPtr(intstr.FromInt(2)),
			},
			want: &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(2))},
		},
		{
			name: "fallback maximum",
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MaxUnavailable: intOrStringPtr(intstr.FromString("25%")),
			},
			want: &Budget{MaxUnavailable: intOrStringPtr(intstr.FromString("25%"))},
		},
		{
			name: "component minimum overrides fallback maximum",
			component: &v1beta1.ComponentExtensionSpec{
				MinAvailable: intOrStringPtr(intstr.FromInt(3)),
			},
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MaxUnavailable: intOrStringPtr(intstr.FromInt(1)),
			},
			want: &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(3))},
		},
		{
			name: "component maximum overrides fallback minimum",
			component: &v1beta1.ComponentExtensionSpec{
				MaxUnavailable: intOrStringPtr(intstr.FromInt(2)),
			},
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MinAvailable: intOrStringPtr(intstr.FromInt(4)),
			},
			want: &Budget{MaxUnavailable: intOrStringPtr(intstr.FromInt(2))},
		},
		{
			name:      "empty selected fallback",
			fallback:  &controllerconfig.PodDisruptionBudgetPolicy{},
			wantError: "exactly one",
		},
		{
			name: "component sets both fields",
			component: &v1beta1.ComponentExtensionSpec{
				MinAvailable:   intOrStringPtr(intstr.FromInt(2)),
				MaxUnavailable: intOrStringPtr(intstr.FromInt(1)),
			},
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MaxUnavailable: intOrStringPtr(intstr.FromInt(1)),
			},
			wantError: "exactly one",
		},
		{
			name: "invalid component remains authoritative",
			component: &v1beta1.ComponentExtensionSpec{
				MinAvailable: intOrStringPtr(intstr.FromInt(-1)),
			},
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MaxUnavailable: intOrStringPtr(intstr.FromInt(1)),
			},
			wantError: "minAvailable",
		},
		{
			name: "malformed fallback",
			fallback: &controllerconfig.PodDisruptionBudgetPolicy{
				MaxUnavailable: intOrStringPtr(intstr.FromString("many")),
			},
			wantError: "maxUnavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBudget(tt.component, tt.fallback)
			if tt.wantError != "" {
				assert.Nil(t, got)
				assert.ErrorContains(t, err, tt.wantError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if got == nil {
				return
			}
			if got.MinAvailable != nil {
				if tt.component != nil && tt.component.MinAvailable != nil {
					assert.NotSame(t, tt.component.MinAvailable, got.MinAvailable)
				}
				if tt.fallback != nil && tt.fallback.MinAvailable != nil {
					assert.NotSame(t, tt.fallback.MinAvailable, got.MinAvailable)
				}
			}
			if got.MaxUnavailable != nil {
				if tt.component != nil && tt.component.MaxUnavailable != nil {
					assert.NotSame(t, tt.component.MaxUnavailable, got.MaxUnavailable)
				}
				if tt.fallback != nil && tt.fallback.MaxUnavailable != nil {
					assert.NotSame(t, tt.fallback.MaxUnavailable, got.MaxUnavailable)
				}
			}
		})
	}
}

func TestDesiredPodCount(t *testing.T) {
	replicas := func(value int32) *int32 { return &value }
	tests := []struct {
		name      string
		ir        *v1beta1.InferenceReplica
		want      int32
		wantError string
	}{
		{
			name: "three single-pod replicas",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(3),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}},
			}},
			want: 3,
		},
		{
			name: "four leader-worker replicas",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(4),
				Runners: []v1beta1.Runner{
					{Name: v1beta1.RunnerNameLeader, Size: 1},
					{Name: v1beta1.RunnerNameWorker, Size: 3},
				},
			}},
			want: 16,
		},
		{
			name: "zero replicas",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(0),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}},
			}},
			want: 0,
		},
		{
			name:      "nil inference replica",
			wantError: "InferenceReplica is required",
		},
		{
			name: "nil replicas",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Runners: []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}},
			}},
			wantError: "spec.replicas is required",
		},
		{
			name: "negative replicas",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(-1),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}},
			}},
			wantError: "spec.replicas must be non-negative",
		},
		{
			name: "nil runners",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(1),
			}},
			wantError: "spec.runners must not be empty",
		},
		{
			name: "empty runners",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(1),
				Runners:  []v1beta1.Runner{},
			}},
			wantError: "spec.runners must not be empty",
		},
		{
			name: "zero runner size",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(1),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 0}},
			}},
			wantError: "spec.runners[0].size must be positive",
		},
		{
			name: "negative runner size",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(1),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: -1}},
			}},
			wantError: "spec.runners[0].size must be positive",
		},
		{
			name: "exact int32 maximum",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(math.MaxInt32),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}},
			}},
			want: math.MaxInt32,
		},
		{
			name: "runner sum overflow",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(1),
				Runners: []v1beta1.Runner{
					{Name: v1beta1.RunnerNameLeader, Size: math.MaxInt32},
					{Name: v1beta1.RunnerNameWorker, Size: 1},
				},
			}},
			wantError: "runner size sum exceeds int32",
		},
		{
			name: "desired pod product overflow",
			ir: &v1beta1.InferenceReplica{Spec: v1beta1.InferenceReplicaSpec{
				Replicas: replicas(2),
				Runners:  []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: math.MaxInt32}},
			}},
			wantError: "desired pod count exceeds int32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DesiredPodCount(tt.ir)
			if tt.wantError != "" {
				assert.Zero(t, got)
				assert.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeOMENativeBudget(t *testing.T) {
	tests := []struct {
		name        string
		budget      *Budget
		desiredPods int32
		want        *Budget
		wantError   string
	}{
		{
			name: "nil budget",
		},
		{
			name:        "integer minimum",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(3))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(3))},
		},
		{
			name:        "seventy-five percent minimum",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromString("75%"))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(6))},
		},
		{
			name:        "fractional minimum rounds up",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromString("50%"))},
			desiredPods: 3,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(2))},
		},
		{
			name:        "integer maximum",
			budget:      &Budget{MaxUnavailable: intOrStringPtr(intstr.FromInt(1))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(7))},
		},
		{
			name:        "twenty-five percent maximum",
			budget:      &Budget{MaxUnavailable: intOrStringPtr(intstr.FromString("25%"))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(6))},
		},
		{
			name:        "fractional maximum rounds up",
			budget:      &Budget{MaxUnavailable: intOrStringPtr(intstr.FromString("33%"))},
			desiredPods: 10,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(6))},
		},
		{
			name:        "one hundred percent maximum",
			budget:      &Budget{MaxUnavailable: intOrStringPtr(intstr.FromString("100%"))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(0))},
		},
		{
			name:        "maximum exceeds zero desired pods",
			budget:      &Budget{MaxUnavailable: intOrStringPtr(intstr.FromInt(1))},
			desiredPods: 0,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(0))},
		},
		{
			name:        "integer minimum above desired is preserved",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(9))},
			desiredPods: 8,
			want:        &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(9))},
		},
		{
			name:        "malformed percentage",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromString("half"))},
			desiredPods: 8,
			wantError:   "minAvailable",
		},
		{
			name: "both fields",
			budget: &Budget{
				MinAvailable:   intOrStringPtr(intstr.FromInt(3)),
				MaxUnavailable: intOrStringPtr(intstr.FromInt(1)),
			},
			desiredPods: 8,
			wantError:   "exactly one",
		},
		{
			name:        "neither field",
			budget:      &Budget{},
			desiredPods: 8,
			wantError:   "exactly one",
		},
		{
			name:        "negative desired pods",
			budget:      &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(1))},
			desiredPods: -1,
			wantError:   "desired pod count must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeOMENativeBudget(tt.budget, tt.desiredPods)
			if tt.wantError != "" {
				assert.Nil(t, got)
				assert.ErrorContains(t, err, tt.wantError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if got != nil {
				assert.NotSame(t, tt.budget, got)
				require.NotNil(t, got.MinAvailable)
				assert.Equal(t, intstr.Int, got.MinAvailable.Type)
				assert.Nil(t, got.MaxUnavailable)
				if tt.budget.MinAvailable != nil {
					assert.NotSame(t, tt.budget.MinAvailable, got.MinAvailable)
				}
			}
		})
	}
}

func TestNormalizeOMENativeBudgetReturnsDisjointResult(t *testing.T) {
	input := &Budget{MinAvailable: intOrStringPtr(intstr.FromInt(3))}
	got, err := NormalizeOMENativeBudget(input, 8)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.MinAvailable)

	got.MinAvailable.IntVal = 7
	assert.Equal(t, int32(3), input.MinAvailable.IntVal)
}
