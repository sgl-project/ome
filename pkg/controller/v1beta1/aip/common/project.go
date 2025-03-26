package common

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewProject returns a new Project
func NewProject(ctx context.Context, c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) (ResourceOperation, error) {
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
		return NewOpenAIProject(c, cs, log, scheme, project), nil
	case v1beta1.VendorGemini:
		return NewGeminiProject(c, cs, log, scheme, project), nil
	case v1beta1.VendorXAI:
		return NewXAIProject(c, cs, log, scheme, project), nil
	default:
		return nil, fmt.Errorf("Unsupport vendor %s", *org.Spec.Vendor)
	}
}
