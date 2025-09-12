package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPServerStatus defines the observed state of MCPServer
type MCPServerStatus struct {
	// Conditions represent the latest available observations of the MCPServer's state
	// Standard condition types: Ready, Available, Progressing
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL is the URL where the MCP server can be accessed
	// For Hosted servers, this is the cluster-internal or external service URL
	// For Remote servers, this reflects the configured external URL
	// +optional
	URL string `json:"url,omitempty"`

	// Phase is the current phase of the MCPServer lifecycle
	// +optional
	Phase MCPServerPhase `json:"phase,omitempty"`

	// Message provides additional information about the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the most recently observed number of replicas for hosted servers
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas for hosted servers
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// MCPServerPhase is the phase of the MCPServer
// +kubebuilder:validation:Enum=Pending;Starting;Running;Updating;Failed;Terminating
type MCPServerPhase string

const (
	// MCPServerPhasePending means the MCPServer is being created
	MCPServerPhasePending MCPServerPhase = "Pending"

	// MCPServerPhaseStarting means the MCPServer is starting up
	MCPServerPhaseStarting MCPServerPhase = "Starting"

	// MCPServerPhaseRunning means the MCPServer is running and ready
	MCPServerPhaseRunning MCPServerPhase = "Running"

	// MCPServerPhaseUpdating means the MCPServer is being updated
	MCPServerPhaseUpdating MCPServerPhase = "Updating"

	// MCPServerPhaseFailed means the MCPServer failed to start or run
	MCPServerPhaseFailed MCPServerPhase = "Failed"

	// MCPServerPhaseTerminating means the MCPServer is being deleted
	MCPServerPhaseTerminating MCPServerPhase = "Terminating"
)

// MCPGatewayStatus defines the observed state of MCPGateway.
type MCPGatewayStatus struct {
	// Conditions represent the latest available observations of the MCPGateway's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase is the current phase of the MCPGateway lifecycle.
	// +optional
	Phase MCPGatewayPhase `json:"phase,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available replicas.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ActiveSessions is the current number of active sessions.
	// +optional
	ActiveSessions int32 `json:"activeSessions,omitempty"`

	// TotalRequests is the total number of requests processed.
	// +optional
	TotalRequests int64 `json:"totalRequests,omitempty"`

	// TotalErrors is the total number of errors encountered.
	// +optional
	TotalErrors int64 `json:"totalErrors,omitempty"`

	// AverageLatency is the average request latency in milliseconds.
	// +optional
	AverageLatency int32 `json:"averageLatency,omitempty"`

	// ConnectedServers is the list of currently connected servers.
	// +optional
	// +listType=map
	// +listMapKey=name
	ConnectedServers []MCPGatewayServerStatus `json:"connectedServers,omitempty"`

	// ServerStatusSummary provides an aggregated summary of server statuses.
	// +optional
	ServerStatusSummary *ServerStatusSummary `json:"serverStatusSummary,omitempty"`

	// UnhealthyServers lists servers that are currently unhealthy.
	// +optional
	// +listType=map
	// +listMapKey=name
	UnhealthyServers []MCPGatewayServerStatus `json:"unhealthyServers,omitempty"`

	// ServiceURL is the URL where the gateway service can be accessed.
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// LastUpdateTime is the last time the status was updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Metrics provides real-time performance metrics.
	// +optional
	Metrics *GatewayMetrics `json:"metrics,omitempty"`

	// ConfigStatus provides configuration validation status.
	// +optional
	ConfigStatus *ConfigValidationStatus `json:"configStatus,omitempty"`

	// ResourceUsage provides current resource utilization information.
	// +optional
	ResourceUsage *ResourceUsageStatus `json:"resourceUsage,omitempty"`

	// ToolRegistry provides status of the tool registry.
	// +optional
	ToolRegistry *ToolRegistryStatus `json:"toolRegistry,omitempty"`

	// ResourceRegistry provides status of the resource registry.
	// +optional
	ResourceRegistry *ResourceRegistryStatus `json:"resourceRegistry,omitempty"`

	// PromptRegistry provides status of the prompt registry.
	// +optional
	PromptRegistry *PromptRegistryStatus `json:"promptRegistry,omitempty"`

	// WorkflowStatus provides status of workflow orchestration.
	// +optional
	WorkflowStatus *WorkflowStatus `json:"workflowStatus,omitempty"`
}

// MCPGatewayPhase is the phase of the MCPGateway.
// +kubebuilder:validation:Enum=Pending;Starting;Running;Updating;Failed;Terminating
type MCPGatewayPhase string

const (
	// MCPGatewayPhasePending means the MCPGateway is being created.
	MCPGatewayPhasePending MCPGatewayPhase = "Pending"

	// MCPGatewayPhaseStarting means the MCPGateway is starting up.
	MCPGatewayPhaseStarting MCPGatewayPhase = "Starting"

	// MCPGatewayPhaseRunning means the MCPGateway is running and ready.
	MCPGatewayPhaseRunning MCPGatewayPhase = "Running"

	// MCPGatewayPhaseUpdating means the MCPGateway is being updated.
	MCPGatewayPhaseUpdating MCPGatewayPhase = "Updating"

	// MCPGatewayPhaseFailed means the MCPGateway failed to start or run.
	MCPGatewayPhaseFailed MCPGatewayPhase = "Failed"

	// MCPGatewayPhaseTerminating means the MCPGateway is being deleted.
	MCPGatewayPhaseTerminating MCPGatewayPhase = "Terminating"
)

// MCPGatewayServerStatus defines the status of a connected MCP server.
type MCPGatewayServerStatus struct {
	// Name is the name of the MCP server.
	Name string `json:"name"`

	// Namespace is the namespace of the MCP server.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// State is the current connection state.
	// +kubebuilder:validation:Enum=Connected;Disconnected;Connecting;Unhealthy;CircuitOpen
	State string `json:"state"`

	// LastConnected is the timestamp of the last successful connection.
	// +optional
	LastConnected *metav1.Time `json:"lastConnected,omitempty"`

	// LastError contains the last error encountered.
	// +optional
	LastError string `json:"lastError,omitempty"`

	// RequestCount is the total number of requests sent to this server.
	// +optional
	RequestCount int64 `json:"requestCount,omitempty"`

	// ErrorCount is the total number of errors from this server.
	// +optional
	ErrorCount int64 `json:"errorCount,omitempty"`

	// AverageResponseTime is the average response time in milliseconds.
	// +optional
	AverageResponseTime int32 `json:"averageResponseTime,omitempty"`

	// CircuitBreakerState is the current circuit breaker state.
	// +kubebuilder:validation:Enum=Closed;Open;HalfOpen
	// +optional
	CircuitBreakerState string `json:"circuitBreakerState,omitempty"`

	// Weight is the current effective weight for load balancing.
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Tags are the current tags associated with this server.
	// +optional
	// +listType=set
	Tags []string `json:"tags,omitempty"`

	// Capabilities are the server's current capabilities.
	// +optional
	Capabilities *MCPCapabilities `json:"capabilities,omitempty"`
}

// ServerStatusSummary provides an aggregated summary of server statuses.
type ServerStatusSummary struct {
	// Total number of configured servers.
	Total int32 `json:"total"`

	// Number of servers that are connected and healthy.
	Connected int32 `json:"connected"`

	// Number of servers that are disconnected.
	Disconnected int32 `json:"disconnected"`

	// Number of servers that are unhealthy.
	Unhealthy int32 `json:"unhealthy"`

	// Number of servers with open circuit breakers.
	CircuitOpen int32 `json:"circuitOpen"`
}

// GatewayMetrics provides real-time performance metrics for the gateway.
type GatewayMetrics struct {
	// RequestsPerSecond is the current requests per second rate.
	// +optional
	RequestsPerSecond string `json:"requestsPerSecond,omitempty"`

	// P50Latency is the 50th percentile latency in milliseconds.
	// +optional
	P50Latency int32 `json:"p50Latency,omitempty"`

	// P90Latency is the 99th percentile latency in milliseconds.
	// +optional
	P90Latency int32 `json:"p90Latency,omitempty"`

	// ErrorRate is the current error rate as a percentage.
	// +optional
	ErrorRate string `json:"errorRate,omitempty"`

	// ActiveConnections is the number of currently active connections.
	// +optional
	ActiveConnections int32 `json:"activeConnections,omitempty"`

	// CacheHitRate is the cache hit rate as a percentage.
	// +optional
	CacheHitRate string `json:"cacheHitRate,omitempty"`

	// ToolInvocations tracks tool invocation statistics.
	// +optional
	// +mapType=atomic
	ToolInvocations map[string]int64 `json:"toolInvocations,omitempty"`
}

// ConfigValidationStatus provides configuration validation status.
type ConfigValidationStatus struct {
	// Valid indicates whether the current configuration is valid.
	Valid bool `json:"valid"`

	// ValidationErrors contains any configuration validation errors.
	// +optional
	// +listType=atomic
	ValidationErrors []ConfigValidationError `json:"validationErrors,omitempty"`

	// Warnings contains non-blocking configuration warnings.
	// +optional
	// +listType=atomic
	Warnings []string `json:"warnings,omitempty"`

	// LastValidated is when the configuration was last validated.
	// +optional
	LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}

// ConfigValidationError represents a configuration validation error.
type ConfigValidationError struct {
	// Field is the configuration field that failed validation.
	Field string `json:"field"`

	// Message describes the validation error.
	Message string `json:"message"`

	// Severity indicates the error severity.
	// +kubebuilder:validation:Enum=Error;Warning
	Severity ValidationSeverity `json:"severity"`
}

// ValidationSeverity defines validation error severities.
type ValidationSeverity string

const (
	ValidationSeverityError   ValidationSeverity = "Error"
	ValidationSeverityWarning ValidationSeverity = "Warning"
)

// ResourceUsageStatus provides current resource utilization information.
type ResourceUsageStatus struct {
	// CPU usage as a percentage of allocated resources.
	// +optional
	CPUUsage string `json:"cpuUsage,omitempty"`

	// Memory usage as a percentage of allocated resources.
	// +optional
	MemoryUsage string `json:"memoryUsage,omitempty"`

	// Storage usage for context and cache storage.
	// +optional
	StorageUsage *StorageUsageInfo `json:"storageUsage,omitempty"`

	// Network usage statistics.
	// +optional
	NetworkUsage *NetworkUsageInfo `json:"networkUsage,omitempty"`
}

// StorageUsageInfo provides storage usage information.
type StorageUsageInfo struct {
	// ContextStorage usage information.
	// +optional
	ContextStorage *StorageMetrics `json:"contextStorage,omitempty"`

	// CacheStorage usage information.
	// +optional
	CacheStorage *StorageMetrics `json:"cacheStorage,omitempty"`
}

// StorageMetrics provides storage metrics.
type StorageMetrics struct {
	// Used storage in bytes.
	Used resource.Quantity `json:"used"`

	// Available storage in bytes.
	Available resource.Quantity `json:"available"`

	// Usage percentage.
	UsagePercent string `json:"usagePercent"`
}

// NetworkUsageInfo provides network usage information.
type NetworkUsageInfo struct {
	// BytesIn is the total bytes received.
	BytesIn int64 `json:"bytesIn"`

	// BytesOut is the total bytes sent.
	BytesOut int64 `json:"bytesOut"`

	// ConnectionsPerSecond is the rate of new connections.
	ConnectionsPerSecond string `json:"connectionsPerSecond"`
}

// ToolRegistryStatus provides status of the tool registry.
type ToolRegistryStatus struct {
	// TotalTools is the total number of registered tools.
	// +optional
	TotalTools int32 `json:"totalTools,omitempty"`

	// MCPTools is the number of native MCP tools.
	// +optional
	MCPTools int32 `json:"mcpTools,omitempty"`

	// RestTools is the number of virtualized REST API tools.
	// +optional
	RestTools int32 `json:"restTools,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// ResourceRegistryStatus provides status of the resource registry.
type ResourceRegistryStatus struct {
	// TotalResources is the total number of registered resources.
	// +optional
	TotalResources int32 `json:"totalResources,omitempty"`

	// CacheHitRate is the resource cache hit rate as a percentage.
	// +optional
	CacheHitRate string `json:"cacheHitRate,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// PromptRegistryStatus provides status of the prompt registry.
type PromptRegistryStatus struct {
	// TotalPrompts is the total number of registered prompts.
	// +optional
	TotalPrompts int32 `json:"totalPrompts,omitempty"`

	// LastUpdated is when the registry was last updated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// WorkflowStatus provides status of workflow orchestration.
type WorkflowStatus struct {
	// ActiveWorkflows is the number of currently active workflows.
	// +optional
	ActiveWorkflows int32 `json:"activeWorkflows,omitempty"`

	// CompletedWorkflows is the total number of completed workflows.
	// +optional
	CompletedWorkflows int64 `json:"completedWorkflows,omitempty"`

	// FailedWorkflows is the total number of failed workflows.
	// +optional
	FailedWorkflows int64 `json:"failedWorkflows,omitempty"`

	// AverageExecutionTime is the average workflow execution time in seconds.
	// +optional
	AverageExecutionTime string `json:"averageExecutionTime,omitempty"`
}
