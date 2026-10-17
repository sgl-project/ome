package autoscaler

import (
	"context"
	"testing"
	"time"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

var switchStatusTarget = v1beta1.ScaleTargetRef{
	APIVersion: v1beta1.SchemeGroupVersion.String(),
	Kind:       "InferenceReplica",
	Name:       switchName,
}

func writeSwitchStatus(t *testing.T, c client.Client, autoscaler *v1beta1.ComponentAutoscaler, prev *v1beta1.ComponentAutoscalerStatus) *v1beta1.ComponentAutoscalerStatus {
	t.Helper()
	status, _, err := WriteAutoscalerStatus(context.Background(), c, switchNS, switchName, autoscaler, SpecSourceISVC, switchStatusTarget, prev)
	require.NoError(t, err)
	return status
}

func switchedCondition(status *v1beta1.ComponentAutoscalerStatus) *metav1.Condition {
	return meta.FindStatusCondition(status.Conditions, v1beta1.AutoscalerSwitchedCondition)
}

// takeoverScaledObject is a ScaledObject that names the Component's HPA.
func takeoverScaledObject(ready metav1.ConditionStatus) *kedav1.ScaledObject {
	return &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   switchNS,
			Name:        switchSOKey.Name,
			UID:         "takeover-scaledobject-uid",
			Annotations: map[string]string{kedav1.ScaledObjectTransferHpaOwnershipAnnotation: "true"},
		},
		Spec: kedav1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1.ScaleTarget{APIVersion: switchStatusTarget.APIVersion, Kind: switchStatusTarget.Kind, Name: switchName},
			Advanced: &kedav1.AdvancedConfig{
				HorizontalPodAutoscalerConfig: &kedav1.HorizontalPodAutoscalerConfig{Name: switchName},
			},
		},
		Status: kedav1.ScaledObjectStatus{
			Conditions: kedav1.Conditions{{Type: kedav1.ConditionReady, Status: ready, Reason: "Test"}},
		},
	}
}

func TestSwitchStatus_KEDATakeover(t *testing.T) {
	takenOver := func(so *kedav1.ScaledObject) *autoscalingv2.HorizontalPodAutoscaler {
		hpa := kedaCreatedHPA(switchName, so)
		hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas = 3, 4
		return hpa
	}
	tests := []struct {
		name       string
		objs       func() []client.Object
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{
			name: "workload still controls the HPA",
			objs: func() []client.Object {
				return []client.Object{takeoverScaledObject(metav1.ConditionUnknown), workingHPA()}
			},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForKEDA,
		},
		{
			name: "HPA not created yet",
			objs: func() []client.Object {
				return []client.Object{takeoverScaledObject(metav1.ConditionUnknown)}
			},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForKEDA,
		},
		{
			name: "KEDA controls the HPA but the ScaledObject is not Ready",
			objs: func() []client.Object {
				so := takeoverScaledObject(metav1.ConditionFalse)
				return []client.Object{so, takenOver(so)}
			},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonScaledObjectNotReady,
		},
		{
			name: "KEDA controls the HPA but scales another target",
			objs: func() []client.Object {
				so := takeoverScaledObject(metav1.ConditionTrue)
				hpa := takenOver(so)
				hpa.Spec.ScaleTargetRef = dispatchDeploymentTargetRef(switchName)
				return []client.Object{so, hpa}
			},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForKEDA,
		},
		{
			name: "KEDA controls the HPA and reports Ready",
			objs: func() []client.Object {
				so := takeoverScaledObject(metav1.ConditionTrue)
				return []client.Object{so, takenOver(so)}
			},
			wantStatus: metav1.ConditionTrue,
			wantReason: v1beta1.AutoscalerSwitchedReasonHPAControlledByKEDA,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(tt.objs()...).Build()
			status := writeSwitchStatus(t, c, kedaClass(), nil)
			cond := switchedCondition(status)
			require.NotNil(t, cond)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.NotEmpty(t, cond.Message)
			assert.False(t, cond.LastTransitionTime.IsZero(), "the ISVC CRD requires lastTransitionTime")
		})
	}
}

func TestSwitchStatus_KEDATakeoverMirrorsTheHPABeforeKEDARecordsItsName(t *testing.T) {
	so := takeoverScaledObject(metav1.ConditionTrue)
	hpa := kedaCreatedHPA(switchName, so)
	hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas = 3, 4
	c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(so, hpa).Build()

	status := writeSwitchStatus(t, c, kedaClass(), nil)
	assert.Equal(t, int32(3), status.CurrentReplicas, "status.hpaName is empty, so the configured name is used")
	assert.Equal(t, int32(4), status.DesiredReplicas)
}

