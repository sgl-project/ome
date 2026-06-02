package replicationjob

import (
	"context"
	"strings"
	"testing"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	opensourcev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const (
	validSrc  = "oci://n/source_namespace/b/source_bucket/o/source_prefix"
	validDest = "oci://n/dest_namespace/b/dest_bucket/o/dest_prefix"
)

func TestReplicationJobReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))

	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
		expectedResult ctrl.Result
		expectedError  bool
		expectedState  v1beta1.ReplicationJobPhase
	}{
		{
			name:           "ReplicationJob not found",
			replicationJob: nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
			expectedState:  "",
		},
		{
			name:           "ReplicationJob with deletion timestamp",
			replicationJob: createTestReplicationJob("test-job", &metav1.Time{Time: time.Now()}),
			expectedResult: ctrl.Result{},
			expectedError:  false,
			expectedState:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := cfake.NewClientBuilder().WithScheme(scheme)
			if tt.replicationJob != nil {
				clientBuilder = clientBuilder.
					WithObjects(tt.replicationJob).
					WithStatusSubresource(tt.replicationJob)
			}
			fakeClient := clientBuilder.Build()

			reconciler := &ReplicationJobReconciler{
				Client:    fakeClient,
				Clientset: kubernetes.Interface(nil),
				Log:       zap.New(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(10),
			}

			ctx := context.Background()
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-job",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			if tt.replicationJob != nil && tt.replicationJob.DeletionTimestamp == nil {
				updatedCcr := &v1beta1.ReplicationJob{}
				err = fakeClient.Get(ctx, req.NamespacedName, updatedCcr)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, updatedCcr.Status.Status)
			}
		})
	}
}

func createTestReplicationJob(name string, deletionTimestamp *metav1.Time) *v1beta1.ReplicationJob {
	return &v1beta1.ReplicationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			DeletionTimestamp: deletionTimestamp,
			Finalizers:        []string{constants.ReplicationJobFinalizer},
		},
		Spec: v1beta1.ReplicationJobSpec{
			Source: &opensourcev1beta1.StorageSpec{
				StorageUri: utils.Ptr(validSrc),
			},
			Destination: &opensourcev1beta1.StorageSpec{
				StorageUri: utils.Ptr(validDest),
			},
		},
	}
}

func TestReplicationJobReconciler_ensureFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
		expectedError  bool
	}{
		{
			name: "add finalizer when not present",
			replicationJob: &v1beta1.ReplicationJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			expectedError: false,
		},
		{
			name: "finalizer already present",
			replicationJob: &v1beta1.ReplicationJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-job",
					Namespace:  "default",
					Finalizers: []string{constants.ReplicationJobFinalizer},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.replicationJob).
				Build()

			r := &ReplicationJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			if !controllerutil.ContainsFinalizer(tt.replicationJob, constants.ReplicationJobFinalizer) {
				controllerutil.AddFinalizer(tt.replicationJob, constants.ReplicationJobFinalizer)
				err := r.Update(context.Background(), tt.replicationJob)
				if (err != nil) != tt.expectedError {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if !controllerutil.ContainsFinalizer(tt.replicationJob, constants.ReplicationJobFinalizer) {
				t.Errorf("finalizer not added")
			}
		})
	}
}

func TestReplicationJobReconciler_handleDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
		expectedError  bool
	}{
		{
			name: "successful deletion",
			replicationJob: &v1beta1.ReplicationJob{
				TypeMeta: metav1.TypeMeta{
					APIVersion: strings.Join([]string{constants.OMEAPIGroupName, constants.OMEAPIVersion}, "/"),
					Kind:       constants.ReplicationJobKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-job",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{constants.ReplicationJobFinalizer},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.replicationJob).
				Build()

			r := &ReplicationJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			result, err := r.handleDeletion(context.Background(), tt.replicationJob)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, ctrl.Result{}, result)
			}
		})
	}
}

func TestReplicationJobReconciler_buildMetadata(t *testing.T) {
	replicationJob := &v1beta1.ReplicationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	r := &ReplicationJobReconciler{}
	meta := r.buildMetadata(replicationJob)

	expectedLabels := map[string]string{
		constants.ReplicationJobLabel: replicationJob.Name,
		"logging-forward":             "enabled",
	}

	assert.Equal(t, replicationJob.Name, meta.Name)
	assert.Equal(t, replicationJob.Namespace, meta.Namespace)
	assert.Equal(t, expectedLabels, meta.Labels)
}

