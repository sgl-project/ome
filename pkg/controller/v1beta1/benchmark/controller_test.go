package benchmark

import (
	"context"
	"testing"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestBenchmarkJobReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		benchmarkJob   *v1beta1.BenchmarkJob
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name:           "benchmark job not found",
			benchmarkJob:   nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "benchmark job with deletion timestamp",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-job",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"benchmarkjob.finalizers"},
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := cfake.NewClientBuilder().WithScheme(scheme)
			if tt.benchmarkJob != nil {
				clientBuilder = clientBuilder.WithObjects(tt.benchmarkJob)
			}
			client := clientBuilder.Build()

			r := &BenchmarkJobReconciler{
				Client:    client,
				Clientset: kfake.NewSimpleClientset(),
				Log:       zap.New(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(10),
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-job",
					Namespace: "default",
				},
			}

			result, err := r.Reconcile(context.Background(), req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestBenchmarkJobReconciler_ensureFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		expectedError bool
	}{
		{
			name: "add finalizer when not present",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			expectedError: false,
		},
		{
			name: "finalizer already present",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-job",
					Namespace:  "default",
					Finalizers: []string{"benchmarkjob.finalizers"},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				Build()

			r := &BenchmarkJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			err := r.ensureFinalizer(context.Background(), tt.benchmarkJob)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify finalizer is present
				updatedJob := &v1beta1.BenchmarkJob{}
				err = client.Get(context.Background(), types.NamespacedName{
					Name:      tt.benchmarkJob.Name,
					Namespace: tt.benchmarkJob.Namespace,
				}, updatedJob)
				assert.NoError(t, err)
				assert.Contains(t, updatedJob.Finalizers, "benchmarkjob.finalizers")
			}
		})
	}
}

func TestBenchmarkJobReconciler_handleDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		expectedError bool
	}{
		{
			name: "successful deletion",
			benchmarkJob: &v1beta1.BenchmarkJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "ome.io/v1beta1",
					Kind:       "BenchmarkJob",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-job",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"benchmarkjob.finalizers"},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				Build()

			r := &BenchmarkJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			result, err := r.handleDeletion(context.Background(), tt.benchmarkJob)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, ctrl.Result{}, result)
			}
		})
	}
}

func TestBenchmarkJobReconciler_buildMetadata(t *testing.T) {
	benchmarkJob := &v1beta1.BenchmarkJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	r := &BenchmarkJobReconciler{}
	meta := r.buildMetadata(benchmarkJob)

	expectedLabels := map[string]string{
		"benchmark": benchmarkJob.Name,
	}
	expectedAnnotations := map[string]string{
		"logging-forward": "true",
	}

	assert.Equal(t, benchmarkJob.Name, meta.Name)
	assert.Equal(t, benchmarkJob.Namespace, meta.Namespace)
	assert.Equal(t, expectedLabels, meta.Labels)
	assert.Equal(t, expectedAnnotations, meta.Annotations)
}

