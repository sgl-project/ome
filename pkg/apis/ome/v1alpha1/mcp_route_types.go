package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPRoute is the Schema for the mcproutes API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=mcpr
// +kubebuilder:printcolumn:name="Backends",type=string,JSONPath=`.spec.backendRefs[*]`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
type MCPRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPRouteSpec   `json:"spec"`
	Status MCPRouteStatus `json:"status,omitempty"`
}

type MCPRouteSpec struct {
	// BackendRefs defines where to route requests
	// All backends must be MCPServers in the same namespace
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	BackendRefs []MCPBackendRef `json:"backendRefs"`

	// Matches defines routing rules (optional)
	// If not specified, routes all tools from backend servers
	// +optional
	Matches []MCPRouteMatch `json:"matches,omitempty"`

	// Authentication policy for this route
	// Overrides/extends gateway default if present
	// +optional
	Authentication *MCPAuthentication `json:"authentication,omitempty"`

	// Authorization policy for this route
	// Adds to gateway default authorization
	// +optional
	Authorization *MCPAuthorization `json:"authorization,omitempty"`

	// RateLimit for this route
	// Adds to gateway default rate limits
	// +optional
	RateLimit *MCPRateLimit `json:"rateLimit,omitempty"`

	// Filters for request/response modification
	// +optional
	Filters []MCPRouteFilter `json:"filters,omitempty"`
}