func TestSwitchStatus_KEDATakeoverKeepsTransitionTime(t *testing.T) {
	so := takeoverScaledObject(metav1.ConditionTrue)
	c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(so, kedaCreatedHPA(switchName, so)).Build()

	first := writeSwitchStatus(t, c, kedaClass(), nil)
	firstCond := switchedCondition(first)
	require.NotNil(t, firstCond)
	firstCond.LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))

	second := writeSwitchStatus(t, c, kedaClass(), first)
	secondCond := switchedCondition(second)
	require.NotNil(t, secondCond)
	assert.Equal(t, firstCond.LastTransitionTime, secondCond.LastTransitionTime, "an unchanged condition keeps its timestamp")
}

func TestSwitchStatus_KEDAWithoutTakeoverHasNoCondition(t *testing.T) {
	so := takeoverScaledObject(metav1.ConditionTrue)
	so.Annotations = nil
	so.Spec.Advanced = nil
	c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(so).Build()

	assert.Nil(t, switchedCondition(writeSwitchStatus(t, c, kedaClass(), nil)))
}

func TestSwitchStatus_BackToHPA(t *testing.T) {
	gone := &kedav1.ScaledObject{ObjectMeta: metav1.ObjectMeta{Namespace: switchNS, Name: switchSOKey.Name, UID: "deleted-scaledobject-uid"}}
	pendingPrev := &v1beta1.ComponentAutoscalerStatus{Conditions: []metav1.Condition{{
		Type:               v1beta1.AutoscalerSwitchedCondition,
		Status:             metav1.ConditionFalse,
		Reason:             v1beta1.AutoscalerSwitchedReasonWaitingForScaledObjectDeletion,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute)),
	}}}
	tests := []struct {
		name       string
		objs       []client.Object
		prev       *v1beta1.ComponentAutoscalerStatus
		wantStatus metav1.ConditionStatus
		wantReason string
		wantNone   bool
	}{
		{
			name:       "ScaledObject still exists",
			objs:       []client.Object{takeoverScaledObject(metav1.ConditionTrue)},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForScaledObjectDeletion,
		},
		{
			name:       "the HPA KEDA took over still exists",
			objs:       []client.Object{kedaCreatedHPA(switchName, gone)},
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForHPADeletion,
		},
		{
			name:       "another HPA KEDA created still exists",
			objs:       []client.Object{kedaCreatedHPA("custom-keda-hpa", gone)},
			prev:       pendingPrev,
			wantStatus: metav1.ConditionFalse,
			wantReason: v1beta1.AutoscalerSwitchedReasonWaitingForHPADeletion,
		},
		{
			name:       "everything KEDA created is gone",
			objs:       []client.Object{workingHPA()},
			prev:       pendingPrev,
			wantStatus: metav1.ConditionTrue,
			wantReason: v1beta1.AutoscalerSwitchedReasonScaledObjectRemoved,
		},
		{
			name:     "no switch observed",
			objs:     []client.Object{workingHPA()},
			wantNone: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(tt.objs...).Build()
			cond := switchedCondition(writeSwitchStatus(t, c, hpaClass(), tt.prev))
			if tt.wantNone {
				assert.Nil(t, cond)
				return
			}
			require.NotNil(t, cond)
			assert.Equal(t, tt.wantStatus, cond.Status)
			assert.Equal(t, tt.wantReason, cond.Reason)
			assert.NotEmpty(t, cond.Message)
		})
	}
}

func TestSwitchStatus_KEDAHPAIsNotMirroredAsOMEs(t *testing.T) {
	so := takeoverScaledObject(metav1.ConditionTrue)
	hpa := kedaCreatedHPA(switchName, so)
	hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas = 3, 4
	hpa.Status.Conditions = []autoscalingv2.HorizontalPodAutoscalerCondition{{
		Type: autoscalingv2.ScalingActive, Status: "True", Reason: "ValidMetricFound", LastTransitionTime: metav1.Now(),
	}}
	c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(so, hpa).Build()

	status := writeSwitchStatus(t, c, hpaClass(), nil)
	assert.Zero(t, status.CurrentReplicas, "the HPA belongs to KEDA until OME replaces it")
	assert.Zero(t, status.DesiredReplicas)
	assert.Nil(t, meta.FindStatusCondition(status.Conditions, string(autoscalingv2.ScalingActive)))
	require.NotNil(t, switchedCondition(status))
}

