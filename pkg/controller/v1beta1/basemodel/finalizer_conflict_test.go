package basemodel

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/shared"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

type finalizerTestBackend struct{ reconciled bool }

func (b *finalizerTestBackend) Name() string                          { return "test" }
func (b *finalizerTestBackend) Matches(_ *v1beta1.BaseModelSpec) bool { return true }
func (b *finalizerTestBackend) Reconcile(_ context.Context, _ shared.BackendArgs) (ctrl.Result, error) {
	b.reconciled = true
	return ctrl.Result{}, nil
}
func (b *finalizerTestBackend) HandleDeletion(_ context.Context, _ shared.BackendArgs) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func TestReconcileModel_FinalizerWriteOutcome_OnlyConflictRequeuesQuietly(t *testing.T) {
	conflict := apierrors.NewConflict(
		schema.GroupResource{Group: "ome.io", Resource: "clusterbasemodels"},
		"model", errors.New("the object has been modified"))
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "ome.io", Resource: "clusterbasemodels"},
		"model", errors.New("forbidden"))

	tests := []struct {
		name        string
		updateErr   error
		wantResult  ctrl.Result
		wantErr     error
		wantBackend bool
	}{
		{
			name:        "successful write reaches the backend",
			wantBackend: true,
		},
		{
			name:       "conflict requeues without an error",
			updateErr:  conflict,
			wantResult: ctrl.Result{Requeue: true},
		},
		{
			name:      "non-conflict error is returned",
			updateErr: forbidden,
			wantErr:   forbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := &v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "model"}}
			opts := []fakeclient.Option{fakeclient.WithObjects(obj)}
			if test.updateErr != nil {
				opts = append(opts, fakeclient.FailingVerb(fakeclient.Update, fakeclient.EveryCall, test.updateErr))
			}
			c := fakeclient.New(t, opts...)

			backend := &finalizerTestBackend{}
			result, err := reconcileModel(
				context.Background(), c, c.Scheme(), logr.Discard(),
				[]shared.Backend{backend}, obj.DeepCopy(),
				modelKind{name: "ClusterBaseModel", finalizer: "test.ome.io/finalizer", clusterScoped: true},
			)

			if result != test.wantResult {
				t.Errorf("result = %+v, want %+v", result, test.wantResult)
			}
			if test.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Errorf("error = %v, want %v", err, test.wantErr)
			}
			if backend.reconciled != test.wantBackend {
				t.Errorf("backend reconciled = %v, want %v", backend.reconciled, test.wantBackend)
			}
		})
	}
}
