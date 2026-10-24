package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestGatewayAPIPublisher_UnpublishRouteHeldByFinalizer_WaitsKeepingChildren(t *testing.T) {
	c := gatewayClient(t)
	p := NewGatewayAPIPublisher(c, gatewayBackendConfig(), WithBackendAddressResolver(staticBackendAddressResolver{
		"cluster-a": {{Address: "192.0.2.10", Type: discoveryv1.AddressTypeIPv4}},
		"cluster-b": {{Address: "2001:db8::10", Type: discoveryv1.AddressTypeIPv6}},
	}))
	isvc := testISVC()
	target := Target{GlobalHost: "svc.prod.global.example", Clusters: []ServingCluster{
		{Cluster: "cluster-a", BackendHost: "a.example", Weight: 3},
		{Cluster: "cluster-b", BackendHost: "b.example", Weight: 1},
	}}
	require.NoError(t, p.Publish(context.Background(), isvc, target))

	routeKey := types.NamespacedName{Namespace: "prod", Name: "svc-global"}
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	route.Finalizers = []string{"example.com/hold"}
	route.Spec.Rules[0].BackendRefs[0].Weight = nil
	require.NoError(t, c.Update(context.Background(), route))

	err := p.Unpublish(context.Background(), isvc)
	require.ErrorContains(t, err, "deletion is pending")
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	require.NotNil(t, route.DeletionTimestamp)
	assert.Equal(t, []string{"example.com/hold"}, route.Finalizers)
	for _, rule := range route.Spec.Rules {
		for _, backend := range rule.BackendRefs {
			require.NotNil(t, backend.Weight)
			assert.Zero(t, *backend.Weight)
		}
	}
	assertLegacyGatewayChildren(t, p, isvc, 2, 2, 2)

	route.Finalizers = nil
	require.NoError(t, c.Update(context.Background(), route))
	require.NoError(t, p.Unpublish(context.Background(), isvc))
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), routeKey, &gatewayapiv1.HTTPRoute{})))
	assertLegacyGatewayChildren(t, p, isvc, 0, 0, 0)
}

func TestGatewayAPIPublisher_UnpublishRouteCallFails_KeepsChildren(t *testing.T) {
	routeReadFailed := errors.New("injected HTTPRoute read failure")
	routeDeleteFailed := apierrors.NewConflict(gatewayapiv1.Resource("httproutes"), "svc-global", errors.New("injected HTTPRoute delete failure"))
	for _, tc := range []struct {
		name      string
		publisher func(client.WithWatch) *GatewayAPIPublisher
	}{
		{
			name: "read",
			publisher: func(c client.WithWatch) *GatewayAPIPublisher {
				reader := fakeclient.Wrap(t, c, fakeclient.FailingVerb(fakeclient.Get, 1, routeReadFailed).OnKind(&gatewayapiv1.HTTPRoute{}))
				return NewGatewayAPIPublisher(c, baseConfig(), WithGatewayAPIReader(reader))
			},
		},
		{
			name: "update",
			publisher: func(c client.WithWatch) *GatewayAPIPublisher {
				return NewGatewayAPIPublisher(fakeclient.Wrap(t, c, routeUpdatesRefused()), baseConfig(), WithGatewayAPIReader(c))
			},
		},
		{
			name: "delete",
			publisher: func(c client.WithWatch) *GatewayAPIPublisher {
				refusing := fakeclient.Wrap(t, c, fakeclient.FailingVerb(fakeclient.Delete, fakeclient.EveryCall, routeDeleteFailed).OnKind(&gatewayapiv1.HTTPRoute{}))
				return NewGatewayAPIPublisher(refusing, baseConfig(), WithGatewayAPIReader(c))
			},
		},
		{
			name: "confirmation",
			publisher: func(c client.WithWatch) *GatewayAPIPublisher {
				reader := fakeclient.Wrap(t, c, fakeclient.FailingVerb(fakeclient.Get, 2, routeReadFailed).OnKind(&gatewayapiv1.HTTPRoute{}))
				return NewGatewayAPIPublisher(c, baseConfig(), WithGatewayAPIReader(reader))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, isvc, serviceKey := publishLegacyGatewayFixture(t)
			err := tc.publisher(c).Unpublish(context.Background(), isvc)
			require.Error(t, err)
			require.NoError(t, c.Get(context.Background(), serviceKey, &corev1.Service{}),
				"route teardown failures must retain subordinate resources")
		})
	}
}