type MCPBackendRef struct {
	// ServerRef references an MCPServer in the same namespace
	// +kubebuilder:validation:Required
	ServerRef corev1.LocalObjectReference `json:"serverRef"`

	// Weight for traffic splitting across backends
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

type MCPRouteMatch struct {
	// Tools to match - supports simple wildcards in tool names
	// Examples: "db_query", "db_*", "*_query"
	// +optional
	Tools []string `json:"tools,omitempty"`

	// ToolMatch defines advanced tool matching (alternative to Tools)
	// +optional
	ToolMatch *ToolMatcher `json:"toolMatch,omitempty"`

	// Method to match (tools/call, tools/list, prompts/get, etc.)
	// +optional
	Method *string `json:"method,omitempty"`

	// Headers to match
	// +optional
	Headers []HeaderMatch `json:"headers,omitempty"`

	// BackendRefs for this match (optional)
	// If specified, overrides route-level backendRefs for matching requests
	// +optional
	BackendRefs []MCPBackendRef `json:"backendRefs,omitempty"`
}

type ToolMatcher struct {
	// PrefixMatch matches tools with this prefix
	// +optional
	PrefixMatch *string `json:"prefixMatch,omitempty"`

	// ExactMatch matches exact tool names
	// +optional
	ExactMatch *string `json:"exactMatch,omitempty"`

	// RegexMatch matches tools using regex
	// +optional
	RegexMatch *string `json:"regexMatch,omitempty"`
}

type HeaderMatch struct {
	// Name of the header
	Name string `json:"name"`

	// Value to match
	Value string `json:"value"`

	// Type of match (Exact, Prefix, Regex)
	// +kubebuilder:validation:Enum=Exact;Prefix;Regex
	// +kubebuilder:default=Exact
	Type string `json:"type"`
}

type MCPRouteFilter struct {
	// Type of filter
	// +kubebuilder:validation:Enum=RequestHeaderModifier;ResponseHeaderModifier
	Type string `json:"type"`

	// RequestHeaderModifier configuration
	// +optional
	RequestHeaderModifier *HeaderModifier `json:"requestHeaderModifier,omitempty"`

	// ResponseHeaderModifier configuration
	// +optional
	ResponseHeaderModifier *HeaderModifier `json:"responseHeaderModifier,omitempty"`
}

type HeaderModifier struct {
	// Set headers (replaces if exists)
	// +optional
	Set []Header `json:"set,omitempty"`

	// Add headers (appends if exists)
	// +optional
	Add []Header `json:"add,omitempty"`

	// Remove headers
	// +optional
	Remove []string `json:"remove,omitempty"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MCPRouteStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// GatewayURL is the endpoint LLMs should connect to for this route
	// Format: http://gateway.namespace/routes/{namespace}/{route-name}
	GatewayURL string `json:"gatewayURL,omitempty"`

	// BackendStatuses shows health of each backend
	BackendStatuses []BackendStatus `json:"backendStatuses,omitempty"`
}

type BackendStatus struct {
	ServerRef corev1.LocalObjectReference `json:"serverRef"`
	Ready     bool                        `json:"ready"`
	Endpoint  string                      `json:"endpoint,omitempty"`
	Message   string                      `json:"message,omitempty"`
}

type MCPAuthentication struct {
	// OIDC defines OpenID Connect authentication
	// +optional
	OIDC *OIDCAuthentication `json:"oidc,omitempty"`

	// JWT defines JWT token authentication
	// +optional
	JWT *JWTAuthentication `json:"jwt,omitempty"`

	// APIKey defines API key authentication
	// +optional
	APIKey *APIKeyAuthentication `json:"apiKey,omitempty"`
}

type OIDCAuthentication struct {
	// Issuer is the OIDC issuer URL
	// +kubebuilder:validation:Required
	Issuer string `json:"issuer"`

	// ClientID is the OAuth2 client ID
	// +kubebuilder:validation:Required
	ClientID string `json:"clientID"`

	// ClientSecretRef references a Secret containing the client secret
	// +kubebuilder:validation:Required
	ClientSecretRef corev1.SecretKeySelector `json:"clientSecretRef"`

	// Scopes defines the OAuth2 scopes to request
	// +optional
	Scopes []string `json:"scopes,omitempty"`
}

type JWTAuthentication struct {
	// Audiences defines valid JWT audiences
	// +kubebuilder:validation:MinItems=1
	Audiences []string `json:"audiences"`

	// JWKSURI is the URI for the JSON Web Key Set
	// +kubebuilder:validation:Required
	JWKSURI string `json:"jwksURI"`

	// Issuer defines the expected JWT issuer (optional)
	// +optional
	Issuer *string `json:"issuer,omitempty"`
}

type APIKeyAuthentication struct {
	// Header is the name of the header containing the API key
	// +kubebuilder:default="X-API-Key"
	Header string `json:"header"`

	// SecretRefs references Secrets containing valid API keys
	// +kubebuilder:validation:MinItems=1
	SecretRefs []corev1.SecretKeySelector `json:"secretRefs"`
}

type MCPAuthorization struct {
	// Rules defines authorization rules
	// +kubebuilder:validation:MinItems=1
	Rules []AuthorizationRule `json:"rules"`
}

type AuthorizationRule struct {
	// Principals this rule applies to (users, groups, service accounts)
	// Format: "user:alice", "group:developers", "serviceaccount:my-sa"
	// +kubebuilder:validation:MinItems=1
	Principals []string `json:"principals"`

	// Permissions define allowed actions
	// +kubebuilder:validation:MinItems=1
	Permissions []Permission `json:"permissions"`

	// Conditions for additional filtering (optional)
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

type Permission struct {
	// Tools this permission applies to (supports wildcards)
	// +kubebuilder:validation:MinItems=1
	Tools []string `json:"tools"`

	// Actions allowed
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=tools/call;tools/list
	Actions []string `json:"actions"`
}

type Condition struct {
	// Type of condition check
	// +kubebuilder:validation:Enum=IPAddress;TimeOfDay;RequestHeader
	Type string `json:"type"`

	// Key for the condition (e.g., header name for RequestHeader type)
	// +optional
	Key *string `json:"key,omitempty"`

	// Value to match against
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// Operator for matching
	// +kubebuilder:validation:Enum=Equal;NotEqual;In;NotIn;Matches;NotMatches
	// +kubebuilder:default=Equal
	Operator string `json:"operator"`
}

type MCPRateLimit struct {
	// Limits defines rate limiting rules
	// +kubebuilder:validation:MinItems=1
	Limits []RateLimit `json:"limits"`
}

type RateLimit struct {
	// Dimension defines what to rate limit by
	// +kubebuilder:validation:Enum=user;ip;tool;principal;namespace
	// +kubebuilder:validation:Required
	Dimension string `json:"dimension"`

	// Tools restricts this limit to specific tools (optional)
	// +optional
	Tools []string `json:"tools,omitempty"`

	// Requests is the maximum number of requests allowed
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	Requests int32 `json:"requests"`

	// Unit is the time unit for the limit
	// +kubebuilder:validation:Enum=second;minute;hour;day
	// +kubebuilder:validation:Required
	Unit string `json:"unit"`
}

// +kubebuilder:object:root=true

// MCPRouteList contains a list of MCPRoute
type MCPRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPRoute{}, &MCPRouteList{})
}
