package serviceaccount

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// TestServiceAccountReconciler_Reconcile tests the reconciliation process of the ServiceAccountReconciler.
//
// This test uses a table-driven approach to verify different scenarios:
// 1. When the service account is not found, it expects no error and an empty result.
// 2. When a service account exists, it expects the creation of a secret with the correct API key.
//
// Each test case initializes its own fake client and reconciler, ensuring isolation between tests.
// The verifyFunc is used to assert the expected state of the system after reconciliation.
func TestServiceAccountReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name           string
		serviceAccount *v1beta1.ServiceAccount
		expectedResult ctrl.Result
		expectedError  bool
		verifyFunc     func(t *testing.T, client client.Client)
	}{
		{
			name:           "service account not found",
			serviceAccount: nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
			verifyFunc: func(t *testing.T, client client.Client) {
				// No verification needed as the service account is not found
			},
		},
		{
			name: "create secret for service account",
			serviceAccount: &v1beta1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
				},
				Spec: v1beta1.ServiceAccountSpec{
					Name: "test-sa",
					ProjectRef: v1beta1.CrossReference{
						Name: "test-project",
					},
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  true, // TODO: change to false and uncomment verifyFunc after implementation of project controller
			// verifyFunc: func(t *testing.T, actualClient client.Client) {
			// 	secret := &corev1.Secret{}
			// 	err := actualClient.Get(context.Background(), client.ObjectKey{Name: "test-sa-apikey", Namespace: "default"}, secret)
			// 	require.NoError(t, err)
			// 	assert.Equal(t, []byte("real-api-key-value"), secret.Data["api-key"])
			// },
			verifyFunc: func(t *testing.T, client client.Client) {
				// No verification needed as the service account is not found
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &ServiceAccountReconciler{
				Client:    fakeClient,
				Clientset: kfake.NewClientset(),
				Log:       logr.Discard(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(100),
			}

			if tt.serviceAccount != nil {
				err := fakeClient.Create(context.Background(), tt.serviceAccount)
				require.NoError(t, err)
			}

			req := reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-sa", Namespace: "default"}}
			res, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, res)

			tt.verifyFunc(t, fakeClient)
		})
	}
}

func TestServiceAccountReconciler_getProjectID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		serviceAccount *v1beta1.ServiceAccount
		expectedError  bool
		expectedID     string
	}{
		{
			name:           "service account with no project ID",
			serviceAccount: &v1beta1.ServiceAccount{},
			expectedError:  true,
			expectedID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &ServiceAccountReconciler{
				Client:   fakeClient,
				Log:      logr.Discard(),
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(100),
			}

			ctx := context.Background()
			projectID, err := reconciler.getProjectID(ctx, tt.serviceAccount)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedID, projectID)
		})
	}
}

func TestServiceAccountReconciler_handleDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		serviceAccount *v1beta1.ServiceAccount
		expectedError  bool
	}{
		{
			name:           "handle deletion with no finalizer",
			serviceAccount: &v1beta1.ServiceAccount{},
			expectedError:  true,
		},
		// TODO: add successful deletion test after Project Controller is done
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &ServiceAccountReconciler{
				Client:   fakeClient,
				Log:      logr.Discard(),
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(100),
			}

			ctx := context.Background()
			err := reconciler.handleDeletion(ctx, tt.serviceAccount)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
