package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func pubScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(s))
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, gatewayapiv1.Install(s))
	return s
}

func testISVC() *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "prod", UID: "uid-1"},
	}
}

func baseConfig() Config {
	return Config{
		GlobalHostTemplate: "{{.Name}}.{{.Namespace}}.global.example",
		GlobalGateway:      "ome-system/global-gw",
		BackendPort:        8080,
		Labels:             map[string]string{"team": "platform"},
	}
}

// oneHome builds a single-home Target (Single mode shape).
func oneHome(globalHost, cluster, backendHost string) Target {
	return Target{GlobalHost: globalHost, Homes: []Home{{Cluster: cluster, BackendHost: backendHost}}}
}

func TestBuildExternalNameService(t *testing.T) {
	p := NewGatewayAPIPublisher(nil, baseConfig())
	isvc := testISVC()
	home := Home{Cluster: "cluster-a", BackendHost: "svc.prod.cloud-a.example"}

	svc := p.buildExternalNameService(isvc, p.serviceName(isvc, "cluster-a"), home)

	assert.Equal(t, "svc-global-cluster-a", svc.Name, "per-home Service name carries the cluster")
	assert.Equal(t, "prod", svc.Namespace, "namespace falls back to the ISVC namespace")
	assert.Equal(t, corev1.ServiceTypeExternalName, svc.Spec.Type)
	assert.Equal(t, "svc.prod.cloud-a.example", svc.Spec.ExternalName, "ExternalName aliases the home ingress host")
	require.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
	assert.Equal(t, ManagedByValue, svc.Labels[ManagedByLabel])
	assert.Equal(t, "cluster-a", svc.Labels[PlacementClusterLabel])
	assert.Equal(t, "svc", svc.Labels[PlacementEndpointISVCLabel], "per-ISVC grouping label for GC/teardown")
	assert.Equal(t, "prod", svc.Labels[PlacementEndpointISVCNamespaceLabel])
	assert.Equal(t, "platform", svc.Labels["team"], "operator labels are merged in")
}

func TestBuildHTTPRoute_SingleHome(t *testing.T) {
	p := NewGatewayAPIPublisher(nil, baseConfig())

	route := p.buildHTTPRoute(testISVC(), oneHome("svc.prod.global.example", "cluster-a", "svc.prod.cloud-a.example"))

	assert.Equal(t, "svc-global", route.Name)
	assert.Equal(t, "prod", route.Namespace)
	require.Len(t, route.Spec.Hostnames, 1)
	assert.Equal(t, gatewayapiv1.Hostname("svc.prod.global.example"), route.Spec.Hostnames[0])
	assert.Equal(t, "svc", route.Labels[PlacementEndpointISVCLabel])
	assert.Equal(t, "prod", route.Labels[PlacementEndpointISVCNamespaceLabel])

	require.Len(t, route.Spec.ParentRefs, 1)
	pr := route.Spec.ParentRefs[0]
	require.NotNil(t, pr.Namespace)
	assert.Equal(t, "ome-system", string(*pr.Namespace), "gateway namespace parsed from namespace/name")
	assert.Equal(t, "global-gw", string(pr.Name))
	require.NotNil(t, pr.Kind)
	assert.Equal(t, constants.GatewayKind, string(*pr.Kind))

	require.Len(t, route.Spec.Rules, 1)
	require.Len(t, route.Spec.Rules[0].BackendRefs, 1)
	br := route.Spec.Rules[0].BackendRefs[0]
	assert.Equal(t, gatewayapiv1.ObjectName("svc-global-cluster-a"), br.Name, "route forwards to the home's ExternalName Service")
	require.NotNil(t, br.Kind)
	assert.Equal(t, constants.ServiceKind, string(*br.Kind))
	require.NotNil(t, br.Port)
	assert.Equal(t, gatewayapiv1.PortNumber(8080), *br.Port)
	require.NotNil(t, br.Weight)
	assert.Equal(t, int32(1), *br.Weight)

	require.Len(t, route.Spec.Rules[0].Matches, 1)
	require.NotNil(t, route.Spec.Rules[0].Matches[0].Path)
	assert.Equal(t, "/", *route.Spec.Rules[0].Matches[0].Path.Value)
}

