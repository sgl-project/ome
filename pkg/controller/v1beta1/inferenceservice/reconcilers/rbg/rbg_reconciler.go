// Package rbg contains the reconciler that materialises a set of
// per-component RoleConfigs into a single RoleBasedGroup custom resource.
//
// It is consumed by the workload-layer RBGStrategy and is intentionally
// kept narrow: it does not select strategies, extract component configs
// or manage HPAs / RBAC. Those concerns live in the strategy.
package rbg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	rbgv1alpha2 "sigs.k8s.io/rbgs/api/workloads/v1alpha2"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/components"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

// RBGReconciler creates and updates the RoleBasedGroup CR backing an
// InferenceService when the RBG workload strategy is active.
type RBGReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// NewRBGReconciler constructs an RBGReconciler.
func NewRBGReconciler(c client.Client, scheme *runtime.Scheme, log logr.Logger) *RBGReconciler {
	return &RBGReconciler{
		Client: c,
		Scheme: scheme,
		Log:    log.WithName("RBGReconciler"),
	}
}

// Reconcile creates or updates the RoleBasedGroup for the given
// InferenceService from the supplied per-component RoleConfigs. On update
// it preserves the per-role replica counts that may have been adjusted by
// HPA, only the pod template / labels / annotations are reconciled.
func (r *RBGReconciler) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService, configs []*components.RoleConfig) (ctrl.Result, error) {
	if len(configs) == 0 {
		return ctrl.Result{}, errors.New("no role configs provided to RBG reconciler")
	}

	desiredRoles, err := buildRoles(configs)
	if err != nil {
		return ctrl.Result{}, err
	}

	desired := &rbgv1alpha2.RoleBasedGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      isvc.Name,
			Namespace: isvc.Namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvc.Name,
			},
		},
		Spec: rbgv1alpha2.RoleBasedGroupSpec{
			Roles: desiredRoles,
		},
	}
	if err := controllerutil.SetControllerReference(isvc, desired, r.Scheme); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to set owner reference on RBG")
	}

	existing := &rbgv1alpha2.RoleBasedGroup{}
	getErr := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	switch {
	case apierrors.IsNotFound(getErr):
		r.Log.Info("Creating RoleBasedGroup", "namespace", desired.Namespace, "name", desired.Name)
		if err := r.Client.Create(ctx, desired); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to create RoleBasedGroup")
		}
		return ctrl.Result{}, nil
	case getErr != nil:
		return ctrl.Result{}, errors.Wrap(getErr, "failed to get RoleBasedGroup")
	}

	// Preserve replica counts from existing roles - HPA owns scaling once the role exists.
	preserveReplicas(desired.Spec.Roles, existing.Spec.Roles)

	merged := existing.DeepCopy()
	merged.Spec = desired.Spec
	// Merge desired labels into existing labels rather than replacing
	// wholesale, so labels managed by the RBG controller are preserved.
	if merged.Labels == nil {
		merged.Labels = make(map[string]string)
	}
	for k, v := range desired.Labels {
		merged.Labels[k] = v
	}
	merged.OwnerReferences = desired.OwnerReferences

	r.Log.Info("Updating RoleBasedGroup", "namespace", merged.Namespace, "name", merged.Name)
	if err := r.Client.Update(ctx, merged); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to update RoleBasedGroup")
	}
	return ctrl.Result{}, nil
}

// preserveReplicas copies the Replicas field from existingRoles into
// desiredRoles for every role that is present in both. Roles that are new
// in desiredRoles keep their initial replica count.
func preserveReplicas(desiredRoles, existingRoles []rbgv1alpha2.RoleSpec) {
	if len(existingRoles) == 0 {
		return
	}
	existingByName := make(map[string]*rbgv1alpha2.RoleSpec, len(existingRoles))
	for i := range existingRoles {
		existingByName[existingRoles[i].Name] = &existingRoles[i]
	}
	for i := range desiredRoles {
		if prev, ok := existingByName[desiredRoles[i].Name]; ok && prev.Replicas != nil {
			r := *prev.Replicas
			desiredRoles[i].Replicas = &r
		}
	}
}

