package builders

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
)

// builderWithConfig builds an HTTPRouteBuilder over an explicit IngressConfig so
// each Endpoints case pins the exact routing knobs (subdomain vs shared-host,
// gateways, scheme) the correctness matrix enumerates.
func builderWithConfig(cfg *controllerconfig.IngressConfig) *HTTPRouteBuilder {
	return &HTTPRouteBuilder{
		ingressConfig: cfg,
		isvcConfig:    &controllerconfig.InferenceServicesConfig{},
		domainService: services.NewDomainService(),
		pathService:   services.NewPathService(),
	}
}

// TestHTTPRouteBuilder_Endpoints walks the correctness matrix: the
// endpoint the status writer publishes must equal <scheme>://<host><path> for
// every gateway, in both routing modes.
func TestHTTPRouteBuilder_Endpoints(t *testing.T) {
	base := func() *controllerconfig.IngressConfig {
		return &controllerconfig.IngressConfig{
			IngressDomain:     "int-gw-https.cluster.example",
			DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			OmeIngressGateway: "istio-system/int-gw",
			UrlScheme:         "https",
		}
	}

	for _, tc := range []struct {
		name     string
		mutate   func(*controllerconfig.IngressConfig)
		expected []interfaces.Endpoint
	}{
		{
			name:   "perISVCSubdomain, primary only",
			mutate: func(c *controllerconfig.IngressConfig) { c.PerISVCSubdomain = true },
			expected: []interfaces.Endpoint{
				{Host: "svc.ns.int-gw-https.cluster.example", Path: "/", Scheme: "https", Primary: true},
			},
		},
		{
			name: "perISVCSubdomain, primary+additional int/ext with classes",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = true
				c.OmeIngressGatewayClass = "internal"
				c.AdditionalIngressGateways = []controllerconfig.IngressGatewaySpec{{
					OmeIngressGateway: "istio-system/ext-gw",
					IngressDomain:     "ext-gw-https.cluster.example",
					Class:             "external",
				}}
			},
			expected: []interfaces.Endpoint{
				{Class: "internal", Host: "svc.ns.int-gw-https.cluster.example", Path: "/", Scheme: "https", Primary: true},
				{Class: "external", Host: "svc.ns.ext-gw-https.cluster.example", Path: "/", Scheme: "https", Primary: false},
			},
		},
		{
			name: "shared-host/path, sharedHostPrefix llm",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = false
				c.SharedHostPrefix = "llm"
			},
			expected: []interfaces.Endpoint{
				{Host: "llm.int-gw-https.cluster.example", Path: "/ns/svc/", Scheme: "https", Primary: true},
			},
		},
		{
			name: "scheme http is honored",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = true
				c.UrlScheme = "http"
			},
			expected: []interfaces.Endpoint{
				{Host: "svc.ns.int-gw-https.cluster.example", Path: "/", Scheme: "http", Primary: true},
			},
		},
		{
			name: "per-namespace override resolves primary+additional from the namespace",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = true
				c.OmeIngressGatewayClass = "internal"
				c.NamespaceIngressGateways = map[string]controllerconfig.NamespaceIngressGateway{
					"ns": {
						Primary: controllerconfig.IngressGatewaySpec{
							OmeIngressGateway: "istio-system/ns-int-gw",
							IngressDomain:     "int-gw-https.cluster.example",
							Class:             "internal",
						},
						Additional: []controllerconfig.IngressGatewaySpec{{
							OmeIngressGateway: "istio-system/ns-ext-gw",
							IngressDomain:     "ext-gw-https.cluster.example",
							Class:             "external",
						}},
					},
				}
			},
			expected: []interfaces.Endpoint{
				{Class: "internal", Host: "svc.ns.int-gw-https.cluster.example", Path: "/", Scheme: "https", Primary: true},
				{Class: "external", Host: "svc.ns.ext-gw-https.cluster.example", Path: "/", Scheme: "https", Primary: false},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(cfg)
			b := builderWithConfig(cfg)
			isvc := createTestInferenceServiceHTTPRoute("svc", "ns")

			got, err := b.Endpoints(isvc, isvc.Name)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestHTTPRouteBuilder_Endpoints_MatchRoutes is the drift guard: for the same
// input, the endpoints the status writer would publish must carry exactly the
// hostnames the HTTPRoute exposes and the path its match requires. This is the
// regression test that keeps status and routing derived from one renderer
// (two independent renderers would drift).
func TestHTTPRouteBuilder_Endpoints_MatchRoutes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*controllerconfig.IngressConfig)
	}{
		{
			name:   "perISVCSubdomain",
			mutate: func(c *controllerconfig.IngressConfig) { c.PerISVCSubdomain = true },
		},
		{
			name: "shared-host/path",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = false
				c.SharedHostPrefix = "llm"
			},
		},
		{
			name: "perISVCSubdomain, int+ext",
			mutate: func(c *controllerconfig.IngressConfig) {
				c.PerISVCSubdomain = true
				c.AdditionalIngressGateways = []controllerconfig.IngressGatewaySpec{{
					OmeIngressGateway: "istio-system/ext-gw",
					IngressDomain:     "ext-gw-https.cluster.example",
					Class:             "external",
				}}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &controllerconfig.IngressConfig{
				IngressDomain:     "int-gw-https.cluster.example",
				DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				OmeIngressGateway: "istio-system/int-gw",
				UrlScheme:         "https",
			}
			tc.mutate(cfg)
			b := builderWithConfig(cfg)
			isvc := createTestInferenceServiceHTTPRoute("svc", "ns")
			setEngineReady(isvc)

			// The route the builder emits for the top-level "<isvc>" route.
			obj, err := b.buildTopLevelHTTPRoute(context.Background(), isvc)
			require.NoError(t, err)
			require.NotNil(t, obj)

			endpoints, err := b.Endpoints(isvc, isvc.Name)
			require.NoError(t, err)

			// Hostnames: endpoint hosts must equal the route hostnames, in order.
			routeHosts := make([]gatewayapiv1.Hostname, 0, len(obj.Spec.Hostnames))
			routeHosts = append(routeHosts, obj.Spec.Hostnames...)
			epHosts := make([]gatewayapiv1.Hostname, 0, len(endpoints))
			for _, ep := range endpoints {
				epHosts = append(epHosts, gatewayapiv1.Hostname(ep.Host))
			}
			assert.Equal(t, routeHosts, epHosts, "endpoint hosts must equal HTTPRoute hostnames")

			// Path: every endpoint's path must equal the route's path-prefix match.
			require.Len(t, obj.Spec.Rules, 1)
			require.Len(t, obj.Spec.Rules[0].Matches, 1)
			routePath := *obj.Spec.Rules[0].Matches[0].Path.Value
			for _, ep := range endpoints {
				assert.Equal(t, routePath, ep.Path, "endpoint path must equal HTTPRoute path match")
			}
		})
	}
}