func TestBuildHTTPRoute_MultiHomeEqualWeightSorted(t *testing.T) {
	p := NewGatewayAPIPublisher(nil, baseConfig())
	// Homes passed out of order; the route's backendRefs must be sorted by cluster.
	target := Target{GlobalHost: "h", Homes: []Home{
		{Cluster: "cluster-b", BackendHost: "b.example"},
		{Cluster: "cluster-a", BackendHost: "a.example"},
	}}

	route := p.buildHTTPRoute(testISVC(), target)

	refs := route.Spec.Rules[0].BackendRefs
	require.Len(t, refs, 2, "one backendRef per home")
	assert.Equal(t, gatewayapiv1.ObjectName("svc-global-cluster-a"), refs[0].Name)
	assert.Equal(t, gatewayapiv1.ObjectName("svc-global-cluster-b"), refs[1].Name)
	require.NotNil(t, refs[0].Weight)
	require.NotNil(t, refs[1].Weight)
	assert.Equal(t, int32(1), *refs[0].Weight, "equal weight across homes")
	assert.Equal(t, int32(1), *refs[1].Weight)
}

func TestBuildResources_RouteNamespaceOverride(t *testing.T) {
	cfg := baseConfig()
	cfg.RouteNamespace = "ome-gateways"
	p := NewGatewayAPIPublisher(nil, cfg)
	isvc := testISVC()

	svc := p.buildExternalNameService(isvc, p.serviceName(isvc, "c"), Home{Cluster: "c", BackendHost: "b"})
	route := p.buildHTTPRoute(isvc, oneHome("h", "c", "b"))

	assert.Equal(t, "ome-gateways", svc.Namespace)
	assert.Equal(t, "ome-gateways", route.Namespace)
	require.NotNil(t, route.Spec.Rules[0].BackendRefs[0].Namespace)
	assert.Equal(t, "ome-gateways", string(*route.Spec.Rules[0].BackendRefs[0].Namespace),
		"backendRef namespace tracks the route namespace")
}

type routeUpdateFailClient struct {
	client.Client
}

func (c routeUpdateFailClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*gatewayapiv1.HTTPRoute); ok {
		return errors.New("route update failed")
	}
	return c.Client.Update(ctx, obj, opts...)
}

func TestBuildHTTPRoute_BareGatewayName(t *testing.T) {
	cfg := baseConfig()
	cfg.GlobalGateway = "global-gw" // no namespace
	p := NewGatewayAPIPublisher(nil, cfg)

	route := p.buildHTTPRoute(testISVC(), oneHome("h", "c", "b"))

	pr := route.Spec.ParentRefs[0]
	require.NotNil(t, pr.Namespace)
	assert.Equal(t, "", string(*pr.Namespace), "bare gateway name yields empty (route-local) namespace")
	assert.Equal(t, "global-gw", string(pr.Name))
}

