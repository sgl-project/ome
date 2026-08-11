package endpoint

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

const (
	// resourceSuffix names the backing resources the Gateway API publisher
	// creates for an InferenceService. The HTTPRoute is "<isvc>-global"; each
	// per-home ExternalName Service is "<isvc>-global-<cluster>" — distinct from
	// the per-cluster ingress resources OME's normal ingress reconciler emits
	// ("<isvc>", "<isvc>-engine", ...) so the two never collide.
	resourceSuffix = "-global"
)

// GatewayAPIPublisher implements EndpointPublisher by programming, on the
// control-plane cluster, one Gateway API HTTPRoute per InferenceService whose
// backends are ExternalName Services aliasing each serving workload cluster's
// ingress host. Because the homes' ingresses live on other clusters, an
// ExternalName Service is the portable, implementation-agnostic way to name an
// off-cluster backend a Gateway API HTTPRoute can reference (no vendor-specific
// external-backend CRD).
//
// Single mode yields one backend Service and a single-backendRef route; All/Split
// yield one Service per home and a route that load-balances across all of them
// (equal weight today). As homes come and go the Service set is reconciled — new
// homes get a Service, departed homes' Services are garbage-collected — and the
// route's backendRefs track the current set. Teardown removes the route and every
// per-home Service.
type GatewayAPIPublisher struct {
	client client.Client
	config Config
}

var _ EndpointPublisher = (*GatewayAPIPublisher)(nil)

// NewGatewayAPIPublisher constructs the Gateway API backend. The client is the
// control-plane cluster client (the global gateway and the published resources
// live there).
func NewGatewayAPIPublisher(c client.Client, cfg Config) *GatewayAPIPublisher {
	return &GatewayAPIPublisher{client: c, config: cfg}
}

func (p *GatewayAPIPublisher) Name() string { return "GatewayAPI" }

// routeName is the HTTPRoute name for an InferenceService ("<isvc>-global").
func routeName(isvc *v1beta1.InferenceService) string {
	return isvc.Name + resourceSuffix
}

// serviceName is the per-home ExternalName Service name for a cluster
// ("<isvc>-global-<cluster>").
func serviceName(isvc *v1beta1.InferenceService, cluster string) string {
	return isvc.Name + resourceSuffix + "-" + cluster
}

// routeNamespace resolves the namespace the published resources live in: the
// configured RouteNamespace, or the ISVC's own namespace when unset.
func (p *GatewayAPIPublisher) routeNamespace(isvc *v1beta1.InferenceService) string {
	if ns := strings.TrimSpace(p.config.RouteNamespace); ns != "" {
		return ns
	}
	return isvc.Namespace
}

// Publish reconciles the per-home ExternalName Services and the HTTPRoute so the
// global host load-balances across exactly target.Homes.
func (p *GatewayAPIPublisher) Publish(ctx context.Context, isvc *v1beta1.InferenceService, target Target) error {
	if err := p.applyServices(ctx, isvc, target); err != nil {
		return err
	}
	return p.applyRoute(ctx, isvc, target)
}

// Unpublish deletes the HTTPRoute and every per-home Service this publisher owns
// for the ISVC. Missing resources are tolerated so a double-teardown (finalizer +
// a re-queued unplaced pass) is a no-op.
func (p *GatewayAPIPublisher) Unpublish(ctx context.Context, isvc *v1beta1.InferenceService) error {
	ns := p.routeNamespace(isvc)
	var errs []error
	route := &gatewayapiv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: routeName(isvc), Namespace: ns}}
	if err := p.client.Delete(ctx, route); err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("delete global HTTPRoute %s/%s: %w", ns, routeName(isvc), err))
	}
	owned, err := p.ownedServices(ctx, isvc)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for i := range owned {
		s := &owned[i]
		if err := p.client.Delete(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete global backend Service %s/%s: %w", s.Namespace, s.Name, err))
		}
	}
	return errors.Join(errs...)
}

