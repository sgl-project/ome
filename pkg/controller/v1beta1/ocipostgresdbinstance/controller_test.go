package ocipostgresdbinstance

import (
	"context"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// -------------------- tlsPgConfig tests --------------------

func TestTlsPgConfig_InvalidCAPEM(t *testing.T) {
	// invalid PEM should return an error "invalid CA certificate PEM"
	_, err := tlsPgConfig(
		"db.example.com",
		5432,
		"admin",
		"secret",
		"postgres",
		"NOT_A_VALID_PEM",
		"db.example.com",
	)
	if err == nil {
		t.Fatalf("expected error for invalid CA PEM, got nil")
	}
}

// -------------------- ensureAppUserAndSecret tests --------------------

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1): %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
}

func TestEnsureAppUserAndSecret_CreateNew(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	appNS := "test-ns"
	secretName := "mydb-db-credentials"
	dbName := "mydb"

	creds, err := ensureAppUserAndSecret(ctx, c, appNS, secretName, dbName)
	if err != nil {
		t.Fatalf("ensureAppUserAndSecret returned error: %v", err)
	}

	// Check returned struct
	wantUser := "app_mydb_owner"
	if creds.User != wantUser {
		t.Fatalf("appCreds.User mismatch: got %q, want %q", creds.User, wantUser)
	}
	if creds.SecretName != secretName {
		t.Fatalf("appCreds.SecretName mismatch: got %q, want %q", creds.SecretName, secretName)
	}
	if creds.Password == "" {
		t.Fatalf("appCreds.Password should not be empty")
	}

	// Check the Secret in the fake client
	sec := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: appNS, Name: secretName}, sec); err != nil {
		t.Fatalf("failed to get created Secret: %v", err)
	}

	// Because we used StringData in production code, fake client will keep it there.
	gotUser := sec.StringData[secretKeyUsername]
	if gotUser == "" {
		t.Fatalf("secret username mismatch: got %q", gotUser)
	}
	if gotUser != wantUser {
		t.Fatalf("secret username mismatch: got %q, want %q", gotUser, wantUser)
	}
}

func TestEnsureAppUserAndSecret_ReuseExisting(t *testing.T) {
	ctx := context.Background()
	ns := "test-ns"
	secretName := "demo-db-credentials"
	dbName := "demo"

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
		},
		Data: map[string][]byte{
			secretKeyUsername: []byte("existing_user"),
			secretKeyPassword: []byte("existing_pass"),
		},
	}

	cl := newFakeClient(t, existing)

	creds, err := ensureAppUserAndSecret(ctx, cl, ns, secretName, dbName)
	if err != nil {
		t.Fatalf("ensureAppUserAndSecret returned error: %v", err)
	}

	if creds.User != "existing_user" {
		t.Fatalf("expected user existing_user, got %q", creds.User)
	}
	if creds.Password != "existing_pass" {
		t.Fatalf("expected password existing_pass, got %q", creds.Password)
	}
	if creds.SecretName != secretName {
		t.Fatalf("expected secret name %q, got %q", secretName, creds.SecretName)
	}
}

// -------------------- getClusterAdminCreds tests --------------------

func TestGetClusterAdminCreds_Success(t *testing.T) {
	ctx := context.Background()
	ns := "db-ns"
	name := "admin-secret"

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			secretKeyUsername: []byte("admin_user"),
			secretKeyPassword: []byte("admin_pass"),
		},
	}

	cl := newFakeClient(t, sec)

	creds, err := getClusterAdminCreds(ctx, cl, ns, name)
	if err != nil {
		t.Fatalf("getClusterAdminCreds returned error: %v", err)
	}
	if creds.Admin != "admin_user" || creds.AdminPass != "admin_pass" {
		t.Fatalf("unexpected creds: %+v", creds)
	}
}

func TestGetClusterAdminCreds_MissingFields(t *testing.T) {
	ctx := context.Background()
	ns := "db-ns"
	name := "admin-secret"

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			secretKeyUsername: []byte("admin_user"),
			// password intentionally missing
		},
	}

	cl := newFakeClient(t, sec)

	if _, err := getClusterAdminCreds(ctx, cl, ns, name); err == nil {
		t.Fatalf("expected error when password is missing, got nil")
	}
}

// -------------------- finalizer helpers tests --------------------

func TestAddFinalizerIfNeeded_AddsOnce(t *testing.T) {
	cr := &v1beta1.OciPostgresDBInstance{}
	added := addFinalizerIfNeeded(cr)
	if !added {
		t.Fatalf("expected finalizer to be added on empty list")
	}
	if len(cr.Finalizers) != 1 || cr.Finalizers[0] != finalizerName {
		t.Fatalf("unexpected finalizers: %#v", cr.Finalizers)
	}

	// call again: should not add a duplicate
	added = addFinalizerIfNeeded(cr)
	if added {
		t.Fatalf("expected no change when finalizer already present")
	}
	if len(cr.Finalizers) != 1 {
		t.Fatalf("expected exactly one finalizer, got %#v", cr.Finalizers)
	}
}

func TestRemoveFinalizer(t *testing.T) {
	cr := &v1beta1.OciPostgresDBInstance{
		ObjectMeta: metav1.ObjectMeta{
			Finalizers: []string{"other", finalizerName},
		},
	}
	removeFinalizer(cr)
	for _, f := range cr.Finalizers {
		if f == finalizerName {
			t.Fatalf("finalizer %q should have been removed", finalizerName)
		}
	}
}

// -------------------- quoting helpers tests --------------------

func TestQuoteIdent(t *testing.T) {
	got := quoteIdent(`role"name`)
	want := `"role""name"`
	if got != want {
		t.Fatalf("quoteIdent mismatch: got %q, want %q", got, want)
	}
}

func TestQuoteLiteral(t *testing.T) {
	got := quoteLiteral(`p@ss'word`)
	want := `'p@ss''word'`
	if got != want {
		t.Fatalf("quoteLiteral mismatch: got %q, want %q", got, want)
	}
}
