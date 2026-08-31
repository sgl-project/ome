// Package v1beta1 — traffic management types.
//
// This file defines the typed core of the InferenceService traffic
// management surface: load-balancer algorithm, consistent-hash key,
// and endpoint override. Long-tail knobs (circuit breaker, retries,
// sub-second timeouts) live as ome.io/* annotations on the
// InferenceService metadata. Progressive rollout lives on spec.rollout,
// not here — this block is steady-state load balancing only.
package v1beta1

// TrafficSpec configures the load-balancing policy applied to all
// HTTPRoutes OME emits for the InferenceService. The translated
// backend policy resource (e.g. Envoy Gateway BackendTrafficPolicy)
// targets every OME-managed HTTPRoute via targetRefs, so behavior is
// consistent whether clients address the top-level URL or a
// per-Component subdomain.
//
// For less-common settings (circuit breaker, retries, sub-second
// timeouts), see the ome.io/* annotation contract on the
// InferenceService metadata.
//
// For Component-specific routing (one Component needs a different LB
// algorithm), hand-author a separate BackendTrafficPolicy targeting
// only that Component's HTTPRoute — OME defers to the hand-authored
// policy.
type TrafficSpec struct {
	// Algorithm selects the load-balancing algorithm. Absent = the
	// Gateway implementation's default (LeastRequest for Envoy Gateway).
	// +kubebuilder:validation:Enum=RoundRobin;LeastRequest;Random;ConsistentHash
	// +optional
	Algorithm *LoadBalancingType `json:"algorithm,omitempty"`

	// ConsistentHash configures the hash key. Required when
	// Algorithm=ConsistentHash; rejected otherwise (webhook).
	// +optional
	ConsistentHash *ConsistentHashSpec `json:"consistentHash,omitempty"`

	// EndpointOverride routes requests to a specific pod when a request
	// header (e.g. "X-Endpoint-HostPort: 10.0.0.5:30000") is present
	// and the endpoint is healthy; falls back to the configured
	// Algorithm when the endpoint is unhealthy or the header is absent.
	//
	// Envoy Gateway only in alpha (no Istio equivalent). Translators
	// that do not support endpoint override surface this as an
	// UnsupportedField condition on the InferenceService.
	// +optional
	EndpointOverride *EndpointOverrideSpec `json:"endpointOverride,omitempty"`
}

// LoadBalancingType selects the load-balancing algorithm applied to
// a Component's backend Service(s).
type LoadBalancingType string

const (
	// LoadBalancingTypeRoundRobin distributes connections evenly across
	// backend endpoints in order.
	LoadBalancingTypeRoundRobin LoadBalancingType = "RoundRobin"
	// LoadBalancingTypeLeastRequest sends a new connection to the
	// backend endpoint with the fewest active requests. Envoy Gateway
	// default.
	LoadBalancingTypeLeastRequest LoadBalancingType = "LeastRequest"
	// LoadBalancingTypeRandom selects a backend endpoint at random.
	LoadBalancingTypeRandom LoadBalancingType = "Random"
	// LoadBalancingTypeConsistentHash hashes a request attribute
	// (header, cookie, or source IP) and consistently routes requests
	// with the same hash to the same backend endpoint. Useful for
	// session affinity.
	LoadBalancingTypeConsistentHash LoadBalancingType = "ConsistentHash"
)

// ConsistentHashSpec selects what request attribute to hash on when
// LoadBalancingType=ConsistentHash. Exactly one of Headers or Cookie
// must be set when Type is Header or Cookie respectively; both
// forbidden when Type is SourceIP.
type ConsistentHashSpec struct {
	// Type selects the hash source.
	// +kubebuilder:validation:Enum=Header;Cookie;SourceIP
	Type HashType `json:"type"`

	// Headers to hash on. Required when Type=Header (one or more).
	// Translators that support multi-header hashing concatenate the
	// headers before hashing; when the active translator supports only
	// single-header hashing, multi-header specs are rejected at
	// admission. Forbidden when Type is not Header.
	// +optional
	// +listType=atomic
	Headers []HashHeader `json:"headers,omitempty"`

	// Cookie configuration. Required when Type=Cookie.
	// Forbidden when Type is not Cookie.
	// +optional
	Cookie *HashCookie `json:"cookie,omitempty"`
}

// HashType selects what request attribute is hashed for
// ConsistentHash load balancing.
type HashType string

const (
	// HashTypeHeader hashes one or more request headers.
	HashTypeHeader HashType = "Header"
	// HashTypeCookie hashes a named cookie.
	HashTypeCookie HashType = "Cookie"
	// HashTypeSourceIP hashes the client source IP.
	HashTypeSourceIP HashType = "SourceIP"
)

// HashHeader names a single request header to include in the
// consistent-hash input.
type HashHeader struct {
	// Name is the header name (case-insensitive per HTTP).
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// HashCookie names a cookie and an optional TTL for auto-issuance.
type HashCookie struct {
	// Name is the cookie name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// TTLSeconds, when greater than zero, makes the gateway auto-issue
	// the cookie if absent on the request. When zero or unset, the
	// client is responsible for sending the cookie.
	// +optional
	// +kubebuilder:validation:Minimum=0
	TTLSeconds *int64 `json:"ttlSeconds,omitempty"`
}

// EndpointOverrideSpec configures Envoy Gateway's loadBalancer
// endpoint-override feature. When a request carries an endpoint
// reference (typically a header), the gateway routes the request to
// that exact endpoint if healthy, otherwise falls back to the
// configured LoadBalancer Algorithm.
type EndpointOverrideSpec struct {
	// Type selects where to read the endpoint override from.
	// +kubebuilder:validation:Enum=Header;Metadata
	Type EndpointOverrideType `json:"type"`

	// Headers to read the endpoint reference from. Required when
	// Type=Header. Each header value must be "<host>:<port>" or
	// "<ip>:<port>".
	// +optional
	// +listType=atomic
	Headers []HashHeader `json:"headers,omitempty"`
}

// EndpointOverrideType selects how the override endpoint reference
// is extracted from the request.
type EndpointOverrideType string

const (
	// EndpointOverrideTypeHeader reads the override from a request
	// header.
	EndpointOverrideTypeHeader EndpointOverrideType = "Header"
	// EndpointOverrideTypeMetadata reads the override from request
	// metadata. Reserved: no translator emits it, and admission
	// rejects the value until one declares the capability.
	EndpointOverrideTypeMetadata EndpointOverrideType = "Metadata"
)
