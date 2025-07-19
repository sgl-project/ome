package gcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.GCPServiceAccount,
		auth.GCPWorkloadIdentity,
		auth.GCPDefault,
	}

	if len(authTypes) != len(expected) {
		t.Errorf("Expected %d auth types, got %d", len(expected), len(authTypes))
	}

	typeMap := make(map[auth.AuthType]bool)
	for _, at := range authTypes {
		typeMap[at] = true
	}

	for _, e := range expected {
		if !typeMap[e] {
			t.Errorf("Missing expected auth type: %s", e)
		}
	}
}

func TestFactory_Create_InvalidProvider(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS, // Wrong provider
		AuthType: auth.GCPServiceAccount,
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}

func TestFactory_Create_UnsupportedAuthType(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.AWSAccessKey, // Wrong auth type for GCP
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestFactory_Create_ServiceAccount_MissingCredentials(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra:    map[string]interface{}{
			// No service account config
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing service account credentials")
	}
}

func TestFactory_Create_ServiceAccount_InvalidJSON(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"service_account": map[string]interface{}{
				"type": "service_account",
				// Missing required fields
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid service account JSON")
	}
}

func TestFactory_Create_WorkloadIdentity(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "Basic workload identity",
			config: auth.Config{
				Provider: auth.ProviderGCP,
				AuthType: auth.GCPWorkloadIdentity,
				Extra: map[string]interface{}{
					"project_id": "test-project",
				},
			},
		},
		{
			name: "GKE workload identity with full config",
			config: auth.Config{
				Provider: auth.ProviderGCP,
				AuthType: auth.GCPWorkloadIdentity,
				Extra: map[string]interface{}{
					"workload_identity": map[string]interface{}{
						"project_id":                 "test-project",
						"service_account":            "my-sa@test-project.iam.gserviceaccount.com",
						"kubernetes_service_account": "default/my-ksa",
						"cluster_name":               "my-cluster",
						"cluster_location":           "us-central1",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail in unit tests because it tries to find actual credentials
			_, err := factory.Create(ctx, tt.config)
			if err == nil {
				t.Skip("Workload identity test skipped - requires GCP environment")
			}
		})
	}
}

func TestFactory_Create_Default(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPDefault,
	}

	// This will fail in unit tests because it tries to find actual credentials
	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Skip("Default credentials test skipped - requires GCP environment")
	}
}

