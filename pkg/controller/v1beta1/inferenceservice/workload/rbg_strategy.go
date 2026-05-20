package workload

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	rbgv1alpha2 "sigs.k8s.io/rbgs/api/workloads/v1alpha2"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/components"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/rbac"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/rbg"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/status"
)

// RBGStrategyName is the registered name of the RBG workload strategy.
const RBGStrategyName = "RBG"

// RBGStrategy implements WorkloadStrategy by packing every InferenceService
// component into a single RoleBasedGroup CR. It is the All-in-One workload
// path for OME and supports RawDeployment and MultiNode per-component
// deployment modes.
//
// Resources reconciled per call:
//   - One RoleBasedGroup with a role per component
//   - One ServiceAccount (and Role/RoleBinding for the Router component)
//     per component, mirroring SingleComponentStrategy
//   - One HPA per role to retain independent scaling behaviour
type RBGStrategy struct {
	Client    client.Client
	Clientset kubernetes.Interface
	Scheme    *runtime.Scheme
	Log       logr.Logger
}

// NewRBGStrategy constructs an RBGStrategy.
func NewRBGStrategy(c client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme, log logr.Logger) *RBGStrategy {
	return &RBGStrategy{
		Client:    c,
		Clientset: clientset,
		Scheme:    scheme,
		Log:       log.WithName("RBGStrategy"),
	}
}

// GetStrategyName returns the registration name.
func (s *RBGStrategy) GetStrategyName() string { return RBGStrategyName }

// IsApplicable returns true when the InferenceService selects the
// RoleBasedGroup deployment mode via annotation / config.
func (s *RBGStrategy) IsApplicable(_ *v1beta1.InferenceService, deploymentMode constants.DeploymentModeType) bool {
	return deploymentMode == constants.RoleBasedGroup
}

// ValidateDeploymentModes restricts per-component deployment modes to those
// the underlying RBG roles can express. Serverless / MultiNodeRayVLLM /
// VirtualDeployment are explicitly unsupported in the Alpha RBG strategy.
func (s *RBGStrategy) ValidateDeploymentModes(modes *ComponentDeploymentModes) error {
	if modes == nil {
		return nil
	}
	if err := validateRBGComponentMode("engine", modes.Engine, false); err != nil {
		return err
	}
	if err := validateRBGComponentMode("decoder", modes.Decoder, true); err != nil {
		return err
	}
	if err := validateRBGComponentMode("router", modes.Router, true); err != nil {
		return err
	}
	return nil
}

func validateRBGComponentMode(name string, mode constants.DeploymentModeType, optional bool) error {
	if mode == "" {
		if optional {
			return nil
		}
		return fmt.Errorf("RBG strategy requires a deployment mode for %s", name)
	}
	switch mode {
	case constants.RawDeployment, constants.MultiNode:
		return nil
	default:
		return fmt.Errorf("RBG strategy does not support deployment mode %q for %s (only RawDeployment and MultiNode are supported)", mode, name)
	}
}

// roleConfigEntry pairs a RoleConfig with the Component that produced it so
// the strategy can call both resource-reconciliation and status-propagation
// methods on the same component instance.
type roleConfigEntry struct {
	config    *components.RoleConfig
	component components.Component
}

