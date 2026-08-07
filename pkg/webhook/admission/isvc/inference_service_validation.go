package isvc

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	isvcutils "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"sigs.k8s.io/ome/pkg/runtimerevision"
	"sigs.k8s.io/ome/pkg/runtimeselector"
	"sigs.k8s.io/ome/pkg/utils/storage"
	"sigs.k8s.io/ome/pkg/validation"
)

var validatorLogger = logf.Log.WithName("inferenceservice-v1beta1-validation-webhook")

// InferenceServiceValidator wires the webhook framework into the
// per-rule validators in pkg/validation. The kubebuilder markers below
// suppress DeepCopy + OpenAPI codegen since this is a stateless
// wired-once helper, not a persisted CR.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceValidator struct {
	Client          client.Client
	RuntimeSelector runtimeselector.Selector

	// KnownPassthroughPrefixes lists the ome.io/* pass-through
	// annotation prefixes the active Gateway-implementation translator
	// supports (e.g. ["ome.io/btp."] when Envoy Gateway is loaded).
	// When non-empty, the validator rejects pass-through annotations
	// whose prefix is not in the set with `UnsupportedPassthrough`,
	// surfacing the misconfiguration at admission instead of letting
	// the controller catch it at runtime via the
	// BackendPolicyUnsupportedFields condition.
	//
	// Empty (default) keeps the prior permissive behavior — all
	// documented pass-through prefixes are accepted. Production
	// (cmd/manager/main.go) populates this from the active translator's
	// SupportedPassthroughPrefixes(); test wiring may leave it empty
	// unless explicitly exercising the rejection path.
	KnownPassthroughPrefixes []string
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-inferenceservice,mutating=false,failurePolicy=fail,groups=ome.io,resources=inferenceservices,versions=v1beta1,name=inferenceservice.ome-webhook-server.validator
var _ webhook.CustomValidator = &InferenceServiceValidator{}

