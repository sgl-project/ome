package common

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	bytePlusSecretNamespace = "genai-byteplus"
)

type BytePlusServiceAccount struct {
	ResourceBase
	Resource *v1beta1.ServiceAccount
}

func NewBytePlusServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *BytePlusServiceAccount {
	return &BytePlusServiceAccount{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: sa,
	}
}

func (sa *BytePlusServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}
	if err := sa.Client.Update(ctx, sa.Resource); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError,
			fmt.Errorf("failed to update service account with owner reference: %w", err))
	}

	serviceName := sa.serviceName(project)
	serviceAccountID := strings.ToLower(GenerateId("user-", sa.Resource.UID))
	apiKeyID := strings.ToLower(GenerateId("key-", sa.Resource.UID))
	apiKeyValue, err := randomSmallString(24)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountID,
			Namespace: bytePlusSecretNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{serviceAccountID: []byte(apiKeyValue)},
	}
	if err := sa.upsertSecret(ctx, secret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	commonSecret, err := sa.configuredCommonSecret(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}
	if commonSecret.Data == nil {
		commonSecret.Data = map[string][]byte{}
	}
	commonSecret.Data[serviceAccountID] = []byte(apiKeyValue)
	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	creationTime := metav1.NewTime(time.Now())
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountId: &serviceAccountID,
		CreationTime:     &creationTime,
		APIKey: &v1beta1.APIKeySpec{
			Name:     &serviceName,
			APIKeyId: &apiKeyID,
			APIKeySecretRef: &v1beta1.SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
				Key:       serviceAccountID,
			},
		},
	}

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
}

func (sa *BytePlusServiceAccount) Delete(ctx context.Context) error {
	commonSecret, err := sa.configuredCommonSecret(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	if err := sa.deleteAPIKeySecret(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	if serviceAccountID := sa.serviceAccountIDForSecretKey(); serviceAccountID != "" && commonSecret.Data != nil {
		delete(commonSecret.Data, serviceAccountID)
		if err := sa.Client.Update(ctx, commonSecret); err != nil {
			return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
		}
	}

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
}

func (sa *BytePlusServiceAccount) configuredCommonSecret(ctx context.Context) (*corev1.Secret, error) {
	aiPlatformConfig, err := controllerconfig.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return nil, err
	}
	return sa.getCommonSecret(ctx, aiPlatformConfig)
}

func (sa *BytePlusServiceAccount) deleteAPIKeySecret(ctx context.Context, commonSecret *corev1.Secret) error {
	secretRef := sa.apiKeySecretRef()
	if secretRef == nil {
		return nil
	}
	if secretRef.Name == commonSecret.Name && secretRef.Namespace == commonSecret.Namespace {
		return nil
	}

	secret := &corev1.Secret{}
	err := sa.Client.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := sa.Client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (sa *BytePlusServiceAccount) apiKeySecretRef() *v1beta1.SecretReference {
	if sa.Resource.Status.APIKey != nil && sa.Resource.Status.APIKey.APIKeySecretRef != nil {
		return sa.Resource.Status.APIKey.APIKeySecretRef
	}
	if sa.Resource.Status.ServiceAccountId == nil {
		return nil
	}
	return &v1beta1.SecretReference{
		Name:      *sa.Resource.Status.ServiceAccountId,
		Namespace: bytePlusSecretNamespace,
		Key:       *sa.Resource.Status.ServiceAccountId,
	}
}

func (sa *BytePlusServiceAccount) serviceAccountIDForSecretKey() string {
	if sa.Resource.Status.ServiceAccountId != nil {
		return *sa.Resource.Status.ServiceAccountId
	}
	if sa.Resource.Status.APIKey != nil && sa.Resource.Status.APIKey.APIKeySecretRef != nil {
		return sa.Resource.Status.APIKey.APIKeySecretRef.Key
	}
	return ""
}

func (sa *BytePlusServiceAccount) Update(ctx context.Context, resource *v1beta1.ServiceAccount) error {
	return sa.Client.Update(ctx, resource)
}

func (sa *BytePlusServiceAccount) serviceName(project *v1beta1.Project) string {
	if sa.Resource.Spec.Name != nil && *sa.Resource.Spec.Name != "" {
		return *sa.Resource.Spec.Name
	}
	if sa.Resource.Name != "" {
		return sa.Resource.Name
	}
	return project.Spec.Name
}

func (sa *BytePlusServiceAccount) upsertSecret(ctx context.Context, desired *corev1.Secret) error {
	existing := &corev1.Secret{}
	err := sa.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return sa.Client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}
	for key, value := range desired.Data {
		existing.Data[key] = value
	}
	if desired.Type != "" {
		existing.Type = desired.Type
	}
	return sa.Client.Update(ctx, existing)
}

func randomSmallString(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random API key: %w", err)
	}

	out := make([]byte, length)
	for i, b := range buf {
		out[i] = base62chars[int(b)%len(base62chars)]
	}
	return string(out), nil
}
