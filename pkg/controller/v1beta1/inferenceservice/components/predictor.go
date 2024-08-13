package components

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinodevllm"
	predictorpv "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/pv"
	predictorpvc "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/pvc"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/raw"
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/knative"
	isvcutils "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/utils"
)

var _ Component = &Predictor{}

// Predictor reconciles resources for this component.
type Predictor struct {
	client                 client.Client
	clientset              kubernetes.Interface
	scheme                 *runtime.Scheme
	inferenceServiceConfig *v1beta1.InferenceServicesConfig
	deploymentMode         constants.DeploymentModeType
	Log                    logr.Logger
}

func NewPredictor(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme,
	inferenceServiceConfig *v1beta1.InferenceServicesConfig, deploymentMode constants.DeploymentModeType) Component {
	return &Predictor{
		client:                 client,
		clientset:              clientset,
		scheme:                 scheme,
		inferenceServiceConfig: inferenceServiceConfig,
		deploymentMode:         deploymentMode,
		Log:                    ctrl.Log.WithName("PredictorReconciler"),
	}
}

// Reconcile observes the predictor and attempts to drive the status towards the desired state.
func (p *Predictor) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	var container *v1.Container
	var podSpec v1.PodSpec
	var sRuntimeLabels map[string]string
	var sRuntimeAnnotations map[string]string

	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})
	predictor := isvc.Spec.Predictor.GetImplementation()

	// If Model is specified, prioritize using that. Otherwise, we will assume a framework object was specified.

	var sRuntime v1beta1.ServingRuntimeSpec
	var err error
	var baseModel v1beta1.BaseModelSpec

	pvReconciler := predictorpv.NewPredictorPVReconciler(p.client, p.clientset, p.scheme)
	pvcReconciler := predictorpvc.NewPredictorPVCReconciler(p.client, p.clientset, p.scheme)

	if isvc.Spec.Predictor.Model.BaseModel != nil {
		bm, err := isvcutils.GetBaseModel(p.client, *isvc.Spec.Predictor.Model.BaseModel, isvc.Namespace)
		if err != nil {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.BaseModelNotFound,
				Message: "Waiting for base model to become available",
			})
			return ctrl.Result{}, err
		}
		baseModel = *bm
	}

	if result, err := pvReconciler.Reconcile(isvc, &baseModel); err != nil {
		return result, err
	}

	if result, err := pvcReconciler.Reconcile(isvc); err != nil {
		return result, err
	}

	if isvc.Spec.Predictor.Model.Runtime != nil {
		// set runtime defaults
		isvc.SetRuntimeDefaults()
		r, err := isvcutils.GetServingRuntime(p.client, *isvc.Spec.Predictor.Model.Runtime, isvc.Namespace)
		if err != nil {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.RuntimeNotRecognized,
				Message: "Waiting for runtime to become available",
			})
			return ctrl.Result{}, err
		}

		if r.IsDisabled() {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.RuntimeDisabled,
				Message: "Specified runtime is disabled",
			})
			return ctrl.Result{}, fmt.Errorf("specified runtime %s is disabled", *isvc.Spec.Predictor.Model.Runtime)
		}

		if isvc.Spec.Predictor.Model.ProtocolVersion != nil &&
			!r.IsProtocolVersionSupported(*isvc.Spec.Predictor.Model.ProtocolVersion) {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.NoSupportingRuntime,
				Message: "Specified runtime does not support specified protocol version",
			})
			return ctrl.Result{}, fmt.Errorf("specified runtime %s does not support specified protocol version", *isvc.Spec.Predictor.Model.Runtime)
		}

		// Verify that the selected runtime supports the specified framework.
		if !isvc.Spec.Predictor.Model.RuntimeSupportsModel(r, &baseModel) {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.NoSupportingRuntime,
				Message: "Specified runtime does not support specified framework/version",
			})
			return ctrl.Result{}, fmt.Errorf("specified runtime %s does not support specified framework/version", *isvc.Spec.Predictor.Model.Runtime)
		}

		sRuntime = *r
	} else {
		runtimes, err := isvc.Spec.Predictor.Model.GetSupportingRuntimes(p.client, isvc.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if len(runtimes) == 0 {
			isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
				Reason:  v1beta1.NoSupportingRuntime,
				Message: "No runtime found to support specified framework/version",
			})
			return ctrl.Result{}, fmt.Errorf("no runtime found to support predictor with model type: %v", baseModel.ModelFormat.Name)
		}
		// Get first supporting runtime.
		sRuntime = runtimes[0].Spec
		isvc.Spec.Predictor.Model.Runtime = &runtimes[0].Name

		// set runtime defaults
		isvc.SetRuntimeDefaults()
	}

	if len(sRuntime.Containers) == 0 {
		isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
			Reason:  v1beta1.InvalidPredictorSpec,
			Message: "No container configuration found in selected serving runtime",
		})
		return ctrl.Result{}, errors.New("no container configuration found in selected serving runtime")
	}

	omeContainerIdx := -1
	for i := range sRuntime.Containers {
		if sRuntime.Containers[i].Name == constants.InferenceServiceContainerName {
			omeContainerIdx = i
			break
		}
	}
	if omeContainerIdx == -1 {
		return ctrl.Result{}, errors.New("failed to find ome-container in ServingRuntime containers")
	}

	container, err = isvcutils.MergeRuntimeContainers(&sRuntime.Containers[omeContainerIdx], &isvc.Spec.Predictor.Model.Container)
	if err != nil {
		isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
			Reason:  v1beta1.InvalidPredictorSpec,
			Message: "Failed to get runtime container",
		})
		return ctrl.Result{}, errors.Wrapf(err, "failed to get runtime container")
	}

	mergedPodSpec, err := isvcutils.MergePodSpec(&sRuntime.ServingRuntimePodSpec, &isvc.Spec.Predictor.PodSpec)
	if err != nil {
		isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
			Reason:  v1beta1.InvalidPredictorSpec,
			Message: "Failed to consolidate serving runtime PodSpecs",
		})
		return ctrl.Result{}, errors.Wrapf(err, "failed to consolidate serving runtime PodSpecs")
	}

	// Replace placeholders in runtime container by values from inferenceservice metadata
	if err = isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
		isvc.Status.UpdateModelTransitionStatus(v1beta1.InvalidSpec, &v1beta1.FailureInfo{
			Reason:  v1beta1.InvalidPredictorSpec,
			Message: "Failed to replace placeholders in serving runtime Container",
		})
		return ctrl.Result{}, errors.Wrapf(err, "failed to replace placeholders in serving runtime Container")
	}

	// Update image tag if GPU is enabled or runtime version is provided
	isvcutils.UpdateImageTag(container, isvc.Spec.Predictor.Model.RuntimeVersion, isvc.Spec.Predictor.Model.Runtime)

	p.Log.Info("Update volume mounts", "inference service", isvc.Name, "namespace", isvc.Namespace)
	modelMountPath := fmt.Sprintf("/model/%s", *isvc.Spec.Predictor.Model.BaseModel)
	vm := v1.VolumeMount{Name: constants.PVCName(isvc.Name), MountPath: modelMountPath, ReadOnly: true}
	volume := v1.Volume{Name: constants.PVCName(isvc.Name), VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: constants.PVCName(isvc.Name)}}}
	containerArg := []string{fmt.Sprintf("--model=%s", modelMountPath)}
	isvcutils.UpdateContainerArgs(container, &containerArg)
	isvcutils.UpdateVolumeMounts(container, &vm)
	isvcutils.AppendEnvVars(container, &[]v1.EnvVar{{Name: "MODEL_PATH", Value: modelMountPath}})

	p.Log.Info("Update volume mounts", "inference service", isvc.Name, "container", container)

	podSpec = *mergedPodSpec
	podSpec.Containers = []v1.Container{
		*container,
	}
	podSpec.Volumes = append(podSpec.Volumes, volume)
	podSpec.Containers = append(podSpec.Containers, sRuntime.Containers[:omeContainerIdx]...)
	podSpec.Containers = append(podSpec.Containers, sRuntime.Containers[omeContainerIdx+1:]...)

	// Label filter will be handled in ksvc_reconciler
	sRuntimeLabels = sRuntime.ServingRuntimePodSpec.Labels
	sRuntimeAnnotations = utils.Filter(sRuntime.ServingRuntimePodSpec.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	// Knative does not support INIT containers or mounting, so we add annotations that trigger the
	// StorageInitializer injector to mutate the underlying deployment to provision model data
	if sourceURI := predictor.GetStorageUri(); sourceURI != nil {
		if _, ok := annotations[constants.StorageInitializerSourceUriInternalAnnotationKey]; ok {
			return ctrl.Result{}, errors.New("must provide only one of storageUri and storage.path")
		}
		annotations[constants.StorageInitializerSourceUriInternalAnnotationKey] = *sourceURI
		err := isvcutils.ValidateStorageURI(sourceURI, p.client)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("StorageURI not supported: %w", err)
		}
	}

	predictorName := constants.PredictorServiceName(isvc.Name)
	if p.deploymentMode == constants.RawDeployment {
		existing := &v1.Service{}
		err := p.client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultPredictorServiceName(isvc.Name), Namespace: isvc.Namespace}, existing)
		if err == nil {
			predictorName = constants.DefaultPredictorServiceName(isvc.Name)
		}
	} else {
		existing := &knservingv1.Service{}
		err := p.client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultPredictorServiceName(isvc.Name), Namespace: isvc.Namespace}, existing)
		if err == nil {
			predictorName = constants.DefaultPredictorServiceName(isvc.Name)
		}
	}

	// Labels and annotations from predictor component
	// Label filter will be handled in ksvc_reconciler
	predictorLabels := isvc.Spec.Predictor.Labels
	predictorAnnotations := utils.Filter(isvc.Spec.Predictor.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	// Labels and annotations priority: predictor component > isvc > ServingRuntimePodSpec
	// Labels and annotations from high priority will overwrite that from low priority
	objectMeta := metav1.ObjectMeta{
		Name:      predictorName,
		Namespace: isvc.Namespace,
		Labels: utils.Union(
			sRuntimeLabels,
			isvc.Labels,
			predictorLabels,
			map[string]string{
				constants.InferenceServicePodLabelKey: isvc.Name,
				constants.KServiceComponentLabel:      string(v1beta1.PredictorComponent),
			},
		),
		Annotations: utils.Union(
			sRuntimeAnnotations,
			annotations,
			predictorAnnotations,
		),
	}

	p.Log.Info("Resolved container", "container", container, "podSpec", podSpec)
	var rawDeployment bool
	var podLabelKey string
	var podLabelValue string

	// Here we allow switch between knative and vanilla deployment
	if p.deploymentMode == constants.RawDeployment {
		rawDeployment = true
		podLabelKey = constants.RawDeploymentAppLabel
		// If PipelineParallelism is enabled, we will not create raw deployment
		if sRuntime.PipelineParallelism == nil || *sRuntime.PipelineParallelism == false {
			r, err := raw.NewRawKubeReconciler(p.client, p.clientset, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec,
				&podSpec)
			if err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to create NewRawKubeReconciler for predictor")
			}
			// set Deployment Controller
			if err := controllerutil.SetControllerReference(isvc, r.Deployment.Deployment, p.scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to set deployment owner reference for predictor")
			}
			// set Service Controller
			if err := controllerutil.SetControllerReference(isvc, r.Service.Service, p.scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to set service owner reference for predictor")
			}
			// set autoscaler Controller
			if err := r.Scaler.Autoscaler.SetControllerReferences(isvc, p.scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to set autoscaler owner references for predictor")
			}

			deployment, err := r.Reconcile()
			if err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to reconcile predictor")
			}
			isvc.Status.PropagateRawStatus(v1beta1.PredictorComponent, deployment, r.URL)
		} else {
			p.Log.Info("PipelineParallelism is enabled, will not create raw deployment", "inference service", isvc.Name)
			r, err := multinodevllm.NewMultiNodeVllmReconciler(p.client, p.clientset, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec, &podSpec)
			if err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to create NewMultiNodeVllmReconciler for predictor")
			}
			// set Ray controller
			if err := controllerutil.SetControllerReference(isvc, r.Ray.RayCluster, p.scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to set ray owner reference for predictor")
			}
			rayCluster, err := r.Reconcile()
			if err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "fails to reconcile predictor")
			}
			isvc.Status.PropagateMultiNodeStatus(v1beta1.PredictorComponent, rayCluster, r.URL)
		}

	} else {
		podLabelKey = constants.RevisionLabel
		r := knative.NewKsvcReconciler(p.client, p.scheme, objectMeta, &isvc.Spec.Predictor.ComponentExtensionSpec,
			&podSpec, isvc.Status.Components[v1beta1.PredictorComponent])
		if err := controllerutil.SetControllerReference(isvc, r.Service, p.scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "fails to set owner reference for predictor")
		}
		status, err := r.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "fails to reconcile predictor")
		}
		isvc.Status.PropagateStatus(v1beta1.PredictorComponent, status)
	}
	statusSpec := isvc.Status.Components[v1beta1.PredictorComponent]
	if rawDeployment {
		podLabelValue = constants.GetRawServiceLabel(predictorName)
	} else {
		podLabelValue = statusSpec.LatestCreatedRevision
	}
	predictorPods, err := isvcutils.ListPodsByLabel(p.client, isvc.ObjectMeta.Namespace, podLabelKey, podLabelValue)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "fails to list inferenceservice pods by label")
	}
	isvc.Status.PropagateModelStatus(statusSpec, predictorPods, rawDeployment)
	return ctrl.Result{}, nil
}
