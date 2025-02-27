package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/aip/common"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KeyRotator handles the rotation of OpenAI API keys
type KeyRotator struct {
	client           client.Client
	clientset        kubernetes.Interface
	base             common.ResourceBase
	rotationInterval time.Duration
	logger           *zap.SugaredLogger
}

// NewKeyRotator creates a new key rotator
func NewKeyRotator(client client.Client, clientset kubernetes.Interface, logger *zap.SugaredLogger) *KeyRotator {
	return &KeyRotator{
		client:    client,
		clientset: clientset,
		base: common.ResourceBase{
			Client:    client,
			Clientset: clientset,
		},
		rotationInterval: 30 * 24 * time.Hour, // default to 30 days
		logger:           logger,
	}
}

// SetRotationInterval sets the interval after which keys should be rotated
func (r *KeyRotator) SetRotationInterval(interval time.Duration) {
	r.rotationInterval = interval
}

// Start begins processing organizations from the channel
func (r *KeyRotator) Start(orgChan <-chan *v1beta1.Organization) {
	for org := range orgChan {
		if err := r.processOrganization(org); err != nil {
			r.logger.Errorw("Error processing organization", "error", err, "name", org.Name)
			keyRotationStatus.WithLabelValues("error").Inc()
			lastKeyRotationTime.WithLabelValues(org.Name, "error").Set(float64(time.Now().Unix()))
		}
	}
}

