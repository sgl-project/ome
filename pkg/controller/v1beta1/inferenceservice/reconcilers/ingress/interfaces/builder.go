package interfaces

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

// ResourceBuilder builds Kubernetes resources for ingress
type ResourceBuilder interface {
	Build(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error)
	GetResourceType() string
}

// VirtualServiceBuilder builds Istio VirtualService resources
type VirtualServiceBuilder interface {
	ResourceBuilder
	BuildVirtualService(ctx context.Context, isvc *v1beta1.InferenceService, domainList *[]string) (client.Object, error)
}

// HTTPRouteBuilder builds Gateway API HTTPRoute resources
type HTTPRouteBuilder interface {
	ResourceBuilder
	BuildHTTPRoute(ctx context.Context, isvc *v1beta1.InferenceService, componentType string) (client.Object, error)
}

// IngressBuilder builds Kubernetes Ingress resources
type IngressBuilder interface {
	ResourceBuilder
	BuildIngress(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error)
}
