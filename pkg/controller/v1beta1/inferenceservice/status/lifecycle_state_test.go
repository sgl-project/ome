package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

func TestDeriveLifecycleState(t *testing.T) {
	tests := []struct {
		name          string
		isvc          *v1beta1.InferenceService
		previousState v1beta1.InferenceServiceLifecycleState
		expected      v1beta1.InferenceServiceLifecycleState
	}{
		{
			name:     "nil service returns creating",
			expected: v1beta1.InferenceServiceLifecycleStateCreating,
		},
		{
			name:     "deleting service returns deleting",
			isvc:     deletingInferenceService(),
			expected: v1beta1.InferenceServiceLifecycleStateDeleting,
		},
		{
			name:     "ready service returns ready",
			isvc:     readyInferenceService(),
			expected: v1beta1.InferenceServiceLifecycleStateReady,
		},
		{
			name: "invalid spec returns failed",
			isvc: inferenceServiceWithStatus(v1beta1.InferenceServiceStatus{
				ModelStatus: v1beta1.ModelStatus{TransitionStatus: v1beta1.InvalidSpec},
			}),
			expected: v1beta1.InferenceServiceLifecycleStateFailed,
		},
		{
			name: "last failure info returns failed",
			isvc: inferenceServiceWithStatus(v1beta1.InferenceServiceStatus{
				ModelStatus: v1beta1.ModelStatus{
					TransitionStatus: v1beta1.InProgress,
					LastFailureInfo:  &v1beta1.FailureInfo{Reason: v1beta1.ModelLoadFailed},
				},
			}),
			expected: v1beta1.InferenceServiceLifecycleStateFailed,
		},
		{
			name: "in progress initial reconcile returns creating",
			isvc: inferenceServiceWithStatus(v1beta1.InferenceServiceStatus{
				ModelStatus: v1beta1.ModelStatus{TransitionStatus: v1beta1.InProgress},
			}),
			expected: v1beta1.InferenceServiceLifecycleStateCreating,
		},
		{
			name: "in progress established service returns updating",
			isvc: inferenceServiceWithStatus(v1beta1.InferenceServiceStatus{
				ModelStatus: v1beta1.ModelStatus{TransitionStatus: v1beta1.InProgress},
			}),
			previousState: v1beta1.InferenceServiceLifecycleStateReady,
			expected:      v1beta1.InferenceServiceLifecycleStateUpdating,
		},
		{
			name: "component rollout initial reconcile returns creating",
			isvc: inferenceServiceWithStatus(v1beta1.InferenceServiceStatus{
				Components: map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
					v1beta1.EngineComponent: {
						LatestCreatedRevision: "engine-2",
						LatestReadyRevision:   "engine-1",
					},
				},
			}),
			expected: v1beta1.InferenceServiceLifecycleStateCreating,
		},
		{
			name: "ready false after established state returns failed",
			isvc: inferenceServiceWithStatus(func() v1beta1.InferenceServiceStatus {
				status := v1beta1.InferenceServiceStatus{}
				status.InitializeConditions()
				status.SetCondition(v1beta1.IngressReady, &apis.Condition{
					Type:    v1beta1.IngressReady,
					Status:  v1.ConditionFalse,
					Reason:  "IngressNotReady",
					Message: "ingress failed",
				})
				return status
			}()),
			previousState: v1beta1.InferenceServiceLifecycleStateReady,
			expected:      v1beta1.InferenceServiceLifecycleStateFailed,
		},
		{
			name: "ready false during initial reconcile returns creating",
			isvc: inferenceServiceWithStatus(func() v1beta1.InferenceServiceStatus {
				status := v1beta1.InferenceServiceStatus{}
				status.InitializeConditions()
				status.SetCondition(v1beta1.IngressReady, &apis.Condition{
					Type:    v1beta1.IngressReady,
					Status:  v1.ConditionFalse,
					Reason:  "IngressNotReady",
					Message: "ingress failed",
				})
				return status
			}()),
			expected: v1beta1.InferenceServiceLifecycleStateCreating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DeriveLifecycleState(tt.isvc, tt.previousState))
		})
	}
}

func inferenceServiceWithStatus(status v1beta1.InferenceServiceStatus) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		Status: status,
	}
}

func readyInferenceService() *v1beta1.InferenceService {
	status := v1beta1.InferenceServiceStatus{}
	status.InitializeConditions()
	status.SetCondition(v1beta1.IngressReady, &apis.Condition{
		Type:   v1beta1.IngressReady,
		Status: v1.ConditionTrue,
	})
	status.SetCondition(v1beta1.EngineReady, &apis.Condition{
		Type:   v1beta1.EngineReady,
		Status: v1.ConditionTrue,
	})
	return inferenceServiceWithStatus(status)
}

func deletingInferenceService() *v1beta1.InferenceService {
	now := metav1.Now()
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			DeletionTimestamp: &now,
		},
	}
}
