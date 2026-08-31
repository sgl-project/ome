package pdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestOMENativeStateReady(t *testing.T) {
	desired := int32(2)
	ready := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
		Spec:       v1beta1.InferenceReplicaSpec{Replicas: &desired},
		Status: v1beta1.InferenceReplicaStatus{
			ObservedGeneration:   3,
			ReadyReplicas:        desired,
			AvailableReplicas:    desired,
			UpdatedReadyReplicas: desired - 1,
			CurrentRevision:      "source",
			UpdateRevision:       "target",
		},
	}

	tests := []struct {
		name      string
		nilObject bool
		mutate    func(*v1beta1.InferenceReplica)
		want      bool
	}{
		{name: "fully available across revisions", want: true},
		{name: "nil replica", nilObject: true},
		{name: "missing desired replicas", mutate: func(ir *v1beta1.InferenceReplica) { ir.Spec.Replicas = nil }},
		{name: "generation not observed", mutate: func(ir *v1beta1.InferenceReplica) { ir.Status.ObservedGeneration-- }},
		{name: "instances not ready", mutate: func(ir *v1beta1.InferenceReplica) { ir.Status.ReadyReplicas-- }},
		{name: "instances not available", mutate: func(ir *v1beta1.InferenceReplica) { ir.Status.AvailableReplicas-- }},
		{name: "terminating target", mutate: func(ir *v1beta1.InferenceReplica) { now := metav1.Now(); ir.DeletionTimestamp = &now }},
		{
			name: "zero desired instances",
			mutate: func(ir *v1beta1.InferenceReplica) {
				zero := int32(0)
				ir.Spec.Replicas = &zero
				ir.Status.ReadyReplicas = 0
				ir.Status.AvailableReplicas = 0
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ir *v1beta1.InferenceReplica
			if !tt.nilObject {
				ir = ready.DeepCopy()
				if tt.mutate != nil {
					tt.mutate(ir)
				}
			}
			assert.Equal(t, tt.want, omeNativeStateReady(ir))
		})
	}
}

func TestRawDeploymentStateReady(t *testing.T) {
	desired := int32(2)
	ready := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 4},
		Spec:       appsv1.DeploymentSpec{Replicas: &desired},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 4,
			ReadyReplicas:      desired,
			AvailableReplicas:  desired,
		},
	}

	tests := []struct {
		name      string
		nilObject bool
		mutate    func(*appsv1.Deployment)
		want      bool
	}{
		{name: "fully available", want: true},
		{name: "nil deployment", nilObject: true},
		{name: "missing desired replicas", mutate: func(deployment *appsv1.Deployment) { deployment.Spec.Replicas = nil }},
		{name: "generation not observed", mutate: func(deployment *appsv1.Deployment) { deployment.Status.ObservedGeneration-- }},
		{name: "ready replicas incomplete", mutate: func(deployment *appsv1.Deployment) { deployment.Status.ReadyReplicas-- }},
		{name: "available replicas incomplete", mutate: func(deployment *appsv1.Deployment) { deployment.Status.AvailableReplicas-- }},
		{name: "terminating target", mutate: func(deployment *appsv1.Deployment) { now := metav1.Now(); deployment.DeletionTimestamp = &now }},
		{
			name: "zero desired replicas",
			mutate: func(deployment *appsv1.Deployment) {
				zero := int32(0)
				deployment.Spec.Replicas = &zero
				deployment.Status.ReadyReplicas = 0
				deployment.Status.AvailableReplicas = 0
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deployment *appsv1.Deployment
			if !tt.nilObject {
				deployment = ready.DeepCopy()
				if tt.mutate != nil {
					tt.mutate(deployment)
				}
			}
			assert.Equal(t, tt.want, rawDeploymentStateReady(deployment))
		})
	}
}

func TestOMENativeCutoverReadyUsesLiveState(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	desired := int32(1)
	expected := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Name: "engine", Namespace: "default", UID: types.UID("ir-uid"), Generation: 2},
		Spec:       v1beta1.InferenceReplicaSpec{Replicas: &desired},
	}
	live := expected.DeepCopy()
	live.Status.ObservedGeneration = live.Generation
	live.Status.ReadyReplicas = desired
	live.Status.AvailableReplicas = desired
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live).Build()

	ready, err := OMENativeCutoverReady(context.Background(), reader, expected)
	require.NoError(t, err)
	assert.True(t, ready)

	newer := live.DeepCopy()
	newer.Generation++
	newer.Status.ObservedGeneration = newer.Generation
	reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(newer).Build()
	ready, err = OMENativeCutoverReady(context.Background(), reader, expected)
	require.NoError(t, err)
	assert.False(t, ready)
}

func TestRawDeploymentCutoverReadyUsesLiveState(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	desired := int32(1)
	expected := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "engine", Namespace: "default", UID: types.UID("deployment-uid"), Generation: 2},
		Spec:       appsv1.DeploymentSpec{Replicas: &desired},
	}
	live := expected.DeepCopy()
	live.Status.ObservedGeneration = live.Generation
	live.Status.ReadyReplicas = desired
	live.Status.AvailableReplicas = desired
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live).Build()

	ready, err := RawDeploymentCutoverReady(context.Background(), reader, expected)
	require.NoError(t, err)
	assert.True(t, ready)

	newer := live.DeepCopy()
	newer.Generation++
	newer.Status.ObservedGeneration = newer.Generation
	reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(newer).Build()
	ready, err = RawDeploymentCutoverReady(context.Background(), reader, expected)
	require.NoError(t, err)
	assert.False(t, ready)
}