// TestSwitchStatus_FollowsDispatchThroughAFullSwitch drives dispatch and the
// status writer together through hpa → keda → hpa, carrying the status from
// pass to pass the way the InferenceService reconciler does.
func TestSwitchStatus_FollowsDispatchThroughAFullSwitch(t *testing.T) {
	c := switchClient(t, nil)
	var status *v1beta1.ComponentAutoscalerStatus
	pass := func(autoscaler *v1beta1.ComponentAutoscaler) error {
		err := DispatchAutoscaler(context.Background(), switchParams(c, autoscaler))
		status = writeSwitchStatus(t, c, autoscaler, status)
		return err
	}

	require.NoError(t, pass(hpaClass()))
	assert.Nil(t, switchedCondition(status))

	requirePending(t, pass(kedaClass()), v1beta1.AutoscalerSwitchedReasonWaitingForKEDA)
	assert.Equal(t, v1beta1.AutoscalerSwitchedReasonWaitingForKEDA, switchedCondition(status).Reason)

	simulateKEDATakeover(t, c, getSwitchSO(t, c), getSwitchHPA(t, c))
	markScaledObjectReady(t, c, switchSOKey)
	require.NoError(t, pass(kedaClass()))
	assert.Equal(t, metav1.ConditionTrue, switchedCondition(status).Status)
	assert.Equal(t, v1beta1.AutoscalerSwitchedReasonHPAControlledByKEDA, switchedCondition(status).Reason)

	so := getSwitchSO(t, c)
	so.Finalizers = []string{"finalizer.keda.sh"}
	require.NoError(t, c.Update(context.Background(), so))
	requirePending(t, pass(hpaClass()), v1beta1.AutoscalerSwitchedReasonWaitingForScaledObjectDeletion)
	assert.Equal(t, v1beta1.AutoscalerSwitchedReasonWaitingForScaledObjectDeletion, switchedCondition(status).Reason)

	terminating := getSwitchSO(t, c)
	terminating.Finalizers = nil
	require.NoError(t, c.Update(context.Background(), terminating))
	require.NoError(t, pass(hpaClass()))
	assert.Equal(t, metav1.ConditionTrue, switchedCondition(status).Status)
	assert.Equal(t, v1beta1.AutoscalerSwitchedReasonScaledObjectRemoved, switchedCondition(status).Reason)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: switchNS, Name: switchName}, hpa))
	assert.Equal(t, dispatchOwner(switchName), *metav1.GetControllerOf(hpa))
}

// TestSwitchStatus_SettledSwitchStopsListingHPAs pins the cost bound: after a
// switch back to an HPA reported ScaledObjectRemoved, status writes stop
// listing the namespace's HPAs.
func TestSwitchStatus_SettledSwitchStopsListingHPAs(t *testing.T) {
	for _, tt := range []struct {
		name      string
		prev      metav1.Condition
		wantLists int
	}{
		{
			name:      "settled",
			prev:      metav1.Condition{Type: v1beta1.AutoscalerSwitchedCondition, Status: metav1.ConditionTrue, Reason: v1beta1.AutoscalerSwitchedReasonScaledObjectRemoved},
			wantLists: 0,
		},
		{
			name:      "settling",
			prev:      metav1.Condition{Type: v1beta1.AutoscalerSwitchedCondition, Status: metav1.ConditionFalse, Reason: v1beta1.AutoscalerSwitchedReasonWaitingForHPADeletion},
			wantLists: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lists := 0
			c := fake.NewClientBuilder().WithScheme(dispatchScheme(t)).WithObjects(workingHPA()).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
						if _, ok := list.(*autoscalingv2.HorizontalPodAutoscalerList); ok {
							lists++
						}
						return cl.List(ctx, list, opts...)
					},
				}).Build()
			tt.prev.LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Minute))
			prev := &v1beta1.ComponentAutoscalerStatus{Conditions: []metav1.Condition{tt.prev}}

			cond := switchedCondition(writeSwitchStatus(t, c, hpaClass(), prev))
			require.NotNil(t, cond)
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, v1beta1.AutoscalerSwitchedReasonScaledObjectRemoved, cond.Reason)
			assert.Equal(t, tt.wantLists, lists)
		})
	}
}