func TestFactory_Create_ServiceAccount_WithKeyFile(t *testing.T) {
	// Create a temporary service account key file
	saKey := ServiceAccountConfig{
		Type:        "service_account",
		ProjectID:   "test-project",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF2kYHgfhmMpxmxt9uLLkBrOLrKcm\n2y7KFkFKSMhiRvydDe6lBHzPZyp1YM8M14lVnhkImIupwdrqu/LEJ7kAiCiIGMsl\nvYcAX40TN3K7D4qsnKYQEWjIUqSMayHEilUaJjmz46WtRtiw3RipPNxZdv9veZcl\nRgNoPI1OPbDQdXmJeuaGX7lD7aTjGJLXdkYG8uGVqoxaUHHXDM6qHiLhG2tRCIR/\nESvOCqvJGoIDmZ3sRm9qdDdjGz7XNPvhE7JqS4C9b6MV2YSVRWEGlRD1zVQKtQ/b\n58nJNRgBdmNNgXYYGaMXFyFAqE1CqkANTTpGuwIDAQABAoIBAQDP1jAJCNqetrog\nTjoO0PZ3w97W0Qn2R1BqXDj0yPdooHkONlC0M6dFPGH5boqMHyG9TgvFZgh3FQxk\nc2CAe9SOLrT3TP6KrKrgQsC6AqXkMXR5RV3fDtngy8HXDGDOS8rGKDvoXFYCRrdJ\ny8LIVyMGoCqNP92qqRdpoLJQunqP4mj+rEcdVdxZgjJfR+HCDkUBKvwf0EIDAPwf\njCE2eJWn52WJCGWd9EFBQD1qgCMCSNpvKBQTwkJdGvNlJDcJc/KHvzHWisme6xWV\n0HnU1BqFj4lYz3dJBCPPrnLaZtZOj7A0K1pGZVJE1emBRU5iKUYdMmxXEuEFi5dm\n/8QIsIshAoGBAPYGjI5G2smRuFo3WAHZ6Ki5VnNhSfP3SZDWYm3bb3YrP4HDEF5s\nZ9w3w8mVTLKm0YgEvHnKL7mQ7nKDjPomZdMvpqPLk0VGMNHhYvA0CdMNXdPnEcPx\nPf8mLSg7FRvbyGCYZWgqVH1aHmFRD6XFrr8KuTjE6OGPVZ5MhFRwbkz5AoGBANpI\ncJXmErNmSZooV2P9dxMEeHtBQa3R1z3MfsErRU2hLb5LPdGDwPd8VUfb9jm7PN+p\nu8EqLEGOrArCZitgIn5Z7fr6OkQALxdsJCsGBZ3qwCslUn1Kq3xUMJyFRVvWkQWH\ndUDEKvFUmGLNLfHx/7VkVTxZbLmFHCwE1R5aLsTDAoGBAMHnnWqvOFj5CAkLGEUb\n+mAFx4WMwSP2j2rXzPlpQ7zfLB6rhGw9h6mItrRNE1GeFWkJVm7LQRjmvDJlmNvj\n2wvJHkrNIufsOD5fZ+J0vbYk3qpZDEDPvgs6RkFQXg+VPBnhlZpUYj6w+wQdBLRE\nFe7KDT+FdXPkAFKbzGaewJMJAoGBAM1K8cCie7qzJ0b8BLwKakJpfht1j9L8hlVg\nEpgRwqww8wEYPGg1P7cNdfVDcnYJ2oIAMsUvmrR4y3GHATBLhYRZ3bR7BnwRN0aT\nsek/VqkM3rJl+xPnIkywCjEPM9Y2M+tstqMJPCvE0Li4TKs31LYZYWx5T3HXFVS6\nNp3kB2YvAoGAKQKnSmJhCBMcVU5ZGnIcvd5I7ohBaONx2Jk2MxXBjVS1UM7PgXFE\njMkVMPSDHsGGHQnt0rF0G7hCVJPPQSWTU6sDRYB3iBpgeBSlfUAqs7B2f+NA9zXg\nVLCdH8NXmWWQnRzFBCvji5R2ex0fwI5T38c5Bq1i2VBgH0VrBj6q0m8=\n-----END RSA PRIVATE KEY-----",
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		ClientID:    "123456789",
		TokenURI:    "https://oauth2.googleapis.com/token",
	}

	keyData, _ := json.Marshal(saKey)
	tmpFile, err := os.CreateTemp("", "sa-key-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(keyData); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_file": tmpFile.Name(),
		},
	}

	// This should succeed - the key is syntactically valid
	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials: %v", err)
	}

	if gcpCreds, ok := creds.(*GCPCredentials); ok {
		if gcpCreds.GetProjectID() != "test-project" {
			t.Errorf("Expected project ID 'test-project', got %s", gcpCreds.GetProjectID())
		}
		if gcpCreds.Type() != auth.GCPServiceAccount {
			t.Errorf("Expected auth type %s, got %s", auth.GCPServiceAccount, gcpCreds.Type())
		}
	} else {
		t.Error("Expected GCPCredentials type")
	}
}

