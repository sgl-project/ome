package training

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	trainingruntime "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
	testing2 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

func TestReconcileObjectsTreatsExistingJobSetAsCreateSucceeded(t *testing.T) {
	ctx := context.Background()
	jobSet := testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").Obj()
	fakeClient := testing2.NewClientBuilder().
		WithObjects(jobSet.DeepCopy()).
		Build()
	updateTrackingClient := &jobSetUpdateTrackingClient{Client: fakeClient}
	reconciler := &TrainingJobReconciler{
		Client: updateTrackingClient,
		Log:    ctrl.Log.WithName("test"),
	}

	opState, err := reconciler.reconcileObjects(
		ctx,
		fakeRuntime{objects: []client.Object{jobSet.DeepCopy()}},
		testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").Obj(),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("reconcileObjects() returned unexpected error: %v", err)
	}
	if opState != CreateObjectSucceeded {
		t.Fatalf("reconcileObjects() state = %s, want %s", opState, CreateObjectSucceeded)
	}
	if updateTrackingClient.jobSetUpdates != 0 {
		t.Fatalf("JobSet updates = %d, want 0", updateTrackingClient.jobSetUpdates)
	}
}

type jobSetUpdateTrackingClient struct {
	client.Client
	jobSetUpdates int
}

func (c *jobSetUpdateTrackingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*jobsetv1alpha2.JobSet); ok {
		c.jobSetUpdates++
		return fmt.Errorf("unexpected JobSet update")
	}
	return c.Client.Update(ctx, obj, opts...)
}

type fakeRuntime struct {
	objects []client.Object
}

func (f fakeRuntime) NewObjects(context.Context, *omev1beta1.TrainingJob, *string, *corev1.Affinity) ([]client.Object, error) {
	return f.objects, nil
}

func (f fakeRuntime) TerminalCondition(context.Context, *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	return nil, nil
}

func (f fakeRuntime) EventHandlerRegistrars() []trainingruntime.ReconcilerBuilder {
	return nil
}

func (f fakeRuntime) ValidateObjects(context.Context, *omev1beta1.TrainingJob, *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	return nil, nil
}

var _ trainingruntime.Runtime = fakeRuntime{}
