package components

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/omenative"
	isvcutils "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

// ComponentDeps are the dependencies shared by every component
// constructor. All fields are process-lifetime (prototype built once in
// the ISVC controller's SetupWithManager) except Config, which the
// controller attaches per reconcile from the ConfigMap-backed cache.
// Per-reconcile data goes in ComponentInputs instead.
type ComponentDeps struct {
	Client       client.Client
	Clientset    kubernetes.Interface
	APIReader    client.Reader // AuthoritativeReader role (see workload/types)
	Expectations *omenative.Expectations
	Recorder     record.EventRecorder
	Scheme       *runtime.Scheme
	Config       *controllerconfig.InferenceServicesConfig
	// GangSchedulingAvailable is the cluster-discovery boolean — true
	// when the scheduler-plugins PodGroup CRD is installed. OMENative
	// reads it to decide whether to emit PodGroups for multi-pod
	// Instances; other deployment modes ignore it.
	GangSchedulingAvailable bool
}

// ComponentInputs are resolved fresh each reconcile by the controller
// pipeline (model resolution, runtime selection, spec merge, overlays).
type ComponentInputs struct {
	DeploymentMode constants.DeploymentModeType
	BaseModel      *v1beta1.BaseModelSpec
	BaseModelMeta  *metav1.ObjectMeta
	Runtime        *v1beta1.ServingRuntimeSpec
	RuntimeName    string
	ModelFormat    *v1beta1.SupportedModelFormat
	// AcceleratorClass is the resolved accelerator class spec for this
	// component (nil when the ISVC declares no accelerator preference).
	// Drives node-selector/affinity/resource merging and per-class
	// runtime arg/env overrides during pod construction.
	AcceleratorClass     *v1beta1.AcceleratorClassSpec
	AcceleratorClassName string
	Overlays             []isvcutils.ResolvedOverlay
}
