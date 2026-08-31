package isvc

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

var mutatorLogger = logf.Log.WithName("inferenceservice-v1beta1-mutating-webhook")

// InferenceServiceDefaulter wires the webhook framework into the
// DefaultInferenceService entrypoint. The kubebuilder markers below
// suppress DeepCopy + OpenAPI codegen since this is a stateless
// wired-once helper, not a persisted CR.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceDefaulter struct {
	Client    client.Client
	ClientSet kubernetes.Interface
}

// +kubebuilder:webhook:path=/mutate-ome-io-v1beta1-inferenceservice,mutating=true,failurePolicy=fail,groups=ome.io,resources=inferenceservices,verbs=create;update,versions=v1beta1,name=inferenceservice.ome-webhook-server.defaulter
var _ webhook.CustomDefaulter = &InferenceServiceDefaulter{}

func (d *InferenceServiceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		mutatorLogger.Error(err, "Unable to convert object to InferenceService")
		return err
	}
	deployConfig, err := controllerconfig.NewDeployConfig(d.ClientSet)
	if err != nil {
		mutatorLogger.Error(err, "Failed to get deploy config")
		return err
	}
	return DefaultInferenceService(ctx, d.Client, isvc, deployConfig)
}

// DefaultInferenceService sets default values on the InferenceService.
func DefaultInferenceService(ctx context.Context, c client.Client, isvc *v1beta1.InferenceService, deployConfig *controllerconfig.DeployConfig) error {
	if isvc.ObjectMeta.Annotations == nil {
		isvc.ObjectMeta.Annotations = map[string]string{}
	}

	if _, modeExists := isvc.ObjectMeta.Annotations[constants.DeploymentMode]; !modeExists {
		// Engine + Decoder ⇒ PDDisaggregated.
		// Engine + Leader + Worker (size>0) ⇒ OMENative (native multi-node).
		// Otherwise fall back to deployConfig.DefaultDeploymentMode
		// (only RawDeployment is honored — other modes require an
		// explicit operator annotation).
		if isvc.Spec.Engine != nil && isvc.Spec.Decoder != nil {
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = string(constants.PDDisaggregated)
		} else if isvc.Spec.Engine != nil {
			if isvc.Spec.Engine.Leader != nil &&
				isvc.Spec.Engine.Worker != nil &&
				isvc.Spec.Engine.Worker.Size != nil &&
				*isvc.Spec.Engine.Worker.Size > 0 {
				isvc.ObjectMeta.Annotations[constants.DeploymentMode] = string(constants.OMENative)
			} else if deployConfig != nil && deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
				isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
			}
		} else if deployConfig != nil && deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
		}
	}

	// resolvedMode is the effective deployment mode after the ObjectMeta
	// resolution above: the canonical top-level ome.io/deploymentMode
	// annotation (set explicitly by the operator or by the leader+worker
	// heuristic), falling back to spec.DeploymentMode. Components must
	// resolve OMENative from it so a multi-node engine declared the normal
	// way (top-level annotation, no spec.deploymentMode) still receives
	// defaultOMENativeComponent — including the rollout budget defaults
	// that keep its rollouts capped.
	resolvedMode := isvc.Spec.DeploymentMode
	if m := isvc.ObjectMeta.Annotations[constants.DeploymentMode]; m != "" {
		mm := constants.DeploymentModeType(m)
		resolvedMode = &mm
	}

	if isvc.Spec.Engine != nil {
		defaultEngine(isvc.Spec.Engine, resolvedMode)
	}
	if isvc.Spec.Decoder != nil {
		defaultDecoder(isvc.Spec.Decoder, resolvedMode)
	}
	if isvc.Spec.Router != nil {
		defaultRouter(isvc.Spec.Router, resolvedMode)
	}
	// Rollout pacing/structure defaults are applied at runtime by the resolve
	// layer (coordination.ResolveGroups), not at admission.
	return nil
}

func defaultEngine(engine *v1beta1.EngineSpec, specMode *constants.DeploymentModeType) {
	defaultWorkerSize(engine.Leader, engine.Worker)
	// A raw spec without a runner shape may inherit Leader/Worker from its
	// runtime.
	shape := podShapeUnresolved
	if engineIsMultiPod(engine) {
		shape = podShapeMulti
	}
	defaultOMENativeComponent(&engine.ComponentExtensionSpec, shape, specMode)
}

func defaultDecoder(decoder *v1beta1.DecoderSpec, specMode *constants.DeploymentModeType) {
	defaultWorkerSize(decoder.Leader, decoder.Worker)
	// A raw spec without a runner shape may inherit Leader/Worker from its
	// runtime.
	shape := podShapeUnresolved
	if decoderIsMultiPod(decoder) {
		shape = podShapeMulti
	}
	defaultOMENativeComponent(&decoder.ComponentExtensionSpec, shape, specMode)
}
func defaultWorkerSize(leader *v1beta1.LeaderSpec, worker *v1beta1.WorkerSpec) {
	if leader == nil || worker == nil {
		return
	}
	if worker.Size != nil {
		return
	}
	size := 1
	worker.Size = &size
}

func defaultRouter(router *v1beta1.RouterSpec, specMode *constants.DeploymentModeType) {
	// Router has no Leader/Worker; always single-pod.
	defaultOMENativeComponent(&router.ComponentExtensionSpec, podShapeSingle, specMode)
}

// engineIsMultiPod reports whether Engine declares a complete Leader+Worker
// pair with a positive Worker.Size.
func engineIsMultiPod(engine *v1beta1.EngineSpec) bool {
	if engine == nil {
		return false
	}
	return engine.Leader != nil && engine.Worker != nil && engine.Worker.Size != nil && *engine.Worker.Size > 0
}