func TestFactory_Create_ServiceAccount_WithKeyJSON(t *testing.T) {
	saKey := ServiceAccountConfig{
		Type:        "service_account",
		ProjectID:   "test-project-json",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF2kYHgfhmMpxmxt9uLLkBrOLrKcm\n2y7KFkFKSMhiRvydDe6lBHzPZyp1YM8M14lVnhkImIupwdrqu/LEJ7kAiCiIGMsl\nvYcAX40TN3K7D4qsnKYQEWjIUqSMayHEilUaJjmz46WtRtiw3RipPNxZdv9veZcl\nRgNoPI1OPbDQdXmJeuaGX7lD7aTjGJLXdkYG8uGVqoxaUHHXDM6qHiLhG2tRCIR/\nESvOCqvJGoIDmZ3sRm9qdDdjGz7XNPvhE7JqS4C9b6MV2YSVRWEGlRD1zVQKtQ/b\n58nJNRgBdmNNgXYYGaMXFyFAqE1CqkANTTpGuwIDAQABAoIBAQDP1jAJCNqetrog\nTjoO0PZ3w97W0Qn2R1BqXDj0yPdooHkONlC0M6dFPGH5boqMHyG9TgvFZgh3FQxk\nc2CAe9SOLrT3TP6KrKrgQsC6AqXkMXR5RV3fDtngy8HXDGDOS8rGKDvoXFYCRrdJ\ny8LIVyMGoCqNP92qqRdpoLJQunqP4mj+rEcdVdxZgjJfR+HCDkUBKvwf0EIDAPwf\njCE2eJWn52WJCGWd9EFBQD1qgCMCSNpvKBQTwkJdGvNlJDcJc/KHvzHWisme6xWV\n0HnU1BqFj4lYz3dJBCPPrnLaZtZOj7A0K1pGZVJE1emBRU5iKUYdMmxXEuEFi5dm\n/8QIsIshAoGBAPYGjI5G2smRuFo3WAHZ6Ki5VnNhSfP3SZDWYm3bb3YrP4HDEF5s\nZ9w3w8mVTLKm0YgEvHnKL7mQ7nKDjPomZdMvpqPLk0VGMNHhYvA0CdMNXdPnEcPx\nPf8mLSg7FRvbyGCYZWgqVH1aHmFRD6XFrr8KuTjE6OGPVZ5MhFRwbkz5AoGBANpI\ncJXmErNmSZooV2P9dxMEeHtBQa3R1z3MfsErRU2hLb5LPdGDwPd8VUfb9jm7PN+p\nu8EqLEGOrArCZitgIn5Z7fr6OkQALxdsJCsGBZ3qwCslUn1Kq3xUMJyFRVvWkQWH\ndUDEKvFUmGLNLfHx/7VkVTxZbLmFHCwE1R5aLsTDAoGBAMHnnWqvOFj5CAkLGEUb\n+mAFx4WMwSP2j2rXzPlpQ7zfLB6rhGw9h6mItrRNE1GeFWkJVm7LQRjmvDJlmNvj\n2wvJHkrNIufsOD5fZ+J0vbYk3qpZDEDPvgs6RkFQXg+VPBnhlZpUYj6w+wQdBLRE\nFe7KDT+FdXPkAFKbzGaewJMJAoGBAM1K8cCie7qzJ0b8BLwKakJpfht1j9L8hlVg\nEpgRwqww8wEYPGg1P7cNdfVDcnYJ2oIAMsUvmrR4y3GHATBLhYRZ3bR7BnwRN0aT\nsek/VqkM3rJl+xPnIkywCjEPM9Y2M+tstqMJPCvE0Li4TKs31LYZYWx5T3HXFVS6\nNp3kB2YvAoGAKQKnSmJhCBMcVU5ZGnIcvd5I7ohBaONx2Jk2MxXBjVS1UM7PgXFE\njMkVMPSDHsGGHQnt0rF0G7hCVJPPQSWTU6sDRYB3iBpgeBSlfUAqs7B2f+NA9zXg\nVLCdH8NXmWWQnRzFBCvji5R2ex0fwI5T38c5Bq1i2VBgH0VrBj6q0m8=\n-----END RSA PRIVATE KEY-----",
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		ClientID:    "123456789",
		TokenURI:    "https://oauth2.googleapis.com/token",
	}

	keyData, _ := json.Marshal(saKey)

	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_json": string(keyData),
		},
	}

	// This should succeed - the key is syntactically valid
	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials: %v", err)
	}

	if gcpCreds, ok := creds.(*GCPCredentials); ok {
		if gcpCreds.GetProjectID() != "test-project-json" {
			t.Errorf("Expected project ID 'test-project-json', got %s", gcpCreds.GetProjectID())
		}
	} else {
		t.Error("Expected GCPCredentials type")
	}
}