// ownedServices lists the per-home backend Services this publisher created for
// the ISVC (by the managed-by + per-ISVC labels), so teardown and stale-home GC
// find them all regardless of how many homes there are.
func (p *GatewayAPIPublisher) ownedServices(ctx context.Context, isvc *v1beta1.InferenceService) ([]corev1.Service, error) {
	list := &corev1.ServiceList{}
	if err := p.client.List(ctx, list, client.InNamespace(p.routeNamespace(isvc)),
		client.MatchingLabels{ManagedByLabel: ManagedByValue, PlacementEndpointISVCLabel: isvc.Name}); err != nil {
		return nil, fmt.Errorf("list global backend Services for %s/%s: %w", isvc.Namespace, isvc.Name, err)
	}
	return list.Items, nil
}

// applyServices ensures exactly one ExternalName Service per home exists, then
// deletes any owned Service no longer backing a home (a home that left).
func (p *GatewayAPIPublisher) applyServices(ctx context.Context, isvc *v1beta1.InferenceService, target Target) error {
	desired := make(map[string]Home, len(target.Homes))
	for _, h := range target.Homes {
		desired[serviceName(isvc, h.Cluster)] = h
	}
	// Apply in a stable order so behavior is deterministic.
	names := make([]string, 0, len(desired))
	for n := range desired {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := p.applyService(ctx, isvc, name, desired[name]); err != nil {
			return err
		}
	}

	owned, err := p.ownedServices(ctx, isvc)
	if err != nil {
		return err
	}
	for i := range owned {
		s := &owned[i]
		if _, keep := desired[s.Name]; keep {
			continue
		}
		if err := p.client.Delete(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale global backend Service %s/%s: %w", s.Namespace, s.Name, err)
		}
	}
	return nil
}

func (p *GatewayAPIPublisher) applyService(ctx context.Context, isvc *v1beta1.InferenceService, name string, home Home) error {
	desired := p.buildExternalNameService(isvc, name, home)
	existing := &corev1.Service{}
	err := p.client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := p.client.Create(ctx, desired); err != nil {
			return fmt.Errorf("create global backend Service %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		return nil
	}
	if err != nil {
		return err
	}
	// Repoint (re-placement) or label drift: update in place. Carry the live
	// ResourceVersion and preserve the cluster-assigned spec fields we do not own.
	desired.ResourceVersion = existing.ResourceVersion
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs
	if equality.Semantic.DeepEqual(desired.Spec, existing.Spec) &&
		maps.Equal(desired.Labels, existing.Labels) {
		return nil
	}
	if err := p.client.Update(ctx, desired); err != nil {
		return fmt.Errorf("update global backend Service %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	return nil
}

func (p *GatewayAPIPublisher) applyRoute(ctx context.Context, isvc *v1beta1.InferenceService, target Target) error {
	desired := p.buildHTTPRoute(isvc, target)
	existing := &gatewayapiv1.HTTPRoute{}
	err := p.client.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := p.client.Create(ctx, desired); err != nil {
			return fmt.Errorf("create global HTTPRoute %s/%s: %w", desired.Namespace, desired.Name, err)
		}
		return nil
	}
	if err != nil {
		return err
	}
	desired.ResourceVersion = existing.ResourceVersion
	if equality.Semantic.DeepEqual(desired.Spec, existing.Spec) &&
		maps.Equal(desired.Labels, existing.Labels) {
		return nil
	}
	if err := p.client.Update(ctx, desired); err != nil {
		return fmt.Errorf("update global HTTPRoute %s/%s: %w", desired.Namespace, desired.Name, err)
	}
	return nil
}

// buildExternalNameService renders the ExternalName Service that aliases one
// home cluster's ingress host. The externalName is repointed if the home's
// backend changes; the Service name is stable per cluster so the HTTPRoute
// backendRef for that home never has to change.
func (p *GatewayAPIPublisher) buildExternalNameService(isvc *v1beta1.InferenceService, name string, home Home) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.routeNamespace(isvc),
			Labels:    p.serviceLabels(isvc, home.Cluster),
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: home.BackendHost,
			Ports: []corev1.ServicePort{{
				Port: p.config.BackendPort,
			}},
		},
	}
}

