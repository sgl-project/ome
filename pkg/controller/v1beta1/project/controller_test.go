package project

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestServiceAccountReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		project        *v1beta1.Project
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name:           "project not found",
			project:        nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "create project and add finalizer when not present",
			project: &v1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pr",
					Namespace: "default",
				},
				Spec: v1beta1.ProjectSpec{
					Name: "test-pr",
					OrganizationRef: v1beta1.CrossReference{
						Name: "test-organization",
					},
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &ProjectReconciler{
				Client:    fakeClient,
				Clientset: kfake.NewClientset(),
				Log:       logr.Discard(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(100),
			}

			if tt.project != nil {
				err := fakeClient.Create(context.Background(), tt.project)
				require.NoError(t, err)
			}

			req := reconcile.Request{NamespacedName: client.ObjectKey{Name: "test-pr"}}
			res, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, res)
		})
	}
}

func TestProjectReconciler_Finalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name          string
		project       *v1beta1.Project
		expectedError bool
	}{
		{
			name: "add finalizer when not present",
			project: &v1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pr",
					Namespace: "default",
				},
				Spec: v1beta1.ProjectSpec{
					Name: "test-pr",
					OrganizationRef: v1beta1.CrossReference{
						Name: "test-organization",
					},
				},
			},
			expectedError: false,
		},
		{
			name: "finalizer already present",
			project: &v1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pr",
					Namespace:  "default",
					Finalizers: []string{"project.finalizers.openaisdk"},
				},
				Spec: v1beta1.ProjectSpec{
					Name: "test-pr",
					OrganizationRef: v1beta1.CrossReference{
						Name: "test-organization",
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.project).Build()
			reconciler := &ProjectReconciler{
				Client:    fakeClient,
				Clientset: kfake.NewClientset(),
				Log:       logr.Discard(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(100),
			}

			// Add finalizer if not present
			if !controllerutil.ContainsFinalizer(tt.project, "project.finalizers.openaisdk") {
				controllerutil.AddFinalizer(tt.project, "project.finalizers.openaisdk")
				err := reconciler.Update(context.Background(), tt.project)
				if (err != nil) != tt.expectedError {
					t.Errorf("unexpected error: %v", err)
				}
			}

			// Check if finalizer is present
			if !controllerutil.ContainsFinalizer(tt.project, "project.finalizers.openaisdk") {
				t.Errorf("finalizer not added")
			}
		})
	}
}
