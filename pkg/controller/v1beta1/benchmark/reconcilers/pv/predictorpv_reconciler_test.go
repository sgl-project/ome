package benchmarkjobpv

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

func TestReconcilePV(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		benchmarkJob  *v1beta1.BenchmarkJob
		baseModelName string
		baseModelSpec *v1beta1.BaseModelSpec
		wantErr       bool
		expectedPV    *corev1.PersistentVolume
	}{
		{
			name: "successfully create new PV",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
			},
			baseModelName: "test-model",
			baseModelSpec: &v1beta1.BaseModelSpec{
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/test/path"),
				},
			},
			wantErr: false,
			expectedPV: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.PVName("test-job", "test-ns", "test-model"),
					Annotations: map[string]string{
						"benchmarkJob": "test-job",
						"namespace":    "test-ns",
						"path":         "/test/path",
					},
				},
				Spec: corev1.PersistentVolumeSpec{
					StorageClassName: "manual",
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1000Gi"),
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/test/path",
						},
					},
				},
			},
		},
		{
			name: "PV already exists",
			benchmarkJob: &v1beta1.BenchmarkJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-job",
					Namespace: "test-ns",
				},
			},
			baseModelName: "test-model",
			baseModelSpec: &v1beta1.BaseModelSpec{
				Storage: &v1beta1.StorageSpec{
					Path: stringPtr("/test/path"),
				},
			},
			wantErr: false,
			expectedPV: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.PVName("existing-job", "test-ns", "test-model"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clients
			fakeClient := fakectrl.NewClientBuilder().WithScheme(scheme).Build()
			fakeClientset := fakekube.NewSimpleClientset()

			// If testing existing PV case, create it first
			if tt.name == "PV already exists" {
				_, err := fakeClientset.CoreV1().PersistentVolumes().Create(context.TODO(), tt.expectedPV, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			reconciler := NewBenchmarkJobPVReconciler(fakeClient, fakeClientset, scheme)
			result, err := reconciler.Reconcile(tt.benchmarkJob, tt.baseModelName, tt.baseModelSpec)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, result.Requeue, false)

			// Verify PV was created correctly
			pv, err := fakeClientset.CoreV1().PersistentVolumes().Get(
				context.TODO(),
				constants.PVName(tt.benchmarkJob.Name, tt.benchmarkJob.Namespace, tt.baseModelName),
				metav1.GetOptions{},
			)
			if tt.name == "successfully create new PV" {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPV.Name, pv.Name)
				assert.Equal(t, tt.expectedPV.Annotations, pv.Annotations)
				assert.Equal(t, tt.expectedPV.Spec.StorageClassName, pv.Spec.StorageClassName)
				assert.Equal(t, tt.expectedPV.Spec.Capacity, pv.Spec.Capacity)
				assert.Equal(t, tt.expectedPV.Spec.AccessModes, pv.Spec.AccessModes)
				assert.Equal(t, tt.expectedPV.Spec.PersistentVolumeReclaimPolicy, pv.Spec.PersistentVolumeReclaimPolicy)
				assert.Equal(t, tt.expectedPV.Spec.PersistentVolumeSource.HostPath.Path, pv.Spec.PersistentVolumeSource.HostPath.Path)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPV.Name, pv.Name)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
