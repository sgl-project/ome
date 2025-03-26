package common

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// NewServiceAccount returns a new ServiceAccount
func NewServiceAccount(ctx context.Context, c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) (ServiceAccountOperation, error) {
	project := &v1beta1.Project{}
	if err := c.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name}, project); err != nil {
		// For NotFound errors, log without stack trace as this is expected in some cases
		if apierrors.IsNotFound(err) {
			log.Info("Project not found",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		} else {
			// For unexpected errors, log with more details
			log.Error(err, "Failed to get project",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"projectRef", sa.Spec.ProjectRef.Name)
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	org := &v1beta1.Organization{}
	if err := c.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", project.Spec.OrganizationRef.Name, err)
	}

	var vendor v1beta1.Vendor
	if org.Spec.Vendor != nil {
		vendor = *org.Spec.Vendor
	} else {
		vendor = v1beta1.VendorOpenAI
	}

	switch vendor {
	case v1beta1.VendorOpenAI:
		return NewOpenAIServiceAccount(c, cs, log, scheme, sa), nil
	case v1beta1.VendorGemini:
		return NewGeminiServiceAccount(c, cs, log, scheme, sa), nil
	case v1beta1.VendorXAI:
		return NewXAIServiceAccount(c, cs, log, scheme, sa), nil
	default:
		return nil, fmt.Errorf("Unsupport vendor %s", *org.Spec.Vendor)
	}
}