func TestReplicationJobReconciler_reconcileJob(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
		podSpec        *corev1.PodSpec
		meta           metav1.ObjectMeta
		expectedError  bool
	}{
		{
			name: "successful job reconciliation",
			replicationJob: &v1beta1.ReplicationJob{
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
				WithObjects(tt.replicationJob).
				Build()

			r := &ReplicationJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			_, err := r.reconcileJob(tt.replicationJob, tt.podSpec, tt.meta)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReplicationJobReconciler_reconcilePodSpec(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	tests := []struct {
		name                 string
		replicationJob       *v1beta1.ReplicationJob
		replicationJobConfig *controllerconfig.ReplicationJobConfig
		expectedError        bool
	}{
		{
			name:           "successful pod spec creation",
			replicationJob: createTestReplicationJob("test-job", nil),
			replicationJobConfig: &controllerconfig.ReplicationJobConfig{
				PodConfig: controllerconfig.PodConfig{
					Image: "test-image",
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("100Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("100Gi"),
						},
					},
				},
				Source: controllerconfig.StorageAccessConfig{
					AuthType:       "instance_principal",
					EnableOboToken: true,
					OboToken:       "test-obo-token",
				},
				Target: controllerconfig.StorageAccessConfig{
					AuthType:       "instance_principal",
					EnableOboToken: false,
				},
				EnableChecksumUpload: true,
				ChecksumAlgorithm:    "sha256",
				DownloadSizeLimit:    "3072",
				EnableSizeLimitCheck: true,
				CompartmentId:        "compart1234",
			},
			expectedError: false,
		},
		{
			name:           "test podOverride",
			replicationJob: createTestReplicationJobWithPodOverride("test-job", nil),
			replicationJobConfig: &controllerconfig.ReplicationJobConfig{
				PodConfig: controllerconfig.PodConfig{
					Image: "test-image2",
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("16"),
							corev1.ResourceMemory: resource.MustParse("256Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("100Gi"),
						},
					},
				},
				Source: controllerconfig.StorageAccessConfig{
					AuthType:       "instance_principal",
					EnableOboToken: false,
				},
				Target: controllerconfig.StorageAccessConfig{
					AuthType:       "instance_principal",
					EnableOboToken: false,
				},
				EnableChecksumUpload: false,
				DownloadSizeLimit:    "3072",
				EnableSizeLimitCheck: true,
				CompartmentId:        "compart1234",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.replicationJob).
				Build()

			r := &ReplicationJobReconciler{
				Client: client,
				Scheme: scheme,
			}

			_, podSpec, err := r.reconcilePodSpec(tt.replicationJob, tt.replicationJobConfig)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, podSpec)
				assert.Equal(t, tt.replicationJobConfig.PodConfig.Image, podSpec.Containers[0].Image)
			}
		})
	}
}

func createTestReplicationJobWithPodOverride(name string, deletionTimestamp *metav1.Time) *v1beta1.ReplicationJob {
	return &v1beta1.ReplicationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			DeletionTimestamp: deletionTimestamp,
			Finalizers:        []string{constants.ReplicationJobFinalizer},
		},
		Spec: v1beta1.ReplicationJobSpec{
			Source: &opensourcev1beta1.StorageSpec{
				StorageUri: utils.Ptr(validSrc),
			},
			Destination: &opensourcev1beta1.StorageSpec{
				StorageUri: utils.Ptr(validDest),
			},
			ContainerOverride: &corev1.Container{
				Name:  "test-container2",
				Image: "test-image2",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("256Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("100Gi"),
					},
				},
			},
		},
	}
}

func TestReplicationJobReconciler_buildArgs(t *testing.T) {
	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
	}{
		{
			name:           "successfully build args",
			replicationJob: createTestReplicationJob("test-job", nil),
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
				WithObjects(tt.replicationJob).
				Build()

			r := &ReplicationJobReconciler{
				Client: client,
			}

			args := r.buildJobArgs()

			if len(args) == 0 {
				t.Error("buildJobArgs() args is empty")
			}
			// Verify that "replica" is the first argument
			if len(args) > 0 && args[0] != "replica" {
				t.Errorf("buildJobArgs() first argument = %v, want 'replica'", args[0])
			}
			// Verify that "--config" is present
			hasConfig := false
			for _, arg := range args {
				if arg == "--config" {
					hasConfig = true
				}
			}
			if !hasConfig {
				t.Error("buildJobArgs() missing '--config' argument")
			}
		},
		)
	}
}