// decoderIsMultiPod reports whether Decoder declares a complete Leader+Worker
// pair with a positive Worker.Size.
func decoderIsMultiPod(decoder *v1beta1.DecoderSpec) bool {
	if decoder == nil {
		return false
	}
	return decoder.Leader != nil && decoder.Worker != nil && decoder.Worker.Size != nil && *decoder.Worker.Size > 0
}

type componentPodShape uint8

const (
	podShapeUnresolved componentPodShape = iota
	podShapeSingle
	podShapeMulti
)

// defaultOMENativeComponent fills lifecycle defaults for an OMENative Component.
// Shape-dependent policies remain unset when runtime resolution may change
// the shape.
func defaultOMENativeComponent(ext *v1beta1.ComponentExtensionSpec, shape componentPodShape, specMode *constants.DeploymentModeType) {
	if ext == nil {
		return
	}
	if !componentResolvesToOMENative(ext.Annotations, specMode) {
		return
	}
	if ext.Lifecycle == nil {
		ext.Lifecycle = &v1beta1.LifecycleSpec{}
	}
	spec := ext.Lifecycle

	if shape != podShapeUnresolved && spec.RestartPolicy == nil {
		rp := v1beta1.InstanceRestartPolicyNone
		if shape == podShapeMulti {
			rp = v1beta1.InstanceRestartPolicyRecreateInstance
		}
		spec.RestartPolicy = &rp
	}

	if spec.UpdateStrategy == nil {
		spec.UpdateStrategy = &v1beta1.UpdateStrategy{}
	}
	if spec.UpdateStrategy.Type == "" {
		// SurgeThenDrain is the default. Single-pod Instances get a real
		// zero-downtime surge (new pod at the alternate ordinal, drain
		// the old, promote); multi-pod gangs surge a whole replacement
		// gang at a fresh surge index before the source drains. Either
		// way it is safer than in-place because the MaxUnavailable gate
		// throttles drains.
		spec.UpdateStrategy.Type = v1beta1.UpdateStrategySurgeThenDrain
	}
	if spec.UpdateStrategy.InPlaceUpdateStrategy == nil {
		spec.UpdateStrategy.InPlaceUpdateStrategy = &v1beta1.InPlaceUpdateStrategy{}
	}
	if spec.UpdateStrategy.InPlaceUpdateStrategy.GracePeriodSeconds == nil {
		grace := int32(30)
		spec.UpdateStrategy.InPlaceUpdateStrategy.GracePeriodSeconds = &grace
	}
	if spec.UpdateStrategy.InPlaceUpdateStrategy.MarkNotReadyDuringLifecycle == nil {
		mark := true
		spec.UpdateStrategy.InPlaceUpdateStrategy.MarkNotReadyDuringLifecycle = &mark
	}

	// Default the per-Component rollout budgets so an unset RollingUpdate
	// never resolves to the uncapped BudgetNoLimit. Without a cap the
	// dispatcher can start every Instance's surge/drain in a single
	// reconcile pass, draining an entire fleet at once on a spec bump.
	// 25% mirrors the upstream appsv1.Deployment RollingUpdate defaults and
	// paces both the surge path (gated on MaxSurge) and the recreate path
	// (gated on MaxUnavailable). Percent values scale with replica count at
	// reconcile time, so this is safe from tiny to large Components.
	if spec.UpdateStrategy.RollingUpdate == nil {
		spec.UpdateStrategy.RollingUpdate = &v1beta1.RollingUpdate{}
	}
	if spec.UpdateStrategy.RollingUpdate.MaxSurge == nil {
		ms := intstr.FromString("25%")
		spec.UpdateStrategy.RollingUpdate.MaxSurge = &ms
	}
	if spec.UpdateStrategy.RollingUpdate.MaxUnavailable == nil {
		mu := intstr.FromString("25%")
		spec.UpdateStrategy.RollingUpdate.MaxUnavailable = &mu
	}

	if shape != podShapeUnresolved && spec.ReadyPolicy == nil {
		rp := v1beta1.InstanceReadyPolicyNone
		if shape == podShapeMulti {
			rp = v1beta1.InstanceReadyPolicyAllPodReady
		}
		spec.ReadyPolicy = &rp
	}

	if spec.InstanceReadyTimeout == nil {
		spec.InstanceReadyTimeout = &metav1.Duration{Duration: 30 * time.Minute}
	}

	if spec.MigrationPolicy == nil {
		spec.MigrationPolicy = &v1beta1.MigrationPolicy{}
	}
	if spec.MigrationPolicy.Mode == "" {
		spec.MigrationPolicy.Mode = v1beta1.MigrationPolicyModeAuto
	}
}

// componentResolvesToOMENative reports whether a Component dispatches
// to the OMENative backend after applying the precedence chain:
//  1. Per-Component ome.io/deploymentMode annotation (escape hatch).
//  2. Top-level spec.deploymentMode (typed default).
//
// Returns false when neither resolves to OMENative. Used by the
// lifecycle-policy defaulter so spec.deploymentMode=OMENative also
// triggers the OMENative defaults without forcing operators to repeat
// the annotation on every Component.
func componentResolvesToOMENative(annotations map[string]string, specMode *constants.DeploymentModeType) bool {
	if annotations[constants.DeploymentMode] != "" {
		return annotations[constants.DeploymentMode] == string(constants.OMENative)
	}
	if specMode != nil {
		return *specMode == constants.OMENative
	}
	return false
}