func TestGatewayAPIPublisher_UnpublishRouteReplacedAfterDrain_LeavesReplacement(t *testing.T) {
	isvc := testISVC()
	renderer := NewGatewayAPIPublisher(nil, baseConfig())
	route := renderer.buildHTTPRoute(isvc, oneCluster("svc.prod.global.example", "cluster-a", "a.example"))
	service := renderer.buildExternalNameService(isvc, renderer.serviceName(isvc, "cluster-a"), ServingCluster{
		Cluster: "cluster-a", BackendHost: "a.example",
	})
	route.UID = types.UID("owned-route")
	route.ResourceVersion = "1"
	service.UID = types.UID("owned-service")
	service.ResourceVersion = "1"
	routeKey := client.ObjectKeyFromObject(route)
	serviceKey := client.ObjectKeyFromObject(service)
	initialResourceVersion := route.ResourceVersion
	var postDrainResourceVersion string

	c := fakeclient.New(t,
		fakeclient.WithTypes(gatewayapiv1.Install),
		fakeclient.WithObjects(route, service),
		fakeclient.WithInterceptors(interceptor.Funcs{
			Update: func(ctx context.Context, c client.WithWatch, object client.Object, opts ...client.UpdateOption) error {
				if updated, ok := object.(*gatewayapiv1.HTTPRoute); ok {
					for _, rule := range updated.Spec.Rules {
						for _, backend := range rule.BackendRefs {
							require.NotNil(t, backend.Weight)
							assert.Zero(t, *backend.Weight)
						}
					}
				}
				if err := c.Update(ctx, object, opts...); err != nil {
					return err
				}
				if _, ok := object.(*gatewayapiv1.HTTPRoute); ok {
					postDrainResourceVersion = object.GetResourceVersion()
				}
				return nil
			},
			Delete: func(ctx context.Context, c client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
				deleted, ok := object.(*gatewayapiv1.HTTPRoute)
				require.True(t, ok)
				deleteOptions := (&client.DeleteOptions{}).ApplyOptions(opts)
				require.NotNil(t, deleteOptions.Preconditions)
				require.NotNil(t, deleteOptions.Preconditions.UID)
				require.NotNil(t, deleteOptions.Preconditions.ResourceVersion)
				assert.Equal(t, deleted.UID, *deleteOptions.Preconditions.UID)
				assert.Equal(t, postDrainResourceVersion, *deleteOptions.Preconditions.ResourceVersion)

				current := &gatewayapiv1.HTTPRoute{}
				require.NoError(t, c.Get(ctx, routeKey, current))
				require.NoError(t, c.Delete(ctx, current))
				replacement := current.DeepCopy()
				replacement.UID = types.UID("replacement-route")
				replacement.ResourceVersion = ""
				replacement.DeletionTimestamp = nil
				replacement.Labels[PlacementEndpointISVCLabel] = "other"
				replacement.Spec.Rules[0].BackendRefs[0].Weight = ptr.To(int32(9))
				require.NoError(t, c.Create(ctx, replacement))
				return apierrors.NewConflict(
					gatewayapiv1.Resource("httproutes"), routeKey.Name, errors.New("delete preconditions no longer match"),
				)
			},
		}),
	)
	p := NewGatewayAPIPublisher(c, baseConfig(), WithGatewayAPIReader(c))

	err := p.Unpublish(context.Background(), isvc)
	require.Error(t, err)
	assert.NotEmpty(t, postDrainResourceVersion)
	assert.NotEqual(t, initialResourceVersion, postDrainResourceVersion)
	replacement := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, replacement))
	assert.Equal(t, types.UID("replacement-route"), replacement.UID)
	assert.Equal(t, "other", replacement.Labels[PlacementEndpointISVCLabel])
	require.NoError(t, c.Get(context.Background(), serviceKey, &corev1.Service{}))
}

func TestGatewayAPIPublisher_UnpublishWithStaleCache_ReadsRouteLive(t *testing.T) {
	isvc := testISVC()
	renderer := NewGatewayAPIPublisher(nil, baseConfig())
	route := renderer.buildHTTPRoute(isvc, oneCluster("svc.prod.global.example", "cluster-a", "a.example"))
	route.Finalizers = []string{"example.com/hold"}
	service := renderer.buildExternalNameService(isvc, renderer.serviceName(isvc, "cluster-a"), ServingCluster{
		Cluster: "cluster-a", BackendHost: "a.example",
	})
	setGatewayFakeObjectIdentity(route, service)
	live := fakeclient.New(t, fakeclient.WithTypes(gatewayapiv1.Install), fakeclient.WithObjects(route, service))
	stale := fakeclient.Wrap(t, live, blindCache())
	p := NewGatewayAPIPublisher(stale, baseConfig(), WithGatewayAPIReader(live))

	err := p.Unpublish(context.Background(), isvc)
	require.ErrorContains(t, err, "deletion is pending")
	current := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, live.Get(context.Background(), client.ObjectKeyFromObject(route), current))
	require.NotNil(t, current.DeletionTimestamp)
	require.NotNil(t, current.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Zero(t, *current.Spec.Rules[0].BackendRefs[0].Weight)
	require.NoError(t, live.Get(context.Background(), client.ObjectKeyFromObject(service), &corev1.Service{}))
}

