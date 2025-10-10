package basemodel

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// TEST 1: Retry and Backoff Logic Tests
func TestRetryBackoffCalculation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name          string
		retryCount    int
		expectedDelay time.Duration
	}{
		{
			name:          "First retry",
			retryCount:    0,
			expectedDelay: 2 * time.Second, // BaseDelay
		},
		{
			name:          "Second retry (exponential)",
			retryCount:    1,
			expectedDelay: 4 * time.Second, // BaseDelay * 2^1
		},
		{
			name:          "Third retry",
			retryCount:    2,
			expectedDelay: 8 * time.Second, // BaseDelay * 2^2
		},
		{
			name:          "Cap at MaxDelay",
			retryCount:    15,
			expectedDelay: 5 * time.Minute, // MaxDelay cap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewDefaultRetryConfig()
			delay := config.calculateBackoffDelay(tt.retryCount)
			g.Expect(delay).To(gomega.Equal(tt.expectedDelay))
		})
	}
}

func TestRetryCountManagement(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name          string
		initialCount  int
		operation     string // "increment", "get", "clear"
		expectedCount int
	}{
		{
			name:          "Get retry count from annotations",
			initialCount:  3,
			operation:     "get",
			expectedCount: 3,
		},
		{
			name:          "Increment retry count",
			initialCount:  2,
			operation:     "increment",
			expectedCount: 3,
		},
		{
			name:          "Clear retry count",
			initialCount:  5,
			operation:     "clear",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create BaseModel with annotation
			baseModel := &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
					Annotations: map[string]string{
						RetryCountAnnotationKey: fmt.Sprintf("%d", tt.initialCount),
					},
				},
			}

			switch tt.operation {
			case "get":
				count := getRetryCount(baseModel)
				g.Expect(count).To(gomega.Equal(tt.expectedCount))
			case "increment":
				incrementRetryCount(baseModel)
				count := getRetryCount(baseModel)
				g.Expect(count).To(gomega.Equal(tt.expectedCount))
			case "clear":
				clearRetryCount(baseModel)
				count := getRetryCount(baseModel)
				g.Expect(count).To(gomega.Equal(tt.expectedCount))
			}
		})
	}
}

// TEST 2: Job Creation Tests
func TestCreateMetadataExtractionJob(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name         string
		baseModel    *v1beta1.BaseModel
		pvc          *corev1.PersistentVolumeClaim
		subPath      string
		existingJob  *batchv1.Job
		shouldCreate bool
		validateJob  func(*testing.T, *batchv1.Job)
		wantErr      bool
	}{
		{
			name: "Create new job successfully",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
					UID:       types.UID("test-uid-123"),
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://default:model-data/models/llama"),
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "model-data",
					Namespace: "default",
				},
			},
			subPath:      "models/llama",
			existingJob:  nil,
			shouldCreate: true,
			validateJob: func(t *testing.T, job *batchv1.Job) {
				// Verify job name
				g.Expect(job.Name).To(gomega.Equal("test-model-metadata-extraction"))

				// Verify timeout
				g.Expect(job.Spec.ActiveDeadlineSeconds).ToNot(gomega.BeNil())
				g.Expect(*job.Spec.ActiveDeadlineSeconds).To(gomega.Equal(int64(300)))

				// Verify TTL
				g.Expect(job.Spec.TTLSecondsAfterFinished).ToNot(gomega.BeNil())
				g.Expect(*job.Spec.TTLSecondsAfterFinished).To(gomega.Equal(int32(300)))

				// Verify ServiceAccount
				g.Expect(job.Spec.Template.Spec.ServiceAccountName).To(gomega.Equal("basemodel-metadata-extractor"))

				// Verify image is NOT :latest
				image := job.Spec.Template.Spec.Containers[0].Image
				g.Expect(image).ToNot(gomega.ContainSubstring(":latest"))

				// Verify PVC mount is read-only
				g.Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly).To(gomega.BeTrue())

				// Verify subpath
				g.Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts[0].SubPath).To(gomega.Equal("models/llama"))

				// Verify resource limits
				g.Expect(job.Spec.Template.Spec.Containers[0].Resources.Requests).ToNot(gomega.BeNil())
				g.Expect(job.Spec.Template.Spec.Containers[0].Resources.Limits).ToNot(gomega.BeNil())
			},
			wantErr: false,
		},
		{
			name: "Job already exists - no error",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-model",
					Namespace: "default",
					UID:       types.UID("existing-uid-456"),
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://default:model-data"),
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "model-data",
					Namespace: "default",
				},
			},
			subPath: "",
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-model-metadata-extraction",
					Namespace: "default",
				},
			},
			shouldCreate: false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build client with PVC
			builder := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pvc)

			if tt.existingJob != nil {
				builder = builder.WithObjects(tt.existingJob)
			}

			c := builder.Build()

			// Create reconciler
			recorder := record.NewFakeRecorder(10)
			reconciler := &BaseModelReconciler{
				Client:   c,
				Scheme:   scheme,
				Recorder: recorder,
				Log:      zap.New(zap.UseDevMode(true)),
			}

			// Create PVCComponents
			pvcComponents := &utilstorage.PVCStorageComponents{
				PVCName:   tt.pvc.Name,
				Namespace: tt.pvc.Namespace,
				SubPath:   tt.subPath,
			}

			// Call function
			job, err := reconciler.createMetadataExtractionJob(
				context.TODO(),
				tt.baseModel,
				tt.pvc,
				pvcComponents,
			)

			// Validate error
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			// Validate job if created
			if tt.shouldCreate && tt.validateJob != nil {
				tt.validateJob(t, job)

				// Verify owner reference
				g.Expect(job.OwnerReferences).ToNot(gomega.BeEmpty())
				g.Expect(job.OwnerReferences[0].Name).To(gomega.Equal(tt.baseModel.Name))
			}
		})
	}
}

