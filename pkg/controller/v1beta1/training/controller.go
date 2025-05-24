package training

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"

	trainjobpv "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/pv"
	trainjobpvc "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/pvc"
	"k8s.io/client-go/kubernetes"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"

	"github.com/sgl-project/sgl-ome/pkg/constants"

	"github.com/go-logr/logr"
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	trainingruntimes "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
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
	Client    client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Runtimes  map[string]trainingruntimes.Runtime
}

// TrainingSidecarConfig represents configuration parameters for the training sidecar container.
type TrainingSidecarConfig struct {
	Image                 string `json:"image" validate:"required"`
	Region                string `json:"region"`
	Namespace             string `json:"namespace"`
	FineTunedModelBucket  string `json:"fineTunedModelBucket"`
	TrainingMetricsBucket string `json:"trainingMetricsBucket"`
	CompartmentId         string `json:"compartmentId"`
}

type ObjectOperationState string

var errorUnsupportedRuntime = errors.New("the specified runtime is not supported")

const (
	CreateObjectSucceeded       ObjectOperationState = "CreateObjectSucceeded"
	BuildObjectFailed           ObjectOperationState = "BuildObjectFailed"
	CreateObjectFailed          ObjectOperationState = "CreateObjectFailed"
	UpdateObjectFailed          ObjectOperationState = "UpdateObjectFailed"
	CreateFinetuneWeightsFailed ObjectOperationState = "CreateFinetuneWeightsFailed"
)

// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=finetunedweights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=finetunedweights;finetunedweights/finalizers,verbs=get;list;watch;create;update;patch;delete

func (r *TrainingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	trainJob := &v1beta1.TrainingJob{}
	if err := r.Client.Get(ctx, req.NamespacedName, trainJob); err != nil {
		if apierr.IsNotFound(err) {
			r.Log.Error(err, "TrainingJob not found", "namespace", req.NamespacedName, "name", trainJob.Name)
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Error getting TrainingJob", "namespace", req.NamespacedName, "name", trainJob.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling training job", "namespace", req.NamespacedName, "name", trainJob.Name)

	r.Log.Info("Getting base model for training job", "namespace", req.NamespacedName, "name", trainJob.Name)
	baseModel, err := utils.GetClusterBaseModel(r.Client, *trainJob.Spec.ModelConfig.InputModel)

	if err != nil {
		r.Log.Error(err, "Error getting model", "namespace", req.NamespacedName, "name", trainJob.Name, "basemodel", trainJob.Spec.ModelConfig.InputModel)
		return ctrl.Result{}, nil
	}

	configMap, err := r.Clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		r.Log.Error(err, "Failed to find config map", "name", constants.InferenceServiceConfigMapName)
		return ctrl.Result{}, nil
	}

	trainingRuntime, err := utils.GetTrainingRuntime(r.Client, trainJob.Spec.RuntimeRef.Name, trainJob.Namespace)
	if err != nil {
		r.Log.Error(err, "Error getting training runtime", "namespace", req.NamespacedName, "name", trainJob.Name, "training runtime", trainJob.Spec.RuntimeRef.Name)
		return ctrl.Result{}, nil
	}

	finetuneWeights := &v1beta1.FineTunedWeight{}
	if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: utils.GetFineTunedModelName(trainJob.Name)}, finetuneWeights); err != nil {
		if apierr.IsNotFound(err) {
			// Finetune weights not found, create a new one
			finetuneWeights = r.createFinetuneWeights(trainJob, *configMap, trainingRuntime)
			if err = r.Client.Create(ctx, finetuneWeights); err != nil {
				if apierr.IsAlreadyExists(err) {
					// Requeue it when model already exists
					return ctrl.Result{}, nil
				} else {
					r.Log.Error(err, "Failed to create Finetune weights", "tjob", trainJob.Name, "model", finetuneWeights.Name)
					updateCreatedCondition(trainJob, CreateFinetuneWeightsFailed)
					return ctrl.Result{}, err
				}
			} else {
				r.Log.Info("Finetune weights created", "tjob", trainJob.Name, "model", finetuneWeights.Name)
				err = r.updateFineTunedWeight(ctx, finetuneWeights, v1beta1.LifeCycleStateCreating)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		} else {
			r.Log.Error(err, "Failed to get Finetune weights", "tjob", trainJob.Name, "model", finetuneWeights.Name)
			return ctrl.Result{}, err
		}
	}

	if isTrainJobFinished(trainJob) {
		r.Log.Info("TrainJob has finished, updating FineTuneWeights", "namespace", req.NamespacedName, "name", trainJob.Name, "FineTuneWeights", finetuneWeights)
		if meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobFailed) {
			err := r.updateFineTunedWeight(ctx, finetuneWeights, v1beta1.LifeCycleStateFailed)
			return ctrl.Result{}, err
		}

		if meta.IsStatusConditionTrue(trainJob.Status.Conditions, v1beta1.TrainJobComplete) {
			err := r.updateFineTunedWeight(ctx, finetuneWeights, v1beta1.LifeCycleStateReady)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if result, err := r.reconcilePVPVC(trainJob, baseModel.Spec); err != nil {
		r.Log.Error(err, "Error reconciling PV/PVC for train job", "namespace", req.NamespacedName, "name", trainJob.Name)
		return result, err
	}

	if err := r.prepareJobAnnotations(trainJob, baseModel, trainingRuntime); err != nil {
		r.Log.Error(err, "Error preparing training job annotations", "namespace", req.NamespacedName, "name", trainJob.Name)
		return ctrl.Result{}, nil
	}

	runtimeRefGK := runtimeRefToGroupKind(trainJob.Spec.RuntimeRef).String()
	runtime, ok := r.Runtimes[runtimeRefGK]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("%w, %s", errorUnsupportedRuntime, runtimeRefGK)
	}

	opState, err := r.reconcileObjects(ctx, runtime, trainJob, req, baseModel.Spec.Vendor)

	originStatus := trainJob.Status.DeepCopy()
	updateSuspendedCondition(trainJob)
	updateCreatedCondition(trainJob, opState)
	if terminalCondErr := updateTerminalCondition(ctx, runtime, trainJob); terminalCondErr != nil {
		return ctrl.Result{}, errors.Join(err, terminalCondErr)
	}
	if !equality.Semantic.DeepEqual(&trainJob, originStatus) {
		return ctrl.Result{}, errors.Join(err, r.Client.Status().Update(ctx, trainJob))
	}

	return ctrl.Result{}, nil
}