func TestReplicationJobReconciler_updateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		replicationJob *v1beta1.ReplicationJob
		existingJob    *batchv1.Job
		existingPod    *corev1.Pod
		expectedError  bool
	}{
		{
			name: "job not found",
			replicationJob: &v1beta1.ReplicationJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			},
			existingJob:   nil,
			existingPod:   nil,
			expectedError: false,
		},
		{
			name: "job exists and completed",
			replicationJob: &v1beta1.ReplicationJob{
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
					CompletionTime: &metav1.Time{Time: time.Now()},
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels: map[string]string{
						"job-name": "test-job",
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-container",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode:   0,
									FinishedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "job exists and fails on client error",
			replicationJob: &v1beta1.ReplicationJob{
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
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels: map[string]string{
						"job-name": "test-job",
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-container",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode:   1,
									Reason:     "Error",
									Message:    "401 unauthorized",
									FinishedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "job exists and fails on node error",
			replicationJob: &v1beta1.ReplicationJob{
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
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			existingPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels: map[string]string{
						"job-name": "test-job",
					},
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:    corev1.PodConditionType("DisruptionTarget"),
							Status:  corev1.ConditionTrue,
							Reason:  "TerminationByKubelet",
							Message: "The node was low on resource: ephemeral-storage. Threshold quantity: 1142927001550, available: 1112205360Ki.",
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicationJobCopy := tt.replicationJob.DeepCopy()
			replicationJobCopy.ResourceVersion = ""
			replicationJobCopy.Status = v1beta1.ReplicationJobStatus{}
			replicationJobCopy.SetGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind(constants.ReplicationJobKind))

			clientBuilder := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(replicationJobCopy).
				WithStatusSubresource(tt.replicationJob)

			if tt.existingJob != nil {
				tt.existingJob.SetGroupVersionKind(batchv1.SchemeGroupVersion.WithKind("Job"))
				clientBuilder = clientBuilder.WithObjects(tt.existingJob)
			}
			if tt.existingPod != nil {
				tt.existingPod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
				clientBuilder = clientBuilder.WithObjects(tt.existingPod)
			}

			client := clientBuilder.Build()

			r := &ReplicationJobReconciler{
				Client: client,
				Scheme: scheme,
			}
			if tt.name == "job exists and completed" {
				r.PodLogReader = func(ctx context.Context, namespace, podName, containerName string) (string, error) {
					return `{"level":"info","ts":1775704325.9316795,"caller":"replica/replica.go:91","msg":"Total model size: 123456789 bytes"}`, nil
				}
			}

			err := r.updateStatus(context.Background(), replicationJobCopy, logr.Logger{})
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.name == "job exists and fails on client error" {
				// Fetch the updated ReplicationJob from the client after status update
				updated := &v1beta1.ReplicationJob{}
				err := client.Get(context.Background(), ctrlclient.ObjectKey{
					Name:      replicationJobCopy.Name,
					Namespace: replicationJobCopy.Namespace,
				}, updated)
				assert.NoError(t, err)

				// Find the condition of type ClientError
				found := false
				for _, cond := range updated.Status.Conditions {
					if cond.Type == constants.ClientError && cond.Status == metav1.ConditionTrue {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected a status condition type ClientError with status True")
			}

			if tt.name == "job exists and fails on node error" {
				// Fetch the updated ReplicationJob from the client after status update
				updated := &v1beta1.ReplicationJob{}
				err := client.Get(context.Background(), ctrlclient.ObjectKey{
					Name:      replicationJobCopy.Name,
					Namespace: replicationJobCopy.Namespace,
				}, updated)
				assert.NoError(t, err)

				// Assert that there is a condition with Type NodeError and Status True
				found := false
				for _, cond := range updated.Status.Conditions {
					if cond.Type == constants.NodeError && cond.Status == metav1.ConditionTrue {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected a status condition with Type NodeError and Status True")
			}

			if tt.name == "job exists and completed" {
				updated := &v1beta1.ReplicationJob{}
				err := client.Get(context.Background(), ctrlclient.ObjectKey{
					Name:      replicationJobCopy.Name,
					Namespace: replicationJobCopy.Namespace,
				}, updated)
				assert.NoError(t, err)
				require.NotNil(t, updated.Status.Observed)
				require.NotNil(t, updated.Status.Observed.SourceArtifactSizeBytes)
				assert.Equal(t, int64(123456789), *updated.Status.Observed.SourceArtifactSizeBytes)
			}
		})
	}
}

func TestParseSourceArtifactSizeBytesFromLog(t *testing.T) {
	tests := []struct {
		name      string
		logs      string
		want      *int64
		wantError bool
	}{
		{
			name: "parses size from json log line",
			logs: `{"level":"info","ts":1775704325.9316795,"caller":"replica/replica.go:91","msg":"Total model size: 230162590339 bytes"}`,
			want: utils.Ptr(int64(230162590339)),
		},
		{
			name: "uses the last matching line",
			logs: strings.Join([]string{
				`{"msg":"Total model size: 111 bytes"}`,
				`{"msg":"Total model size: 222 bytes"}`,
			}, "\n"),
			want: utils.Ptr(int64(222)),
		},
		{
			name:      "returns error when no size is present",
			logs:      `{"msg":"Replication completed"}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSourceArtifactSizeBytesFromLog(tt.logs)
			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}
