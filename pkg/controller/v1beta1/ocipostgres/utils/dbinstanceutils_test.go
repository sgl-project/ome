package utils

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple identifier",
			in:   "foo",
			want: `"foo"`,
		},
		{
			name: "empty identifier",
			in:   "",
			want: `""`,
		},
		{
			name: `identifier with " inside`,
			in:   `app"user`,
			want: `"app""user"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteIdent(tt.in)
			if got != tt.want {
				t.Fatalf("quoteIdent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestQuoteLiteral(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple literal",
			in:   "foo",
			want: `'foo'`,
		},
		{
			name: "empty literal",
			in:   "",
			want: `''`,
		},
		{
			name: "literal with single quote",
			in:   "O'Reilly",
			want: `'O''Reilly'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteLiteral(tt.in)
			if got != tt.want {
				t.Fatalf("quoteLiteral(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEnsureAppUserAndSecret_CreateNewSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.Background()
	appNS := "test-ns"
	secretName := "app-creds"
	dbName := "mydb"

	creds, err := EnsureAppUserAndSecret(ctx, cl, appNS, secretName, dbName)
	if err != nil {
		t.Fatalf("EnsureAppUserAndSecret returned error: %v", err)
	}

	if creds == nil {
		t.Fatal("expected non-nil creds")
	}

	expectedUser := "app_" + dbName + "_owner"
	if creds.User != expectedUser {
		t.Fatalf("expected user %q, got %q", expectedUser, creds.User)
	}
	if creds.SecretName != secretName {
		t.Fatalf("expected secretName %q, got %q", secretName, creds.SecretName)
	}
	if len(creds.Password) == 0 {
		t.Fatalf("expected non-empty password")
	}
	if len(creds.Password) < 20 {
		t.Fatalf("expected password length >= 20, got %d", len(creds.Password))
	}

	// Ensure the Secret was actually created in the fake client
	var sec corev1.Secret
	if err := cl.Get(ctx, clientKey(appNS, secretName), &sec); err != nil {
		t.Fatalf("expected secret to be created, get error: %v", err)
	}

	// For a newly created secret, data is in StringData when using fake client
	if sec.StringData[secretKeyUsername] != expectedUser {
		t.Fatalf("secret username = %q, want %q", sec.StringData[secretKeyUsername], expectedUser)
	}
	if sec.StringData[secretKeyPassword] != creds.Password {
		t.Fatalf("secret password mismatch with returned creds")
	}
	if sec.Labels["app"] != "postgres-instance" {
		t.Fatalf(`expected label app="postgres-instance", got %q`, sec.Labels["app"])
	}
}

// Reuse existing secret path: EnsureAppUserAndSecret should NOT recreate or change it.
func TestEnsureAppUserAndSecret_ReusesExistingSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}

	ctx := context.Background()
	appNS := "test-ns"
	secretName := "existing-secret"

	origUser := "existing_user"
	origPass := "existing_pass"

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: appNS,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretKeyUsername: []byte(origUser),
			secretKeyPassword: []byte(origPass),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existing).
		Build()

	creds, err := EnsureAppUserAndSecret(ctx, cl, appNS, secretName, "ignored-db-name")
	if err != nil {
		t.Fatalf("EnsureAppUserAndSecret returned error: %v", err)
	}

	if creds.User != origUser {
		t.Fatalf("expected user %q, got %q", origUser, creds.User)
	}
	if creds.Password != origPass {
		t.Fatalf("expected password %q, got %q", origPass, creds.Password)
	}
	if creds.SecretName != secretName {
		t.Fatalf("expected secretName %q, got %q", secretName, creds.SecretName)
	}

	// Ensure it didn't create a new secret or modify username/password
	var sec corev1.Secret
	if err := cl.Get(ctx, clientKey(appNS, secretName), &sec); err != nil {
		t.Fatalf("expected existing secret to still exist, get error: %v", err)
	}
	if string(sec.Data[secretKeyUsername]) != origUser {
		t.Fatalf("secret username changed: got %q, want %q", sec.Data[secretKeyUsername], origUser)
	}
	if string(sec.Data[secretKeyPassword]) != origPass {
		t.Fatalf("secret password changed: got %q, want %q", sec.Data[secretKeyPassword], origPass)
	}
}

// helper to avoid repeating NamespacedName construction
func clientKey(ns, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: ns, Name: name}
}
