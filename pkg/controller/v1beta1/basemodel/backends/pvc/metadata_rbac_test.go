package pvc

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/constants"
)

func newRBACTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1 AddToScheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("rbacv1 AddToScheme: %v", err)
	}
	return scheme
}

func TestEnsureMetadataJobRBAC_CreatesSAAndOMENamespaceRoleBinding(t *testing.T) {
	scheme := newRBACTestScheme(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.Background()
	const ns = "playground"
	const sa = "ome-model-metadata"

	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, ns, sa, sa); err != nil {
		t.Fatalf("ensureMetadataJobRBAC: %v", err)
	}

	gotSA := &corev1.ServiceAccount{}
	if err := c.Get(ctx, types.NamespacedName{Name: sa, Namespace: ns}, gotSA); err != nil {
		t.Fatalf("expected SA %s/%s to be created, got: %v", ns, sa, err)
	}
	if gotSA.Labels["app.kubernetes.io/component"] != metadataJobRBACComponent {
		t.Errorf("SA missing component label, got labels %v", gotSA.Labels)
	}

	rbName := metadataJobOMENamespaceRoleBindingName(sa, ns)
	gotRB := &rbacv1.RoleBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: rbName, Namespace: constants.OMENamespace}, gotRB); err != nil {
		t.Fatalf("expected RoleBinding %s/%s to be created, got: %v", constants.OMENamespace, rbName, err)
	}
	if gotRB.RoleRef.Kind != "ClusterRole" || gotRB.RoleRef.Name != sa {
		t.Errorf("RoleRef = %+v, want ClusterRole/%s", gotRB.RoleRef, sa)
	}
	if len(gotRB.Subjects) != 1 || gotRB.Subjects[0].Namespace != ns || gotRB.Subjects[0].Name != sa {
		t.Errorf("Subjects = %+v, want one ServiceAccount %s/%s", gotRB.Subjects, ns, sa)
	}
	if gotRB.Labels[metadataJobSourceNamespaceLabel] != ns {
		t.Errorf("RB missing source-ns label, got %v", gotRB.Labels)
	}
}

func TestEnsureMetadataJobRBAC_NoOpInOMENamespace(t *testing.T) {
	// Chart already provisions the SA, ClusterRole, and ClusterRoleBinding
	// in the OME namespace. The controller must NOT create a duplicate
	// RoleBinding there — it'd collide with the chart-managed objects on
	// upgrade and confuse operators about ownership.
	scheme := newRBACTestScheme(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.Background()
	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, constants.OMENamespace, "ome-model-metadata", "ome-model-metadata"); err != nil {
		t.Fatalf("ensureMetadataJobRBAC: %v", err)
	}

	// Nothing should have been created.
	sa := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: "ome-model-metadata", Namespace: constants.OMENamespace}, sa)
	if !errors.IsNotFound(err) {
		t.Errorf("expected no SA created in OME namespace, got %v", err)
	}
	rb := &rbacv1.RoleBinding{}
	rbName := metadataJobOMENamespaceRoleBindingName("ome-model-metadata", constants.OMENamespace)
	err = c.Get(ctx, types.NamespacedName{Name: rbName, Namespace: constants.OMENamespace}, rb)
	if !errors.IsNotFound(err) {
		t.Errorf("expected no RoleBinding created in OME namespace, got %v", err)
	}
}

func TestEnsureMetadataJobRBAC_IdempotentWhenObjectsAlreadyExist(t *testing.T) {
	// Re-reconciles must be safe — second call observes the existing
	// objects via Get and short-circuits without an Update or duplicate
	// Create. Pin this so a future "always patch labels" change doesn't
	// accidentally start writing on every reconcile (causes etcd churn
	// and resourceVersion bumps that fan out to every watch consumer).
	scheme := newRBACTestScheme(t)
	const ns = "playground"
	const sa = "ome-model-metadata"

	preExistingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: sa, Namespace: ns, Labels: map[string]string{"chart-managed": "true"}},
	}
	rbName := metadataJobOMENamespaceRoleBindingName(sa, ns)
	preExistingRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: constants.OMENamespace, Labels: map[string]string{"chart-managed": "true"}},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: sa},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa, Namespace: ns}},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(preExistingSA, preExistingRB).Build()

	ctx := context.Background()
	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, ns, sa, sa); err != nil {
		t.Fatalf("ensureMetadataJobRBAC: %v", err)
	}

	// SA labels must be unchanged — proves we did NOT overwrite.
	gotSA := &corev1.ServiceAccount{}
	if err := c.Get(ctx, types.NamespacedName{Name: sa, Namespace: ns}, gotSA); err != nil {
		t.Fatalf("get SA: %v", err)
	}
	if gotSA.Labels["chart-managed"] != "true" {
		t.Errorf("ensureMetadataJobRBAC overwrote pre-existing SA labels, got %v", gotSA.Labels)
	}
	gotRB := &rbacv1.RoleBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: rbName, Namespace: constants.OMENamespace}, gotRB); err != nil {
		t.Fatalf("get RB: %v", err)
	}
	if gotRB.Labels["chart-managed"] != "true" {
		t.Errorf("ensureMetadataJobRBAC overwrote pre-existing RB labels, got %v", gotRB.Labels)
	}
}

func TestEnsureMetadataJobRBAC_NoOpWhenSANameEmpty(t *testing.T) {
	// The Image-must-be-configured guard in buildMetadataJob fires later
	// and surfaces a clear PVCConfigMissing status. This function should
	// not preempt that with a confusing nameless SA create error.
	scheme := newRBACTestScheme(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.Background()
	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, "playground", "", ""); err != nil {
		t.Errorf("ensureMetadataJobRBAC with empty saName must be a no-op, got %v", err)
	}
}

func TestEnsureMetadataJobRBAC_PerSourceNSRoleBindingNamesAreDistinct(t *testing.T) {
	// Two BaseModels in two different user namespaces must both
	// successfully ensure their RBAC — the OME-ns RoleBindings must be
	// distinct so they don't overwrite each other's subjects.
	scheme := newRBACTestScheme(t)
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.Background()
	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, "playground", "ome-model-metadata", "ome-model-metadata"); err != nil {
		t.Fatalf("playground RBAC: %v", err)
	}
	if err := ensureMetadataJobRBAC(ctx, c, logf.Log, "research", "ome-model-metadata", "ome-model-metadata"); err != nil {
		t.Fatalf("research RBAC: %v", err)
	}

	for _, ns := range []string{"playground", "research"} {
		rbName := metadataJobOMENamespaceRoleBindingName("ome-model-metadata", ns)
		rb := &rbacv1.RoleBinding{}
		if err := c.Get(ctx, types.NamespacedName{Name: rbName, Namespace: constants.OMENamespace}, rb); err != nil {
			t.Errorf("expected RB %s/%s for source ns %s, got %v", constants.OMENamespace, rbName, ns, err)
			continue
		}
		if len(rb.Subjects) != 1 || rb.Subjects[0].Namespace != ns {
			t.Errorf("RB %q has subjects %+v, want single subject in ns %q", rbName, rb.Subjects, ns)
		}
	}
}