// TEST 3: Job Status Monitoring Tests
func TestHandleJobStatus(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name            string
		baseModel       *v1beta1.BaseModel
		job             *batchv1.Job
		configMap       *corev1.ConfigMap
		expectedRequeue bool
		wantErr         bool
		validateResult  func(*testing.T, ctrl.Result, error)
	}{
		{
			name: "Job running - should requeue",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
					UID:       types.UID("test-uid-789"),
				},
			},
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model-metadata-extraction",
					Namespace: "default",
				},
				Status: batchv1.JobStatus{
					Active:    1,
					StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
				},
			},
			configMap:       nil,
			expectedRequeue: true,
			wantErr:         false,
			validateResult: func(t *testing.T, result ctrl.Result, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(result.RequeueAfter).To(gomega.Equal(30 * time.Second))
			},
		},
		{
			name: "Job succeeded - no requeue",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-model",
					Namespace: "default",
					UID:       types.UID("success-uid-101"),
				},
			},
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-model-metadata-extraction",
					Namespace: "default",
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					StartTime: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
					CompletionTime: &metav1.Time{
						Time: time.Now(),
					},
				},
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "success-model-metadata",
					Namespace: "default",
				},
				Data: map[string]string{
					"model-type": "llama",
				},
			},
			expectedRequeue: false,
			wantErr:         false,
			validateResult: func(t *testing.T, result ctrl.Result, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(result.RequeueAfter).To(gomega.Equal(time.Duration(0)))
			},
		},
		{
			name: "Job failed - returns error",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failed-model",
					Namespace: "default",
					UID:       types.UID("failed-uid-202"),
				},
			},
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failed-model-metadata-extraction",
					Namespace: "default",
				},
				Status: batchv1.JobStatus{
					Failed:    1,
					StartTime: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
					Conditions: []batchv1.JobCondition{
						{
							Type:    batchv1.JobFailed,
							Status:  corev1.ConditionTrue,
							Reason:  "BackoffLimitExceeded",
							Message: "Job has reached the specified backoff limit",
						},
					},
				},
			},
			configMap:       nil,
			expectedRequeue: false,
			wantErr:         true,
			validateResult: func(t *testing.T, result ctrl.Result, err error) {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring("metadata extraction job failed"))
			},
		},
		{
			name: "Job nil without ConfigMap - requeue",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ttl-model",
					Namespace: "default",
					UID:       types.UID("ttl-uid-303"),
				},
			},
			job:             nil, // Job was TTL cleaned
			configMap:       nil, // No ConfigMap found
			expectedRequeue: true,
			wantErr:         false,
			validateResult: func(t *testing.T, result ctrl.Result, err error) {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(result.RequeueAfter).To(gomega.Equal(30 * time.Second))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build client
			builder := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.baseModel).
				WithStatusSubresource(tt.baseModel)

			if tt.job != nil {
				builder = builder.WithObjects(tt.job)
			}
			if tt.configMap != nil {
				builder = builder.WithObjects(tt.configMap)
			}

			c := builder.Build()

			// Create reconciler
			recorder := record.NewFakeRecorder(10)
			reconciler := &BaseModelReconciler{
				Client:   c,
				Scheme:   scheme,
				Recorder: recorder,
				Log:      zap.New(zap.UseDevMode(true)),
			}

			// Create PVCComponents
			pvcComponents := &utilstorage.PVCStorageComponents{
				PVCName:   "test-pvc",
				Namespace: "default",
			}

			// Call function
			result, err := reconciler.handleJobStatus(
				context.TODO(),
				tt.baseModel,
				tt.job,
				pvcComponents,
			)

			// Run validation
			if tt.validateResult != nil {
				tt.validateResult(t, result, err)
			}
		})
	}
}