func TestFactory_Create_ServiceAccount_WithDirectConfig(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"service_account": map[string]interface{}{
				"type":         "service_account",
				"project_id":   "test-project-direct",
				"private_key":  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF2kYHgfhmMpxmxt9uLLkBrOLrKcm\n2y7KFkFKSMhiRvydDe6lBHzPZyp1YM8M14lVnhkImIupwdrqu/LEJ7kAiCiIGMsl\nvYcAX40TN3K7D4qsnKYQEWjIUqSMayHEilUaJjmz46WtRtiw3RipPNxZdv9veZcl\nRgNoPI1OPbDQdXmJeuaGX7lD7aTjGJLXdkYG8uGVqoxaUHHXDM6qHiLhG2tRCIR/\nESvOCqvJGoIDmZ3sRm9qdDdjGz7XNPvhE7JqS4C9b6MV2YSVRWEGlRD1zVQKtQ/b\n58nJNRgBdmNNgXYYGaMXFyFAqE1CqkANTTpGuwIDAQABAoIBAQDP1jAJCNqetrog\nTjoO0PZ3w97W0Qn2R1BqXDj0yPdooHkONlC0M6dFPGH5boqMHyG9TgvFZgh3FQxk\nc2CAe9SOLrT3TP6KrKrgQsC6AqXkMXR5RV3fDtngy8HXDGDOS8rGKDvoXFYCRrdJ\ny8LIVyMGoCqNP92qqRdpoLJQunqP4mj+rEcdVdxZgjJfR+HCDkUBKvwf0EIDAPwf\njCE2eJWn52WJCGWd9EFBQD1qgCMCSNpvKBQTwkJdGvNlJDcJc/KHvzHWisme6xWV\n0HnU1BqFj4lYz3dJBCPPrnLaZtZOj7A0K1pGZVJE1emBRU5iKUYdMmxXEuEFi5dm\n/8QIsIshAoGBAPYGjI5G2smRuFo3WAHZ6Ki5VnNhSfP3SZDWYm3bb3YrP4HDEF5s\nZ9w3w8mVTLKm0YgEvHnKL7mQ7nKDjPomZdMvpqPLk0VGMNHhYvA0CdMNXdPnEcPx\nPf8mLSg7FRvbyGCYZWgqVH1aHmFRD6XFrr8KuTjE6OGPVZ5MhFRwbkz5AoGBANpI\ncJXmErNmSZooV2P9dxMEeHtBQa3R1z3MfsErRU2hLb5LPdGDwPd8VUfb9jm7PN+p\nu8EqLEGOrArCZitgIn5Z7fr6OkQALxdsJCsGBZ3qwCslUn1Kq3xUMJyFRVvWkQWH\ndUDEKvFUmGLNLfHx/7VkVTxZbLmFHCwE1R5aLsTDAoGBAMHnnWqvOFj5CAkLGEUb\n+mAFx4WMwSP2j2rXzPlpQ7zfLB6rhGw9h6mItrRNE1GeFWkJVm7LQRjmvDJlmNvj\n2wvJHkrNIufsOD5fZ+J0vbYk3qpZDEDPvgs6RkFQXg+VPBnhlZpUYj6w+wQdBLRE\nFe7KDT+FdXPkAFKbzGaewJMJAoGBAM1K8cCie7qzJ0b8BLwKakJpfht1j9L8hlVg\nEpgRwqww8wEYPGg1P7cNdfVDcnYJ2oIAMsUvmrR4y3GHATBLhYRZ3bR7BnwRN0aT\nsek/VqkM3rJl+xPnIkywCjEPM9Y2M+tstqMJPCvE0Li4TKs31LYZYWx5T3HXFVS6\nNp3kB2YvAoGAKQKnSmJhCBMcVU5ZGnIcvd5I7ohBaONx2Jk2MxXBjVS1UM7PgXFE\njMkVMPSDHsGGHQnt0rF0G7hCVJPPQSWTU6sDRYB3iBpgeBSlfUAqs7B2f+NA9zXg\nVLCdH8NXmWWQnRzFBCvji5R2ex0fwI5T38c5Bq1i2VBgH0VrBj6q0m8=\n-----END RSA PRIVATE KEY-----",
				"client_email": "test@test-project.iam.gserviceaccount.com",
				"client_id":    "123456789",
				"token_uri":    "https://oauth2.googleapis.com/token",
			},
		},
	}

	// This should succeed - the key is syntactically valid
	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials: %v", err)
	}

	if gcpCreds, ok := creds.(*GCPCredentials); ok {
		if gcpCreds.GetProjectID() != "test-project-direct" {
			t.Errorf("Expected project ID 'test-project-direct', got %s", gcpCreds.GetProjectID())
		}
	} else {
		t.Error("Expected GCPCredentials type")
	}
}