func (v *InferenceServiceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		validatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	return v.validateInferenceService(ctx, isvc)
}
func (v *InferenceServiceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	isvc, err := convertToInferenceService(newObj)
	if err != nil {
		validatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	oldIsvc, err := convertToInferenceService(oldObj)
	if err != nil {
		validatorLogger.Error(err, "Unable to convert prior object to InferenceService")
		return nil, err
	}
	if err := validation.ValidateCoordinationUpdate(&oldIsvc.Spec, &isvc.Spec); err != nil {
		return nil, err
	}
	return v.validateInferenceService(ctx, isvc)
}

func (v *InferenceServiceValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	if _, err := convertToInferenceService(obj); err != nil {
		validatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	return nil, nil
}

// GetIntReference returns the pointer for the integer input
func GetIntReference(number int) *int {
	num := number
	return &num
}

// deploymentStrategyWarnings flags components that set deploymentStrategy while
// resolving to a non-Raw deployment mode. deploymentStrategy is the k8s-native
// appsv1.DeploymentStrategy and is honored only by RawDeployment (createRawDeployment
// copies it onto the Deployment); OMENative and PD-disaggregated manage pods
// directly and ignore it, so it is a silent no-op there. Point operators at
// lifecycle.updateStrategy, which OMENative reads for rollout pacing.
func deploymentStrategyWarnings(isvc *v1beta1.InferenceService) admission.Warnings {
	mode := isvc.Annotations[constants.DeploymentMode]
	if mode == "" && isvc.Spec.DeploymentMode != nil {
		mode = string(*isvc.Spec.DeploymentMode)
	}
	// Unknown mode (defaulter hasn't run, e.g. a unit test) or Raw: nothing to warn.
	if mode == "" || mode == string(constants.RawDeployment) {
		return nil
	}
	var warnings admission.Warnings
	check := func(name string, ext *v1beta1.ComponentExtensionSpec) {
		if ext != nil && ext.DeploymentStrategy != nil {
			warnings = append(warnings, fmt.Sprintf(
				"%s.deploymentStrategy is set but deploymentMode is %q; deploymentStrategy applies only to RawDeployment and is ignored here — use %s.lifecycle.updateStrategy.rollingUpdate for rollout pacing",
				name, mode, name))
		}
	}
	if isvc.Spec.Engine != nil {
		check("engine", &isvc.Spec.Engine.ComponentExtensionSpec)
	}
	if isvc.Spec.Decoder != nil {
		check("decoder", &isvc.Spec.Decoder.ComponentExtensionSpec)
	}
	if isvc.Spec.Router != nil {
		check("router", &isvc.Spec.Router.ComponentExtensionSpec)
	}
	return warnings
}
func (v *InferenceServiceValidator) validateInferenceService(ctx context.Context, isvc *v1beta1.InferenceService) (admission.Warnings, error) {
	var allWarnings admission.Warnings

	if err := validation.ValidateInferenceServiceName(isvc.Name); err != nil {
		return allWarnings, err
	}

	autoscalerWarnings, err := validation.ValidateAutoscalerConfig(isvc)
	if err != nil {
		return allWarnings, err
	}
	allWarnings = append(allWarnings, autoscalerWarnings...)

	// Per-Component Autoscaler block shape validation: class enum,
	// required fields, IdleReplicaCount vs MinReplicas, plus
	// annotation/block conflict. The per-Component dispatch is shared
	// with the ServingRuntime and InferenceReplica webhooks via
	// validation.ValidateComponentsAutoscalers.
	if err := validation.ValidateComponentsAutoscalers(isvcAutoscalerChecks(isvc)); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidateAutoscalerAnnotationConflict(isvc); err != nil {
		return allWarnings, err
	}

	// Legacy autoscaler surfaces (annotation class + spec.kedaConfig /
	// per-Component scaleMetric) remain supported; validate them alongside
	// the typed Autoscaler block.
	if err := validateInferenceServiceAutoscaler(isvc); err != nil {
		return allWarnings, err
	}

	if err := validation.ValidateAutoscalerTargetUtilizationPercentage(isvc); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidateEngineDecoderConfig(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidateLeaderWorkerPairing(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	// Forward-looking guard that the leader Runner stays at size=1
	// (today enforced structurally by LeaderSpec having no Size field;
	// see ValidateLeaderSize).
	if err := validation.ValidateLeaderSize(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	// Lean (no-model, no-runtime) multi-pod specs must declare both
	// leader.runner.image and worker.runner.image — the controller has
	// no way to fill them in without a runtime or model.
	if err := validation.ValidateMultiPodLeanRunner(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidateEngineDecoderDeploymentMode(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidateScaleToZero(isvc); err != nil {
		return allWarnings, err
	}
	if err := validation.ValidatePlacement(&isvc.Spec); err != nil {
		return allWarnings, err
	}

	// Traffic block: typed core load-balancing config + ome.io/* annotations.
	if err := validation.ValidateTrafficSpec(isvc.Spec.Traffic); err != nil {
		return allWarnings, err
	}
	trafficWarnings, err := validation.ValidateTrafficAnnotations(isvc.Annotations, isvc.Spec.Traffic, v.KnownPassthroughPrefixes...)
	if err != nil {
		return allWarnings, err
	}
	allWarnings = append(allWarnings, trafficWarnings...)

	// spec.rollout.canary: validate the progressive-traffic rollout plan.
	if err := validation.ValidateCanary(&isvc.Spec); err != nil {
		return allWarnings, err
	}

	// Cross-Component rollout coordination: structural + RatioBalanced
	// tolerance warning.
	if err := validation.ValidateCoordination(&isvc.Spec); err != nil {
		return allWarnings, err
	}
	if warning := validation.CoordinationRatioToleranceWarning(&isvc.Spec); warning != "" {
		allWarnings = append(allWarnings, warning)
	}

	// Per-Component LifecycleSpec.UpdateStrategy.RollingUpdate semantic
	// validation: percent strings must be "<n>%" with 0 <= n <= 100;
	// integer values must be >= 0. The CRD XIntOrString marker handles
	// the structural "must be int or string" check; this validator adds
	// the range / format check.
	if err := validation.ValidateLifecycle(&isvc.Spec); err != nil {
		return allWarnings, err
	}

	// deploymentStrategy (k8s appsv1) is honored only by RawDeployment; on
	// OMENative / PD it is silently ignored. Warn so operators move rollout
	// pacing to lifecycle.updateStrategy, which OMENative actually reads.
	allWarnings = append(allWarnings, deploymentStrategyWarnings(isvc)...)

	// At least one of spec.model or spec.runtime must be set. When
	// spec.model is omitted (lean path), spec.runtime must name the
	// runtime to use — the controller skips model-fetch and runtime
	// auto-selection in that mode.
	modelSet := isvc.Spec.Model != nil && isvc.Spec.Model.Name != ""
	runtimeSet := isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != ""
	if !modelSet && !runtimeSet {
		return allWarnings, fmt.Errorf("at least one of spec.model or spec.runtime must be set")
	}

	if err := validateRuntimePin(isvc); err != nil {
		return allWarnings, err
	}
	if err := v.validateModelExists(ctx, isvc); err != nil {
		return allWarnings, err
	}
	if err := v.validateOverlays(isvc); err != nil {
		return allWarnings, err
	}

	if isvc.Spec.Engine != nil {
		resWarnings, err := v.validateRuntimeAndModelResolution(ctx, isvc)
		if err != nil {
			return allWarnings, err
		}
		allWarnings = append(allWarnings, resWarnings...)
	}
	return allWarnings, nil
}
func convertToInferenceService(obj runtime.Object) (*v1beta1.InferenceService, error) {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil, fmt.Errorf("expected an InferenceService object but got %T", obj)
	}
	return isvc, nil
}

// validateModelExists ensures the referenced BaseModel/ClusterBaseModel
// (and each overlay) is reachable in the ISVC's namespace, and that
// cross-namespace PVC rules pass per-overlay.
func (v *InferenceServiceValidator) validateModelExists(ctx context.Context, isvc *v1beta1.InferenceService) error {
	if isvc.Spec.Model == nil || isvc.Spec.Model.Name == "" {
		return nil
	}
	spec, meta, err := isvcutils.GetBaseModel(v.Client, isvc.Spec.Model.Name, isvc.Namespace)
	if err != nil {
		return fmt.Errorf("referenced model %q not found in namespace %q: ensure a BaseModel exists in this namespace or a ClusterBaseModel exists cluster-wide with this name",
			isvc.Spec.Model.Name, isvc.Namespace)
	}
	if err := validateClusterBaseModelPVCNamespace(spec, meta, isvc); err != nil {
		return err
	}
	for _, ov := range isvc.Spec.Model.Overlays {
		ovSpec, ovMeta, err := isvcutils.GetBaseModel(v.Client, ov.Name, isvc.Namespace)
		if err != nil {
			return fmt.Errorf("overlay %q not found in namespace %q: %w", ov.Name, isvc.Namespace, err)
		}
		if err := validateClusterBaseModelPVCNamespace(ovSpec, ovMeta, isvc); err != nil {
			return fmt.Errorf("overlay %q: %w", ov.Name, err)
		}
	}
	return nil
}

// validateClusterBaseModelPVCNamespace rejects InferenceServices that
// reference a ClusterBaseModel whose PVC URI points to a namespace other
// than the InferenceService's own namespace. Kubernetes does not allow a
// pod to mount a PVC from a different namespace, so without this check the
// generated ISVC pods would fail at scheduling with a confusing
// "persistentvolumeclaim ... not found" error and no upstream signal.
//
// Skipped for namespaced BaseModel (already same-namespace by construction)
// and for non-PVC URIs.
func validateClusterBaseModelPVCNamespace(spec *v1beta1.BaseModelSpec, meta *metav1.ObjectMeta, isvc *v1beta1.InferenceService) error {
	// namespaced BaseModel ⇒ same-namespace by construction; skip
	if meta == nil || meta.Namespace != "" {
		return nil
	}
	if spec == nil || spec.Storage == nil || spec.Storage.StorageUri == nil {
		return nil
	}
	uri := *spec.Storage.StorageUri
	storageType, err := storage.GetStorageType(uri)
	if err != nil || storageType != storage.StorageTypePVC {
		return nil
	}
	components, err := storage.ParsePVCStorageURI(uri)
	if err != nil || components.Namespace == "" || components.Namespace == isvc.Namespace {
		return nil
	}
	return fmt.Errorf(
		"ClusterBaseModel %q references PVC in namespace %q, but InferenceService is in namespace %q; "+
			"Kubernetes does not allow pods to mount PVCs from another namespace. "+
			"Either move the InferenceService to namespace %q or replicate the PVC into namespace %q.",
		isvc.Spec.Model.Name, components.Namespace, isvc.Namespace, components.Namespace, isvc.Namespace)
}

// validateOverlays rejects overlay sets that would produce a broken
// pod spec: duplicate Name, env-var name collision after sanitization
// (foo-bar and foo_bar both → FOO_BAR), or self-reference.
// Per-overlay existence + runtime compatibility live in
// validateRuntimeAndModelResolution.
func (v *InferenceServiceValidator) validateOverlays(isvc *v1beta1.InferenceService) error {
	if isvc.Spec.Model == nil || len(isvc.Spec.Model.Overlays) == 0 {
		return nil
	}
	primary := isvc.Spec.Model.Name
	seenName := map[string]bool{}
	seenEnv := map[string]string{} // sanitized → original name
	for _, ov := range isvc.Spec.Model.Overlays {
		if ov.Name == "" {
			return fmt.Errorf("overlay name cannot be empty")
		}
		if ov.Name == primary {
			return fmt.Errorf("overlay %q must not reference the primary model of the same kind/apiGroup", ov.Name)
		}
		if seenName[ov.Name] {
			return fmt.Errorf("overlay %q is declared more than once", ov.Name)
		}
		seenName[ov.Name] = true

		envName := isvcutils.OverlayEnvVarName(ov.Name)
		if prev, ok := seenEnv[envName]; ok {
			return fmt.Errorf("overlays %q and %q both sanitize to env var %s; rename one (hyphens and underscores collapse to the same form)",
				prev, ov.Name, envName)
		}
		seenEnv[envName] = ov.Name
	}
	return nil
}

// validateRuntimeAndModelResolution gates runtime/model resolution on
// the Engine-architecture path. Lean specs (full runner config + no
// model) short-circuit; everything else needs the runtime selector.
func (v *InferenceServiceValidator) validateRuntimeAndModelResolution(ctx context.Context, isvc *v1beta1.InferenceService) (admission.Warnings, error) {
	var warnings admission.Warnings

	if isvc.Spec.Engine == nil {
		return warnings, nil
	}
	if isvc.Spec.Model == nil {
		if isvc.Spec.Runtime == nil && !hasFullRunnerConfig(isvc.Spec.Engine) {
			return warnings, fmt.Errorf("model reference is required when runtime is not specified and engine does not have complete runner configuration")
		}
		return warnings, nil
	}
	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		return v.resolveModelAndRuntime(ctx, isvc, warnings)
	}
	if !hasFullRunnerConfig(isvc.Spec.Engine) {
		return v.resolveModelAndRuntime(ctx, isvc, warnings)
	}
	return warnings, nil
}
func (v *InferenceServiceValidator) resolveModelAndRuntime(ctx context.Context, isvc *v1beta1.InferenceService, warnings admission.Warnings) (admission.Warnings, error) {
	baseModel, _, err := isvcutils.GetBaseModel(v.Client, isvc.Spec.Model.Name, isvc.Namespace)
	if err != nil {
		return warnings, fmt.Errorf("failed to resolve model %s: %w", isvc.Spec.Model.Name, err)
	}
	if baseModel.Disabled != nil && *baseModel.Disabled {
		return warnings, fmt.Errorf("model %s is disabled", isvc.Spec.Model.Name)
	}

	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		if err := v.RuntimeSelector.ValidateRuntime(ctx, isvc.Spec.Runtime.Name, baseModel, isvc); err != nil {
			// The operator named this runtime explicitly, so OME should
			// not block on the runtime's *declared* supportedModelFormats:
			// a generic runtime (e.g. sglang) can serve many architectures
			// it never enumerates. Downgrade a pure compatibility mismatch
			// (format / architecture / framework) to an advisory warning
			// and admit — the deliberate choice wins over the declaration.
			//
			// Everything else stays a hard error: runtime not found /
			// disabled / malformed model means the runtime genuinely
			// cannot run, so admitting would only produce a broken ISVC.
			if runtimeselector.IsRuntimeCompatibilityError(err) {
				warnings = append(warnings, fmt.Sprintf(
					"runtime %q does not declare support for model %q (%v); proceeding because the runtime was named explicitly",
					isvc.Spec.Runtime.Name, isvc.Spec.Model.Name, err))
				return warnings, nil
			}
			return warnings, fmt.Errorf("runtime %s does not support model %s: %w",
				isvc.Spec.Runtime.Name, isvc.Spec.Model.Name, err)
		}
		// Success is the common case and emits nothing — a bare "is valid"
		// admission Warning showed up as `Warning:` in kubectl and read as
		// if something were wrong.
		return warnings, nil
	}
	selection, err := v.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc)
	if err != nil {
		return warnings, fmt.Errorf("no supporting runtime found for model %s and engine does not have complete runner configuration: %w", isvc.Spec.Model.Name, err)
	}
	warnings = append(warnings, fmt.Sprintf("Runtime %s will be auto-selected for model %s",
		selection.Name, isvc.Spec.Model.Name))
	return warnings, nil
}

// validateRuntimePin rejects malformed explicit ControllerRevision pins
// at admission. The full existence check is left to reconcile (the
// webhook can't reliably reach across cluster scope), but the cheap
// shape checks happen here:
//
//   - spec.runtime.revision is meaningful only when autoSync is false;
//     setting it with autoSync=true (default) is a confused-user
//     signal worth surfacing immediately.
//   - The revision name must conform to the convention for the named
//     runtime's kind + scope; an obviously-wrong name (e.g., pinning
//     to runtime A but naming a revision of runtime B) is rejected
//     before the controller ever sees it.
func validateRuntimePin(isvc *v1beta1.InferenceService) error {
	if isvc.Spec.Runtime == nil || isvc.Spec.Runtime.Revision == nil || *isvc.Spec.Runtime.Revision == "" {
		return nil
	}
	if isvc.Spec.Runtime.AutoSync == nil || *isvc.Spec.Runtime.AutoSync {
		return fmt.Errorf(
			"spec.runtime.revision requires spec.runtime.autoSync=false; AutoSync=true would silently ignore the pin")
	}
	kind := runtimerevision.KindClusterServingRuntime
	sourceNS := ""
	if isvc.Spec.Runtime.Kind != nil && *isvc.Spec.Runtime.Kind == string(runtimerevision.KindServingRuntime) {
		kind = runtimerevision.KindServingRuntime
		sourceNS = isvc.Namespace
	}
	revName := *isvc.Spec.Runtime.Revision
	if !runtimerevision.MatchesRuntime(revName, kind, sourceNS, isvc.Spec.Runtime.Name) {
		return fmt.Errorf(
			"spec.runtime.revision %q does not match the expected naming convention for runtime %q (%s); "+
				"expected the form %s",
			revName, isvc.Spec.Runtime.Name, kind, expectedPinPattern(kind, sourceNS, isvc.Spec.Runtime.Name))
	}
	return nil
}
func expectedPinPattern(kind runtimerevision.SourceKind, sourceNS, runtimeName string) string {
	switch kind {
	case runtimerevision.KindClusterServingRuntime:
		return fmt.Sprintf("cr-%s-<8 lowercase hex chars>", runtimeName)
	case runtimerevision.KindServingRuntime:
		return fmt.Sprintf("r-%s-%s-<8 lowercase hex chars>", sourceNS, runtimeName)
	default:
		return "<unknown kind>"
	}
}

// isvcAutoscalerChecks projects each declared Component (Engine,
// Decoder, Router) into the per-Component slice consumed by
// validation.ValidateComponentsAutoscalers. Nil Components are skipped
// so an absent Component never produces a spurious "<name>:" error.
func isvcAutoscalerChecks(isvc *v1beta1.InferenceService) []validation.ComponentAutoscalerCheck {
	var checks []validation.ComponentAutoscalerCheck
	if isvc.Spec.Engine != nil {
		checks = append(checks, validation.ComponentAutoscalerCheck{
			Name:        "engine",
			Autoscaler:  isvc.Spec.Engine.Autoscaler,
			MinReplicas: isvc.Spec.Engine.MinReplicas,
		})
	}
	if isvc.Spec.Decoder != nil {
		checks = append(checks, validation.ComponentAutoscalerCheck{
			Name:        "decoder",
			Autoscaler:  isvc.Spec.Decoder.Autoscaler,
			MinReplicas: isvc.Spec.Decoder.MinReplicas,
		})
	}
	if isvc.Spec.Router != nil {
		checks = append(checks, validation.ComponentAutoscalerCheck{
			Name:        "router",
			Autoscaler:  isvc.Spec.Router.Autoscaler,
			MinReplicas: isvc.Spec.Router.MinReplicas,
		})
	}
	return checks
}

func hasFullRunnerConfig(engine *v1beta1.EngineSpec) bool {
	if engine == nil {
		return false
	}
	if engine.Runner != nil && engine.Runner.Image != "" {
		return true
	}
	if engine.Leader != nil && engine.Worker != nil {
		leaderHasRunner := engine.Leader.Runner != nil && engine.Leader.Runner.Image != ""
		workerHasRunner := engine.Worker.Runner != nil && engine.Worker.Runner.Image != ""
		return leaderHasRunner && workerHasRunner
	}
	return false
}

func validateInferenceServiceAutoscaler(isvc *v1beta1.InferenceService) error {
	annotations := isvc.ObjectMeta.Annotations
	value, ok := annotations[constants.AutoscalerClass]
	class := constants.AutoscalerClassType(value)
	if ok {
		for _, item := range constants.AutoscalerAllowedClassList {
			if class == item {
				switch class {
				case constants.AutoscalerClassHPA:
					if metric, ok := annotations[constants.AutoscalerMetrics]; ok {
						return validateHPAMetrics(v1beta1.ScaleMetric(metric))
					} else {
						return nil
					}
				case constants.AutoscalerClassKEDA:
					_, err := validateKEDAConfig(isvc)
					return err
				case constants.AutoscalerClassExternal:
					return nil
				default:
					return fmt.Errorf("unknown autoscaler class [%s]", class)
				}
			}
		}
		return fmt.Errorf("[%s] is not a supported autoscaler class type", value)
	}

	return nil
}

// Validate of autoscaler HPA metrics

func validateHPAMetrics(metric v1beta1.ScaleMetric) error {
	for _, item := range constants.AutoscalerAllowedMetricsList {
		if item == constants.AutoscalerMetricsType(metric) {
			return nil
		}
	}
	return fmt.Errorf("[%s] is not a supported metric", metric)
}

// kedaValidScalingOperators defines the valid KEDA scaling operators

var kedaValidScalingOperators = []string{
	"GreaterThan",
	"GreaterThanOrEqual",
	"LessThan",
	"LessThanOrEqual",
}

// kedaValidAuthModes defines the valid KEDA authentication modes

var kedaValidAuthModes = []string{
	"basic",
	"tls",
	"bearer",
	"custom",
}

// validateKEDAConfig validates KEDA-specific configuration in KedaConfig and annotations

func validateKEDAConfig(isvc *v1beta1.InferenceService) (admission.Warnings, error) {
	var warnings admission.Warnings
	kedaConfig := isvc.Spec.KedaConfig
	annotations := isvc.ObjectMeta.Annotations

	// Validate scaling operator from KedaConfig
	if kedaConfig != nil && kedaConfig.ScalingOperator != "" {
		if err := validateKEDAScalingOperator(kedaConfig.ScalingOperator); err != nil {
			return warnings, err
		}
	}

	// Validate scaling operator from annotations (takes precedence)
	if operatorAnnotation, ok := annotations[constants.KedaScalingOperator]; ok {
		if err := validateKEDAScalingOperator(operatorAnnotation); err != nil {
			return warnings, err
		}
	}

	// Validate scaling threshold from KedaConfig
	if kedaConfig != nil && kedaConfig.ScalingThreshold != "" {
		if err := validateKEDAScalingThreshold(kedaConfig.ScalingThreshold); err != nil {
			return warnings, err
		}
	}

	// Validate scaling threshold from annotations
	if thresholdAnnotation, ok := annotations[constants.KedaScalingThreshold]; ok {
		if err := validateKEDAScalingThreshold(thresholdAnnotation); err != nil {
			return warnings, err
		}
	}

	// Validate Prometheus server address from KedaConfig
	if kedaConfig != nil && kedaConfig.PromServerAddress != "" {
		if err := validateKEDAPrometheusServerAddress(kedaConfig.PromServerAddress); err != nil {
			return warnings, err
		}
	}

	// Validate Prometheus server address from annotations
	if promAddrAnnotation, ok := annotations[constants.KedaPrometheusServerAddress]; ok {
		if err := validateKEDAPrometheusServerAddress(promAddrAnnotation); err != nil {
			return warnings, err
		}
	}

	// Validate authModes if provided
	if kedaConfig != nil && kedaConfig.AuthModes != "" {
		if err := validateKEDAAuthModes(kedaConfig.AuthModes); err != nil {
			return warnings, err
		}

		// Warn if authModes is set without authenticationRef
		if kedaConfig.AuthenticationRef == nil {
			warnings = append(warnings, "KEDA authModes is specified but authenticationRef is not set; authModes will be ignored by KEDA")
		}
	}

	return warnings, nil
}

// validateKEDAScalingOperator validates that the scaling operator is one of the valid KEDA operators

func validateKEDAScalingOperator(operator string) error {
	for _, valid := range kedaValidScalingOperators {
		if operator == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid KEDA scaling operator %q, must be one of: %s", operator, strings.Join(kedaValidScalingOperators, ", "))
}

// validateKEDAScalingThreshold validates that the scaling threshold is a valid number

func validateKEDAScalingThreshold(threshold string) error {
	_, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		return fmt.Errorf("invalid KEDA scaling threshold %q: must be a valid number", threshold)
	}
	return nil
}

// validateKEDAPrometheusServerAddress validates that the Prometheus server address is a valid URL

func validateKEDAPrometheusServerAddress(address string) error {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("invalid KEDA Prometheus server address %q: %v", address, err)
	}

	// Check that scheme is http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid KEDA Prometheus server address %q: scheme must be http or https", address)
	}

	// Check that host is not empty
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid KEDA Prometheus server address %q: host is required", address)
	}

	return nil
}

// validateKEDAAuthModes validates that all auth modes are valid

func validateKEDAAuthModes(authModes string) error {
	modes := strings.Split(authModes, ",")
	for _, mode := range modes {
		mode = strings.TrimSpace(mode)
		if mode == "" {
			continue
		}
		valid := false
		for _, validMode := range kedaValidAuthModes {
			if mode == validMode {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid KEDA auth mode %q, must be one of: %s", mode, strings.Join(kedaValidAuthModes, ", "))
		}
	}
	return nil
}

// Validate of autoscaler targetUtilizationPercentage