// TEST 4: Full PVC Reconciliation Flow Test
func TestHandlePVCStorageWithValidation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Create scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(corev1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(batchv1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

	tests := []struct {
		name       string
		baseModel  *v1beta1.BaseModel
		setupMocks func(client.Client)
		validate   func(*testing.T, client.Client, *v1beta1.BaseModel, ctrl.Result, error)
		wantErr    bool
	}{
		{
			name: "Valid PVC - creates job successfully",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc-model",
					Namespace: "default",
					UID:       types.UID("pvc-uid-404"),
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://default:model-data/llama"),
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create bound PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "model-data",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				}
				err := c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Should not error
				g.Expect(reconcileErr).NotTo(gomega.HaveOccurred())

				// Should requeue for job monitoring
				g.Expect(result.RequeueAfter).To(gomega.Equal(30 * time.Second))

				// Verify job was created
				job := &batchv1.Job{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "pvc-model-metadata-extraction",
					Namespace: "default",
				}, job)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(job.Name).To(gomega.Equal("pvc-model-metadata-extraction"))
			},
			wantErr: false,
		},
		{
			name: "Retry logic with transient error",
			baseModel: &v1beta1.BaseModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "retry-model",
					Namespace: "default",
					UID:       types.UID("retry-uid-505"),
					Annotations: map[string]string{
						RetryCountAnnotationKey: "0",
					},
				},
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						StorageUri: stringPtr("pvc://default:retry-pvc/models"),
					},
				},
			},
			setupMocks: func(c client.Client) {
				// Create bound PVC
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "retry-pvc",
						Namespace: "default",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
					Status: corev1.PersistentVolumeClaimStatus{
						Phase: corev1.ClaimBound,
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				}
				err := c.Create(context.TODO(), pvc)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			},
			validate: func(t *testing.T, c client.Client, baseModel *v1beta1.BaseModel, result ctrl.Result, reconcileErr error) {
				// Should succeed on retry
				g.Expect(reconcileErr).NotTo(gomega.HaveOccurred())
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client
			c := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.baseModel).
				WithStatusSubresource(tt.baseModel).
				Build()

			// Setup mocks
			tt.setupMocks(c)

			// Create reconciler
			recorder := record.NewFakeRecorder(10)
			reconciler := &BaseModelReconciler{
				Client:   c,
				Scheme:   scheme,
				Recorder: recorder,
				Log:      zap.New(zap.UseDevMode(true)),
			}

			// Call function
			result, err := reconciler.handlePVCStorageWithValidation(
				context.TODO(),
				tt.baseModel,
			)

			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}

			// Run validation
			tt.validate(t, c, tt.baseModel, result, err)
		})
	}
}