func TestBenchmarkJobReconciler_reconcileModelPVPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	baseModelName := "test-model"
	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		isvcRef       *v1beta1.InferenceService
		expectedError bool
	}{
		{
			name: "no inference service specified",
			benchmarkJob: &v1beta1.BenchmarkJob{
				Spec: v1beta1.BenchmarkJobSpec{
					Endpoint: v1beta1.EndpointSpec{},
				},
			},
			isvcRef:       nil,
			expectedError: false,
		},
		{
			name: "inference service with base model",
			benchmarkJob: &v1beta1.BenchmarkJob{
				Spec: v1beta1.BenchmarkJobSpec{
					Endpoint: v1beta1.EndpointSpec{
						InferenceService: &v1beta1.InferenceServiceReference{
							Name:      "test-isvc",
							Namespace: "default",
						},
					},
					HuggingFaceSecretReference: &v1beta1.HuggingFaceSecretReference{
						Name: "hf-secret",
					},
					Task:                    "chat",
					MaxTimePerIteration:     pointer.Int(60),
					MaxRequestsPerIteration: pointer.Int(100),
					TrafficScenarios:        []string{"scenario1", "scenario2"},
					NumConcurrency:          []int{1, 2, 4},
				},
			},
			isvcRef: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: pointer.String("test-model"),
							PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
								StorageURI: pointer.String("oci://bucket/path"),
							},
						},
					},
				},
				Status: v1beta1.InferenceServiceStatus{
					URL: &apis.URL{
						Scheme: "http",
						Host:   "test-isvc.default.svc.cluster.local",
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				WithObjects(&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      baseModelName,
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelFormat: v1beta1.ModelFormat{
							Name: "onnx",
						},
						Storage: &v1beta1.StorageSpec{
							Path: pointer.String("oci://bucket/model"),
						},
					},
				}).
				WithObjects(&v1beta1.InferenceService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc",
						Namespace: "default",
					},
					Spec: v1beta1.InferenceServiceSpec{
						Predictor: v1beta1.PredictorSpec{
							Model: &v1beta1.ModelSpec{
								BaseModel: pointer.String("test-model"),
							},
						},
					},
					Status: v1beta1.InferenceServiceStatus{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-isvc.default.svc.cluster.local",
						},
					},
				}).
				Build()

			r := &BenchmarkJobReconciler{
				Client:    client,
				Clientset: kfake.NewSimpleClientset(),
				Scheme:    scheme,
			}

			_, err := r.reconcileModelPVPVC(tt.benchmarkJob, tt.isvcRef)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBenchmarkJobReconciler_reconcileJob(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		podSpec       *corev1.PodSpec
		meta          metav1.ObjectMeta
		expectedError bool
	}{
		{
			name: "successful job reconciliation",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image",
					},
				},
			},
			meta: metav1.ObjectMeta{
				Name:      "test-job",
				Namespace: "default",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				Build()

			r := &BenchmarkJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			_, err := r.reconcileJob(tt.benchmarkJob, tt.podSpec, tt.meta)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBenchmarkJobReconciler_reconcilePodSpec(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name            string
		benchmarkJob    *v1beta1.BenchmarkJob
		benchmarkConfig *v1beta1.BenchmarkJobConfig
		expectedError   bool
	}{
		{
			name: "successful pod spec creation",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Spec: v1beta1.BenchmarkJobSpec{
					HuggingFaceSecretReference: &v1beta1.HuggingFaceSecretReference{
						Name: "hf-secret",
					},
					Endpoint: v1beta1.EndpointSpec{
						InferenceService: &v1beta1.InferenceServiceReference{
							Name:      "test-isvc",
							Namespace: "default",
						},
					},
					Task:                    "chat",
					MaxTimePerIteration:     pointer.Int(60),
					MaxRequestsPerIteration: pointer.Int(100),
					TrafficScenarios:        []string{"scenario1", "scenario2"},
					NumConcurrency:          []int{1, 2, 4},
					OutputLocation: &v1beta1.StorageSpec{
						StorageUri: pointer.String("oci://bucket/path"),
					},
				},
			},
			benchmarkConfig: &v1beta1.BenchmarkJobConfig{
				PodConfig: v1beta1.PodConfig{
					Image:         "test-image",
					CPURequest:    "100m",
					CPULimit:      "200m",
					MemoryRequest: "100Mi",
					MemoryLimit:   "200Mi",
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				WithObjects(&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelFormat: v1beta1.ModelFormat{
							Name: "onnx",
						},
						Storage: &v1beta1.StorageSpec{
							Path: pointer.String("oci://bucket/model"),
						},
					},
				}).
				WithObjects(&v1beta1.InferenceService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc",
						Namespace: "default",
					},
					Spec: v1beta1.InferenceServiceSpec{
						Predictor: v1beta1.PredictorSpec{
							Model: &v1beta1.ModelSpec{
								BaseModel: pointer.String("test-model"),
							},
						},
					},
					Status: v1beta1.InferenceServiceStatus{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-isvc.default.svc.cluster.local",
						},
					},
				}).
				Build()

			r := &BenchmarkJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			_, podSpec, err := r.reconcilePodSpec(tt.benchmarkJob, tt.benchmarkConfig)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, podSpec)
				assert.Equal(t, tt.benchmarkConfig.PodConfig.Image, podSpec.Containers[0].Image)
			}
		})
	}
}

