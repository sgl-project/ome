package rbac

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// RBACReconciler reconciles RBAC resources for components
type RBACReconciler struct {
	client        client.Client
	scheme        *runtime.Scheme
	objectMeta    metav1.ObjectMeta
	componentType v1beta1.ComponentType
	Log           logr.Logger
}

// NewRBACReconciler creates a new RBAC reconciler
func NewRBACReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	objectMeta metav1.ObjectMeta,
	componentType v1beta1.ComponentType,
) *RBACReconciler {
	return &RBACReconciler{
		client:        client,
		scheme:        scheme,
		objectMeta:    objectMeta,
		componentType: componentType,
		Log:           ctrl.Log.WithName("RBACReconciler"),
	}
}

// Reconcile ensures the RBAC resources are created and configured correctly
func (r *RBACReconciler) Reconcile() error {
	r.Log.Info("Reconciling RBAC resources", "name", r.objectMeta.Name, "namespace", r.objectMeta.Namespace, "component", r.componentType)

	// Create ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.objectMeta.Name,
			Namespace: r.objectMeta.Namespace,
			Labels:    r.objectMeta.Labels,
		},
	}

	// Set owner reference
	if len(r.objectMeta.OwnerReferences) > 0 {
		sa.OwnerReferences = r.objectMeta.OwnerReferences
	}

	// Create or update ServiceAccount
	if err := r.createOrUpdate(sa); err != nil {
		return fmt.Errorf("failed to reconcile ServiceAccount: %w", err)
	}

	// Only create Role and RoleBinding for Router component
	if r.componentType == v1beta1.RouterComponent {
		// Create Role
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.objectMeta.Name,
				Namespace: r.objectMeta.Namespace,
				Labels:    r.objectMeta.Labels,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		// Set owner reference
		if len(r.objectMeta.OwnerReferences) > 0 {
			role.OwnerReferences = r.objectMeta.OwnerReferences
		}

		// Create or update Role
		if err := r.createOrUpdate(role); err != nil {
			return fmt.Errorf("failed to reconcile Role: %w", err)
		}

		// Create RoleBinding
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.objectMeta.Name,
				Namespace: r.objectMeta.Namespace,
				Labels:    r.objectMeta.Labels,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     r.objectMeta.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      r.objectMeta.Name,
					Namespace: r.objectMeta.Namespace,
				},
			},
		}

		// Set owner reference
		if len(r.objectMeta.OwnerReferences) > 0 {
			roleBinding.OwnerReferences = r.objectMeta.OwnerReferences
		}

		// Create or update RoleBinding
		if err := r.createOrUpdate(roleBinding); err != nil {
			return fmt.Errorf("failed to reconcile RoleBinding: %w", err)
		}
	}

	r.Log.Info("Successfully reconciled RBAC resources", "name", r.objectMeta.Name, "namespace", r.objectMeta.Namespace)
	return nil
}

// GetServiceAccountName returns the name of the ServiceAccount
func (r *RBACReconciler) GetServiceAccountName() string {
	return r.objectMeta.Name
}

// createOrUpdate creates or updates a Kubernetes resource
func (r *RBACReconciler) createOrUpdate(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	existing := obj.DeepCopyObject().(client.Object)

	err := r.client.Get(context.Background(), key, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the resource
			r.Log.Info("Creating resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
			return r.client.Create(context.Background(), obj)
		}
		return err
	}

	// Update the resource
	r.Log.Info("Updating resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
	obj.SetResourceVersion(existing.GetResourceVersion())
	return r.client.Update(context.Background(), obj)
}