// ReconcileWorkload extracts per-component RoleConfigs, then reconciles
// RBAC, the RoleBasedGroup CR, per-role HPAs, and propagates component
// status back into the InferenceService.
func (s *RBGStrategy) ReconcileWorkload(ctx context.Context, request *WorkloadReconcileRequest) (ctrl.Result, error) {
	if request == nil || request.InferenceService == nil {
		return ctrl.Result{}, errors.New("RBG strategy received empty reconcile request")
	}
	isvc := request.InferenceService
	s.Log.Info("Reconciling with RBG strategy", "namespace", isvc.Namespace, "inferenceService", isvc.Name)

	entries, err := s.extractRoleConfigs(request)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(entries) == 0 {
		return ctrl.Result{}, errors.New("RBG strategy requires at least one component (engine, decoder or router) to be defined")
	}

	configs := make([]*components.RoleConfig, len(entries))
	for i, e := range entries {
		configs[i] = e.config
	}

	if err := s.reconcileRBAC(isvc, configs); err != nil {
		return ctrl.Result{}, err
	}

	rbgRec := rbg.NewRBGReconciler(s.Client, s.Scheme, s.Log)
	if result, err := rbgRec.Reconcile(ctx, isvc, configs); err != nil {
		return result, err
	} else if result.Requeue || result.RequeueAfter > 0 {
		return result, nil
	}

	if err := s.reconcileComponentStatus(ctx, isvc, entries); err != nil {
		return ctrl.Result{}, err
	}

	s.Log.Info("RBG strategy reconciliation completed", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
	return ctrl.Result{}, nil
}

// extractRoleConfigs builds RoleConfigs for every component the
// InferenceService defines, in a stable order so the resulting RBG roles
// list is deterministic across reconciles. It also returns the Component
// instances so the caller can propagate component status.
func (s *RBGStrategy) extractRoleConfigs(request *WorkloadReconcileRequest) ([]roleConfigEntry, error) {
	if request.ComponentBuilderFactory == nil {
		return nil, errors.New("RBG strategy requires a non-nil ComponentBuilderFactory")
	}
	if request.DeploymentModes == nil {
		return nil, errors.New("RBG strategy requires non-nil DeploymentModes")
	}
	entries := make([]roleConfigEntry, 0, 3)

	if request.MergedEngine != nil {
		comp := request.ComponentBuilderFactory.CreateEngineComponent(
			request.DeploymentModes.Engine,
			request.BaseModel,
			request.BaseModelMeta,
			request.MergedEngine,
			request.Runtime,
			request.RuntimeName,
			request.EngineSupportedModelFormat,
			request.EngineAcceleratorClass,
			request.EngineAcceleratorClassName,
		)
		cfg, err := extractFromComponent(comp, request.InferenceService, "engine")
		if err != nil {
			return nil, err
		}
		entries = append(entries, roleConfigEntry{config: cfg, component: comp})
	}

	if request.MergedDecoder != nil {
		comp := request.ComponentBuilderFactory.CreateDecoderComponent(
			request.DeploymentModes.Decoder,
			request.BaseModel,
			request.BaseModelMeta,
			request.MergedDecoder,
			request.Runtime,
			request.RuntimeName,
			request.DecoderSupportedModelFormat,
			request.DecoderAcceleratorClass,
			request.DecoderAcceleratorClassName,
		)
		cfg, err := extractFromComponent(comp, request.InferenceService, "decoder")
		if err != nil {
			return nil, err
		}
		entries = append(entries, roleConfigEntry{config: cfg, component: comp})
	}

	if request.MergedRouter != nil {
		comp := request.ComponentBuilderFactory.CreateRouterComponent(
			request.DeploymentModes.Router,
			request.BaseModel,
			request.BaseModelMeta,
			request.MergedRouter,
			request.Runtime,
			request.RuntimeName,
		)
		cfg, err := extractFromComponent(comp, request.InferenceService, "router")
		if err != nil {
			return nil, err
		}
		entries = append(entries, roleConfigEntry{config: cfg, component: comp})
	}

	return entries, nil
}

func extractFromComponent(comp components.Component, isvc *v1beta1.InferenceService, name string) (*components.RoleConfig, error) {
	extractor, ok := comp.(components.RoleConfigExtractor)
	if !ok {
		return nil, fmt.Errorf("component %s does not implement RoleConfigExtractor and cannot be used with the RBG strategy", name)
	}
	cfg, err := extractor.ExtractRoleConfig(isvc)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to extract %s role config", name)
	}
	return cfg, nil
}

