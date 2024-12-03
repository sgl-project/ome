package training

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/reconcilers"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/singlenode/cohere"
	trainingJobUtils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
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

	if v1beta1.IsTerminalJobCondition(trainingJob.Status.GetLatestTrainingJobConditionType()) {
		return ctrl.Result{}, nil
	}

	ftModel := &v1beta1.FineTunedWeight{}
	if err := r.Get(ctx, types.NamespacedName{Name: trainingJobUtils.GetFineTunedModelName(trainingJob.Name)}, ftModel); err != nil {
		if apierr.IsNotFound(err) {
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

		return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobFailed, err.Error(), false)
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

	if err := r.Update(ctx, ftModel); err != nil {
		r.Log.Info("Warning: failed to update finetuned model, will retry", "tjob", trainingJob.Name, "message", err.Error())
		return ctrl.Result{}, err
	}

	if trainingJob.Status.IsTrainingJobConditionEmpty() {
		return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobCreated, "Attempting to schedule training job", false)
	}

	if trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobCreated {
		r.Log.Info("Reconciling training job", "apiVersion", trainingJob.APIVersion, "tjob", trainingJob.Name, "namespace", trainingJob.Namespace)

		// Todo: Implement reconciliation logic for other training framework
		var err error
		switch trainingJob.Spec.TrainingFramework.FrameworkType {
		case v1beta1.Peft:
			reconciler := reconcilers.NewPeftTrainingReconciler(r.Client, r.Scheme)
			_, err = reconciler.Reconcile(trainingJob)
			break
		case v1beta1.CohereCommandRFinetune, v1beta1.CohereFinetune:
			reconciler := cohere.NewCohereTrainingReconciler(r.Client, r.Scheme)
			_, err = reconciler.Reconcile(trainingJob)
			break
		default:
			r.Log.Error(err, "invalid training framework specified", "trainingJob", trainingJob.Name, "framework", trainingJob.Spec.TrainingFramework.FrameworkType)
			return ctrl.Result{}, nil
		}

		if err != nil {
			if err := r.updateFTModelStatus(ctx, ftModel, trainingJob, v1beta1.LifeCycleStateFailed); err != nil {
				return ctrl.Result{}, err
			}
			// Todo: emit failed job metrics
			return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobFailed, err.Error(), false)
		}

		if ftModel.Status.State != v1beta1.LifeCycleStateInTraining {
			if err := r.updateFTModelStatus(ctx, ftModel, trainingJob, v1beta1.LifeCycleStateInTraining); err != nil {
				return ctrl.Result{}, err
			}
		}
		return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobRunning, "Training job is in progress", false)
	}

	if trainingJob.Status.GetLatestTrainingJobConditionType() == v1beta1.JobRunning {
		if result, err := r.processTrainingJob(ctx, trainingJob, ftModel); err != nil {
			if ftModel.Status.State == v1beta1.LifeCycleStateReady {
				return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobSucceeded, "Training job completed successfully", false)
			}
			if ftModel.Status.State == v1beta1.LifeCycleStateFailed {
				return r.updateTrainingJobStatus(ctx, trainingJob, v1beta1.JobFailed, err.Error(), false)
			}
			return result, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TrainingJobReconciler) processTrainingJob(ctx context.Context, tjob *v1beta1.TrainingJob, ftModel *v1beta1.FineTunedWeight) (ctrl.Result, error) {
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: tjob.Name, Namespace: tjob.Namespace}, job); err != nil {
		if apierr.IsNotFound(err) {
			if time.Since(tjob.CreationTimestamp.Time) > constants.TrainingK8SJobCreationTimeoutDuration {
				r.Log.Error(err, "Training k8s job creation timed out", "tjob", tjob.Name)
				if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
					return ctrl.Result{}, err
				}

				// Todo: emit failed job metrics

				return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Training k8s job creation timed out", false)
			}
			r.Log.Info("Waiting training k8s job to be created..", "tjob", tjob.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		r.Log.Error(err, "Failed to get training k8s job job in processTrainingJob", "tjob", tjob.Name)
		if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
			return ctrl.Result{}, err
		}

		// Todo: emit failed job metrics

		return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Failed to get training k8s job", false)
	}

	if job.Status.Failed > 0 {
		r.Log.Error(fmt.Errorf("training Job failed"), "Training Job failed, will check if it is a retryable failure", "tjob", tjob.Name)
		return r.handleFailedTrainingJob(ctx, tjob, ftModel)
	}

	if job.Status.Active > 0 {
		r.Log.Info("Training k8s job in active state", "tjob", tjob.Name)
		return r.handleActiveTrainingJob(ctx, tjob, ftModel)
	}

	if job.Status.Succeeded > 0 && job.Status.Active == 0 {
		r.Recorder.Eventf(tjob, v1.EventTypeNormal, "TrainingJobSucceeded",
			fmt.Sprintf("TrainingJob [%v] is Ready", tjob.GetName()))
		r.Log.Info("Training Job succeeded", "tjob", tjob.Name)
		if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateReady); err != nil {
			return ctrl.Result{}, err
		}

		// Todo: emit succeeded job metrics

		return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobSucceeded, "Training job completed successfully", false)
	}

	// Handle the cases if none of the above conditions are met, would be:
	// Training k8s job not started, i.e., job's pod cannot be created
	// or
	// Short transition period between job states, like right after the job is successfully complete, there might be a moment when job.Status.active is already back to 0 but job.Status.Succeeded has not updated to 1.
	if _, err := trainingJobUtils.GetPodsControlledByJob(r.Client, tjob.Name, tjob.Namespace); err != nil {
		r.Log.Info("Waiting training k8s job to be started..", "tjob", tjob.Name)
		if time.Since(job.CreationTimestamp.Time) > constants.TrainingK8SJobStartingTimeoutDuration {
			r.Log.Error(fmt.Errorf("training k8s job starting timed out: %s", err.Error()), "Training k8s job starting timed out", "tjob", tjob.Name)
			r.Recorder.Eventf(tjob, v1.EventTypeWarning, "InternalError", "Training k8s job failed to start")
			if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
				return ctrl.Result{}, err
			}

			// Todo: emit failed job metrics

			return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Training k8s job starting timed out", false)
		}
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *TrainingJobReconciler) handleActiveTrainingJob(ctx context.Context, tjob *v1beta1.TrainingJob, ftModel *v1beta1.FineTunedWeight) (ctrl.Result, error) {
	pods, err := trainingJobUtils.GetPodsControlledByJob(r.Client, tjob.Name, tjob.Namespace)
	if err != nil {
		r.Log.Error(err, "Failed to get TrainingJob pods", "tjob", tjob.Name)
		if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
			return ctrl.Result{}, err
		}

		// Todo: emit failed job metrics

		return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Failed to get TrainingJob pods", false)
	}
	for _, pod := range pods.Items {
		logFields := []interface{}{"namespace", pod.Namespace, "tjob", tjob.Name}
		if pod.Status.HostIP != "" {
			logFields = append(logFields, "hostIP", pod.Status.HostIP)
		}
		r.Log.Info(fmt.Sprintf("Training job pod: %s", pod.Status.Phase), logFields...)

		err, trainingFailedReason := trainingJobUtils.CheckActivePodFailureIfAny(tjob, pod, r.Log)
		if err != nil {
			r.Log.Error(err, "TrainingJob failed", "tjob", tjob.Name)
			eventReason := "InternalError"
			if trainingFailedReason == constants.BadTrainingData {
				eventReason = string(constants.BadTrainingData)
			}
			r.Recorder.Eventf(tjob, v1.EventTypeWarning, eventReason, err.Error())
			if err := r.deleteOwnedResource(ctx, tjob); err != nil {
				r.Log.Info("Failed to delete dependent resources, will retry", "tjob", tjob.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
				return ctrl.Result{}, err
			}

			// Todo: emit failed job metrics

			return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, err.Error(), false)
		}
	}
	return ctrl.Result{RequeueAfter: 20 * time.Second}, nil
}

