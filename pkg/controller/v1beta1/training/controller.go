package training

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	trainingJobUtils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"context"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

// TrainingJobReconciler reconciles a TrainingJob object
type TrainingJobReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//	the TrainingJob object against the actual cluster state, and then
//	perform operations to make the cluster state reflect the state specified by
//	the user.
func (r *TrainingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the TrainingJob instance
	trainingJob := &v1beta1.TrainingJob{}
	if err := r.Get(ctx, req.NamespacedName, trainingJob); err != nil {
		if apierr.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		r.Log.Error(err, "Unable to get TrainingJob", "namespace", req.NamespacedName)
		return reconcile.Result{}, err
	}

	// Name of trainingjob finalizer
	finalizerName := "trainingjob.finalizers"
	// examine DeletionTimestamp to determine if object is under deletion
	if trainingJob.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !utils.Includes(trainingJob.ObjectMeta.Finalizers, finalizerName) {
			trainingJob.ObjectMeta.Finalizers = append(trainingJob.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), trainingJob); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if utils.Includes(trainingJob.ObjectMeta.Finalizers, finalizerName) {
			// remove our finalizer from the list and update it.
			trainingJob.ObjectMeta.Finalizers = utils.RemoveString(trainingJob.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), trainingJob); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobSucceeded || trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobFailed {
		return ctrl.Result{}, nil
	}

	ftModel := &v1beta1.FineTunedWeight{}
	if err := r.Get(ctx, types.NamespacedName{Name: trainingJobUtils.GetFineTunedModelName(trainingJob.Name)}, ftModel); err != nil {
		if apierr.IsNotFound(err) {
			// Model not found, create a new one
			ftModel = r.createFTWeight(trainingJob)
			if err = r.Create(ctx, ftModel); err != nil {
				if apierr.IsAlreadyExists(err) {
					// Requeue it when model already exists
					return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
				} else {
					r.Log.Error(err, "Failed to create Model", "trainingJob", trainingJob.Name, "model", ftModel.Name)
				}
			} else {
				r.Log.Info("Model created", "trainingJob", trainingJob.Name, "model", ftModel.Name)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
		} else {
			r.Log.Error(err, "Failed to get Model", "trainingJob", trainingJob.Name, "model", ftModel.Name)
		}
		r.Recorder.Eventf(trainingJob, v1.EventTypeWarning, "InternalError", err.Error())

		// Todo: Emit failure metrics

		return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobFailed, err.Error())
	}

	if ftModel.Status.State == v1beta1.LifeCycleStateReady || ftModel.Status.State == v1beta1.LifeCycleStateFailed {
		return ctrl.Result{}, nil
	}

	if ftModel.Status.State == "" {
		// Update model status to 'Creating' State with active job ref set
		if err := r.updateFTModelStatus(ctx, ftModel, trainingJob, v1beta1.LifeCycleStateCreating); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateModel(ctx, trainingJob, ftModel); err != nil {
		return ctrl.Result{}, err
	}

	var result ctrl.Result
	var err error
	if trainingJob.Status.IsTrainingJobConditionEmpty() || trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobCreated {
		if trainingJob.Status.IsTrainingJobConditionEmpty() {
			return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobCreated, "Attempting to schedule training job")
		}

		r.Log.Info("Reconciling training job", "apiVersion", trainingJob.APIVersion, "tjob", trainingJob.Name, "namespace", trainingJob.Namespace)

		// Todo: Implement separate reconciliation logic for different training job framework. peft, tfew etc.

		if err != nil {
			if err := r.updateFTModelStatus(ctx, ftModel, trainingJob, v1beta1.LifeCycleStateFailed); err != nil {
				return ctrl.Result{}, err
			}
			// Todo: emit failed job metrics
			return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobFailed, err.Error())
		}

		if ftModel.Status.State != v1beta1.LifeCycleStateInTraining {
			if err := r.updateFTModelStatus(ctx, ftModel, trainingJob, v1beta1.LifeCycleStateInTraining); err != nil {
				return ctrl.Result{}, err
			}
		}
		return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobRunning, "Training job is in progress")
	}

	if trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobRunning {
		// Todo: Process running job
	}

	return result, nil
}

func (r *TrainingJobReconciler) updateTrainingJobStatus(ctx context.Context, tjob *v1beta1.TrainingJob, jobConditionType v1beta1.JobConditionType, details string) (ctrl.Result, error) {
	namespacedName := types.NamespacedName{Name: tjob.Name, Namespace: tjob.Namespace}
	if err := r.Get(ctx, namespacedName, tjob); err != nil {
		r.Log.Error(err, "unable to get TrainingJob", "tjob", tjob.Name)
		return reconcile.Result{}, err
	}

	tjob.Status.UpdateJobStatus(jobConditionType, details)
	if err := r.Status().Update(ctx, tjob); err != nil {
		r.Log.Error(err, "Failed to update training job status", "updated job condition type", string(jobConditionType), "tjob", tjob.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TrainingJobReconciler) updateFTModelStatus(ctx context.Context, ftmodel *v1beta1.FineTunedWeight, tjob *v1beta1.TrainingJob, state v1beta1.LifeCycleState) error {
	if err := r.Get(ctx, types.NamespacedName{Name: trainingJobUtils.GetFineTunedModelName(tjob.Name)}, ftmodel); err != nil {
		r.Log.Info("Failed to get Model when updating model status", "tjob", tjob.Name, "model", ftmodel.Name, "message", err.Error())
		return err
	}

	// Update associated job ref
	ftmodel.Spec.TrainingJobRef = v1beta1.ObjectReference{
		Name:      &tjob.Name,
		Namespace: &tjob.Namespace,
	}
	ftmodel.Status.State = state

	if err := r.Status().Update(ctx, ftmodel); err != nil {
		r.Log.Info("Warning: failed to update finetuned model status, will retry", "failed updated status", string(state), "tjob", tjob.Name, "model", ftmodel.Name, "message", err.Error())
		return err
	}
	r.Log.Info("FTModel status updated successfully", "successfully updated status", string(state), "tjob", tjob.Name, "model", ftmodel.Name)
	return nil
}

func (r *TrainingJobReconciler) updateModel(ctx context.Context, tjob *v1beta1.TrainingJob, model *v1beta1.FineTunedWeight) error {
	if err := r.Update(ctx, model); err != nil {
		r.Log.Info("Warning: failed to update finetuned model, will retry", "tjob", tjob.Name, "message", err.Error())
		return err
	}
	return nil
}

func (r *TrainingJobReconciler) createFTWeight(tjob *v1beta1.TrainingJob) *v1beta1.FineTunedWeight {
	return &v1beta1.FineTunedWeight{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Model",
			APIVersion: constants.OMEAPIGroupName + "/" + v1beta1.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: trainingJobUtils.GetFineTunedModelName(tjob.Name),
		},
		Spec: v1beta1.FineTunedWeightSpec{
			BaseModelRef: v1beta1.ObjectReference{
				Name:      tjob.Spec.BaseModel,
				Namespace: &tjob.Namespace,
			},
			ModelType:       tjob.Spec.BaseModel,
			HyperParameters: tjob.Spec.Hyperparameters,
			Storage:         tjob.Spec.OutputLocation,
		},
	}
}
