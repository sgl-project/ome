package builders

import (
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

// Endpoints returns every reachable endpoint for the route named by serviceName,
// one per gateway the route attaches to. It is the single-renderer
// status derivation: the same resolvedGateways +
// gatewayHostnameForDomain + pathPrefix that produce the HTTPRoute produce these
// tuples, so status.url / status.addresses cannot drift from the actual routing.
//
// serviceName selects the route: "<isvc>" for the top-level route (matching
// buildTopLevelHTTPRoute), or a component name ("<isvc>-engine" etc.). It only
// affects the path in the shared-host scheme ("/<namespace>/<serviceName>/"); in
// the per-ISVC-subdomain scheme the path is always "/".
func (b *HTTPRouteBuilder) Endpoints(isvc *v1beta1.InferenceService, serviceName string) ([]interfaces.Endpoint, error) {
	gateways := b.resolvedGateways(isvc)
	path := b.pathPrefix(isvc, serviceName)
	scheme := b.ingressConfig.UrlScheme

	endpoints := make([]interfaces.Endpoint, 0, len(gateways))
	for _, gw := range gateways {
		host, err := b.gatewayHostnameForDomain(isvc, gw.spec.IngressDomain)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, interfaces.Endpoint{
			Class:   gw.class,
			Host:    string(host),
			Path:    path,
			Scheme:  scheme,
			Primary: gw.primary,
		})
	}
	return endpoints, nil
}