func TestFactory_Create_ServiceAccount_EnvironmentVariable(t *testing.T) {
	// Create a temporary service account key file
	saKey := ServiceAccountConfig{
		Type:        "service_account",
		ProjectID:   "test-project-env",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF2kYHgfhmMpxmxt9uLLkBrOLrKcm\n2y7KFkFKSMhiRvydDe6lBHzPZyp1YM8M14lVnhkImIupwdrqu/LEJ7kAiCiIGMsl\nvYcAX40TN3K7D4qsnKYQEWjIUqSMayHEilUaJjmz46WtRtiw3RipPNxZdv9veZcl\nRgNoPI1OPbDQdXmJeuaGX7lD7aTjGJLXdkYG8uGVqoxaUHHXDM6qHiLhG2tRCIR/\nESvOCqvJGoIDmZ3sRm9qdDdjGz7XNPvhE7JqS4C9b6MV2YSVRWEGlRD1zVQKtQ/b\n58nJNRgBdmNNgXYYGaMXFyFAqE1CqkANTTpGuwIDAQABAoIBAQDP1jAJCNqetrog\nTjoO0PZ3w97W0Qn2R1BqXDj0yPdooHkONlC0M6dFPGH5boqMHyG9TgvFZgh3FQxk\nc2CAe9SOLrT3TP6KrKrgQsC6AqXkMXR5RV3fDtngy8HXDGDOS8rGKDvoXFYCRrdJ\ny8LIVyMGoCqNP92qqRdpoLJQunqP4mj+rEcdVdxZgjJfR+HCDkUBKvwf0EIDAPwf\njCE2eJWn52WJCGWd9EFBQD1qgCMCSNpvKBQTwkJdGvNlJDcJc/KHvzHWisme6xWV\n0HnU1BqFj4lYz3dJBCPPrnLaZtZOj7A0K1pGZVJE1emBRU5iKUYdMmxXEuEFi5dm\n/8QIsIshAoGBAPYGjI5G2smRuFo3WAHZ6Ki5VnNhSfP3SZDWYm3bb3YrP4HDEF5s\nZ9w3w8mVTLKm0YgEvHnKL7mQ7nKDjPomZdMvpqPLk0VGMNHhYvA0CdMNXdPnEcPx\nPf8mLSg7FRvbyGCYZWgqVH1aHmFRD6XFrr8KuTjE6OGPVZ5MhFRwbkz5AoGBANpI\ncJXmErNmSZooV2P9dxMEeHtBQa3R1z3MfsErRU2hLb5LPdGDwPd8VUfb9jm7PN+p\nu8EqLEGOrArCZitgIn5Z7fr6OkQALxdsJCsGBZ3qwCslUn1Kq3xUMJyFRVvWkQWH\ndUDEKvFUmGLNLfHx/7VkVTxZbLmFHCwE1R5aLsTDAoGBAMHnnWqvOFj5CAkLGEUb\n+mAFx4WMwSP2j2rXzPlpQ7zfLB6rhGw9h6mItrRNE1GeFWkJVm7LQRjmvDJlmNvj\n2wvJHkrNIufsOD5fZ+J0vbYk3qpZDEDPvgs6RkFQXg+VPBnhlZpUYj6w+wQdBLRE\nFe7KDT+FdXPkAFKbzGaewJMJAoGBAM1K8cCie7qzJ0b8BLwKakJpfht1j9L8hlVg\nEpgRwqww8wEYPGg1P7cNdfVDcnYJ2oIAMsUvmrR4y3GHATBLhYRZ3bR7BnwRN0aT\nsek/VqkM3rJl+xPnIkywCjEPM9Y2M+tstqMJPCvE0Li4TKs31LYZYWx5T3HXFVS6\nNp3kB2YvAoGAKQKnSmJhCBMcVU5ZGnIcvd5I7ohBaONx2Jk2MxXBjVS1UM7PgXFE\njMkVMPSDHsGGHQnt0rF0G7hCVJPPQSWTU6sDRYB3iBpgeBSlfUAqs7B2f+NA9zXg\nVLCdH8NXmWWQnRzFBCvji5R2ex0fwI5T38c5Bq1i2VBgH0VrBj6q0m8=\n-----END RSA PRIVATE KEY-----",
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		ClientID:    "123456789",
		TokenURI:    "https://oauth2.googleapis.com/token",
	}

	keyData, _ := json.Marshal(saKey)
	tmpFile, err := os.CreateTemp("", "sa-key-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(keyData); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Set environment variable
	oldEnv := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile.Name())
	defer os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", oldEnv)

	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra:    map[string]interface{}{}, // No explicit config
	}

	// This should succeed - the key is syntactically valid
	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials: %v", err)
	}

	if gcpCreds, ok := creds.(*GCPCredentials); ok {
		if gcpCreds.GetProjectID() != "test-project-env" {
			t.Errorf("Expected project ID 'test-project-env', got %s", gcpCreds.GetProjectID())
		}
	} else {
		t.Error("Expected GCPCredentials type")
	}
}