func TestPublish_SingleHomeCreatesResources(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()

	require.NoError(t, p.Publish(context.Background(), isvc, oneHome("svc.prod.global.example", "cluster-a", "svc.prod.cloud-a.example")))

	svc := &corev1.Service{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-a", Namespace: "prod"}, svc))
	assert.Equal(t, "svc.prod.cloud-a.example", svc.Spec.ExternalName)

	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	assert.Equal(t, gatewayapiv1.Hostname("svc.prod.global.example"), route.Spec.Hostnames[0])
	require.Len(t, route.Spec.Rules[0].BackendRefs, 1)
}

func TestPublish_MultiHomeCreatesPerHomeServices(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()
	target := Target{GlobalHost: "svc.prod.global.example", Homes: []Home{
		{Cluster: "cluster-a", BackendHost: "a.example"},
		{Cluster: "cluster-b", BackendHost: "b.example"},
	}}

	require.NoError(t, p.Publish(context.Background(), isvc, target))

	a := &corev1.Service{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-a", Namespace: "prod"}, a))
	assert.Equal(t, "a.example", a.Spec.ExternalName)
	b := &corev1.Service{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-b", Namespace: "prod"}, b))
	assert.Equal(t, "b.example", b.Spec.ExternalName)

	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	require.Len(t, route.Spec.Rules[0].BackendRefs, 2, "route load-balances across both homes")
}

func TestPublish_Idempotent(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()
	target := oneHome("svc.prod.global.example", "cluster-a", "svc.prod.cloud-a.example")

	require.NoError(t, p.Publish(context.Background(), isvc, target))
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	rvBefore := route.ResourceVersion

	require.NoError(t, p.Publish(context.Background(), isvc, target))
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	assert.Equal(t, rvBefore, route.ResourceVersion, "identical re-publish must be a no-op")
}

func TestPublish_HomeLeavesGCsStaleService(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()

	// Two homes, then one leaves.
	require.NoError(t, p.Publish(context.Background(), isvc, Target{GlobalHost: "h", Homes: []Home{
		{Cluster: "cluster-a", BackendHost: "a.example"},
		{Cluster: "cluster-b", BackendHost: "b.example"},
	}}))
	require.NoError(t, p.Publish(context.Background(), isvc, oneHome("h", "cluster-a", "a.example")))

	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-a", Namespace: "prod"}, &corev1.Service{}),
		"surviving home's Service kept")
	err := c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-b", Namespace: "prod"}, &corev1.Service{})
	assert.True(t, apierrors.IsNotFound(err), "departed home's Service garbage-collected")

	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	require.Len(t, route.Spec.Rules[0].BackendRefs, 1, "route drops the departed home's backendRef")
}

func TestPublish_WinnerMovesClustersInSingle(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()

	require.NoError(t, p.Publish(context.Background(), isvc, oneHome("svc.prod.global.example", "cluster-a", "svc.prod.cloud-a.example")))
	// Single re-placement onto a different cluster: old home's Service is GC'd, new one created.
	require.NoError(t, p.Publish(context.Background(), isvc, oneHome("svc.prod.global.example", "cluster-b", "svc.prod.cloud-b.example")))

	err := c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-a", Namespace: "prod"}, &corev1.Service{})
	assert.True(t, apierrors.IsNotFound(err), "old winner's Service GC'd")
	b := &corev1.Service{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-b", Namespace: "prod"}, b))
	assert.Equal(t, "svc.prod.cloud-b.example", b.Spec.ExternalName)

	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, route))
	require.Len(t, route.Spec.Rules[0].BackendRefs, 1)
	assert.Equal(t, gatewayapiv1.ObjectName("svc-global-cluster-b"), route.Spec.Rules[0].BackendRefs[0].Name)
}

func TestPublish_RouteUpdateFailureKeepsOldBackendService(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()
	require.NoError(t, p.Publish(context.Background(), isvc, oneHome("h", "cluster-a", "a.example")))

	p.client = routeUpdateFailClient{Client: c}
	err := p.Publish(context.Background(), isvc, oneHome("h", "cluster-b", "b.example"))
	require.Error(t, err)
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: "svc-global-cluster-a", Namespace: "prod"}, &corev1.Service{}),
		"old Service remains while the route still references it")
}

