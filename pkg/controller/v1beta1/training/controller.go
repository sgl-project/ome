package training

import (
	"context"
	"errors"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	trainingruntimes "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// TrainingJobReconciler reconciles a TrainingJob object
type TrainingJobReconciler struct {
	Client   client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Runtimes map[string]trainingruntimes.Runtime
}

type ObjectOperationState string

var errorUnsupportedRuntime = errors.New("the specified runtime is not supported")

const (
	CreateObjectSucceeded ObjectOperationState = "CreateObjectSucceeded"
	BuildObjectFailed     ObjectOperationState = "BuildObjectFailed"
	CreateObjectFailed    ObjectOperationState = "CreateObjectFailed"
	UpdateObjectFailed    ObjectOperationState = "UpdateObjectFailed"
)

// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs/finalizers,verbs=get;update;patch

func (r *TrainingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var trainJob v1beta1.TrainingJob
	if err := r.Client.Get(ctx, req.NamespacedName, &trainJob); err != nil {
		if apierr.IsNotFound(err) {
			r.Log.Error(err, "TrainingJob not found", "namespace", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Error getting TrainingJob", "namespace", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling training job", "namespace", req.NamespacedName)
	if isTrainJobFinished(&trainJob) {
		r.Log.Info("TrainJob has already been finished", "namespace", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// We use these 2 annotations for every training job to inject init-container and sidecar container. The values will be passed into jobset object, then the pod underneath.
	if trainJob.Spec.Annotations == nil {
		trainJob.Spec.Annotations = make(map[string]string)
	}
	trainJob.Spec.Annotations[constants.TrainingSidecarInjectionKey] = "true"
	trainJob.Spec.Annotations[constants.ModelInitInjectionKey] = "true"

	runtimeRefGK := runtimeRefToGroupKind(trainJob.Spec.RuntimeRef).String()
	runtime, ok := r.Runtimes[runtimeRefGK]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("%w, %s", errorUnsupportedRuntime, runtimeRefGK)
	}

	opState, err := r.reconcileObjects(ctx, runtime, &trainJob, req)

	originStatus := trainJob.Status.DeepCopy()
	updateSuspendedCondition(&trainJob)
	updateCreatedCondition(&trainJob, opState)
	if terminalCondErr := updateTerminalCondition(ctx, runtime, &trainJob); terminalCondErr != nil {
		return ctrl.Result{}, errors.Join(err, terminalCondErr)
	}
	if !equality.Semantic.DeepEqual(&trainJob, originStatus) {
		return ctrl.Result{}, errors.Join(err, r.Client.Status().Update(ctx, &trainJob))
	}

	return ctrl.Result{}, nil
}

func (r *TrainingJobReconciler) reconcileObjects(ctx context.Context, runtime trainingruntimes.Runtime, trainJob *v1beta1.TrainingJob, req ctrl.Request) (ObjectOperationState, error) {
	objs, err := runtime.NewObjects(ctx, trainJob)
	if err != nil {
		return BuildObjectFailed, err
	}
	for _, obj := range objs {
		var gvk schema.GroupVersionKind
		if gvk, err = apiutil.GVKForObject(obj.DeepCopyObject(), r.Client.Scheme()); err != nil {
			return BuildObjectFailed, err
		}
		logKeysAndValues := []any{
			"groupVersionKind", gvk.String(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
		}
		// Non-empty resourceVersion indicates UPDATE operation.
		var creationErr error
		var created bool
		if obj.GetResourceVersion() == "" {
			creationErr = r.Client.Create(ctx, obj)
			created = creationErr == nil
		}
		switch {
		case created:
			r.Log.Info("Successfully created object", "namespace", req.NamespacedName, logKeysAndValues)
			continue
		case client.IgnoreAlreadyExists(creationErr) != nil:
			return CreateObjectFailed, creationErr
		default:
			// This indicates CREATE operation has not been performed or the object has already existed in the cluster.
			if err = r.Client.Update(ctx, obj); err != nil {
				return UpdateObjectFailed, err
			}
			r.Log.Info("Successfully updated object", "namespace", req.NamespacedName, logKeysAndValues)
		}
	}
	return CreateObjectSucceeded, nil
}

func updateCreatedCondition(trainJob *v1beta1.TrainingJob, opState ObjectOperationState) {
	var newCond metav1.Condition
	switch opState {
	case CreateObjectSucceeded:
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobCreated,
			Status:  metav1.ConditionTrue,
			Message: v1beta1.TrainJobJobsCreationSucceededMessage,
			Reason:  v1beta1.TrainJobJobsCreationSucceededReason,
		}
	case BuildObjectFailed:
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobCreated,
			Status:  metav1.ConditionFalse,
			Message: v1beta1.TrainJobJobsBuildFailedMessage,
			Reason:  v1beta1.TrainJobJobsBuildFailedReason,
		}
	case CreateObjectFailed, UpdateObjectFailed:
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobCreated,
			Status:  metav1.ConditionFalse,
			Message: v1beta1.TrainJobJobsCreationFailedMessage,
			Reason:  v1beta1.TrainJobJobsCreationFailedReason,
		}
	default:
		return
	}
	meta.SetStatusCondition(&trainJob.Status.Conditions, newCond)
}

func updateSuspendedCondition(trainJob *v1beta1.TrainingJob) {
	var newCond metav1.Condition
	switch {
	case ptr.Deref(trainJob.Spec.Suspend, false):
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobSuspended,
			Status:  metav1.ConditionTrue,
			Message: v1beta1.TrainJobSuspendedMessage,
			Reason:  v1beta1.TrainJobSuspendedReason,
		}
	case meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobSuspended):
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobSuspended,
			Status:  metav1.ConditionFalse,
			Message: v1beta1.TrainJobResumedMessage,
			Reason:  v1beta1.TrainJobResumedReason,
		}
	default:
		return
	}
	meta.SetStatusCondition(&trainJob.Status.Conditions, newCond)
}

func updateTerminalCondition(ctx context.Context, runtime trainingruntimes.Runtime, trainJob *v1beta1.TrainingJob) error {
	terminalCondition, err := runtime.TerminalCondition(ctx, trainJob)
	if err != nil {
		return err
	}
	if terminalCondition != nil {
		meta.SetStatusCondition(&trainJob.Status.Conditions, *terminalCondition)
	}
	return nil
}

func isTrainJobFinished(trainJob *v1beta1.TrainingJob) bool {
	return meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobComplete) ||
		meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobFailed)
}

func runtimeRefToGroupKind(runtimeRef omev1beta1.RuntimeRef) schema.GroupKind {
	return schema.GroupKind{
		Group: ptr.Deref(runtimeRef.APIGroup, "ome.io"),
		Kind:  ptr.Deref(runtimeRef.Kind, "ClusterTrainingRuntime"),
	}
}

func (r *TrainingJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.TrainingJob{})
	for _, runtime := range r.Runtimes {
		for _, registrar := range runtime.EventHandlerRegistrars() {
			if registrar != nil {
				b = registrar(b, mgr.GetClient(), mgr.GetCache())
			}
		}
	}
	return b.Complete(r)
}
