package kueuequeue

import (
	"context"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

func TestLocalQueueReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = kueuev1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		queueName      string
		existingQueue  *kueuev1beta1.LocalQueue
		wantErr        bool
		expectedQueue  *kueuev1beta1.LocalQueue
		expectedUpdate bool
	}{
		{
			name:           "successfully create new LocalQueue",
			queueName:      "test-queue",
			wantErr:        false,
			expectedQueue:  createTestLocalQueue("test-queue", "test-queue"),
			expectedUpdate: false,
		},
		{
			name:      "update existing LocalQueue",
			queueName: "existing-queue",
			existingQueue: &kueuev1beta1.LocalQueue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-queue",
					Namespace: "existing-queue",
				},
				Spec: kueuev1beta1.LocalQueueSpec{
					ClusterQueue: "old-cluster-queue",
				},
			},
			expectedQueue:  createTestLocalQueue("existing-queue", "existing-queue"),
			expectedUpdate: true,
		},
		{
			name:           "localQueue already exists with same spec",
			queueName:      "existing-same-queue",
			existingQueue:  createTestLocalQueue("existing-same-queue", "existing-same-queue"),
			wantErr:        false,
			expectedQueue:  createTestLocalQueue("existing-same-queue", "existing-same-queue"),
			expectedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fakectrl.NewClientBuilder().WithScheme(scheme).Build()

			// If testing existing queue case, create it first
			if tt.existingQueue != nil {
				err := fakeClient.Create(context.TODO(), tt.existingQueue)
				assert.NoError(t, err)
			}

			// Create reconciler
			reconciler := NewLocalQueueReconciler(fakeClient, scheme, tt.queueName)
			// Perform reconciliation
			result, err := reconciler.Reconcile()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify LocalQueue was created/updated correctly
			createdQueue := &kueuev1beta1.LocalQueue{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{
				Name:      tt.queueName,
				Namespace: tt.queueName,
			}, createdQueue)
			assert.NoError(t, err)

			if tt.expectedQueue != nil {
				// Verify specific fields for new creation
				assert.Equal(t, tt.expectedQueue.Name, createdQueue.Name)
				assert.Equal(t, tt.expectedQueue.Namespace, createdQueue.Namespace)
				assert.Equal(t, tt.expectedQueue.Spec.ClusterQueue, createdQueue.Spec.ClusterQueue)
			}

			if tt.expectedUpdate {
				// Verify the queue was updated
				assert.NotEqual(t, tt.existingQueue.Spec, createdQueue.Spec)
			}
			if !tt.expectedUpdate && tt.existingQueue != nil {
				// Verify for the case clusterqueue already exist with desired spec
				assert.Equal(t, tt.existingQueue.Spec, createdQueue.Spec)
			}
		})
	}
}

func createTestLocalQueue(name string, namespace string) *kueuev1beta1.LocalQueue {
	return &kueuev1beta1.LocalQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kueuev1beta1.LocalQueueSpec{
			ClusterQueue: kueuev1beta1.ClusterQueueReference(name),
			StopPolicy:   &constants.DefaultStopPolicy,
		},
	}
}
