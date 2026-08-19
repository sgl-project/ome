package status

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils"
)

func maxUnavailableExt(v *intstr.IntOrString) *v1beta1.ComponentExtensionSpec {
	return &v1beta1.ComponentExtensionSpec{MaxUnavailable: v}
}

func TestTopLevelComponentReadyFromLifecycle(t *testing.T) {
	cases := []struct {
		name       string
		component  v1beta1.ComponentType
		cs         *v1beta1.LifecycleStatus
		ext        *v1beta1.ComponentExtensionSpec
		wantNil    bool
		wantType   apis.ConditionType
		wantStatus corev1.ConditionStatus
		wantReason string
	}{
		{
			name:      "unknown component leaves the surface untouched",
			component: v1beta1.ComponentType("bogus"),
			cs:        &v1beta1.LifecycleStatus{Replicas: 1},
			wantNil:   true,
		},
		{
			name:       "nil status reports NoReplicas",
			component:  v1beta1.EngineComponent,
			cs:         nil,
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "NoReplicas",
		},
		{
			name:       "zero desired reports NoReplicas",
			component:  v1beta1.DecoderComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 0},
			wantType:   v1beta1.DecoderReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "NoReplicas",
		},
		{
			name:       "fully converged reports Ready",
			component:  v1beta1.EngineComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 3, ReadyReplicas: 3, ServingReplicas: 3},
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionTrue,
			wantReason: "Ready",
		},
		{
			name:       "serving at or above floor reports MinimumAvailable",
			component:  v1beta1.RouterComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 10, ReadyReplicas: 9, ServingReplicas: 9},
			ext:        maxUnavailableExt(utils.PtrIntOrStringFromString("25%")),
			wantType:   v1beta1.RouterReady,
			wantStatus: corev1.ConditionTrue,
			wantReason: "MinimumAvailable",
		},
		{
			name:       "serving below floor reports InsufficientAvailable",
			component:  v1beta1.EngineComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 10, ReadyReplicas: 2, ServingReplicas: 2},
			ext:        maxUnavailableExt(utils.PtrIntOrStringFromString("25%")),
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "InsufficientAvailable",
		},
		{
			name:       "nil ext is strict",
			component:  v1beta1.EngineComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 3, ReadyReplicas: 2, ServingReplicas: 2},
			ext:        nil,
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "InsufficientAvailable",
		},
		{
			// Regression: a budget permitting every Instance to be down
			// yields a zero floor. Without a minimum of 1, "0 >= 0" would
			// report Ready for a component serving no traffic.
			name:       "percent budget allowing full outage does not report Ready at zero serving",
			component:  v1beta1.EngineComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 5, ReadyReplicas: 0, ServingReplicas: 0},
			ext:        maxUnavailableExt(utils.PtrIntOrStringFromString("100%")),
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "InsufficientAvailable",
		},
		{
			// Same zero-floor hazard via an integer MaxUnavailable at or
			// above the replica count.
			name:       "integer budget at replica count does not report Ready at zero serving",
			component:  v1beta1.DecoderComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 4, ReadyReplicas: 0, ServingReplicas: 0},
			ext:        maxUnavailableExt(ptrIntOrString32(4)),
			wantType:   v1beta1.DecoderReady,
			wantStatus: corev1.ConditionFalse,
			wantReason: "InsufficientAvailable",
		},
		{
			// The floor-of-1 must not mask real progress: one serving
			// Instance under a fully-permissive budget is still Ready.
			name:       "one serving Instance under a permissive budget reports MinimumAvailable",
			component:  v1beta1.EngineComponent,
			cs:         &v1beta1.LifecycleStatus{Replicas: 5, ReadyReplicas: 1, ServingReplicas: 1},
			ext:        maxUnavailableExt(utils.PtrIntOrStringFromString("100%")),
			wantType:   v1beta1.EngineReady,
			wantStatus: corev1.ConditionTrue,
			wantReason: "MinimumAvailable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TopLevelComponentReadyFromLifecycle(tc.component, tc.cs, tc.ext)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("want nil condition for unknown component; got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want a condition; got nil")
			}
			if string(got.Type) != string(tc.wantType) {
				t.Errorf("condition type = %q; want %q", got.Type, tc.wantType)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("condition status = %q; want %q (message: %s)", got.Status, tc.wantStatus, got.Message)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("condition reason = %q; want %q (message: %s)", got.Reason, tc.wantReason, got.Message)
			}
		})
	}
}

func ptrIntOrString32(i int32) *intstr.IntOrString {
	out := intstr.FromInt32(i)
	return &out
}