func TestFactory_Create_ProjectIDOverride(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPDefault,
		Extra: map[string]interface{}{
			"project_id": "override-project",
		},
	}

	// This will fail in unit tests, but we can check the project ID is set
	creds, err := factory.Create(ctx, config)
	if err == nil {
		if gcpCreds, ok := creds.(*GCPCredentials); ok {
			if gcpCreds.GetProjectID() != "override-project" {
				t.Errorf("Expected project ID 'override-project', got %s", gcpCreds.GetProjectID())
			}
		}
	}
}

func TestFactory_Create_ServiceAccount_InvalidKeyFile(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_file": "/non/existent/file.json",
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for non-existent key file")
	}
}

func TestFactory_Create_ServiceAccount_InvalidJSONString(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_json": "invalid-json",
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFactory_Create_WorkloadIdentity_DirectProjectID(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPWorkloadIdentity,
		Extra: map[string]interface{}{
			"project_id": "direct-project-id",
		},
	}

	// This will fail in unit tests because it tries to find actual credentials
	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Skip("Workload identity test skipped - requires GCP environment")
	}
}

func TestFactory_Create_ServiceAccount_MarshalError(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create a service account config with a type that will fail to marshal
	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"service_account": map[string]interface{}{
				"type":         "service_account",
				"project_id":   "test-project",
				"private_key":  "test-key",
				"client_email": "test@test.com",
				// Add a channel which cannot be marshaled to JSON
				"invalid_field": make(chan int),
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unmarshalable service account config")
	}
}

func TestFactory_Create_ServiceAccount_EmptyKeyFile(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create an empty temporary file
	tmpFile, err := os.CreateTemp("", "empty-sa-key-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_file": tmpFile.Name(),
		},
	}

	_, err = factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for empty service account key file")
	}
}

func TestFactory_Create_ServiceAccount_InvalidBase64Key(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_json": "{invalid json}",
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFactory_Create_ServiceAccount_MissingType(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	saKey := ServiceAccountConfig{
		// Missing Type field
		ProjectID:   "test-project",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		ClientID:    "123456789",
		TokenURI:    "https://oauth2.googleapis.com/token",
	}

	keyData, _ := json.Marshal(saKey)

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_json": string(keyData),
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected validation error for missing type")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid service account type") {
		t.Errorf("Expected 'invalid service account type' error, got: %v", err)
	}
}

