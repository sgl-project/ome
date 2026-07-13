package pvc

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
)

// metadataJobRBACComponent is the value of the
// app.kubernetes.io/component label on RBAC objects this file creates.
// Matches the value the Helm chart puts on its OME-namespace SA so
// chart-installed and controller-installed objects are filterable
// together (kubectl get -l app.kubernetes.io/component=ome-model-metadata-job).
const metadataJobRBACComponent = "ome-model-metadata-job"

// metadataJobSourceNamespaceLabel records which user namespace a
// cross-ns RoleBinding in the OME namespace was created for. Lets an
// operator answer "which BaseModel namespaces have the controller
// granted OME-ns write access to?" via a single kubectl get -L.
const metadataJobSourceNamespaceLabel = "models.ome/metadata-source-namespace"

// ensureMetadataJobRBAC makes sure the RBAC the metadata-extraction
// Job needs in `jobNamespace` exists. The chart pre-installs everything
// in the OME namespace; this function patches the gap for any other
// namespace (BaseModel's own ns when namespaced, or the URI-derived ns
// when ClusterBaseModel points at a PVC outside ome).
//
// Two objects are needed for a Job in `jobNamespace` to succeed:
//
//  1. A ServiceAccount in `jobNamespace`. Without it, kubelet rejects
//     pod creation with "serviceaccount not found" — the Job sits at
//     0/1 ready forever (no pod ever starts).
//
//  2. A RoleBinding in the OME namespace binding the existing cluster-
//     scoped ClusterRole to the SA in `jobNamespace`. The agent only
//     writes the per-model status ConfigMap in the OME namespace
//     (the model-metadata agent's writeStatus/newPVCMetadataConfigMap
//     hardcode constants.OMENamespace), so an in-namespace RoleBinding
//     wouldn't help — the agent needs
//     cross-namespace write to ome/configmaps.
//
// Both objects are looked up by Get-or-Create so re-reconciles are
// no-ops; they outlive the BaseModel that triggered them so other BMs
// in the same namespace don't lose RBAC. Garbage collection is via
// namespace deletion (typical for short-lived test namespaces) or
// manual cleanup (long-lived production namespaces).
//
// Skips when jobNamespace == omeNamespace because the chart already
// provisions everything there; creating an extra RoleBinding with the
// same name would conflict with the chart-managed ClusterRoleBinding.
func ensureMetadataJobRBAC(ctx context.Context, c client.Client, log logr.Logger, jobNamespace, saName, clusterRoleName string) error {
	if saName == "" {
		// Caller's responsibility — buildMetadataJob would also fail
		// later with "ome-agent image must be configured" or similar.
		// Treat as no-op so we don't create a nameless SA.
		return nil
	}
	if jobNamespace == constants.OMENamespace {
		return nil
	}

	if err := ensureMetadataJobServiceAccount(ctx, c, log, jobNamespace, saName); err != nil {
		return err
	}
	return ensureMetadataJobOMENamespaceRoleBinding(ctx, c, log, jobNamespace, saName, clusterRoleName)
}

func ensureMetadataJobServiceAccount(ctx context.Context, c client.Client, log logr.Logger, namespace, saName string) error {
	sa := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: saName, Namespace: namespace}, sa)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get metadata Job ServiceAccount %s/%s: %w", namespace, saName, err)
	}

	sa = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/component": metadataJobRBACComponent},
		},
	}
	if err := c.Create(ctx, sa); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create metadata Job ServiceAccount %s/%s: %w", namespace, saName, err)
	}
	log.Info("Created metadata Job ServiceAccount", "namespace", namespace, "name", saName)
	return nil
}

// metadataJobOMENamespaceRoleBindingName returns the deterministic name
// of the cross-ns RoleBinding the controller maintains in the OME
// namespace for a given source `jobNamespace`. The pattern keeps a
// 1:1 mapping per source ns so concurrent BaseModels in the same
// jobNamespace reuse the same RoleBinding (idempotent), and BMs in
// different namespaces get distinct RoleBindings (no subject churn).
func metadataJobOMENamespaceRoleBindingName(saName, jobNamespace string) string {
	return saName + "-" + jobNamespace
}

func ensureMetadataJobOMENamespaceRoleBinding(ctx context.Context, c client.Client, log logr.Logger, jobNamespace, saName, clusterRoleName string) error {
	rbName := metadataJobOMENamespaceRoleBindingName(saName, jobNamespace)
	rb := &rbacv1.RoleBinding{}
	err := c.Get(ctx, types.NamespacedName{Name: rbName, Namespace: constants.OMENamespace}, rb)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("get metadata Job RoleBinding %s/%s: %w", constants.OMENamespace, rbName, err)
	}

	rb = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: constants.OMENamespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":   metadataJobRBACComponent,
				metadataJobSourceNamespaceLabel: jobNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      saName,
			Namespace: jobNamespace,
		}},
	}
	if err := c.Create(ctx, rb); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create metadata Job RoleBinding %s/%s: %w", constants.OMENamespace, rbName, err)
	}
	log.Info("Created metadata Job RoleBinding in OME namespace",
		"name", rbName,
		"jobNamespace", jobNamespace,
		"clusterRole", clusterRoleName)
	return nil
}
