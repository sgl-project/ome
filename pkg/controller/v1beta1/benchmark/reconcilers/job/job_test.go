package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewJobReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	objMeta := metav1.ObjectMeta{
		Name:      "test-job",
		Namespace: "default",
	}
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "test-container",
				Image: "test-image",
			},
		},
	}

	reconciler := NewJobReconciler(client, scheme, objMeta, podSpec)

	assert.NotNil(t, reconciler)
	assert.Equal(t, client, reconciler.client)
	assert.Equal(t, scheme, reconciler.scheme)
	assert.NotNil(t, reconciler.Job)
	assert.Equal(t, "test-job", reconciler.Job.Name)
	assert.Equal(t, "default", reconciler.Job.Namespace)
	assert.Equal(t, int32(0), *reconciler.Job.Spec.BackoffLimit)
}

func TestJobReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name          string
		existingJob   *batchv1.Job
		expectedError bool
		shouldCreate  bool
	}{
		{
			name:          "should create new job when none exists",
			existingJob:   nil,
			expectedError: false,
			shouldCreate:  true,
		},
		{
			name: "should not create job when it already exists",
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			expectedError: false,
			shouldCreate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingJob != nil {
				clientBuilder = clientBuilder.WithObjects(tt.existingJob)
			}
			client := clientBuilder.Build()

			objMeta := metav1.ObjectMeta{
				Name:      "test-job",
				Namespace: "default",
			}
			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image",
					},
				},
			}

			reconciler := NewJobReconciler(client, scheme, objMeta, podSpec)
			err := reconciler.Reconcile(context.TODO())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify job creation
			job := &batchv1.Job{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Name:      "test-job",
				Namespace: "default",
			}, job)

			if tt.shouldCreate {
				assert.NoError(t, err)
				assert.Equal(t, "test-job", job.Name)
				assert.Equal(t, "default", job.Namespace)
			}
		})
	}
}

func TestJobReconciler_CheckResult(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name          string
		existingJob   *batchv1.Job
		expectedType  string
		expectedError bool
	}{
		{
			name:          "job does not exist",
			existingJob:   nil,
			expectedType:  "Create",
			expectedError: false,
		},
		{
			name: "job exists",
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			expectedType:  "Existed",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingJob != nil {
				clientBuilder = clientBuilder.WithObjects(tt.existingJob)
			}
			client := clientBuilder.Build()

			reconciler := &JobReconciler{
				client: client,
				scheme: scheme,
				Job: &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-job",
						Namespace: "default",
					},
				},
			}

			result, err := reconciler.CheckResult(context.TODO())

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, result.String())
			}
		})
	}
}