func TestPublish_SharedRouteNamespaceIsolatesSameNameSources(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	cfg := baseConfig()
	cfg.RouteNamespace = "ome-gateways"
	p := NewGatewayAPIPublisher(c, cfg)
	a := testISVC()
	a.Namespace = "team-a"
	b := testISVC()
	b.Namespace = "team-b"

	require.NoError(t, p.Publish(context.Background(), a, oneHome("a.global.example", "cluster-a", "a.example")))
	require.NoError(t, p.Publish(context.Background(), b, oneHome("b.global.example", "cluster-b", "b.example")))
	assert.NotEqual(t, p.routeName(a), p.routeName(b))
	assert.NotEqual(t, p.serviceName(a, "cluster-a"), p.serviceName(b, "cluster-b"))
	ownedA, err := p.ownedServices(context.Background(), a)
	require.NoError(t, err)
	require.Len(t, ownedA, 1)
	assert.Equal(t, "team-a", ownedA[0].Labels[PlacementEndpointISVCNamespaceLabel])

	require.NoError(t, p.Unpublish(context.Background(), a))
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: p.routeName(b), Namespace: "ome-gateways"}, &gatewayapiv1.HTTPRoute{}))
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: p.serviceName(b, "cluster-b"), Namespace: "ome-gateways"}, &corev1.Service{}))
}

func TestUnpublish_DeletesRouteAndAllServices(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()
	require.NoError(t, p.Publish(context.Background(), isvc, Target{GlobalHost: "h", Homes: []Home{
		{Cluster: "cluster-a", BackendHost: "a.example"},
		{Cluster: "cluster-b", BackendHost: "b.example"},
	}}))

	require.NoError(t, p.Unpublish(context.Background(), isvc))

	for _, name := range []string{"svc-global-cluster-a", "svc-global-cluster-b"} {
		err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: "prod"}, &corev1.Service{})
		assert.True(t, apierrors.IsNotFound(err), "%s deleted", name)
	}
	err := c.Get(context.Background(), types.NamespacedName{Name: "svc-global", Namespace: "prod"}, &gatewayapiv1.HTTPRoute{})
	assert.True(t, apierrors.IsNotFound(err), "HTTPRoute deleted")
}

func TestUnpublish_MissingIsNoOp(t *testing.T) {
	s := pubScheme(t)
	c := fakeclient.NewClientBuilder().WithScheme(s).Build()
	p := NewGatewayAPIPublisher(c, baseConfig())
	require.NoError(t, p.Unpublish(context.Background(), testISVC()))
}

func TestBuildHTTPRoute_WeightedByReadyReplicas(t *testing.T) {
	p := NewGatewayAPIPublisher(nil, baseConfig())
	// Split: each home carries its ready-replica count -> proportional weights.
	target := Target{GlobalHost: "h", Homes: []Home{
		{Cluster: "cluster-a", BackendHost: "a.example", Weight: 3},
		{Cluster: "cluster-b", BackendHost: "b.example", Weight: 1},
	}}

	refs := p.buildHTTPRoute(testISVC(), target).Spec.Rules[0].BackendRefs
	require.Len(t, refs, 2)
	require.NotNil(t, refs[0].Weight)
	require.NotNil(t, refs[1].Weight)
	assert.Equal(t, int32(3), *refs[0].Weight, "cluster-a (sorted first) weighted by its ready replicas")
	assert.Equal(t, int32(1), *refs[1].Weight, "cluster-b weighted by its ready replicas")
}

func TestBuildHTTPRoute_UnweightedFallsBackToEqual(t *testing.T) {
	p := NewGatewayAPIPublisher(nil, baseConfig())
	// No weights (Single/All, or a Split placement before any home is ready):
	// every weight is 0, so fall back to equal (1) rather than black-holing.
	target := Target{GlobalHost: "h", Homes: []Home{
		{Cluster: "cluster-a", BackendHost: "a.example"},
		{Cluster: "cluster-b", BackendHost: "b.example"},
	}}

	refs := p.buildHTTPRoute(testISVC(), target).Spec.Rules[0].BackendRefs
	require.Len(t, refs, 2)
	assert.Equal(t, int32(1), *refs[0].Weight)
	assert.Equal(t, int32(1), *refs[1].Weight)
}