func TestGatewayAPIPublisher_UnpublishUnlabeledRoute_DeletesItAsOwn(t *testing.T) {
	c, isvc, serviceKey := publishLegacyGatewayFixture(t)
	p := NewGatewayAPIPublisher(c, baseConfig())
	routeKey := types.NamespacedName{Namespace: "prod", Name: "svc-global"}
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	route.Labels = nil
	require.NoError(t, c.Update(context.Background(), route))

	require.NoError(t, p.Unpublish(context.Background(), isvc))
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), routeKey, &gatewayapiv1.HTTPRoute{})))
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), serviceKey, &corev1.Service{})))
}

func TestGatewayAPIPublisher_UnpublishRouteOwnedByAnotherISVC_FailsKeepingChildren(t *testing.T) {
	c, isvc, serviceKey := publishLegacyGatewayFixture(t)
	p := NewGatewayAPIPublisher(c, baseConfig())
	routeKey := types.NamespacedName{Namespace: "prod", Name: "svc-global"}
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	route.Labels[PlacementEndpointISVCLabel] = "other"
	require.NoError(t, c.Update(context.Background(), route))

	require.ErrorContains(t, p.Unpublish(context.Background(), isvc), "belongs to another InferenceService")
	require.NoError(t, c.Get(context.Background(), serviceKey, &corev1.Service{}))
}

func TestGatewayAPIPublisher_UnpublishTerminatingRoute_SendsNoSecondDelete(t *testing.T) {
	c, isvc, serviceKey := publishLegacyGatewayFixture(t)
	routeKey := types.NamespacedName{Namespace: "prod", Name: "svc-global"}
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	route.Finalizers = []string{"example.com/hold"}
	require.NoError(t, c.Update(context.Background(), route))
	require.NoError(t, c.Delete(context.Background(), route))

	routeDeletes := fakeclient.CountingVerb(fakeclient.Delete).OnKind(&gatewayapiv1.HTTPRoute{})
	p := NewGatewayAPIPublisher(fakeclient.Wrap(t, c, routeDeletes), baseConfig(), WithGatewayAPIReader(c))
	require.ErrorContains(t, p.Unpublish(context.Background(), isvc), "deletion is pending")
	assert.Zero(t, routeDeletes.Count(), "an already terminating route must not receive another delete request")
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	require.NotNil(t, route.Spec.Rules[0].BackendRefs[0].Weight)
	assert.Zero(t, *route.Spec.Rules[0].BackendRefs[0].Weight)
	require.NoError(t, c.Get(context.Background(), serviceKey, &corev1.Service{}))
}

func TestGatewayAPIPublisher_UnpublishWithRouteGone_DeletesChildren(t *testing.T) {
	c, isvc, serviceKey := publishLegacyGatewayFixture(t)
	routeKey := types.NamespacedName{Namespace: "prod", Name: "svc-global"}
	route := &gatewayapiv1.HTTPRoute{}
	require.NoError(t, c.Get(context.Background(), routeKey, route))
	require.NoError(t, c.Delete(context.Background(), route))

	require.NoError(t, NewGatewayAPIPublisher(c, baseConfig()).Unpublish(context.Background(), isvc))
	assert.True(t, apierrors.IsNotFound(c.Get(context.Background(), serviceKey, &corev1.Service{})))
}

func publishLegacyGatewayFixture(
	t *testing.T,
) (client.WithWatch, *v1beta1.InferenceService, types.NamespacedName) {
	t.Helper()
	c := gatewayClient(t)
	p := NewGatewayAPIPublisher(c, baseConfig())
	isvc := testISVC()
	require.NoError(t, p.Publish(context.Background(), isvc,
		oneCluster("svc.prod.global.example", "cluster-a", "a.example")))
	return c, isvc, types.NamespacedName{Namespace: "prod", Name: "svc-global-cluster-a"}
}

func assertLegacyGatewayChildren(
	t *testing.T,
	p *GatewayAPIPublisher,
	isvc *v1beta1.InferenceService,
	serviceCount int,
	endpointSliceCount int,
	policyCount int,
) {
	t.Helper()
	sourceKey := client.ObjectKeyFromObject(isvc)
	namespace := p.routeNamespace(isvc)
	services, err := p.sourceServices(context.Background(), namespace, sourceKey)
	require.NoError(t, err)
	assert.Len(t, services, serviceCount)
	endpointSlices, err := p.sourceEndpointSlices(context.Background(), namespace, sourceKey)
	require.NoError(t, err)
	assert.Len(t, endpointSlices, endpointSliceCount)
	policies, err := p.sourceBackendTLSPolicies(context.Background(), namespace, sourceKey)
	require.NoError(t, err)
	assert.Len(t, policies, policyCount)
}