func TestBenchmarkJobReconciler_buildBenchmarkCommand(t *testing.T) {
	tests := []struct {
		name         string
		benchmarkJob *v1beta1.BenchmarkJob
		isvc         *v1beta1.InferenceService
		wantErr      bool
	}{
		{
			name: "successful command build",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Spec: v1beta1.BenchmarkJobSpec{
					Task:                    "chat",
					MaxTimePerIteration:     pointer.Int(60),
					MaxRequestsPerIteration: pointer.Int(100),
					TrafficScenarios:        []string{"scenario1", "scenario2"},
					NumConcurrency:          []int{1, 2, 4},
					Endpoint: v1beta1.EndpointSpec{
						InferenceService: &v1beta1.InferenceServiceReference{
							Name:      "test-isvc",
							Namespace: "default",
						},
					},
					ServiceMetadata: &v1beta1.ServiceMetadata{
						Engine:   "llama",
						GpuType:  "A100",
						Version:  "v1",
						GpuCount: 1,
					},
					OutputLocation: &v1beta1.StorageSpec{
						StorageUri: pointer.String("oci://bucket/path"),
					},
				},
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: pointer.String("test-model"),
							PredictorExtensionSpec: v1beta1.PredictorExtensionSpec{
								StorageURI: pointer.String("oci://bucket/path"),
							},
						},
					},
				},
				Status: v1beta1.InferenceServiceStatus{
					URL: &apis.URL{
						Scheme: "http",
						Host:   "test-isvc.default.svc.cluster.local",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = v1beta1.AddToScheme(scheme)
			_ = batchv1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				WithObjects(tt.isvc).
				Build()

			r := &BenchmarkJobReconciler{
				Client: client,
			}

			command, args, err := r.buildBenchmarkCommand(tt.benchmarkJob)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildBenchmarkCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(command) == 0 {
					t.Error("buildBenchmarkCommand() command is empty")
				}
				if len(args) == 0 {
					t.Error("buildBenchmarkCommand() args is empty")
				}
			}
		},
		)
	}
}

func TestBenchmarkJobReconciler_updateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		existingJob   *batchv1.Job
		expectedError bool
	}{
		{
			name: "job not found",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			existingJob:   nil,
			expectedError: false,
		},
		{
			name: "job exists and completed",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			existingJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the BenchmarkJob without resourceVersion
			benchmarkJobCopy := tt.benchmarkJob.DeepCopy()
			benchmarkJobCopy.ResourceVersion = ""
			benchmarkJobCopy.Status = v1beta1.BenchmarkJobStatus{}
			benchmarkJobCopy.SetGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind("BenchmarkJob"))

			// Start building the client
			clientBuilder := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(benchmarkJobCopy)

			// Add the Job to the client builder if it exists
			if tt.existingJob != nil {
				tt.existingJob.SetGroupVersionKind(batchv1.SchemeGroupVersion.WithKind("Job"))
				clientBuilder = clientBuilder.WithObjects(tt.existingJob)
			}

			client := clientBuilder.Build()

			r := &BenchmarkJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			err := r.updateStatus(context.Background(), benchmarkJobCopy)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBenchmarkJobReconciler_cleanupResources(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		expectedError bool
	}{
		{
			name: "successful cleanup",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Spec: v1beta1.BenchmarkJobSpec{
					Endpoint: v1beta1.EndpointSpec{
						InferenceService: &v1beta1.InferenceServiceReference{
							Name:      "test-isvc",
							Namespace: "default",
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.benchmarkJob).
				WithObjects(&v1beta1.BaseModel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-model",
						Namespace: "default",
					},
					Spec: v1beta1.BaseModelSpec{
						ModelFormat: v1beta1.ModelFormat{
							Name: "onnx",
						},
						Storage: &v1beta1.StorageSpec{
							Path: pointer.String("oci://bucket/model"),
						},
					},
				}).
				WithObjects(&v1beta1.InferenceService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc",
						Namespace: "default",
					},
					Spec: v1beta1.InferenceServiceSpec{
						Predictor: v1beta1.PredictorSpec{
							Model: &v1beta1.ModelSpec{
								BaseModel: pointer.String("test-model"),
							},
						},
					},
					Status: v1beta1.InferenceServiceStatus{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-isvc.default.svc.cluster.local",
						},
					},
				}).
				Build()

			r := &BenchmarkJobReconciler{
				Client:    client,
				Clientset: kfake.NewSimpleClientset(),
				Scheme:    scheme,
			}

			err := r.cleanupResources(tt.benchmarkJob)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