// reconcileRBAC creates RBAC resources only for the Router component,
// matching SingleComponentStrategy where only router.go calls the RBAC
// reconciler. Router needs a ServiceAccount + Role + RoleBinding so it
// can list/watch pods for service discovery.
func (s *RBGStrategy) reconcileRBAC(isvc *v1beta1.InferenceService, configs []*components.RoleConfig) error {
	for _, cfg := range configs {
		if cfg.ComponentType != v1beta1.RouterComponent {
			continue
		}
		rbacRec := rbac.NewRBACReconciler(s.Client, s.Scheme, cfg.ObjectMeta, cfg.ComponentType, isvc)
		if err := rbacRec.Reconcile(); err != nil {
			return errors.Wrapf(err, "failed to reconcile RBAC for %s", cfg.ComponentType)
		}
		saName := rbacRec.GetServiceAccountName()
		if cfg.PodSpec != nil {
			cfg.PodSpec.ServiceAccountName = saName
		}
	}
	return nil
}

// reconcileComponentStatus fetches the RBG CR status and propagates each
// role's readiness into the InferenceService's component Ready conditions
// (EngineReady, DecoderReady, RouterReady) so the controller can derive
// overall Ready. It also delegates to each component's UpdateStatus for
// model status propagation.
func (s *RBGStrategy) reconcileComponentStatus(ctx context.Context, isvc *v1beta1.InferenceService, entries []roleConfigEntry) error {
	// Propagate model status via the component's own UpdateStatus.
	for _, entry := range entries {
		updater, ok := entry.component.(components.ComponentStatusUpdater)
		if !ok {
			continue
		}
		if err := updater.UpdateStatus(isvc); err != nil {
			s.Log.Error(err, "Failed to update component status", "component", entry.config.ComponentType)
			return errors.Wrapf(err, "failed to update status for %s", entry.config.ComponentType)
		}
	}

	// Fetch the RBG CR to read per-role readiness.
	var rbgCR rbgv1alpha2.RoleBasedGroup
	if err := s.Client.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, &rbgCR); err != nil {
		return errors.Wrap(err, "failed to get RoleBasedGroup for status propagation")
	}

	roleStatusMap := make(map[string]rbgv1alpha2.RoleStatus, len(rbgCR.Status.RoleStatuses))
	for _, rs := range rbgCR.Status.RoleStatuses {
		roleStatusMap[rs.Name] = rs
	}

	statusMgr := status.NewStatusReconciler()
	readyConditions := map[v1beta1.ComponentType]apis.ConditionType{
		v1beta1.EngineComponent:  v1beta1.EngineReady,
		v1beta1.DecoderComponent: v1beta1.DecoderReady,
		v1beta1.RouterComponent:  v1beta1.RouterReady,
	}

	if len(isvc.Status.Components) == 0 {
		isvc.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
	}

	for _, entry := range entries {
		ct := entry.config.ComponentType
		roleName := string(ct)
		condType, ok := readyConditions[ct]
		if !ok {
			continue
		}

		rs, found := roleStatusMap[roleName]
		var cond *apis.Condition
		if !found {
			cond = &apis.Condition{
				Type:    condType,
				Status:  corev1.ConditionFalse,
				Reason:  "RoleNotFound",
				Message: fmt.Sprintf("role %s not found in RoleBasedGroup status", roleName),
			}
		} else if rs.ReadyReplicas >= rs.Replicas && rs.Replicas > 0 {
			cond = &apis.Condition{
				Type:   condType,
				Status: corev1.ConditionTrue,
			}
		} else {
			cond = &apis.Condition{
				Type:    condType,
				Status:  corev1.ConditionFalse,
				Reason:  "RoleNotReady",
				Message: fmt.Sprintf("role %s: %d/%d replicas ready", roleName, rs.ReadyReplicas, rs.Replicas),
			}
		}
		statusMgr.SetComponentCondition(&isvc.Status, condType, cond)

		if _, exists := isvc.Status.Components[ct]; !exists {
			isvc.Status.Components[ct] = v1beta1.ComponentStatusSpec{}
		}
	}

	return nil
}

// Compile-time interface check.
var _ WorkloadStrategy = (*RBGStrategy)(nil)