// processOrganization handles key rotation for a single organization
func (r *KeyRotator) processOrganization(org *v1beta1.Organization) error {
	ctx := context.Background()
	r.logger.Infow("Processing organization", "name", org.Name, "namespace", org.Namespace)

	// Get the secret containing the API key
	r.logger.Infow("Getting secret", "name", org.Spec.SecretRef.Name, "namespace", org.Spec.SecretRef.Namespace)
	secret, err := r.clientset.CoreV1().Secrets(org.Spec.SecretRef.Namespace).Get(ctx, org.Spec.SecretRef.Name, metav1.GetOptions{})
	if err != nil {
		r.logger.Errorw("Failed to get secret", "error", err, "name", org.Spec.SecretRef.Name, "namespace", org.Spec.SecretRef.Namespace)
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Initialize OpenAI client
	r.logger.Infow("Initializing OpenAI client", "org", org.Name)
	openAIClient, err := r.base.InitializeClient(ctx, org)
	if err != nil {
		r.logger.Errorw("Failed to initialize client", "error", err, "org", org.Name)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	// Get the current key ID from the secret
	currentKeyID := string(secret.Data[org.Spec.SecretRef.Key])
	if currentKeyID == "" {
		// If no key ID exists, we need to rotate
		r.logger.Infow("No existing key found, creating new key", "org", org.Name)
		if err := r.rotateKey(ctx, openAIClient, org, secret); err != nil {
			r.logger.Errorw("Failed to rotate key", "error", err, "org", org.Name)
			keyRotationStatus.WithLabelValues("error").Inc()
			lastKeyRotationTime.WithLabelValues(org.Name, "error").Set(float64(time.Now().Unix()))
			return fmt.Errorf("failed to rotate key: %w", err)
		}
		r.logger.Infow("Successfully rotated key", "org", org.Name)
		keyRotationStatus.WithLabelValues("success").Inc()
		lastKeyRotationTime.WithLabelValues(org.Name, "success").Set(float64(time.Now().Unix()))
		return nil
	}

	// Get the current key info directly
	r.logger.Infow("Checking existing key", "keyID", currentKeyID, "org", org.Name)
	key, err := openAIClient.AdminAPIKeys.Get(ctx, currentKeyID)
	if err != nil {
		// If key not found, we need to rotate
		r.logger.Infow("Key not found or error, rotating key", "error", err, "org", org.Name)
		if err := r.rotateKey(ctx, openAIClient, org, secret); err != nil {
			r.logger.Errorw("Failed to rotate key", "error", err, "org", org.Name)
			keyRotationStatus.WithLabelValues("error").Inc()
			lastKeyRotationTime.WithLabelValues(org.Name, "error").Set(float64(time.Now().Unix()))
			return fmt.Errorf("failed to rotate key: %w", err)
		}
		r.logger.Infow("Successfully rotated key", "org", org.Name)
		keyRotationStatus.WithLabelValues("success").Inc()
		lastKeyRotationTime.WithLabelValues(org.Name, "success").Set(float64(time.Now().Unix()))
		return nil
	}

	// Check if key needs rotation based on creation time
	keyAge := time.Since(time.Unix(key.CreatedAt, 0))
	r.logger.Infow("Checking key age", "age", keyAge, "rotationInterval", r.rotationInterval, "org", org.Name)
	if keyAge >= r.rotationInterval {
		r.logger.Infow("Key age exceeds rotation interval, rotating key", "age", keyAge, "rotationInterval", r.rotationInterval, "org", org.Name)
		if err := r.rotateKey(ctx, openAIClient, org, secret); err != nil {
			r.logger.Errorw("Failed to rotate key", "error", err, "org", org.Name)
			keyRotationStatus.WithLabelValues("error").Inc()
			lastKeyRotationTime.WithLabelValues(org.Name, "error").Set(float64(time.Now().Unix()))
			return fmt.Errorf("failed to rotate key: %w", err)
		}
		r.logger.Infow("Successfully rotated key", "org", org.Name)
		keyRotationStatus.WithLabelValues("success").Inc()
		lastKeyRotationTime.WithLabelValues(org.Name, "success").Set(float64(time.Now().Unix()))
	} else {
		r.logger.Infow("Key rotation not needed", "age", keyAge, "rotationInterval", r.rotationInterval, "org", org.Name)
		keyRotationStatus.WithLabelValues("skipped").Inc()
		lastKeyRotationTime.WithLabelValues(org.Name, "skipped").Set(float64(time.Now().Unix()))
	}

	return nil
}

// rotateKey creates a new admin key and updates both the secret and organization
func (r *KeyRotator) rotateKey(ctx context.Context, client *openaisdk.Client, org *v1beta1.Organization, secret *v1.Secret) error {
	r.logger.Infow("Rotating API key", "org", org.Name)

	// Create new admin API key
	r.logger.Info("Creating new admin API key")
	newKey, err := r.createAdminKey(ctx, client, org)
	if err != nil {
		r.logger.Errorw("Failed to create new admin key", "error", err, "org", org.Name)
		return fmt.Errorf("failed to create new admin key: %w", err)
	}

	// Update the secret with the new key
	r.logger.Infow("Updating secret with new API key", "secretName", secret.Name, "namespace", secret.Namespace)
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[org.Spec.SecretRef.Key] = []byte(newKey)

	// Update the secret first
	if _, err := r.clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		r.logger.Errorw("Failed to update secret", "error", err, "secretName", secret.Name, "namespace", secret.Namespace)
		return fmt.Errorf("failed to update secret: %w", err)
	}
	r.logger.Infow("Secret updated successfully", "secretName", secret.Name, "namespace", secret.Namespace)

	// Get the latest version of the organization
	r.logger.Infow("Getting latest organization", "name", org.Name)
	latestOrg := &v1beta1.Organization{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: org.Name}, latestOrg); err != nil {
		r.logger.Errorw("Failed to get latest organization", "error", err, "name", org.Name)
		return fmt.Errorf("failed to get latest organization: %w", err)
	}

	// Update the organization's secret reference
	r.logger.Infow("Updating organization's secret reference", "org", org.Name)
	latestOrg.Spec.SecretRef = &v1beta1.SecretReference{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Key:       org.Spec.SecretRef.Key,
	}

	// Update the organization
	if err := r.client.Update(ctx, latestOrg); err != nil {
		r.logger.Errorw("Failed to update organization", "error", err, "name", org.Name)
		return fmt.Errorf("failed to update organization: %w", err)
	}
	r.logger.Infow("Organization updated successfully", "name", org.Name)

	return nil
}

// createAdminKey creates a new admin API key using the OpenAI client
func (r *KeyRotator) createAdminKey(ctx context.Context, client *openaisdk.Client, org *v1beta1.Organization) (string, error) {
	// Create a descriptive name that includes org name and rotation date
	keyName := fmt.Sprintf("%s-admin-key-%s", org.Name, time.Now().UTC().Format("2006-01-02"))
	r.logger.Infow("Creating admin API key", "keyName", keyName, "org", org.Name)

	// Create a new admin API key
	resp, err := client.AdminAPIKeys.Create(ctx, openaisdk.AdminAPIKeyCreateRequest{
		Name: keyName,
	})
	if err != nil {
		r.logger.Errorw("Failed to create admin API key", "error", err, "org", org.Name)
		return "", fmt.Errorf("failed to create admin API key: %w", err)
	}

	if resp == nil || resp.Value == "" {
		r.logger.Error("No API key value returned in response")
		return "", fmt.Errorf("no API key value returned in response")
	}

	r.logger.Infow("Admin API key created successfully", "keyName", keyName, "org", org.Name)
	return resp.Value, nil
}