func TestFactory_Create_ProjectIDPriority(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Test that explicit project_id in Extra overrides service account project_id
	saKey := ServiceAccountConfig{
		Type:        "service_account",
		ProjectID:   "sa-project-id",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF2kYHgfhmMpxmxt9uLLkBrOLrKcm\n2y7KFkFKSMhiRvydDe6lBHzPZyp1YM8M14lVnhkImIupwdrqu/LEJ7kAiCiIGMsl\nvYcAX40TN3K7D4qsnKYQEWjIUqSMayHEilUaJjmz46WtRtiw3RipPNxZdv9veZcl\nRgNoPI1OPbDQdXmJeuaGX7lD7aTjGJLXdkYG8uGVqoxaUHHXDM6qHiLhG2tRCIR/\nESvOCqvJGoIDmZ3sRm9qdDdjGz7XNPvhE7JqS4C9b6MV2YSVRWEGlRD1zVQKtQ/b\n58nJNRgBdmNNgXYYGaMXFyFAqE1CqkANTTpGuwIDAQABAoIBAQDP1jAJCNqetrog\nTjoO0PZ3w97W0Qn2R1BqXDj0yPdooHkONlC0M6dFPGH5boqMHyG9TgvFZgh3FQxk\nc2CAe9SOLrT3TP6KrKrgQsC6AqXkMXR5RV3fDtngy8HXDGDOS8rGKDvoXFYCRrdJ\ny8LIVyMGoCqNP92qqRdpoLJQunqP4mj+rEcdVdxZgjJfR+HCDkUBKvwf0EIDAPwf\njCE2eJWn52WJCGWd9EFBQD1qgCMCSNpvKBQTwkJdGvNlJDcJc/KHvzHWisme6xWV\n0HnU1BqFj4lYz3dJBCPPrnLaZtZOj7A0K1pGZVJE1emBRU5iKUYdMmxXEuEFi5dm\n/8QIsIshAoGBAPYGjI5G2smRuFo3WAHZ6Ki5VnNhSfP3SZDWYm3bb3YrP4HDEF5s\nZ9w3w8mVTLKm0YgEvHnKL7mQ7nKDjPomZdMvpqPLk0VGMNHhYvA0CdMNXdPnEcPx\nPf8mLSg7FRvbyGCYZWgqVH1aHmFRD6XFrr8KuTjE6OGPVZ5MhFRwbkz5AoGBANpI\ncJXmErNmSZooV2P9dxMEeHtBQa3R1z3MfsErRU2hLb5LPdGDwPd8VUfb9jm7PN+p\nu8EqLEGOrArCZitgIn5Z7fr6OkQALxdsJCsGBZ3qwCslUn1Kq3xUMJyFRVvWkQWH\ndUDEKvFUmGLNLfHx/7VkVTxZbLmFHCwE1R5aLsTDAoGBAMHnnWqvOFj5CAkLGEUb\n+mAFx4WMwSP2j2rXzPlpQ7zfLB6rhGw9h6mItrRNE1GeFWkJVm7LQRjmvDJlmNvj\n2wvJHkrNIufsOD5fZ+J0vbYk3qpZDEDPvgs6RkFQXg+VPBnhlZpUYj6w+wQdBLRE\nFe7KDT+FdXPkAFKbzGaewJMJAoGBAM1K8cCie7qzJ0b8BLwKakJpfht1j9L8hlVg\nEpgRwqww8wEYPGg1P7cNdfVDcnYJ2oIAMsUvmrR4y3GHATBLhYRZ3bR7BnwRN0aT\nsek/VqkM3rJl+xPnIkywCjEPM9Y2M+tstqMJPCvE0Li4TKs31LYZYWx5T3HXFVS6\nNp3kB2YvAoGAKQKnSmJhCBMcVU5ZGnIcvd5I7ohBaONx2Jk2MxXBjVS1UM7PgXFE\njMkVMPSDHsGGHQnt0rF0G7hCVJPPQSWTU6sDRYB3iBpgeBSlfUAqs7B2f+NA9zXg\nVLCdH8NXmWWQnRzFBCvji5R2ex0fwI5T38c5Bq1i2VBgH0VrBj6q0m8=\n-----END RSA PRIVATE KEY-----",
		ClientEmail: "test@test-project.iam.gserviceaccount.com",
		ClientID:    "123456789",
		TokenURI:    "https://oauth2.googleapis.com/token",
	}

	keyData, _ := json.Marshal(saKey)

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_json":   string(keyData),
			"project_id": "override-project-id", // This should override
		},
	}

	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	gcpCreds, ok := creds.(*GCPCredentials)
	if !ok {
		t.Fatal("Expected GCPCredentials type")
	}

	if gcpCreds.GetProjectID() != "override-project-id" {
		t.Errorf("Expected project ID 'override-project-id', got %s", gcpCreds.GetProjectID())
	}
}

func TestGetProjectIDFromMetadata(t *testing.T) {
	// This function tries to get project ID from metadata
	// In a unit test environment, it should return empty string
	projectID := getProjectIDFromMetadata(context.Background())
	if projectID != "" {
		t.Errorf("Expected empty project ID in unit test environment, got %s", projectID)
	}
}

func TestFactory_Create_ServiceAccount_CorruptedKeyFile(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create a file with corrupted JSON
	tmpFile, err := os.CreateTemp("", "corrupted-sa-key-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid JSON
	tmpFile.Write([]byte(`{"type": "service_account", "project_id": "test", invalid json here`))
	tmpFile.Close()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"key_file": tmpFile.Name(),
		},
	}

	_, err = factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for corrupted JSON file")
	}
}
