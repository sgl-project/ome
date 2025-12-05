package common

import (
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	isutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/knative"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinode"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/multinodevllm"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/raw"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/status"
)

// DeploymentReconciler handles common deployment reconciliation logic
type DeploymentReconciler struct {
	Client        client.Client
	Clientset     kubernetes.Interface
	Scheme        *runtime.Scheme
	StatusManager *status.StatusReconciler
	Log           logr.Logger
}

// ReconcileRawDeployment handles raw Kubernetes deployment
func (r *DeploymentReconciler) ReconcileRawDeployment(
	isvc *v1beta1.InferenceService,
	objectMeta isutils.ObjectMetaPack,
	podSpec *v1.PodSpec,
	componentSpec *v1beta1.ComponentExtensionSpec,
	componentType v1beta1.ComponentType,
) (ctrl.Result, error) {
	// Create an InferenceServiceSpec with component spec
	inferenceServiceSpec := &v1beta1.InferenceServiceSpec{
		Predictor: v1beta1.PredictorSpec{
			ComponentExtensionSpec: *componentSpec,
		},
	}

	reconciler, err := raw.NewRawKubeReconciler(r.Client, r.Clientset, r.Scheme, objectMeta, inferenceServiceSpec, podSpec)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create RawKubeReconciler for %s", componentType)
	}

	if err := r.setRawReferences(isvc, reconciler); err != nil {
		return ctrl.Result{}, err
	}

	deployment, err := reconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile %s", componentType)
	}

	r.StatusManager.PropagateRawStatus(&isvc.Status, componentType, deployment, reconciler.URL)
	return ctrl.Result{}, nil
}

// ReconcileMultiNodeDeployment handles multi-node deployment using LeaderWorkerSet
func (r *DeploymentReconciler) ReconcileMultiNodeDeployment(
	isvc *v1beta1.InferenceService,
	objectMeta metav1.ObjectMeta,
	leaderPodSpec *v1.PodSpec,
	workerSize int,
	workerPodSpec *v1.PodSpec,
	componentSpec *v1beta1.ComponentExtensionSpec,
	componentType v1beta1.ComponentType,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling multi-node deployment", "component", componentType, "inferenceService", isvc.Name)

	reconciler, err := multinode.NewMultiNodeReconciler(r.Client, r.Clientset, r.Scheme, objectMeta, componentSpec, leaderPodSpec, workerSize, workerPodSpec)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create MultiNodeReconciler for %s", componentType)
	}

	if err := r.setMultiNodeReferences(isvc, reconciler); err != nil {
		return ctrl.Result{}, err
	}

	lws, err := reconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile %s", componentType)
	}

	r.StatusManager.PropagateMultiNodeStatus(&isvc.Status, componentType, lws, reconciler.URL)
	return ctrl.Result{}, nil
}

// ReconcileKnativeDeployment handles serverless deployment using Knative
func (r *DeploymentReconciler) ReconcileKnativeDeployment(
	isvc *v1beta1.InferenceService,
	objectMeta metav1.ObjectMeta,
	podSpec *v1.PodSpec,
	componentSpec *v1beta1.ComponentExtensionSpec,
	componentType v1beta1.ComponentType,
) (ctrl.Result, error) {
	reconciler := knative.NewKsvcReconciler(r.Client, r.Scheme, objectMeta, componentSpec, podSpec, isvc.Status.Components[componentType])
	if err := controllerutil.SetControllerReference(isvc, reconciler.Service, r.Scheme); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to set owner reference for %s", componentType)
	}

	status, err := reconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile %s", componentType)
	}

	r.StatusManager.PropagateStatus(&isvc.Status, componentType, status)
	return ctrl.Result{}, nil
}

// ReconcileMultiNodeRayVLLMDeployment handles multi-node Ray VLLM deployment
func (r *DeploymentReconciler) ReconcileMultiNodeRayVLLMDeployment(
	isvc *v1beta1.InferenceService,
	objectMeta metav1.ObjectMeta,
	podSpec *v1.PodSpec,
	componentSpec *v1beta1.ComponentExtensionSpec,
	componentType v1beta1.ComponentType,
) (ctrl.Result, error) {
	r.Log.Info("PipelineParallelism is enabled, using MultiNodeRayVLLM deployment",
		"component", componentType, "inferenceService", isvc.Name, "namespace", isvc.Namespace)

	reconciler, err := multinodevllm.NewMultiNodeVllmReconciler(r.Client, r.Clientset, r.Scheme, objectMeta, componentSpec, podSpec)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create MultiNodeVllmReconciler for %s", componentType)
	}

	if err := r.setMultiNodeRayVLLMReferences(isvc, reconciler); err != nil {
		return ctrl.Result{}, err
	}

	_, result, err := reconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile %s", componentType)
	}

	r.StatusManager.PropagateMultiNodeRayVLLMStatus(&isvc.Status, componentType, reconciler.MultiNodeProber.Deployments, reconciler.URL)
	return result, nil
}

// setRawReferences sets the necessary references for raw deployment
func (r *DeploymentReconciler) setRawReferences(isvc *v1beta1.InferenceService, reconciler *raw.RawKubeReconciler) error {
	if err := controllerutil.SetControllerReference(isvc, reconciler.Deployment.Deployment, r.Scheme); err != nil {
		return errors.Wrapf(err, "failed to set deployment owner reference")
	}
	if err := controllerutil.SetControllerReference(isvc, reconciler.Service.Service, r.Scheme); err != nil {
		return errors.Wrapf(err, "failed to set service owner reference")
	}
	if err := controllerutil.SetControllerReference(isvc, reconciler.PodDisruptionBudget.PDB, r.Scheme); err != nil {
		return errors.Wrapf(err, "failed to set pdb owner reference")
	}

	return reconciler.Scaler.Autoscaler.SetControllerReferences(isvc, r.Scheme)
}

// setMultiNodeReferences sets the necessary references for multi-node deployment
func (r *DeploymentReconciler) setMultiNodeReferences(isvc *v1beta1.InferenceService, mnr *multinode.MultiNodeReconciler) error {
	err := controllerutil.SetControllerReference(isvc, mnr.LWS.LWS, r.Scheme)
	if err != nil {
		return errors.Wrapf(err, "failed to set lws owner reference")
	}
	return controllerutil.SetControllerReference(isvc, mnr.Service.Service, r.Scheme)
}

// setMultiNodeRayVLLMReferences sets the necessary references for multi-node Ray VLLM deployment
func (r *DeploymentReconciler) setMultiNodeRayVLLMReferences(isvc *v1beta1.InferenceService, reconciler *multinodevllm.MultiNodeVllmReconciler) error {
	for _, ray := range reconciler.Ray.RayClusters {
		if err := controllerutil.SetControllerReference(isvc, ray, r.Scheme); err != nil {
			return errors.Wrapf(err, "failed to set ray owner reference")
		}
	}
	for _, dply := range reconciler.MultiNodeProber.Deployments {
		if err := controllerutil.SetControllerReference(isvc, dply, r.Scheme); err != nil {
			return errors.Wrapf(err, "failed to set prober owner reference")
		}
	}
	return controllerutil.SetControllerReference(isvc, reconciler.RawMultiNodeService.Service, r.Scheme)
}
