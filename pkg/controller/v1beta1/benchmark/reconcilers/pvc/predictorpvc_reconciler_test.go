package benchmarkjobpvc

import (
	"context"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekube "k8s.io/client-go/kubernetes/fake"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcilePVC(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		baseModelName string
		wantErr       bool
		expectedPVC   *corev1.PersistentVolumeClaim
	}{
		{
			name: "successfully create new PVC",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
			},
			baseModelName: "test-model",
			wantErr:       false,
			expectedPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.PVCName("test-job", "test-model"),
					Namespace: "test-ns",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: stringPtr("manual"),
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("999Gi"),
						},
					},
					VolumeName: constants.PVName("test-job", "test-ns", "test-model"),
				},
			},
		},
		{
			name: "PVC already exists",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-job",
					Namespace: "test-ns",
				},
			},
			baseModelName: "test-model",
			wantErr:       false,
			expectedPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.PVCName("existing-job", "test-model"),
					Namespace: "test-ns",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clients
			fakeClient := fakectrl.NewClientBuilder().WithScheme(scheme).Build()
			fakeClientset := fakekube.NewSimpleClientset()

			// If testing existing PVC case, create it first
			if tt.name == "PVC already exists" {
				_, err := fakeClientset.CoreV1().PersistentVolumeClaims(tt.benchmarkJob.Namespace).Create(
					context.TODO(),
					tt.expectedPVC,
					metav1.CreateOptions{},
				)
				assert.NoError(t, err)
			}

			reconciler := NewBenchmarkJobPVCReconciler(fakeClient, fakeClientset, scheme)
			result, err := reconciler.Reconcile(tt.benchmarkJob, tt.baseModelName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, result.Requeue, false)

			// Verify PVC was created correctly
			pvc, err := fakeClientset.CoreV1().PersistentVolumeClaims(tt.benchmarkJob.Namespace).Get(
				context.TODO(),
				constants.PVCName(tt.benchmarkJob.Name, tt.baseModelName),
				metav1.GetOptions{},
			)
			if tt.name == "successfully create new PVC" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPVC.Name, pvc.Name)
				assert.Equal(t, tt.expectedPVC.Namespace, pvc.Namespace)
				assert.Equal(t, tt.expectedPVC.Spec.StorageClassName, pvc.Spec.StorageClassName)
				assert.Equal(t, tt.expectedPVC.Spec.AccessModes, pvc.Spec.AccessModes)
				assert.Equal(t, tt.expectedPVC.Spec.Resources, pvc.Spec.Resources)
				assert.Equal(t, tt.expectedPVC.Spec.VolumeName, pvc.Spec.VolumeName)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPVC.Name, pvc.Name)
				assert.Equal(t, tt.expectedPVC.Namespace, pvc.Namespace)
			}
		})
	}
}