// buildRoles converts per-component RoleConfigs into RBG RoleSpecs.
func buildRoles(configs []*components.RoleConfig) ([]rbgv1alpha2.RoleSpec, error) {
	roles := make([]rbgv1alpha2.RoleSpec, 0, len(configs))
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		role, err := buildRole(cfg)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// buildRole materialises a single RoleConfig into a RoleSpec. It picks the
// underlying workload type based on the per-component deployment mode:
//
//   - RawDeployment -> apps/v1/Deployment, single template via StandalonePattern
//   - MultiNode     -> leaderworkerset.x-k8s.io/v1/LeaderWorkerSet, leader +
//     worker templates via LeaderWorkerPattern
func buildRole(cfg *components.RoleConfig) (rbgv1alpha2.RoleSpec, error) {
	if cfg.PodSpec == nil {
		return rbgv1alpha2.RoleSpec{}, fmt.Errorf("role %s has nil pod spec", cfg.ComponentType)
	}

	annotations := copyStringMap(cfg.ObjectMeta.Annotations)
	labels := copyStringMap(cfg.ObjectMeta.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[constants.OMEComponentLabel] = string(cfg.ComponentType)
	labels[constants.RawDeploymentAppLabel] = constants.GetRawServiceLabel(cfg.ObjectMeta.Name)
	podMeta := metav1.ObjectMeta{Labels: labels, Annotations: annotations}
	isvcutils.SetPodLabelsFromAnnotations(&podMeta)

	role := rbgv1alpha2.RoleSpec{
		Name:        roleName(cfg.ComponentType),
		Labels:      labels,
		Annotations: annotations,
		Replicas:    initialReplicas(cfg.ComponentExtensionSpec),
	}

	switch cfg.DeploymentMode {
	case constants.RawDeployment:
		role.ScalingAdapter = &rbgv1alpha2.ScalingAdapter{Enable: true}
		role.Pattern = rbgv1alpha2.Pattern{
			StandalonePattern: &rbgv1alpha2.StandalonePattern{
				TemplateSource: rbgv1alpha2.TemplateSource{
					Template: podTemplate(cfg.PodSpec, labels, annotations),
				},
			},
		}
	case constants.MultiNode:
		leaderSpec := cfg.LeaderPodSpec
		if leaderSpec == nil {
			leaderSpec = cfg.PodSpec
		}
		size := int32(cfg.WorkerSize) + 1 // +1 for the leader
		role.Pattern = rbgv1alpha2.Pattern{
			LeaderWorkerPattern: &rbgv1alpha2.LeaderWorkerPattern{
				Size: &size,
				TemplateSource: rbgv1alpha2.TemplateSource{
					Template: podTemplate(leaderSpec, labels, annotations),
				},
			},
		}
		if cfg.WorkerPodSpec != nil {
			patch, err := workerTemplatePatch(cfg.WorkerPodSpec, labels, annotations)
			if err != nil {
				return rbgv1alpha2.RoleSpec{}, errors.Wrapf(err, "failed to build worker template patch for role %s", role.Name)
			}
			role.Pattern.LeaderWorkerPattern.WorkerTemplatePatch = patch
		}
	default:
		return rbgv1alpha2.RoleSpec{}, fmt.Errorf("unsupported deployment mode %q for RBG role %s", cfg.DeploymentMode, cfg.ComponentType)
	}

	return role, nil
}

// roleName returns the RBG role name for the given InferenceService
// component type. This is the short identifier used inside the RBG (e.g.
// "engine"); the full Kubernetes object names produced by the RBG
// controller are derived from the RBG name plus this role name.
func roleName(ct v1beta1.ComponentType) string {
	return string(ct)
}

// initialReplicas returns the initial replicas for a freshly created role.
// Defaults to 1 when MinReplicas is nil so the RBG webhook doesn't reject
// the spec.
func initialReplicas(ext *v1beta1.ComponentExtensionSpec) *int32 {
	r := int32(1)
	if ext != nil && ext.MinReplicas != nil && *ext.MinReplicas > 0 {
		r = int32(*ext.MinReplicas)
	}
	return &r
}

// podTemplate wraps a PodSpec into a PodTemplateSpec, copying labels and
// annotations onto the pod metadata so the underlying workload's selector
// matches them.
func podTemplate(spec *corev1.PodSpec, labels, annotations map[string]string) *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      copyStringMap(labels),
			Annotations: copyStringMap(annotations),
		},
		Spec: *spec.DeepCopy(),
	}
}

// workerTemplatePatch serialises the worker pod template into a
// runtime.RawExtension that can be applied as a strategic merge patch on
// top of the leader template. The patch carries a full PodTemplateSpec so
// strategic-merge replaces the relevant fields (containers, volumes, etc.)
// without leaking leader-only configuration onto worker pods.
func workerTemplatePatch(spec *corev1.PodSpec, labels, annotations map[string]string) (*runtime.RawExtension, error) {
	tpl := podTemplate(spec, labels, annotations)
	bytes, err := json.Marshal(tpl)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: bytes}, nil
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