func (r *TrainingJobReconciler) reconcileObjects(ctx context.Context, runtime trainingruntimes.Runtime, trainJob *v1beta1.TrainingJob, req ctrl.Request, vendor *string) (ObjectOperationState, error) {
	objs, err := runtime.NewObjects(ctx, trainJob, vendor)
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

func (r *TrainingJobReconciler) updateFineTunedWeight(ctx context.Context, fineTuneWeights *v1beta1.FineTunedWeight, state v1beta1.LifeCycleState) error {
	fineTuneWeights.Status.State = state

	r.Log.Info("Updating FineTuneWeights status", "FineTunedWeight", fineTuneWeights)

	if err := r.Client.Status().Update(ctx, fineTuneWeights); err != nil {
		r.Log.Error(err, "Failed to update FineTunedWeight status", "FineTunedWeight", fineTuneWeights)
		return err
	}
	return nil
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
	case CreateFinetuneWeightsFailed:
		newCond = metav1.Condition{
			Type:    v1beta1.TrainJobFailed,
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

// reconcilePVPVC reconciles the PersistentVolume and PersistentVolumeClaim for the training job.
func (r *TrainingJobReconciler) reconcilePVPVC(trainjob *omev1beta1.TrainingJob, baseModel v1beta1.BaseModelSpec) (ctrl.Result, error) {
	pvReconciler := trainjobpv.NewTrainingPVReconciler(r.Client, r.Clientset, r.Scheme)
	pvcReconciler := trainjobpvc.NewTrainingPVCReconciler(r.Client, r.Clientset, r.Scheme)

	if result, err := pvReconciler.Reconcile(trainjob, &baseModel); err != nil {
		return result, err
	}

	if result, err := pvcReconciler.Reconcile(trainjob); err != nil {
		return result, err
	}
	return ctrl.Result{}, nil
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

func (r *TrainingJobReconciler) prepareJobAnnotations(trainJob *v1beta1.TrainingJob, baseModel *v1beta1.ClusterBaseModel, trainingRuntime *v1beta1.TrainingRuntimeSpec) error {
	// We use these 2 annotations for every training job to inject init-container and sidecar container.
	// The values will be passed into jobset object, then the pod underneath.
	// Only cohere model needs model init container.
	if trainJob.Spec.Annotations == nil {
		trainJob.Spec.Annotations = make(map[string]string)
	}
	if trainJob.Spec.Labels == nil {
		trainJob.Spec.Labels = make(map[string]string)
	}
	if *baseModel.Spec.Vendor == "cohere" {
		trainJob.Spec.Annotations[constants.ModelInitInjectionKey] = "true"
		trainJob.Spec.Annotations[constants.BaseModelDecryptionKeyName] = baseModel.Annotations[constants.BaseModelDecryptionKeyName]
		trainJob.Spec.Annotations[constants.BaseModelDecryptionSecretName] = baseModel.Annotations[constants.BaseModelDecryptionSecretName]
		trainJob.Spec.Labels[constants.BaseModelTypeLabelKey] = string(constants.FinetuningBaseModel)
	}
	trainJob.Spec.Annotations[constants.TrainingSidecarInjectionKey] = "true"

	trainingSidecarRuntimeType := trainingRuntime.Annotations[constants.TrainingRuntimeTypeAnnotationKey]
	trainJob.Spec.Annotations[constants.TrainingRuntimeTypeAnnotationKey] = trainingSidecarRuntimeType

	if trainJob.Spec.Datasets.Parameters != nil {
		params := *trainJob.Spec.Datasets.Parameters
		if obo_token, ok := params[constants.OboTokenConfigKey]; ok {
			trainJob.Spec.Annotations[constants.OboTokenConfigKey] = obo_token
		}
	}

	v, err := utils.GetHyperparameterValueByKey(constants.EpochsConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.EpochsConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.EpochsConfigKey] = v.(string)

	v, err = utils.GetHyperparameterValueByKey(constants.LearningRateConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LearningRateConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.LearningRateConfigKey] = v.(string)

	v, err = utils.GetHyperparameterValueByKey(constants.BatchSizeConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.BatchSizeConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.BatchSizeConfigKey] = v.(string)

	v, err = utils.GetHyperparameterValueByKey(constants.EarlyStoppingPatienceConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.EarlyStoppingPatienceConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.EarlyStoppingPatienceConfigKey] = v.(string)

	v, err = utils.GetHyperparameterValueByKey(constants.EarlyStoppingThresholdConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.EarlyStoppingThresholdConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.EarlyStoppingThresholdConfigKey] = v.(string)

	strategy, err := utils.GetHyperparameterValueByKey(constants.StrategyConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.StrategyConfigKey)
		return err
	}
	trainJob.Spec.Annotations[constants.StrategyConfigKey] = strategy.(string)

	bucketName := utils.ExtractBucketNameFromObjectStorageUri(*trainJob.Spec.Datasets.StorageUri)
	trainJob.Spec.Annotations[constants.TrainingDataBucketConfigKey] = bucketName

	namespace := utils.ExtractNamespaceFromObjectStorageUri(*trainJob.Spec.Datasets.StorageUri)
	trainJob.Spec.Annotations[constants.TrainingDataNamespaceConfigKey] = namespace

	objectName := utils.ExtractObjectFileNameFromObjectStorageUri(*trainJob.Spec.Datasets.StorageUri)
	trainJob.Spec.Annotations[constants.TrainingDataFileNameConfigKey] = objectName

	baseModelName := utils.ExtractModelNameFromObjectStorageUri(*baseModel.Spec.Storage.StorageUri)
	trainJob.Spec.Annotations[constants.ModelNameConfigKey] = baseModelName

	if trainingSidecarRuntimeType == "peft" {
		trainJob.Spec.Annotations[constants.ModelVendorConfigKey] = *baseModel.Spec.Vendor

		v, err = utils.GetHyperparameterValueByKey(constants.LoraConfigRankConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
		if err != nil {
			r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LoraConfigRankConfigKey)
			return err
		}
		trainJob.Spec.Annotations[constants.LoraConfigRankConfigKey] = v.(string)

		v, err = utils.GetHyperparameterValueByKey(constants.LoraAlphaConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
		if err != nil {
			r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LoraAlphaConfigKey)
			return err
		}
		trainJob.Spec.Annotations[constants.LoraAlphaConfigKey] = v.(string)

		v, err = utils.GetHyperparameterValueByKey(constants.LoraDropoutConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
		if err != nil {
			r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LoraDropoutConfigKey)
			return err
		}
		trainJob.Spec.Annotations[constants.LoraDropoutConfigKey] = v.(string)

	} else {
		trainJob.Spec.Annotations[constants.ModelSizeConfigKey] = *baseModel.Spec.ModelParameterSize

		if trainingSidecarRuntimeType == "cohere" {
			v, err = utils.GetHyperparameterValueByKey(constants.LogTrainStatusEveryStepConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
			if err != nil {
				r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LogTrainStatusEveryStepConfigKey)
				return err
			}
			trainJob.Spec.Annotations[constants.LogTrainStatusEveryStepConfigKey] = v.(string)

			if strategy == "vanilla" {
				v, err = utils.GetHyperparameterValueByKey(constants.NLastLayersConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
				if err != nil {
					r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.NLastLayersConfigKey)
					return err
				}
				trainJob.Spec.Annotations[constants.NLastLayersConfigKey] = v.(string)
			}
		} else {
			trainJob.Spec.Annotations[constants.BaseModelConfigKey] = baseModel.Name

			tensorParallel, err := utils.GetTensorParallelSize(baseModel)
			if err != nil {
				return err
			}
			trainJob.Spec.Annotations[constants.TensorParallelConfigKey] = tensorParallel

			if strategy == "lora" {
				v, err = utils.GetHyperparameterValueByKey(constants.LoraConfigRankConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
				if err != nil {
					r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LoraConfigRankConfigKey)
					return err
				}
				trainJob.Spec.Annotations[constants.LoraConfigRankConfigKey] = v.(string)

				v, err = utils.GetHyperparameterValueByKey(constants.LoraAlphaConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
				if err != nil {
					r.Log.Error(err, "Error getting hyperparameter", "namespace", trainJob.Namespace, "name", trainJob.Name, "hyperparameter", constants.LoraAlphaConfigKey)
					return err
				}
				trainJob.Spec.Annotations[constants.LoraAlphaConfigKey] = v.(string)
			}
		}
	}

	return nil
}

func (r *TrainingJobReconciler) createFinetuneWeights(trainJob *v1beta1.TrainingJob, configMap v1.ConfigMap, trainingRuntime *v1beta1.TrainingRuntimeSpec) *v1beta1.FineTunedWeight {
	strategy, err := utils.GetHyperparameterValueByKey(constants.StrategyConfigKey, trainJob.Spec.HyperParameterTuningConfig.Parameters)
	if err != nil {
		strategy = "lora"
	}
	// Todo: Now we just put training strategy as the model type.
	modelType := strategy.(string)

	trainingSidecarConfig := &TrainingSidecarConfig{}
	if trainingSidecarConfigVal, ok := configMap.Data[constants.TrainingSidecarConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(trainingSidecarConfigVal), trainingSidecarConfig); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", constants.TrainingSidecarConfigMapKeyName, err))
		}
	}

	fineTunedWeightConfiguration, err := r.prepareFineTuneWeightConfiguration(trainingRuntime)
	if err != nil {
		panic(fmt.Errorf("failed to prepare FineTunedWeight configuration for TrainingJob %s: %+v", trainJob.Name, err))
	}

	storageUri := "oci://n/" + trainingSidecarConfig.Namespace + "/b/" + trainingSidecarConfig.FineTunedModelBucket + "/o/" + utils.GetFineTunedModelName(trainJob.Name)
	return &v1beta1.FineTunedWeight{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FineTunedWeight",
			APIVersion: constants.OMEAPIGroupName + "/" + v1beta1.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.GetFineTunedModelName(trainJob.Name),
		},
		Spec: v1beta1.FineTunedWeightSpec{
			BaseModelRef: v1beta1.ObjectReference{
				Name: trainJob.Spec.ModelConfig.InputModel,
			},
			ModelType:       &modelType,
			HyperParameters: trainJob.Spec.HyperParameterTuningConfig.Parameters,
			Configuration:   fineTunedWeightConfiguration,
			Storage: &v1beta1.StorageSpec{
				StorageUri: &storageUri,
			},
			TrainingJobRef: v1beta1.ObjectReference{
				Name: &trainJob.Name,
			},
		},
		Status: v1beta1.ModelStatusSpec{},
	}
}

func (r *TrainingJobReconciler) prepareFineTuneWeightConfiguration(trainingRuntime *v1beta1.TrainingRuntimeSpec) (runtime.RawExtension, error) {
	var mergedFineTunedWeight bool
	if trainingRuntime.Annotations[constants.TrainingRuntimeTypeAnnotationKey] == string(constants.PeftTrainingRuntime) ||
		trainingRuntime.Annotations[constants.TrainingRuntimeTypeAnnotationKey] == string(constants.CohereCommandRTrainingTraining) {
		mergedFineTunedWeight = true
	} else {
		mergedFineTunedWeight = false
	}

	configuration := map[string]interface{}{
		constants.FineTunedWeightMergedWeightsConfigKey: mergedFineTunedWeight,
	}

	configurationRaw, err := json.Marshal(configuration)
	if err != nil {
		r.Log.Error(err, "Failed to marshal FineTunedWeight configuration", "configuration", configuration)
		return runtime.RawExtension{}, err
	}
	return runtime.RawExtension{Raw: configurationRaw}, nil
}