// buildHTTPRoute renders the global HTTPRoute: it attaches to the configured
// global gateway, matches the global host at root, and forwards to one backend
// per home (each the home's ExternalName Service), equal weight. Homes are
// sorted by cluster so the backendRef order is deterministic (stable idempotency
// comparisons).
func (p *GatewayAPIPublisher) buildHTTPRoute(isvc *v1beta1.InferenceService, target Target) *gatewayapiv1.HTTPRoute {
	gwNS, gwName := splitGatewayRef(p.config.GlobalGateway)
	backendNS := p.routeNamespace(isvc)
	port := gatewayapiv1.PortNumber(p.config.BackendPort)

	homes := append([]Home(nil), target.Homes...)
	sort.Slice(homes, func(i, j int) bool { return homes[i].Cluster < homes[j].Cluster })

	// Traffic weight per home. In Split each home carries its ready-replica
	// count, so traffic follows where replicas landed (a home with 0 ready gets 0
	// — no traffic until it is serving). When no home carries a weight
	// (Single/All, or a Split placement before any home is ready) every weight is
	// zero; Gateway API sends no traffic if ALL backendRef weights are zero, so
	// fall back to equal weight (1 each) rather than black-holing the route.
	var totalWeight int32
	for _, h := range homes {
		totalWeight += h.Weight
	}
	refs := make([]gatewayapiv1.HTTPBackendRef, 0, len(homes))
	for _, h := range homes {
		weight := int32(1)
		if totalWeight > 0 {
			weight = h.Weight
		}
		refs = append(refs, gatewayapiv1.HTTPBackendRef{
			BackendRef: gatewayapiv1.BackendRef{
				BackendObjectReference: gatewayapiv1.BackendObjectReference{
					Kind:      ptr.To(gatewayapiv1.Kind(constants.ServiceKind)),
					Name:      gatewayapiv1.ObjectName(serviceName(isvc, h.Cluster)),
					Namespace: (*gatewayapiv1.Namespace)(&backendNS),
					Port:      &port,
				},
				Weight: ptr.To(weight),
			},
		})
	}

	return &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName(isvc),
			Namespace: backendNS,
			Labels:    p.baseLabels(isvc),
		},
		Spec: gatewayapiv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
				ParentRefs: []gatewayapiv1.ParentReference{{
					Group:     (*gatewayapiv1.Group)(&gatewayapiv1.GroupVersion.Group),
					Kind:      (*gatewayapiv1.Kind)(ptr.To(constants.GatewayKind)),
					Namespace: (*gatewayapiv1.Namespace)(&gwNS),
					Name:      gatewayapiv1.ObjectName(gwName),
				}},
			},
			Hostnames: []gatewayapiv1.Hostname{gatewayapiv1.Hostname(target.GlobalHost)},
			Rules: []gatewayapiv1.HTTPRouteRule{{
				Matches: []gatewayapiv1.HTTPRouteMatch{{
					Path: &gatewayapiv1.HTTPPathMatch{
						Type:  ptr.To(gatewayapiv1.PathMatchPathPrefix),
						Value: ptr.To("/"),
					},
				}},
				BackendRefs: refs,
			}},
		},
	}
}

// baseLabels are the ownership markers stamped on every published resource
// (route + Services): the managed-by marker plus the per-ISVC grouping label,
// merged over the operator-configured Labels (owned keys win).
func (p *GatewayAPIPublisher) baseLabels(isvc *v1beta1.InferenceService) map[string]string {
	out := map[string]string{}
	maps.Copy(out, p.config.Labels)
	out[ManagedByLabel] = ManagedByValue
	out[PlacementEndpointISVCLabel] = isvc.Name
	return out
}

// serviceLabels are baseLabels plus the per-home cluster marker (a backend
// Service points at exactly one cluster).
func (p *GatewayAPIPublisher) serviceLabels(isvc *v1beta1.InferenceService, cluster string) map[string]string {
	out := p.baseLabels(isvc)
	if cluster != "" {
		out[PlacementClusterLabel] = cluster
	}
	return out
}

// splitGatewayRef parses a "namespace/name" gateway reference. A bare "name"
// (no slash) resolves to an empty namespace, which Gateway API treats as the
// route's own namespace.
func splitGatewayRef(ref string) (namespace, name string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
