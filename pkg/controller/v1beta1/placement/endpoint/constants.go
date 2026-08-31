package endpoint

// PlacementEndpointControllerName names the endpoint publisher — explicit so it
// doesn't collide with the placement controller (both watch InferenceService).
const PlacementEndpointControllerName = "placement-endpoint"

const (
	// ManagedByLabel marks the HTTPRoute + ExternalName Service the endpoint
	// publisher creates, so they are discoverable (kubectl get -l ...) and the
	// reconciler can recognize what it owns. Stamped on every published resource.
	ManagedByLabel = "ome.io/managed-by"
	// ManagedByValue is the ManagedByLabel value for the endpoint publisher.
	ManagedByValue = "placement-endpoint"

	// PlacementClusterLabel records which WorkloadCluster a published backend
	// Service points at. Per-Service (each home carries its own), so the value is
	// that home's cluster; observational.
	PlacementClusterLabel = "ome.io/placement-cluster"

	// PlacementEndpointISVCLabel records which source InferenceService a published
	// resource belongs to. In All/Split a service has MANY per-home backend
	// Services, so teardown and stale-home GC list by this label rather than a
	// single fixed name.
	PlacementEndpointISVCLabel = "ome.io/placement-endpoint-isvc"

	// PlacementEndpointISVCNamespaceLabel records the source InferenceService's
	// namespace. Paired with PlacementEndpointISVCLabel it identifies the owner
	// exactly: the name alone is ambiguous when a shared RouteNamespace holds
	// resources from several source namespaces.
	PlacementEndpointISVCNamespaceLabel = "ome.io/placement-endpoint-isvc-namespace"

	// EndpointFinalizer keeps a placed InferenceService around long enough for the
	// publisher to tear down its global-traffic backend before the object is
	// removed. Distinct from the placement controller's own finalizer so the two
	// controllers' teardown order is independent.
	EndpointFinalizer = "ome.io/placement-endpoint"
)
