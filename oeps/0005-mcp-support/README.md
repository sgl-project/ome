# OEP-0005: Model Context Protocol (MCP) Support

This OEP introduces native support for the Model Context Protocol (MCP) in OME through a hybrid architecture that balances simplicity with multi-tenancy. The design uses two CRDs (MCPServer and MCPRoute) with an operator-managed gateway, avoiding the complexity of full Kubernetes Gateway API while supporting platform team infrastructure control and application team routing flexibility.

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
    - [Story 1: Data Scientist Using MCP Tools](#story-1-data-scientist-using-mcp-tools)
    - [Story 2: Direct Server Access for Trusted Environments](#story-2-direct-server-access-for-trusted-environments)
  - [Notes/Constraints/Caveats](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [API Specifications](#api-specifications)
    - [MCPServer Resource](#mcpserver-resource)
    - [InferenceService MCP Integration](#inferenceservice-mcp-integration)
  - [Architecture Overview](#architecture-overview)
    - [Component Interaction](#component-interaction)
    - [Request Flow](#request-flow)
  - [Security Model](#security-model)
  - [Deployment Patterns](#deployment-patterns)
  - [Evolution Path](#evolution-path)
  - [Test Plan](#test-plan)
    - [Unit Tests](#unit-tests)
    - [Integration Tests](#integration-tests)
- [Drawbacks](#drawbacks)
<!-- /toc -->

## Summary

This OEP introduces native support for the Model Context Protocol (MCP) in OME with a **hybrid architecture** that balances simplicity with multi-tenancy capabilities.

**Core Design Principles**:

1. **`MCPServer` CRD**: Defines and manages individual MCP tool servers, supporting both in-cluster hosted servers (via `PodTemplateSpec`) and external remote services. Includes permission profiles for security.

2. **`MCPRoute` CRD**: User-facing routing configuration that defines how requests reach MCP servers. Supports traffic splitting, tool-level matching, and embedded policies (authentication, authorization, rate limiting).

3. **Internal Managed Gateway**: OME deploys gateway infrastructure based on operator-managed default config via ConfigMap or Helm values. Gateway auto-discovers MCPRoute resources and applies routing rules dynamically. No user-visible Gateway CRD (unlike full Gateway API).

This approach provides:
- **Multi-Tenancy Support**: Platform teams set gateway defaults, app teams define routes, security teams enforce policies
- **Simpler than Gateway API**: No parentRef complexity, no cross-namespace ReferenceGrant, fewer CRDs
- **Proven Pattern**: Similar to Istio VirtualService+Gateway, AWS App Mesh - industry-validated architecture
- **Flexible Control**: Gateway-level defaults + route-level overrides
- **Evolution Path**: Start with v1alpha1 embedded config, evolve to v1beta1 hybrid based on validated needs
- **Consistency**: Follows OME's philosophy of "declare intent, operator handles complexity"

## Motivation

Modern AI applications increasingly require LLMs to interact with external systems beyond simple text generation. Use cases include:

-   **Data Analysis**: LLMs querying databases, accessing APIs, and processing files to provide insights.
-   **Infrastructure Management**: AI agents managing cloud resources, deployments, and monitoring systems.
-   **Business Process Automation**: Models performing complex workflows involving multiple systems.
-   **Research and Development**: AI assistants with access to specialized tools and datasets.

Currently, integrating LLMs with external tools requires custom implementations for each service, leading to fragmentation, security complexities, and high operational overhead. The Model Context Protocol (MCP) provides a standard interface, but enterprises need a robust framework for managing and consuming these tool servers securely and at scale. This OEP addresses that need.

### Goals

1.  **Multi-Tenancy Support**: Enable platform teams, application teams, and security teams to work independently with clear RBAC boundaries while sharing infrastructure.
2.  **Flexible Server Definition**: Support both in-cluster `Hosted` servers using native `PodTemplateSpec` and `Remote` external servers with consistent API patterns.
3.  **User-Controlled Routing**: Provide MCPRoute CRD for explicit routing control (traffic splitting, tool matching, policies) while maintaining simple auto-create mode for basic use cases.
4.  **Operator-managed Gateway**: Platform teams to set infrastructure defaults (replicas, resources) and baseline policies directly into the operator configuration that routes inherit.
5.  **Policy Hierarchy**: Support both gateway-level default policies (enforced by platform/security teams) and route-level policy additions (managed by app teams), with more restrictive policies winning.
6.  **Granular Security Model**: Implement comprehensive permission model for MCPServers (K8s resources, network restrictions) and policy enforcement (authentication, authorization, rate limiting).
7. **Proven Pattern**: Follow industry-validated patterns similar to Istio VirtualService+Gateway and AWS App Mesh.
8. **Evolution Path**: Start with v1alpha1 embedded config, evolve to v1beta1 hybrid model based on validated needs, with option to add full Gateway API in v2 if required.

### Non-Goals

1.  **MCP Protocol Implementation**: This OEP focuses on deployment and orchestration, not on building an MCP protocol library.
2.  **Custom Tool Development**: Building domain-specific MCP tools is outside the scope.
3.  **Legacy Protocol Support**: The focus is on the standardized MCP interface, not proprietary protocols.

## Proposal

We introduce an architecture with two CRDs that balances simplicity with multi-tenancy capabilities:

### `MCPServer`: The Tool Server Definition

Defines individual MCP tool servers (namespace-scoped). The `spec` distinguishes between two server types:

-   **`hosted`**: In-cluster servers using `PodTemplateSpec` for full Kubernetes pod configuration flexibility
-   **`remote`**: External servers specified by `url` for SaaS integrations

Key features:
- **Transport protocol**: stdio, streamable-http, or sse
- **Capabilities**: Declares supported MCP features (tools, resources, prompts)
- **Permission profile**: Controls access to Kubernetes resources and outbound network traffic
- **Tool filtering**: Optional whitelist/blacklist of specific tools

### `MCPRoute`: User-Facing Routing Configuration

Defines routing from gateway to backend MCPServers (namespace-scoped). Application teams create routes to control traffic flow:

-   **Backend references**: List of MCPServers with optional traffic weights for canary/blue-green deployments
-   **Tool matching**: Route specific tools to specific backends (e.g., "db_*" → database server)
-   **Embedded policies**: Authentication, authorization, and rate limiting specific to this route
-   **Request filters**: Header modifications, transformations

Routes are **automatically discovered** by gateway (no parentRef needed). 

### Internal Gateway Deployment (Auto-Managed by OME)

OME inject gateway infrastructure per Inference Service and MCPRoute:

-   **Auto-discovery**: Watches MCPRoute resources and dynamically configures routing
-   **Policy merging**: Combines gateway defaults with route-specific policies (more restrictive wins)
-   **Load balancing**: Distributes requests across server replicas with health checks
-   **Protocol handling**: HTTP, gRPC, WebSocket support
-   **Observability**: Metrics and logs labeled by namespace, route, server

The gateway deployment itself is **not a user-facing CRD** (unlike Kubernetes Gateway API). Engineer defines defaults in operator configurations (infrastructure) and users interact with MCPRoute (routing).

### `InferenceService` Integration

Extended with two integration modes for flexibility:

**Simple Mode** (Auto-Create MCPRoute):
```yaml
spec:
  mcpServers:
  - serverRef: {name: my-server}
  mcpRoute:
    authentication: {...}  # Embedded config
```

**Explicit Mode** (Reference Existing MCPRoute):
```yaml
spec:
  mcpRoute:
    routeRef: {name: shared-route}
```
InferenceService references existing MCPRoute. Enables route sharing across multiple InferenceServices.

### User Stories

#### Story 1: Data Scientist Using MCP Tools

Bob is a data scientist who wants his LLM to access a PostgreSQL database through MCP tools. He creates tool servers and references them in his InferenceService.

```yaml
# 1. Deploy MCPServer for database tools
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: postgres-tools
  namespace: my-team
spec:
  transport: streamable-http
  hosted:
    replicas: 2
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/postgres-mcp:1.0.0
          env:
          - name: DATABASE_URI
            valueFrom:
              secretKeyRef:
                name: db-credentials
                key: uri
  # Embedded permission profile
  permissionProfile:
    inline:
      allow:
      - network:
          allowHost:
          - "postgres.my-team.svc.cluster.local"

---
# 2. Deploy InferenceService with MCP tool references
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: my-llm
  namespace: my-team
spec:
  model:
    name: llama-3-70b
  runtime:
    name: vllm
  # NEW: MCP server references
  mcpServers:
  - serverRef:
      name: postgres-tools
    # Optional: per-server overrides
    weight: 100
```

**What OME does automatically**:
1. Creates Deployment and Service for `postgres-tools` MCPServer
2. Detects InferenceService references MCP servers
3. Auto-creates MCPRoute for the server references
4. Deploys internal gateway component (operator-managed)
5. Configures routing from LLM pods → gateway → postgres-tools
6. Injects gateway endpoint into LLM pods as `MCP_GATEWAY_URL` environment variable
7. Applies authentication and rate limiting policies at gateway level

Bob's LLM can now access database tools through a secure, load-balanced gateway without complex configuration.

#### Story 2: Direct Server Access for Trusted Environments

Alice is deploying an LLM in a trusted internal environment where she wants minimal latency and doesn't need centralized audit logging.

```yaml
# 1. Deploy MCPServer
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: k8s-tools
  namespace: my-team
spec:
  transport: streamable-http
  hosted:
    replicas: 1
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/k8s-mcp:1.0.0
  # Grant Kubernetes API access
  permissionProfile:
    inline:
      allow:
      - kubeResources:
          apiGroups: [""]
          resources: ["pods", "services"]
          verbs: ["get", "list"]

---
# 2. InferenceService with MCP server ref
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: my-llm
  namespace: my-team
spec:
  model:
    name: llama-3-70b
  runtime:
    name: vllm
  mcpServers:
  - serverRef:
      name: k8s-tools
```

**What OME does automatically**:
1. Creates Deployment, Service, and RBAC (Role/RoleBinding) for `k8s-tools`
2. Auto-creates MCPRoute for the server reference
3. Deploys managed gateway (single server still uses gateway for policy enforcement)
4. Injects gateway endpoint into LLM pods as `MCP_GATEWAY_URL`

Note: For truly direct pod-to-pod access without gateway, users can configure this via operator-level settings. The default is to always use the managed gateway for consistent policy enforcement.

Alice gets direct server access with minimal overhead while still benefiting from OME's server lifecycle management and RBAC generation.

### Notes/Constraints/Caveats

1.  **Hosted Server Container Name**: When using a `hosted` `MCPServer`, the container running the MCP server process within the `podSpec` must be named `mcp-server` for the controller to correctly inject configurations. (This requirement may be relaxed in future versions using label selectors.)
2.  **Permissions**: The `permissionProfile` with `kubeResources` is very powerful. Misconfiguration can create security risks. The controller creates `Roles`/`RoleBindings`, so RBAC must be enabled in the cluster. Documentation will strongly emphasize the principle of least privilege.
3.  **Operator-Managed Gateway**: The gateway component in v1alpha1 is **not a user-facing CRD**. It's managed internally by OME based on InferenceService configuration. 
4.  **Gateway Scope**: In v1alpha1, each InferenceService gets its own logical gateway configuration. Shared gateway infrastructure (multi-tenant) will be considered for v1beta1 if user feedback indicates strong demand.
5.  **Policy Embedding**: Policies (authentication, authorization, rate limiting) are embedded in MCPRoute and InferenceService specs rather than separate CRDs. This simplifies the API for v1alpha1 while maintaining a path to separate Policy CRDs in future versions.
6.  **Transport Limitations**: `stdio` transport is only suitable for simple, single-shot tools and does not support scaling beyond one replica. `streamable-http` or `sse` are recommended for production deployments.
7.  **Observability**: Both MCPServer and gateway components follow Kubernetes-native observability patterns:
  - Health checks via standard K8s probes
  - Metrics via Prometheus annotations
  - Tracing via service mesh/OpenTelemetry integration
  - Logging via stdout/stderr (structured logging recommended)
8.  **Service Mesh Integration**: When service mesh (Istio, Linkerd) is available, users can choose to disable the managed gateway and rely on service mesh policies for authentication, authorization, and traffic management.

### Risks and Mitigations

-   **Risk 1: Over-privileged Tool Servers**: The `kubeResources` permission could grant excessive permissions.
    -   **Mitigation**: The API is declarative with all permissions explicitly defined in YAML and auditable. The controller generates narrowly scoped `Roles` with least-privilege principles. Documentation and examples will strongly emphasize security best practices. Consider adding validation webhooks to warn about overly broad permissions.

-   **Risk 2: Gateway as Single Point of Failure**: If the managed gateway goes down, all tool access is lost (when gateway mode is enabled).
    -   **Mitigation**: The managed gateway will support multiple replicas with automatic load balancing. Standard Kubernetes practices for HA (Pod anti-affinity, PodDisruptionBudget) will be applied. 

-   **Risk 3: Limited Flexibility in v1alpha1**: Starting with operator-managed gateway means less flexibility for advanced users who need custom gateway configurations.
    -   **Mitigation**: The design includes a clear evolution path. v1beta1 can introduce user-facing MCPGateway CRD and separate Policy CRDs based on validated user needs. The simplified v1alpha1 design accelerates time-to-market while gathering real-world usage patterns to inform future enhancements.

-   **Risk 4: Gateway Data Plane Implementation Complexity**: Building an MCP-aware gateway requires significant development effort (12-18 months estimated).
    -   **Mitigation**: Start with a minimal viable gateway that handles basic routing and load balancing. Advanced features (protocol translation, complex policies) can be added incrementally based on priority. Consider leveraging existing proxy infrastructure (Envoy, nginx) with MCP-specific extensions rather than building from scratch.

## Design Details

### API Specifications

The model introduces two CRDs with clear separation of concerns.

### MCPServer Resource

MCPServer defines individual MCP tool servers (namespace-scoped). 

**Key Points**:
- Namespace-scoped (routes can only reference servers in same namespace)
- Supports both `hosted` (in-cluster) and `remote` (external) servers
- Permission profiles for K8s resource and network access control
- No embedded policies (per-route policies moved to MCPRoute)


**`MCPServerSpec`**

```go
// MCPServerSpec defines the desired state of an MCPServer.
// An MCPServer can either be 'Hosted' within the cluster or a 'Remote' external service.
// +kubebuilder:validation:XValidation:rule="has(self.hosted) || has(self.remote)", message="either hosted or remote must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.hosted) && has(self.remote))", message="hosted and remote are mutually exclusive"
type MCPServerSpec struct {
	// Hosted defines a server that runs as pods within the cluster.
	// +optional
	Hosted *HostedMCPServer `json:"hosted,omitempty"`

	// Remote defines a server that is accessed via an external URL.
	// +optional
	Remote *RemoteMCPServer `json:"remote,omitempty"`

	// Transport specifies the transport protocol for MCP communication.
	// +kubebuilder:default=stdio
	// +optional
	Transport MCPTransportType `json:"transport,omitempty"`

	// Capabilities defines the features supported by this server.
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`

	// Version of the MCP server software.
	// +optional
	Version string `json:"version,omitempty"`

	// PermissionProfile defines the operational permissions for the server.
	// +optional
	PermissionProfile *PermissionProfileSource `json:"permissionProfile,omitempty"`

	// ToolsFilter restricts the tools exposed by this server.
	// +optional
	// +listType=set
	ToolsFilter []string `json:"toolsFilter,omitempty"`
}

```

-   **`hosted` vs `remote`**: The spec enforces that exactly one of these is set.
-   **`HostedMCPServer`**:
    ```go
    type HostedMCPServer struct {
        // PodSpec defines the pod template to use for the MCP server.
        PodSpec corev1.PodTemplateSpec `json:"podSpec"`

        // Replicas is the number of desired replicas for the server.
        // +kubebuilder:validation:Minimum=0
        // +kubebuilder:default=1
        // +optional
        Replicas *int32 `json:"replicas,omitempty"`
    }
    ```
    This structure delegates all pod-level configuration to the standard `PodTemplateSpec`, making it incredibly flexible and familiar to Kubernetes users.
-   **`RemoteMCPServer`**:
    ```go
    type RemoteMCPServer struct {
        // URL is the external URL of the remote MCP server.
        // +kubebuilder:validation:Pattern=`^https?://.*`
        URL string `json:"url"`
    }
    ```
-   **`PermissionProfileSource`**: This defines the permissions for a `Hosted` server.
    ```go
    // +kubebuilder:validation:XValidation:rule="(has(self.builtin) + has(self.configMap) + has(self.inline)) <= 1",message="at most one of builtin, configMap, or inline can be set"
    type PermissionProfileSource struct {
        Builtin   *BuiltinPermissionProfile      `json:"builtin,omitempty"`
        ConfigMap *corev1.ConfigMapKeySelector   `json:"configMap,omitempty"`
        Inline    *PermissionProfileSpec         `json:"inline,omitempty"`
    }

    type PermissionProfileSpec struct {
        // Allow specifies the permissions granted to the server.
        // +listType=atomic
        Allow []PermissionRule `json:"allow"`
    }

    type PermissionRule struct {
        KubeResources *KubeResourcePermission `json:"kubeResources,omitempty"`
        Network       *NetworkPermission      `json:"network,omitempty"`
    }
    ```
    The most powerful feature here is `KubeResourcePermission`, which allows granting fine-grained RBAC permissions to the server's pod.
    ```go
    type KubeResourcePermission struct {
        APIGroups []string `json:"apiGroups"`
        Resources []string `json:"resources"`
        Verbs     []string `json:"verbs"`
        // Namespaces restricts permissions to specific namespaces
        // If empty, permissions apply to the MCPServer's namespace only
        // +optional
        Namespaces []string `json:"namespaces,omitempty"`
    }

    type NetworkPermission struct {
        // AllowHost specifies allowed destination hosts
        // Supports wildcards: "*.internal.svc.cluster.local"
        // +optional
        AllowHost []string `json:"allowHost,omitempty"`

        // AllowCIDR specifies allowed destination CIDR blocks (optional for future)
        // +optional
        AllowCIDR []string `json:"allowCIDR,omitempty"`
    }
    ```

**Network Enforcement Clarification**:

The `NetworkPermission` type provides declarative intent for network access control. Actual enforcement depends on the cluster's network policy implementation:

1. **NetworkPolicy Enforcement** (Recommended):
   - For hosted MCPServers, the controller generates Kubernetes `NetworkPolicy` resources based on `allowHost` and `allowCIDR`
   - NetworkPolicy egress rules restrict pod-to-pod and pod-to-external traffic
   - Requires a CNI plugin with NetworkPolicy support (Calico, Cilium, Weave Net, etc.)
   - Implementation: Controller translates DNS names to ClusterIP selectors and CIDR blocks to ipBlock rules
   - Wildcards (e.g., `*.internal.svc.cluster.local`) are expanded to matching Service selectors

2. **Service Mesh Enforcement** (Optional):
   - When service mesh (Istio, Linkerd) is detected, controller can optionally generate:
     - `ServiceEntry` resources for external destinations (allowHost)
     - `AuthorizationPolicy` resources for egress rules (allowHost/allowCIDR)
   - Provides additional features: mTLS, retry policies, circuit breaking
   - Enforcement happens at sidecar proxy level (more fine-grained than NetworkPolicy)

3. **No Enforcement Mode** (Declarative Only):
   - In clusters without NetworkPolicy support or service mesh, `NetworkPermission` serves as documentation only
   - Controller logs warnings: "NetworkPermission specified but no enforcement mechanism available"
   - Status field `networkEnforcement` indicates actual enforcement mode
   - Suitable for development environments or trusted internal networks

4. **Status Reporting**:
   ```go
   type MCPServerStatus struct {
       // NetworkEnforcement indicates how network permissions are enforced
       // +kubebuilder:validation:Enum=NetworkPolicy;ServiceMesh;None
       // +optional
       NetworkEnforcement *string `json:"networkEnforcement,omitempty"`

       // ... other status fields
   }
   ```
   - Controller populates based on cluster capabilities detected at reconciliation time
   - Users can verify enforcement mode via `kubectl get mcpserver -o jsonpath='{.status.networkEnforcement}'`

**Best Practices**:
- **Production deployments**: Use CNI with NetworkPolicy support or service mesh for security guarantees
- **Development/testing**: Declarative-only mode acceptable with explicit acknowledgment
- **Validation**: Controller webhook warns if NetworkPermission specified but no enforcement available (unless explicitly disabled via annotation)

### MCPRoute Resource

MCPRoute defines routing configuration from gateway to backend MCPServers (namespace-scoped).

```go
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
	ServerRef LocalObjectReference `json:"serverRef"`

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
	ServerRef LocalObjectReference `json:"serverRef"`
	Ready     bool                  `json:"ready"`
	Endpoint  string                `json:"endpoint,omitempty"`
	Message   string                `json:"message,omitempty"`
}
```

**Policy Type Definitions** :

```go
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
	// +kubebuilder:validation:Enum=tools/call,tools/list
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
```

### InferenceService MCP Integration

InferenceService is extended to support with two modes.

```go
type InferenceServiceSpec struct {
	// ... existing fields (model, runtime, etc.) ...

	// MCPServers for auto-creating MCPRoute (backward compatible)
	// Mutually exclusive with MCPRoute.RouteRef
	// +optional
	// +kubebuilder:validation:MaxItems=32
	MCPServers []MCPServerReference `json:"mcpServers,omitempty"`

	// MCPRoute configuration
	// +optional
	MCPRoute *MCPRouteConfig `json:"mcpRoute,omitempty"`
}

type MCPServerReference struct {
	// ServerRef references an MCPServer in the same namespace as the InferenceService
	// Cross-namespace references are not supported (enforced by validation webhook)
	// +kubebuilder:validation:Required
	ServerRef LocalObjectReference `json:"serverRef"`

	// Weight for traffic splitting (default: 1)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	Weight *int32 `json:"weight,omitempty"`
}

type MCPRouteConfig struct {
	// RouteRef references an existing MCPRoute (explicit mode)
	// When set, MCPServers and embedded config are ignored
	// +optional
	RouteRef *LocalObjectReference `json:"routeRef,omitempty"`

	// Embedded config for auto-creating MCPRoute (simple mode)
	// Only used when RouteRef is not set
	// +optional
	Authentication *MCPAuthentication `json:"authentication,omitempty"`
	Authorization *MCPAuthorization `json:"authorization,omitempty"`
	RateLimit *MCPRateLimit `json:"rateLimit,omitempty"`
	Matches []MCPRouteMatch `json:"matches,omitempty"`
	Filters []MCPRouteFilter `json:"filters,omitempty"`
}
```

**How It Works**:

1. **Simple Mode** (Auto-Create MCPRoute):
   - User specifies `mcpServers` with optional embedded config in `mcpRoute`
   - InferenceService controller creates MCPRoute resource automatically
   - MCPRoute name: `{inference-service-name}-route`
   - Maintains v1alpha1 backward compatibility

2. **Explicit Mode** (Reference Existing MCPRoute):
   - User creates MCPRoute separately
   - User specifies `mcpRoute.routeRef` in InferenceService
   - InferenceService just references the route
   - Enables route sharing across multiple InferenceServices

3. **Validation**:
   - Webhook ensures either `mcpServers` OR `mcpRoute.routeRef`, not both
   - If `routeRef` is set, `mcpServers` must be empty
### Architecture Overview

The architecture uses two user-facing CRDs with operator-managed gateway infrastructure.

```mermaid
graph TB
    subgraph "User-Facing Layer"
        direction LR
        U1[Application Team]
        U2[Platform Team]
    end

    subgraph "CRD Resources"
        direction TB
        CRD1[MCPServer<br/>Type: hosted/remote<br/>PermissionProfile]
        CRD2[MCPRoute<br/>BackendRefs<br/>Policies<br/>Matches]
        CRD3[InferenceService<br/>mcpServers list<br/>mcpRoute config]
    end

    subgraph "Control Plane - ome-system namespace"
        direction TB

        subgraph "Controllers"
            direction LR
            C1[MCPServer<br/>Controller]
            C2[MCPRoute<br/>Controller]
            C3[InferenceService<br/>Controller]
        end

        subgraph "Managed Gateway Infrastructure"
            direction TB
            GW[MCP Gateway<br/>┌──────────────┐<br/>│ Auto-Discovery│<br/>│ Policy Enforcement│<br/>│ Load Balancing│<br/>└──────────────┘]

            subgraph "Gateway Functions"
                direction LR
                GW1[Authentication<br/>JWT/OIDC/APIKey]
                GW2[Authorization<br/>RBAC Policies]
                GW3[Rate Limiting<br/>Multi-dimension]
                GW4[Route Table<br/>Dynamic Update]
            end
        end
    end

    subgraph "Data Plane - app namespace"
        direction TB

        subgraph "Hosted MCPServers"
            direction LR
            HS1[MCPServer Pod 1<br/>Deployment<br/>Service<br/>RBAC]
            HS2[MCPServer Pod 2<br/>Deployment<br/>Service<br/>RBAC]
        end

        subgraph "Remote MCPServers"
            RS1[External Service<br/>URL validated]
        end

        subgraph "LLM Workload"
            LLM[InferenceService Pod<br/>ENV: MCP_GATEWAY_URL<br/>http://gateway/routes/{namespace}/{route-name}]
        end
    end

    %% CRD Creation Flow
    U1 -->|1. Create| CRD1
    U1 -->|2a. Create Explicit| CRD2
    U1 -->|2b. Auto-create via| CRD3

    %% Controller Reconciliation
    CRD1 -->|watches| C1
    CRD2 -->|watches| C2
    CRD3 -->|watches| C3

    C1 -->|hosted: creates| HS1
    C1 -->|hosted: creates| HS2
    C1 -->|remote: validates| RS1

    C2 -->|validates backends<br/>updates status| CRD2
    C2 -->|registers route| GW4

    C3 -->|auto-creates| CRD2
    C3 -->|injects gateway URL| LLM
    C3 -->|creates LLM pods| LLM

    %% Gateway Discovery
    GW4 -.->|watches MCPRoute| CRD2
    GW4 -.->|discovers backends| HS1
    GW4 -.->|discovers backends| HS2
    GW4 -.->|discovers backends| RS1

    %% Runtime Request Flow
    LLM ==>|① MCP Request<br/>POST /routes/ns/route<br/>tool: db_query| GW
    GW ==>|② Authenticate| GW1
    GW1 ==>|③ Authorize| GW2
    GW2 ==>|④ Rate Limit| GW3
    GW3 ==>|⑤ Route Lookup| GW4
    GW4 ==>|⑥ Backend Select<br/>weighted/health| HS1
    GW4 -.->|fallback| HS2
    GW4 -.->|or remote| RS1
    HS1 ==>|⑦ Execute Tool<br/>via K8s RBAC| HS1
    HS1 ==>|⑧ Response| GW
    GW ==>|⑨ Return Result| LLM

    %% Policy Enforcement Points
    GW1 -.->|merge policies| CRD2
    GW2 -.->|merge policies| CRD2
    GW3 -.->|merge policies| CRD2

    %% Styling
    classDef userClass fill:#e1f5ff,stroke:#01579b,stroke-width:2px
    classDef crdClass fill:#fff9c4,stroke:#f57f17,stroke-width:2px
    classDef controllerClass fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef gatewayClass fill:#ffebee,stroke:#b71c1c,stroke-width:3px
    classDef workloadClass fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px
    classDef policyClass fill:#fce4ec,stroke:#880e4f,stroke-width:2px

    class U1,U2 userClass
    class CRD1,CRD2,CRD3 crdClass
    class C1,C2,C3 controllerClass
    class GW,GW4 gatewayClass
    class GW1,GW2,GW3 policyClass
    class HS1,HS2,RS1,LLM workloadClass
```

#### Component Interaction

**Application Team Workflow**:

1.  **Create MCPServer**: App team creates tool servers in their namespace:
    - For `hosted`: Controller creates Deployment, Service, RBAC based on `permissionProfile`
    - For `remote`: Controller validates URL accessibility
    - Server becomes available as backend for routing

2.  **Create MCPRoute** (explicit) OR **Use InferenceService** (auto-create):

    **Option A - Explicit MCPRoute**:
    - App team creates MCPRoute with backend MCPServer references
    - Configures traffic weights, tool matching, route-specific policies
    - MCPRoute Controller validates route and updates status with gateway URL

    **Option B - Auto-Create via InferenceService**:
    - App team specifies `mcpServers` in InferenceService spec
    - InferenceService Controller auto-creates MCPRoute resource
    - MCPRoute inherits config from InferenceService.mcpRoute

3.  **Gateway Auto-Discovery**:
    - For each route, gateway:
      - Merges gateway default policies with route-specific policies (more restrictive wins)
      - Configures routing rules to backend MCPServers
      - Updates internal routing table dynamically (no restart needed)

4.  **InferenceService Controller**: Creates LLM pods and injects gateway endpoints:
    - **Simple mode**: Injects `MCP_GATEWAY_URL` environment variable pointing to gateway
    - **Explicit mode**: Reads gateway URL from referenced MCPRoute status
    - LLM pods use this URL to make MCP tool calls

5.  **Runtime**: MCPServer workloads execute tool logic using granted permissions

#### Request Flow

**Model Request Flow**:

1.  **LLM → Gateway**:
    - LLM pod sends MCP request to `http://gateway.ome-system/routes/{namespace}/{route-name}`
    - Request includes tool name, method, and parameters

2.  **Gateway Policy Enforcement**:
    - **Authentication**: Gateway validates JWT/OIDC/API key (from operator defaults or MCPRoute override)
    - **Authorization**: Gateway checks if principal has permission to access requested tool
    - **Rate Limiting**: Gateway enforces limits from both gateway config and route (combined)

3.  **Route Selection**:
    - Gateway extracts namespace and route name from URL path
    - Looks up MCPRoute configuration from internal routing table

4.  **Backend Selection**:
    - Gateway evaluates traffic weights across backend MCPServers
    - Checks backend health status
    - Selects healthy backend based on weighted random or round-robin

5.  **Request Forwarding**:
    - Gateway forwards request to selected MCPServer endpoint
    - For `hosted` servers: `http://{server-name}.{namespace}.svc:8080`
    - For `remote` servers: External URL from MCPServer.remote.url

6.  **Tool Execution**:
    - MCPServer receives request and executes tool logic
    - Server may access K8s API (if granted via permissionProfile)
    - Server may access databases or external APIs (if permitted by network policies)

7.  **Response Path**:
    - MCPServer returns result to gateway
    - Gateway logs request for audit (tool name, user, latency, status)
    - Gateway returns response to LLM pod

8.  **Error Handling**:
    - If backend unhealthy: Gateway automatically retries with different backend
    - If all backends down: Gateway returns 503 Service Unavailable
    - If policy violation: Gateway returns 401/403 with details

### Security Model

The architecture provides a comprehensive security model with policy hierarchy, multi-tenancy RBAC, and workload isolation:

#### 1. Policy Hierarchy

Security policies are defined at two levels with automatic merging:

**Gateway-Level Default Policies** (MCPGateway):
- Managed by OME operator
- Apply to all routes using default gateway config
- Set baseline security requirements (e.g., "all traffic must be authenticated")

**Route-Level Policy Overrides** (MCPRoute):
- Managed by application teams
- Can add more restrictive policies on top of gateway defaults
- Cannot weaken gateway policies
- Configured via `authentication`, `authorization`, `rateLimit` in MCPRoute spec

**Policy Merging Rules**:
- **More restrictive policy wins**: If gateway requires JWT auth and route requires API key, both are enforced (AND logic)
- **Rate limits combine**: If gateway sets 1000 req/hour and route sets 100 req/hour, 100 req/hour wins (minimum)
- **Authorization combines**: Both gateway and route authorization rules must pass (AND logic)

**Policy Merge Algorithm**:

The gateway implements policy merging at request-time with the following precedence order:

1. **Authentication (AND Logic - All Must Pass)**:
   - Gateway authentication config (from operator defaults)
   - Route authentication config (from MCPRoute spec)
   - Implementation: Request must satisfy ALL configured auth methods in sequence
   - Example: Gateway JWT + Route OIDC = Client must present valid JWT AND complete OIDC flow
   - Validation: Webhook warns if route weakens authentication (not possible to disable gateway auth)

2. **Authorization (AND Logic - All Rules Must Pass)**:
   - Collect all authorization rules from gateway config
   - Collect all authorization rules from route config
   - Request must satisfy EVERY rule from combined set
   - Implementation: Gateway evaluates rules in order, short-circuits on first denial
   - Example: Gateway allows "group:developers" AND Route allows "group:finance-admins" = User must be in BOTH groups

3. **Rate Limiting (Minimum Wins - Most Restrictive)**:
   - For each dimension (user, IP, tool, principal, namespace):
     - Find all limits from gateway config
     - Find all limits from route config
     - Apply the LOWEST limit for that dimension
   - Implementation: Gateway maintains per-dimension counters, enforces strictest limit
   - Example: Gateway 1000 req/hour + Route 100 req/hour = Enforce 100 req/hour
   - Special case: If limits have different units, normalize to common unit (seconds) before comparison

4. **Conflict Resolution**:
   - Routes CANNOT weaken gateway policies (validation webhook enforces)
   - Routes CAN add additional restrictions
   - Routes CAN specify finer-grained controls (e.g., tool-specific rate limits)
   - Operator defaults are immutable at runtime (require operator restart to change)

5. **Enforcement Points**:
   - Gateway controller watches both operator ConfigMap and MCPRoute resources
   - On route creation/update: Merge policies and update internal policy cache
   - On request: Look up cached merged policy for the route
   - On policy violation: Return 401 (auth), 403 (authz), or 429 (rate limit) with specific error

**Example Policy Combination**:

Operator config:
```
defaultAuthentication:
  jwt:
    audiences: ["mcp-prod"]
    jwksURI: "https://auth.company.com/.well-known/jwks.json"
defaultRateLimit:
  limits:
  - dimension: namespace
    requests: 10000
    unit: hour
routeConstraints:
  requireAuthentication: true  # Enforce: all routes must have auth

```


```yaml
---
# Route override: Add stricter rate limit + authorization
apiVersion: ome.io/v1alpha1
kind: MCPRoute
metadata:
  name: sensitive-tools
  namespace: finance-team
spec:
  backendRefs:
  - name: payment-server
  # Inherits JWT auth from gateway, adds authorization
  authorization:
    rules:
    - principals: ["group:finance-admins"]
      tools: ["*"]
  # Stricter rate limit (100 wins over 10000)
  rateLimit:
    limits:
    - dimension: user
      requests: 100
      unit: hour
```

**Result**: Requests must pass JWT auth (gateway) AND group membership check (route) AND 100 req/hour limit (route override).

#### 2. Multi-Tenancy RBAC

The model supports clear role separation:

**Platform Team** (Cluster/Namespace Admin):
- Sets infrastructure defaults (replicas, resources, autoscaling)
- Enforces baseline policies via `defaultAuthentication`, `defaultAuthorization`, `defaultRateLimit`

**Application Team** (Namespace Developer):
- Creates `MCPServer`, `MCPRoute`, `InferenceService` in their namespace
- Can add more restrictive policies to routes (but not weaken gateway defaults)
- Cannot access servers in other namespaces (isolation enforced)
- RBAC: `get/list/watch/create/update/delete` on `MCPServer`, `MCPRoute`, `InferenceService` in own namespace

**Namespace Isolation**:
- `MCPRoute.spec.backendRefs` can only reference `MCPServer` resources in the same namespace
- Cross-namespace access requires duplicating servers (intentional security feature)

#### 3. Workload Permissions (`permissionProfile`)

MCPServers run with least-privilege permissions defined declaratively:

- **Kubernetes API Access**: `kubeResources` defines allowed API groups, resources, and verbs. Controller generates scoped `Role` and `RoleBinding`.
- **Network Access**: `network.allowHost` restricts outbound traffic to specific hosts
- **Audit Trail**: All permission grants are declarative and version-controlled in Git

Example:
```yaml
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: k8s-tools
  namespace: my-team
spec:
  permissionProfile:
    inline:
      allow:
      - kubeResources:
          apiGroups: [""]
          resources: ["pods", "services"]
          verbs: ["get", "list"]
          namespaces: ["my-team"]  # Restricted to own namespace
      - network:
          allowHost:
          - "api.company.com"
          - "*.internal.svc.cluster.local"
```

#### 4. Service Account Isolation

Each hosted MCPServer gets a dedicated ServiceAccount with only the permissions specified in `permissionProfile`. No shared credentials.

#### 5. Kubernetes Security Primitives

Users can leverage standard mechanisms via `PodTemplateSpec`:
- `SecurityContext` for pod/container security settings
- `NetworkPolicy` for network-level isolation
- `PodSecurityPolicy` or `PodSecurityStandards` for cluster-wide policies
- Resource limits and quotas

#### 6. Optional Service Mesh Integration

When service mesh (Istio, Linkerd) is available:
- Use service mesh `AuthorizationPolicy` for fine-grained mTLS-based access control
- Leverage distributed tracing and metrics

### Deployment Patterns

The architecture supports four deployment patterns ranging from simple direct access to advanced multi-tenant gateway routing.

#### Pattern 1: Traffic Splitting for Canary Deployment

**Scenario**: Gradual rollout of new tool version using weight-based traffic splitting via MCPRoute.

```yaml
# Deploy v1 (stable)
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: analytics-v1
  namespace: my-app
  labels:
    app: analytics
    version: v1
spec:
  transport: streamable-http
  hosted:
    replicas: 3
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/analytics:1.0.0

---
# Deploy v2 (canary)
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: analytics-v2
  namespace: my-app
  labels:
    app: analytics
    version: v2
spec:
  transport: streamable-http
  hosted:
    replicas: 1
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/analytics:2.0.0

---
# MCPRoute with traffic splitting (90% v1, 10% v2)
apiVersion: ome.io/v1alpha1
kind: MCPRoute
metadata:
  name: analytics-canary
  namespace: my-app
spec:
  backendRefs:
  - name: analytics-v1
    weight: 90
  - name: analytics-v2
    weight: 10
  # Monitor v2 with stricter limits initially
  rateLimit:
    limits:
    - dimension: user
      requests: 100
      unit: hour

---
# InferenceService referencing the route
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: analytics-llm
  namespace: my-app
spec:
  model:
    modelRef:
      name: llama-3-70b
  runtime:
    runtimeRef:
      name: vllm
  mcpRoute:
    routeRef:
      name: analytics-canary
```

**What OME does**:
- Gateway routes 90% of requests to v1, 10% to v2 based on MCPRoute weights
- Gradual adjustment: Edit MCPRoute to change weights (80/20 → 50/50 → 20/80 → 0/100)
- Easy rollback: Update MCPRoute to set v1 weight to 100 if v2 has issues

**Benefits**:
- Safe rollout with controlled traffic percentage
- Gradual validation with metrics monitoring
- Easy rollback by updating single MCPRoute resource
- No InferenceService changes needed during rollout

**Use when**: Deploying new tool versions, A/B testing, validating changes in production

**Rollout Process**:
1. Deploy v2 with MCPRoute weight=10
2. Monitor metrics (error rates, latency, success rates)
3. Gradually increase v2 weight via `kubectl edit mcproute analytics-canary`
4. Eventually scale down v1 and remove from backendRefs
5. If issues found, immediately update route to set v1 weight=100

#### Pattern 2: Tool-Based Routing with Multiple Servers

**Scenario**: Route different tool categories to specialized servers based on tool name patterns.

```yaml
# Deploy specialized servers
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: database-server
  namespace: my-app
spec:
  transport: streamable-http
  hosted:
    replicas: 3
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/db-tools:1.0.0

---
apiVersion: ome.io/v1alpha1
kind: MCPServer
metadata:
  name: k8s-server
  namespace: my-app
spec:
  transport: streamable-http
  hosted:
    replicas: 2
    podSpec:
      spec:
        containers:
        - name: mcp-server
          image: my-registry/k8s-tools:1.0.0
  permissionProfile:
    inline:
      allow:
      - kubeResources:
          apiGroups: [""]
          resources: ["pods", "services"]
          verbs: ["get", "list"]

---
# MCPRoute with tool-based matching
apiVersion: ome.io/v1alpha1
kind: MCPRoute
metadata:
  name: smart-routing
  namespace: my-app
spec:
  # Route database tools to database-server
  matches:
  - toolMatch:
      prefixMatch: "db_"
    backendRefs:
    - name: database-server
  # Route k8s tools to k8s-server
  - toolMatch:
      prefixMatch: "k8s_"
    backendRefs:
    - name: k8s-server
  # Default: Load balance across both
  backendRefs:
  - name: database-server
    weight: 50
  - name: k8s-server
    weight: 50

---
# InferenceService using smart routing
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: smart-llm
  namespace: my-app
spec:
  model:
    modelRef:
      name: llama-3-70b
  runtime:
    runtimeRef:
      name: vllm
  mcpRoute:
    routeRef:
      name: smart-routing
```

**What OME does**:
- Gateway inspects tool name in MCP request
- Routes `db_query`, `db_insert` → database-server
- Routes `k8s_get_pod`, `k8s_list_svc` → k8s-server
- Routes other tools → 50/50 load balance

**Benefits**:
- Intelligent routing based on tool semantics
- Specialized servers for different tool categories
- Fallback routing for unmatched tools

**Use when**: Multiple specialized tool servers, need intelligent routing, performance optimization

### Test Plan

#### Unit Tests

-   **`MCPServer` Controller**:
    -   Test reconciliation logic for `hosted` and `remote` servers
    -   Test correct generation of `Deployment`, `Service`, `Role`, and `RoleBinding` from a `hosted` spec
    -   Test validation of `permissionProfile` rules
    -   Test server status reporting (ready, error conditions)

-   **`MCPRoute` Controller**:
    -   Test route reconciliation with single and multiple backend references
    -   Test weight validation and normalization (weights sum validation)
    -   Test tool-based matching configuration (prefixMatch, exactMatch, regexMatch)
    -   Test policy validation (authentication, authorization, rateLimit)
    -   Test filter configuration (header modifications, transformations)
    -   Test backend reference validation (ensure backends exist in same namespace)
    -   Test route status reporting (accepted, backend not found, policy errors)

-   **`InferenceService` Controller (MCP Integration)**:
    -   Test auto-create MCPRoute mode: verify route creation from `mcpServers` list
    -   Test explicit MCPRoute reference mode: verify route reference validation
    -   Test mutual exclusion validation (mcpServers vs mcpRoute.routeRef)
    -   Test gateway URL injection based on namespace gateway config
    -   Test route ownership (InferenceService owns auto-created routes)
    -   Test route lifecycle (deletion when InferenceService deleted)

-   **Gateway Component**:
    -   Test MCPRoute auto-discovery (watch MCPRoute resources, update routing table)
    -   Test routing logic with single and multiple backends (health checks, failover)
    -   Test weight-based traffic splitting algorithm (verify distribution matches weights)
    -   Test tool-based routing (route by tool name patterns)
    -   Test policy enforcement:
        -   Route-level policy overrides (from MCPRoute)
        -   Policy merging rules (more restrictive wins)
    -   Test authentication enforcement (OIDC, JWT, API key)
    -   Test authorization rule evaluation (principals, tools, conditions)
    -   Test rate limiting with different dimensions (user, namespace, IP, tool)

-   **API Webhooks**:
    -   Test `MCPServer` validation (hosted vs remote mutual exclusion, permissionProfile validation)
    -   Test `MCPRoute` validation (backend references, weight ranges, policy configs)
    -   Test `InferenceService` validation (mcpServers vs mcpRoute.routeRef mutual exclusion)
    -   Test cross-resource validation (routes can only reference servers in same namespace)

#### Integration Tests

-   **Pattern 1: Platform Gateway with Route-Based Routing**:
    -   App team creates MCPServer, MCPRoute, InferenceService in namespace
    -   Verify gateway infrastructure is deployed
    -   Verify gateway auto-discovers MCPRoute in production namespace
    -   Verify InferenceService receives `MCP_GATEWAY_URL` pointing to production gateway
    -   Send MCP requests and verify routing through gateway to backend server
    -   Test policy merging: gateway default auth + route-level rate limit
    -   Verify other namespaces (without production label) do NOT use this gateway

-   **Pattern 2: Simple Auto-Created Route**:
    -   App team creates MCPServer and InferenceService with `mcpServers` list (no explicit MCPRoute)
    -   Verify InferenceService controller auto-creates MCPRoute resource
    -   Verify auto-created route references the correct backend server
    -   Verify gateway auto-discovers the auto-created route
    -   Send MCP requests and verify end-to-end routing
    -   Delete InferenceService and verify auto-created route is also deleted

-   **Pattern 3: Traffic Splitting for Canary Deployment**:
    -   Deploy two MCPServer versions (v1, v2)
    -   Create MCPRoute with weighted backends (90% v1, 10% v2)
    -   Create InferenceService referencing the MCPRoute
    -   Send 1000 requests and verify traffic distribution matches weights (90/10 ± 5%)
    -   Update MCPRoute weights to 50/50 and verify traffic redistribution
    -   Test rollback by updating MCPRoute to set v1 weight=100
    -   Verify no InferenceService changes needed during weight adjustments

-   **Pattern 4: Tool-Based Routing with Multiple Servers**:
    -   Deploy multiple specialized MCPServers (database-server, k8s-server)
    -   Create MCPRoute with tool-based matches (db_* → database-server, k8s_* → k8s-server)
    -   Send requests with different tool names and verify correct routing
    -   Test fallback routing for tools that don't match any pattern

-   **Policy Hierarchy Enforcement**:
    -   Create MCPRoute with authorization rules and stricter 100 req/hour rate limit
    -   Verify requests must pass both JWT auth (gateway) AND authorization check (route)
    -   Verify rate limiting enforces 100 req/hour (route override wins)
    -   Test policy constraint: set `requireAuthentication=true` in gateway config
    -   Attempt to create MCPRoute without authentication and verify it's rejected

-   **Multi-Tenancy RBAC**:
    -   Create app team role with access to MCPServer, MCPRoute in own namespace only
    -   Verify app team can create resources in own namespace
    -   Verify app team CANNOT access resources in other namespaces
    -   Verify app team CANNOT create MCPRoute referencing servers in different namespace

-   **Permissions**:
    -   Test `kubeResources` permission by deploying a tool that accesses allowed/denied K8s resources
    -   Test `network` permission by deploying a tool that connects to allowed/denied hosts
    -   Verify proper RBAC creation for hosted servers

-   **High Availability and Scaling**:
    -   Test backend server failover when one server pod becomes unhealthy
    -   Test MCPRoute update with changing backend references

-   **Edge Cases and Error Handling**:
    -   Create MCPRoute referencing non-existent MCPServer and verify route status shows error
    -   Create MCPRoute referencing MCPServer in different namespace and verify webhook blocks it
    -   Create InferenceService with both `mcpServers` and `mcpRoute.routeRef` and verify webhook blocks it
    -   Test gateway behavior when all backends are unhealthy (503 response)
    -   Test gateway behavior with invalid JWT tokens (401 response)
    -   Test gateway behavior when rate limit exceeded (429 response)

## Drawbacks

1.  **No Separate Policy CRDs**: Policies are embedded in MCPRoute specs rather than being separate, reusable Policy resources. This means:
    - Policies cannot be versioned and managed independently from routes
    - Same policy configuration may need to be duplicated across multiple routes
    - No policy composition (combining multiple policy libraries)
    - **Mitigation**: Operator managed MCPGateway provides default configs that routes inherit, reducing duplication. If >30% of users need reusable policies, v1beta1 can add separate Policy CRDs while maintaining backward compatibility.

2.  **Namespace Isolation**: MCPRoute can only reference MCPServer resources in the same namespace. Cross-namespace access requires duplicating servers.
    - Cannot share a single MCPServer across multiple namespaces
    - Platform teams must duplicate common tools in each namespace
    - **Rationale**: This is an intentional security feature, not a bug. Most organizations want namespace isolation. If >30% of users need cross-namespace sharing, v1beta1 can add `ReferenceGrant` similar to Gateway API.

3.  **Gateway as Internal Component**: Making the gateway an internal component rather than a user-facing CRD means:
    - Less flexibility for advanced gateway customization (custom listeners, TLS configs)
    - Cannot independently manage gateway lifecycle
    - Gateway deployment details abstracted (users cannot directly edit gateway pods)
    - **Mitigation**: If users need more control, v1beta1 can introduce MCPGatewayConfig CRD to provide control over replicas, resources, autoscaling, and policies.

4.  **No Cross-Namespace Route Sharing**: InferenceService can only reference MCPRoute in the same namespace.
    - Platform teams cannot create a single shared route for all namespaces
    - Route configuration must be duplicated per namespace
    - **Mitigation**: v1beta1 introduces MCPGateway Config that provides defaults. Routes inherit these defaults, minimizing duplication. Use auto-create mode to avoid managing routes entirely. 

5.  **Latency Overhead**: Gateway introduces extra network hop and policy evaluation overhead:
    - Estimated latency: 2-10ms per request (varies by policy complexity)
    - Authentication/authorization adds ~1-5ms per request
    - Rate limiting adds ~0.5-2ms per request
    - **Mitigation**: Use direct mode (skip gateway) for latency-sensitive workloads. Optimize gateway deployment with resource allocation and autoscaling.

6.  **Learning Curve (vs Fully Embedded)**: The model requires understanding 2 CRDs:
    - Engineer teams must learn the default configuration for operator-managed MCP gateway
    - App teams must learn MCPRoute (unless using auto-create mode)
    - More cognitive load compared to fully embedded policies
    - **Mitigation**: Provide clear documentation and examples. Auto-create mode hides MCPRoute complexity for simple use cases. Most app teams only need to understand MCPServer.

7.  **Increased Development Complexity**: Implementing requires more development effort:
    - 2 controllers (MCPServer, MCPRoute) vs 1 (embedded approach)
    - Gateway auto-discovery and policy merging logic
    - Estimated: ~50% more development effort than single-CRD approach
    - **Justification**: The additional effort is justified for medium-to-large deployments requiring multi-tenancy (see Evolution Path section for benefits analysis)

8.  **Evolution Uncertainty**: While the v1alpha1 → v1beta1 evolution path is designed, there's uncertainty about:
    - Whether the community will validate the need for separate Policy CRDs
    - Whether cross-namespace sharing becomes a validated requirement
    - Potential API changes if we add MCPGateway CRD in v1beta1
    - **Mitigation**: Clear versioning strategy (`v1alpha1` → `v1beta1` → `v1`). Backward compatibility commitment: v1alpha1 patterns will continue to work in all future versions.

## Evolution Path

This proposal takes a **balanced approach** to MCP support in OME. We start with v1alpha1 hybrid architecture that **provides multi-tenancy benefits at 50% of full Gateway API complexity**, then evolve to more advanced patterns only when validated needs emerge.

**Why** This estimate is based on analysis of multi-tenancy requirements:
- ✅ **Achieved (90%)**: Namespace isolation, policy hierarchy (gateway + route), RBAC separation, auto-discovery, embedded policies, traffic splitting, tool-based routing
- ✅ **Deferred to v1beta1 (10%)**: Cross-namespace sharing (ReferenceGrant), separate Policy CRDs, explicit Gateway lifecycle management
- **Complexity Reduction**: 2 CRDs (MCPServer, MCPRoute) vs 5 CRDs in full Gateway API (Gateway, GatewayClass, HTTPRoute, Policy×3), no parentRef attachment complexity, no cross-namespace ReferenceGrant

### v1alpha1

**API Version**: `ome.io/v1alpha1`

**Resources**:
- `MCPServer` (namespace-scoped) - Define tool servers (hosted or remote)
- `MCPRoute` (namespace-scoped) - Routing configuration with traffic splitting, tool matching, policies
- `InferenceService` extensions - Two integration modes (auto-create MCPRoute or reference existing)
- Managed Gateway (internal component, NOT a user-facing CRD)

**Key Characteristics**:
- **Multi-Tenancy Support**: app teams manage `MCPRoute` and `MCPServer`
- **Policy Hierarchy**: Gateway default policies + route-level overrides (more restrictive wins)
- **Auto-Discovery**: Gateway watches MCPRoute resources, no parentRef complexity
- **Embedded Policies**: Authentication, authorization, rate limiting inline (no separate Policy CRDs)
- **Flexible Integration**: Auto-create MCPRoute (simple) or reference existing MCPRoute (explicit)

**Comparison to Alternatives**:
- **vs Single CRD**: Adds MCPRoute for multi-tenancy support
- **vs Full Gateway API**: Removes MCPGateway CRD and 3 Policy CRDs, no parentRef complexity
- **Result**: Balanced tradeoff (see Evolution Path section for detailed justification)

**Target Users**:
- Platform teams managing shared infrastructure
- Application teams with 1-20 tool servers per namespace
- Organizations with moderate multi-tenancy needs


### v1beta1 (Conditional - Based on Validated Needs)

**When to Consider**: After 6-12 months of v1alpha1 usage, if we observe:
- Strong demand for **reusable policy resources** across many routes (50+ routes per cluster)
- Need for **policy versioning and lifecycle management** independent of routes
- Requests for **cross-namespace route sharing** (current design intentionally restricts to same namespace)
- Complex **policy composition** requirements (combining multiple policy libraries)

**Potential Changes** (only if validated):

**Option 1: Separate Policy CRDs**
```yaml
# Introduce reusable policy resources
apiVersion: ome.io/v1beta1
kind: MCPAuthenticationPolicy
metadata:
  name: company-jwt-auth
  namespace: platform-policies
spec:
  jwt:
    audiences: ["mcp-production"]
    jwksURI: "https://auth.company.com/.well-known/jwks.json"

---
# MCPRoute references policy instead of embedding
apiVersion: ome.io/v1beta1
kind: MCPRoute
metadata:
  name: my-route
spec:
  authenticationPolicyRef:
    name: company-jwt-auth
    namespace: platform-policies
  # Still supports embedded for simple cases
  authorization: {...}
```

**Option 2: Full Gateway API Alignment**
- Add `MCPGateway` CRD for explicit gateway lifecycle management
- Add `parentRef` to MCPRoute for explicit gateway attachment
- Enable cross-namespace `ReferenceGrant` for advanced use cases

**Migration Path**:
```yaml
apiVersion: ome.io/v1alpha1
kind: MCPRoute
metadata:
  name: my-route
spec:
  authentication: {...}  # Embedded
  backendRefs: [...]

# v1beta1 (if separate policies adopted)
apiVersion: ome.io/v1beta1
kind: MCPRoute
metadata:
  name: my-route
spec:
  authenticationPolicyRef: {...}  # Referenced
  # OR authentication: {...}  # Embedded (backward compatible)
  backendRefs: [...]
```

**Key Principles**:
- **Gradual Adoption**: Users opt into Policy CRDs when complexity justifies

### v1 (Stable API)

**When**: After v1alpha1 or v1beta1 proves stable (6-12 months of production usage)

**Criteria for Stability**:
- API tested in production by 10+ organizations
- No major design changes needed for 6+ months
- Comprehensive test coverage (>80%) and documentation
- Clear migration paths from alpha/beta versions
- Performance validated at scale (100+ MCPServers, 1000+ req/sec)

**Graduation Paths**:
- **Direct Path**: `v1alpha1 hybrid` → `v1` (if v1beta1 changes not needed)
- **Iterative Path**: `v1alpha1 hybrid` → `v1beta1 hybrid+` → `v1`

### Decision Points

At each evolution stage, we evaluate:

1. **User Feedback**: What are real users actually requesting? (surveys, GitHub issues, support tickets)
2. **Complexity Justification**: Does the additional API surface justify the benefits? (80/20 rule)
3. **Ecosystem Maturity**: How has the MCP ecosystem evolved? (new transports, protocol changes)
4. **Operational Experience**: What patterns emerge from production deployments? (top 5 pain points)

**Example Decision Framework**:
- If <10% of users need reusable policies → Keep embedded policies in v1
- If 10-30% need reusable policies → Add optional Policy CRDs in v1beta1
- If >30% need reusable policies → Make Policy CRDs primary pattern in v1

### Feature Roadmap (Conditional)

Future enhancements to consider **only if validated needs emerge**:

**Cluster-Scoped Resources** (v1beta1+):
- `ClusterMCPServer` - Shared tool servers across namespaces (avoid duplication)
- `ClusterMCPRoute` - Global routing rules managed by platform team
- `ClusterMCPGateway` - Centralized gateway for entire cluster
- **Benefits**: Reduced duplication, centralized governance, resource efficiency
- **When**: >50% of MCPServers are identical across namespaces

**Advanced Routing Features** (v1beta1+):
- Header-based routing (route by custom headers, user groups)
- Time-based routing (different backends for peak/off-peak hours)
- Geographic routing (route to nearest server based on client location)
- **When**: >20% of routes need advanced matching beyond tool names

**Advanced Gateway Features** (v1+):
- Circuit breaking and retry policies (resilience patterns)
- Request/response transformation (protocol translation, enrichment)
- Tool aggregation and composition (combine multiple tools into one)
- Caching layer with configurable TTL (reduce backend load)
- **When**: >30% of deployments request these specific features

**Enterprise Features** (v1+):
- Multi-cluster federation (route across clusters based on load, cost)
- Cost tracking and quota management (chargeback, budget enforcement)
- Progressive delivery with Flagger integration (automated canary with metrics)
- Advanced observability and audit logging (SIEM integration, compliance)
- **When**: >50% of enterprise deployments require these capabilities

**Cross-Namespace Sharing** (v1beta1+):
- `ReferenceGrant` for cross-namespace MCPServer access (similar to Gateway API)
- Controlled sharing with explicit grants (security by default)
- **When**: >30% of users duplicate servers across namespaces

**Guiding Principles**:
- **Validated Demand**: Each feature needs ≥3 customer requests with specific use cases
- **Complexity Budget**: Total CRDs + fields ≤ 2x v1alpha1 (maintain simplicity advantage)
- **User Choice**: Advanced features always optional, never required
- **Evolution > Revolution**: Iterate based on learning, not speculation

## Open Questions

This section documents unresolved design questions and areas requiring further investigation or community feedback before implementation.

### 1. Remote MCPServer Authentication

**Question**: How should remote MCPServers authenticate to external services they depend on?

**Context**: The current design specifies `RemoteMCPServer.url` for external services, but doesn't address how the remote server authenticates to its own dependencies (databases, APIs, etc.).

**Options**:
- **Option A**: Remote servers handle their own authentication (out of scope for OME). User deploys remote server with credentials externally.
- **Option B**: Add optional `authConfig` to `RemoteMCPServer` with support for common patterns (basic auth, OAuth2, mTLS). OME controller validates connectivity with auth.
- **Option C**: Use Kubernetes External Secrets or similar for credential injection into remote server URLs (e.g., `https://${SECRET_TOKEN}@api.example.com`)

**Recommendation**: Start with Option A (out of scope) for v1alpha1. If >30% of users request remote server credential management, add Option B in v1beta1.

### 2. MCPServer and MCPRoute Status Fields

**Question**: What status information should be exposed in CRD status subresources?

**Current Design**:
- `MCPServerStatus`: Basic ready condition, network enforcement mode
- `MCPRouteStatus`: Gateway URL, backend health, ready condition

**Additional Status Fields to Consider**:
- **MCPServer**:
  - `discoveredTools`: List of tools discovered from server introspection (MCP `tools/list`)
  - `capabilities`: Actual capabilities reported by server vs. declared in spec
  - `serverVersion`: Runtime version of MCP server software
  - `lastHealthCheck`: Timestamp of last successful health check
  - `errorMessage`: Detailed error for debugging failed servers

- **MCPRoute**:
  - `activeBackends`: Count of healthy backends currently serving traffic
  - `routingRules`: Summary of compiled routing rules (for debugging)
  - `policyStatus`: Which policies are active (gateway defaults + route overrides)
  - `requestMetrics`: Basic request count, error rate (last 5 minutes)

**Recommendation**: Start minimal in v1alpha1 (ready condition, gateway URL, basic error messages). Add observability-focused status fields in v1beta1 based on user debugging needs.

### 3. Gateway API Resource Reuse

**Question**: Should we reuse existing Kubernetes Gateway API resources (HTTPRoute, Gateway) instead of defining custom MCPRoute/MCPServer CRDs?

**Context**: Kubernetes Gateway API is now GA (v1.0.0). It provides standardized traffic routing with broad ecosystem support.

**Pros of Reusing Gateway API**:
- Standard Kubernetes pattern (no custom learning curve)
- Works with existing Gateway implementations (Istio, Envoy Gateway, nginx)
- Ecosystem tooling (kubectl plugins, dashboards) already exists
- Cross-team familiarity (SREs already know Gateway API)

**Cons of Reusing Gateway API**:
- HTTPRoute doesn't natively support tool-based routing (requires custom HTTPRoute filters)
- No built-in support for MCP-specific policies (authentication, authorization, rate limiting)
- MCPServer concept (hosted vs remote) doesn't map cleanly to Gateway API backends
- Need to bridge MCP-specific concepts (tools, permissions) into generic HTTP routing
- Requires Gateway controller installation (additional cluster dependency)

**Hybrid Option**: Use Gateway API for routing infrastructure, add MCP-specific CRDs for server definition and policy.

**Recommendation**: Defer to v1beta1 or v2. Start with custom MCPRoute/MCPServer in v1alpha1 to validate MCP-specific patterns (tool routing, permission profiles). If >50% of users request Gateway API alignment, consider migration in v1beta1.

### 4. Multi-Cluster MCP Server Discovery

**Question**: How should MCPServers be discovered and routed across multiple Kubernetes clusters?

**Context**: Large enterprises may want to share tool servers across clusters (cost efficiency, centralized governance).

**Options**:
- **Option A**: Single-cluster only in v1alpha1. Multi-cluster is future enhancement (v1beta1+).
- **Option B**: Support remote MCPServers pointing to servers in other clusters (URL-based, no native discovery).
- **Option C**: Integrate with service mesh federation (Istio multi-cluster, Linkerd) for transparent cross-cluster routing.
- **Option D**: Add ClusterMCPServer (cluster-scoped) with cross-cluster service discovery via DNS or custom controller.

**Recommendation**: Start with Option A (single-cluster) in v1alpha1. Add Option B (remote URLs to other clusters) if needed. Evaluate Option C/D for v1beta1 based on user demand for transparent multi-cluster routing.

### 5. Tool Server Versioning and Compatibility

**Question**: How should OME handle versioning of MCP tool servers and ensure compatibility between LLMs and tool APIs?

**Context**: Tool servers may evolve over time with breaking changes to tool signatures or behavior.

**Considerations**:
- Should MCPServer spec include version constraints (e.g., `minMCPVersion: 2024-11-05`)?
- How to handle deprecated tools during canary deployments (route old traffic to v1, new traffic to v2)?
- Should gateway validate tool call compatibility at runtime (reject calls to removed tools)?
- How to communicate tool schema changes to LLM applications?

**Recommendation**: Start simple in v1alpha1 (no version enforcement, rely on server versioning). Add versioning metadata and compatibility checks in v1beta1 if protocol evolution creates breaking changes.

### 6. Gateway Observability and Debugging

**Question**: What observability features are critical for debugging MCP gateway routing issues?

**Options**:
- **Request tracing**: Distributed tracing with OpenTelemetry (trace ID across LLM → Gateway → MCPServer)
- **Request logs**: Structured logs with tool name, user, latency, backend selection
- **Metrics**: Prometheus metrics (request rate, error rate, p99 latency per tool/route/backend)
- **Debug mode**: Optional verbose logging for specific routes (e.g., annotation `mcp.ome.io/debug: "true"`)
- **Traffic shadowing**: Route requests to both v1 and v2, compare results (for canary validation)

**Recommendation**: Implement basic request logging and Prometheus metrics in v1alpha1. Add distributed tracing and debug mode in v1beta1 based on user debugging experience.

### 7. Policy Validation and Testing

**Question**: How should users validate that gateway policies (authentication, authorization, rate limiting) are correctly configured before production deployment?

**Options**:
- **Dry-run mode**: Annotation on MCPRoute to log policy decisions without enforcement (e.g., `mcp.ome.io/policy-dry-run: "true"`)
- **Policy simulator**: CLI tool or API endpoint to test policy evaluation for hypothetical requests
- **Integration test framework**: Provide test utilities for users to validate policies in CI/CD pipelines
- **Admission webhook warnings**: Webhook warns about common misconfigurations (e.g., overly permissive rules)

**Recommendation**: Start with admission webhook warnings in v1alpha1. Add dry-run mode in v1beta1. Provide policy testing utilities as documentation/examples.

### 8. Cost Tracking and Quota Management

**Question**: Should OME provide built-in support for tracking MCP tool usage costs and enforcing quotas?

**Context**: Tool calls may have associated costs (API calls, database queries, LLM inference). Enterprises want cost attribution and budgets.

**Options**:
- **Option A**: Out of scope. Users implement cost tracking externally via gateway logs/metrics.
- **Option B**: Add cost metadata to MCPServer spec (e.g., `costPerRequest: 0.001`). Gateway tracks cumulative cost in metrics.
- **Option C**: Integrate with Kubernetes ResourceQuota or custom quota CRDs. Enforce cost limits at gateway level (reject requests over budget).
- **Option D**: Add cost reporting API (query usage by namespace, user, tool). Integrate with billing systems.

**Recommendation**: Start with Option A (out of scope) in v1alpha1. If >30% of users request cost tracking, add Option B (cost metadata + metrics) in v1beta1.

### 9. Security Considerations for Remote MCPServers

**Question**: What security controls should OME enforce for remote MCPServers accessed via URL?

**Context**: Remote servers are external services. They may be untrusted or compromised.

**Considerations**:
- **TLS verification**: Should controller validate TLS certificates for remote URLs? Allow self-signed certs for dev?
- **Network egress control**: Should remote servers be subject to NetworkPermission restrictions (even though they're external)?
- **Allowlist enforcement**: Should controller enforce URL allowlist (e.g., only `*.company.com` allowed)?
- **Mutual TLS**: Should OME support mTLS for authenticating gateway to remote servers?
- **Request signing**: Should gateway sign requests to remote servers (verify authenticity)?

**Recommendation**: Require HTTPS for remote servers in v1alpha1. Add TLS verification and optional mTLS in v1beta1. Consider request signing if user feedback indicates trust concerns with remote servers.

### 10. Graceful Degradation and Failover

**Question**: How should the gateway handle backend failures? Should there be built-in fallback strategies?

**Context**: MCPServers may become unhealthy during deployments or outages.

**Options**:
- **Fail fast**: Return 503 immediately if all backends unhealthy (current design).
- **Degraded mode**: Return cached results for read-only tools, fail for write tools.
- **Fallback backends**: Allow MCPRoute to specify fallback servers (different namespace, remote URL).
- **Circuit breaker**: Temporarily stop routing to unhealthy backends, retry after cooldown period.
- **Tool-level failover**: If primary server fails for specific tool, route to alternative server that implements the same tool.

**Recommendation**: Start with fail-fast in v1alpha1. Add circuit breaker logic in v1beta1. Consider tool-level failover in v1 if users report reliability issues.