// Handle failed job to further check the failed reason
func (r *TrainingJobReconciler) handleFailedTrainingJob(ctx context.Context, tjob *v1beta1.TrainingJob, ftModel *v1beta1.FineTunedWeight) (ctrl.Result, error) {
	pods, err := trainingJobUtils.GetPodsControlledByJob(r.Client, tjob.Name, tjob.Namespace)
	if err != nil {
		r.Log.Info("Warning: cannot get pods for failed job", "tjob", tjob.Name)
		r.Recorder.Eventf(tjob, v1.EventTypeWarning, "InternalError", "Training job failed")
		if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
			return ctrl.Result{}, err
		}

		// Todo: emit failed job metrics

		return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Training job failed", false)
	}

	for _, pod := range pods.Items {
		logFields := []interface{}{"namespace", pod.Namespace, "tjob", tjob.Name}
		if pod.Status.HostIP != "" {
			logFields = append(logFields, "hostIP", pod.Status.HostIP)
		}
		r.Log.Info(fmt.Sprintf("Training job faild, pod: %s", pod.Status.Phase), logFields...)

		err, failedReason := trainingJobUtils.CheckFailedPodFailure(tjob, pod, r.Log)
		if err != nil {
			//	UnexpectedAdmissionError - delete current failed k8s job, let reconcile to create a new one as the retrying logic
			if failedReason == constants.K8SJobUnexpectedAdmissionError {
				tjobCreationTime := time.Since(tjob.GetCreationTimestamp().Time)
				if tjob.Status.RetryCount < constants.TrainingK8SJobRetryMaxAttempts && tjobCreationTime < constants.TrainingK8SJobRetryTimeoutDuration {
					r.Log.Info("K8S Job failed due to UnexpectedAdmissionError, can be retried, triggering retry now..", "tjob", tjob.Name, "currentRetryAttempt", tjob.Status.RetryCount+1)
					_ = r.deleteOwnedResource(ctx, tjob)
					return r.updateTrainingJobStatus(ctx, tjob, "", "Retrying training", true)
				} else {
					r.Log.Info("K8S Job failed due to UnexpectedAdmissionError, with all retries failed or timeout", "tjob", tjob.Name, "totalRetryAttempt", tjob.Status.RetryCount, "timeSinceCreation", tjobCreationTime, "maxRetryTimeoutDuration", constants.TrainingK8SJobRetryTimeoutDuration)
				}
			}

			// Handle non-retryable error or retryable error with all retries failed or retry timeout:
			r.Log.Error(err, "TrainingJob failed", "tjob", tjob.Name)
			r.Recorder.Eventf(tjob, v1.EventTypeWarning, string(failedReason), err.Error())
			if err := r.deleteOwnedResource(ctx, tjob); err != nil {
				r.Log.Info("Failed to delete dependent resources, will retry", "tjob", tjob.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
				return ctrl.Result{}, err
			}

			// Todo: emit failed job metrics

			return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, err.Error(), false)
		}
	}
	r.Log.Info("Cannot find failure details from pod and container status", "tjob", tjob.Name)
	r.Recorder.Eventf(tjob, v1.EventTypeWarning, "InternalError", "Training job failed")
	if err := r.deleteOwnedResource(ctx, tjob); err != nil {
		r.Log.Info("Failed to delete dependent resources, will retry", "tjob", tjob.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err := r.updateFTModelStatus(ctx, ftModel, tjob, v1beta1.LifeCycleStateFailed); err != nil {
		return ctrl.Result{}, err
	}

	// Todo: emit failed job metrics

	return r.updateTrainingJobStatus(ctx, tjob, v1beta1.JobFailed, "Training job failed", false)
}

func (r *TrainingJobReconciler) deleteOwnedResource(ctx context.Context, tjob *v1beta1.TrainingJob) error {
	var existingJob batchv1.Job

	err := r.Get(ctx, types.NamespacedName{Namespace: tjob.Namespace, Name: tjob.Name}, &existingJob)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil
		}
		r.Log.Info("Failed to get associated K8S job", "tjob", tjob.Name)
		return err
	}

	deletePolicy := metav1.DeletePropagationBackground
	if err := r.Delete(ctx, &existingJob, &client.DeleteOptions{PropagationPolicy: &deletePolicy}); err != nil {
		if apierr.IsNotFound(err) {
			return nil
		}
		r.Log.Info("Failed to delete K8S job", "tjob", existingJob.Name)
		return err
	}

	r.Log.Info("Training K8S Job deleted successfully", "tjob", tjob.Name)
	return nil
}

func (r *TrainingJobReconciler) updateTrainingJobStatus(ctx context.Context, tjob *v1beta1.TrainingJob, jobConditionType v1beta1.JobConditionType, details string, retry bool) (ctrl.Result, error) {
	namespacedName := types.NamespacedName{Name: tjob.Name, Namespace: tjob.Namespace}
	if err := r.Get(ctx, namespacedName, tjob); err != nil {
		r.Log.Error(err, "unable to get TrainingJob", "tjob", tjob.Name)
		return reconcile.Result{}, err
	}

	if retry {
		tjob.Status.IncrementRetry()
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
