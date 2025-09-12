package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GatewayPolicyConfig defines unified security, authentication, authorization, and traffic policies.
type MCPGatewayPolicyConfig struct {
	// Authentication defines client authentication configuration.
	// +optional
	Authentication *MCPAuthenticationConfig `json:"authentication,omitempty"`

	// RateLimit defines rate limiting configuration.
	// +optional
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"`

	// CircuitBreaker defines the circuit breaking configuration.
	// +optional
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`

	// Audit defines audit logging configuration.
	// +optional
	Audit *AuditConfig `json:"audit,omitempty"`

	// RequestFiltering defines request filtering policies.
	// +optional
	RequestFiltering *RequestFilteringConfig `json:"requestFiltering,omitempty"`

	// ResponseFiltering defines response filtering policies.
	// +optional
	ResponseFiltering *ResponseFilteringConfig `json:"responseFiltering,omitempty"`

	// Compliance defines compliance-related policies.
	// +optional
	Compliance *ComplianceConfig `json:"compliance,omitempty"`
}

// MCPAuthenticationConfig defines simplified client authentication configuration.
type MCPAuthenticationConfig struct {
	// Enabled controls whether authentication is required.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Methods define the supported authentication methods in order of preference.
	// +optional
	// +listType=atomic
	Methods []AuthConfig `json:"methods,omitempty"`

	// Default provides the default authentication method when none is specified.
	// +optional
	Default *AuthConfig `json:"default,omitempty"`
}

// RateLimitConfig defines rate limiting policies.
type RateLimitConfig struct {
	// Global rate limit applied to all requests.
	// +optional
	Global *RateLimitPolicy `json:"global,omitempty"`

	// PerUser defines rate limits per authenticated user.
	// +optional
	PerUser *RateLimitPolicy `json:"perUser,omitempty"`

	// PerIP defines rate limits per client IP address.
	// +optional
	PerIP *RateLimitPolicy `json:"perIP,omitempty"`

	// PerServer defines rate limits per upstream server.
	// +optional
	PerServer *RateLimitPolicy `json:"perServer,omitempty"`
}

// RateLimitPolicy defines a rate limiting policy.
type RateLimitPolicy struct {
	// RequestsPerSecond is the number of requests allowed per second.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerSecond *int32 `json:"requestsPerSecond,omitempty"`

	// RequestsPerMinute is the number of requests allowed per minute.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerMinute *int32 `json:"requestsPerMinute,omitempty"`

	// RequestsPerHour is the number of requests allowed per hour.
	// +kubebuilder:validation:Minimum=1
	// +optional
	RequestsPerHour *int32 `json:"requestsPerHour,omitempty"`

	// Burst is the burst capacity for rate limiting.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Burst *int32 `json:"burst,omitempty"`
}

// AuditConfig defines audit logging configuration.
type AuditConfig struct {
	// Enabled controls whether audit logging is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Level defines the audit logging level.
	// +kubebuilder:validation:Enum=None;Request;Response;Full
	// +kubebuilder:default=Request
	// +optional
	Level AuditLevel `json:"level,omitempty"`

	// Destination defines where audit logs are sent.
	// +optional
	Destination *AuditDestinationConfig `json:"destination,omitempty"`

	// Format defines the audit log format.
	// +kubebuilder:validation:Enum=JSON;CEF;SYSLOG
	// +kubebuilder:default=JSON
	// +optional
	Format AuditFormat `json:"format,omitempty"`

	// IncludeMetadata controls whether to include request metadata in audit logs.
	// +kubebuilder:default=true
	// +optional
	IncludeMetadata *bool `json:"includeMetadata,omitempty"`
}

// AuditLevel defines audit logging levels.
type AuditLevel string

const (
	AuditLevelNone     AuditLevel = "None"
	AuditLevelRequest  AuditLevel = "Request"
	AuditLevelResponse AuditLevel = "Response"
	AuditLevelFull     AuditLevel = "Full"
)

// AuditFormat defines audit log formats.
type AuditFormat string

const (
	AuditFormatJSON   AuditFormat = "JSON"
	AuditFormatCEF    AuditFormat = "CEF"
	AuditFormatSyslog AuditFormat = "SYSLOG"
)

// AuditDestinationConfig defines audit log destinations.
type AuditDestinationConfig struct {
	// Type defines the destination type.
	// +kubebuilder:validation:Enum=File;Syslog;HTTP;S3;Database
	// +kubebuilder:default=File
	// +optional
	Type AuditDestinationType `json:"type,omitempty"`

	// File defines file-based audit logging.
	// +optional
	File *AuditFileConfig `json:"file,omitempty"`

	// HTTP defines HTTP-based audit logging.
	// +optional
	HTTP *AuditHTTPConfig `json:"http,omitempty"`

	// Syslog defines syslog-based audit logging.
	// +optional
	Syslog *AuditSyslogConfig `json:"syslog,omitempty"`
}

// AuditDestinationType defines audit destination types.
type AuditDestinationType string

const (
	AuditDestinationTypeFile     AuditDestinationType = "File"
	AuditDestinationTypeSyslog   AuditDestinationType = "Syslog"
	AuditDestinationTypeHTTP     AuditDestinationType = "HTTP"
	AuditDestinationTypeS3       AuditDestinationType = "S3"
	AuditDestinationTypeDatabase AuditDestinationType = "Database"
)

// AuditFileConfig defines file-based audit logging.
type AuditFileConfig struct {
	// Path defines the audit log file path.
	// +kubebuilder:default="/var/log/mcpgateway/audit.log"
	// +optional
	Path string `json:"path,omitempty"`

	// MaxSize defines the maximum log file size before rotation.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MaxBackups defines the maximum number of backup files to keep.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxBackups *int32 `json:"maxBackups,omitempty"`
}

// AuditHTTPConfig defines HTTP-based audit logging.
type AuditHTTPConfig struct {
	// Endpoint defines the HTTP endpoint for audit logs.
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// Method defines the HTTP method to use.
	// +kubebuilder:validation:Enum=POST;PUT
	// +kubebuilder:default=POST
	// +optional
	Method string `json:"method,omitempty"`

	// Headers define additional HTTP headers.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`

	// Timeout defines the HTTP request timeout.
	// +kubebuilder:default="30s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// AuditSyslogConfig defines syslog-based audit logging.
type AuditSyslogConfig struct {
	// Server defines the syslog server address.
	// +kubebuilder:validation:Required
	Server string `json:"server"`

	// Protocol defines the syslog protocol.
	// +kubebuilder:validation:Enum=UDP;TCP;TLS
	// +kubebuilder:default=UDP
	// +optional
	Protocol SyslogProtocol `json:"protocol,omitempty"`

	// Facility defines the syslog facility.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=23
	// +kubebuilder:default=16
	// +optional
	Facility *int32 `json:"facility,omitempty"`
}

// SyslogProtocol defines syslog protocols.
type SyslogProtocol string

const (
	SyslogProtocolUDP SyslogProtocol = "UDP"
	SyslogProtocolTCP SyslogProtocol = "TCP"
	SyslogProtocolTLS SyslogProtocol = "TLS"
)

// RequestFilteringConfig defines request filtering policies.
type RequestFilteringConfig struct {
	// SizeLimit defines maximum request size.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`

	// ContentTypeFilter defines allowed content types.
	// +optional
	// +listType=set
	ContentTypeFilter []string `json:"contentTypeFilter,omitempty"`

	// HeaderFilters define header filtering rules.
	// +optional
	// +listType=atomic
	HeaderFilters []HeaderFilter `json:"headerFilters,omitempty"`

	// BodyFilters define body content filtering rules.
	// +optional
	// +listType=atomic
	BodyFilters []BodyFilter `json:"bodyFilters,omitempty"`
}

// HeaderFilter defines header filtering rules.
type HeaderFilter struct {
	// Name is the header name to filter.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Action defines the filtering action.
	// +kubebuilder:validation:Enum=Allow;Deny;Remove;Redact
	// +kubebuilder:validation:Required
	Action FilterAction `json:"action"`

	// Pattern is a regex pattern to match header values.
	// +optional
	Pattern string `json:"pattern,omitempty"`
}

// BodyFilter defines body content filtering rules.
type BodyFilter struct {
	// Type defines the filter type.
	// +kubebuilder:validation:Enum=Regex;JSONPath;Size
	// +kubebuilder:validation:Required
	Type BodyFilterType `json:"type"`

	// Pattern defines the pattern to match.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Action defines the filtering action.
	// +kubebuilder:validation:Enum=Allow;Deny;Remove;Redact
	// +kubebuilder:validation:Required
	Action FilterAction `json:"action"`

	// Replacement defines the replacement value for redaction.
	// +optional
	Replacement string `json:"replacement,omitempty"`
}

// FilterAction defines filtering actions.
type FilterAction string

const (
	FilterActionAllow  FilterAction = "Allow"
	FilterActionDeny   FilterAction = "Deny"
	FilterActionRemove FilterAction = "Remove"
	FilterActionRedact FilterAction = "Redact"
)

// BodyFilterType defines body filter types.
type BodyFilterType string

const (
	BodyFilterTypeRegex    BodyFilterType = "Regex"
	BodyFilterTypeJSONPath BodyFilterType = "JSONPath"
	BodyFilterTypeSize     BodyFilterType = "Size"
)

// ResponseFilteringConfig defines response filtering policies.
type ResponseFilteringConfig struct {
	// SizeLimit defines maximum response size.
	// +optional
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`

	// HeaderFilters define response header filtering rules.
	// +optional
	// +listType=atomic
	HeaderFilters []HeaderFilter `json:"headerFilters,omitempty"`

	// BodyFilters define response body filtering rules.
	// +optional
	// +listType=atomic
	BodyFilters []BodyFilter `json:"bodyFilters,omitempty"`

	// RemoveInternalHeaders controls whether to remove internal headers.
	// +kubebuilder:default=true
	// +optional
	RemoveInternalHeaders *bool `json:"removeInternalHeaders,omitempty"`
}

// ComplianceConfig defines compliance-related policies.
type ComplianceConfig struct {
	// DataRetention defines data retention policies.
	// +optional
	DataRetention *DataRetentionConfig `json:"dataRetention,omitempty"`

	// PIIDetection defines PII detection and handling.
	// +optional
	PIIDetection *PIIDetectionConfig `json:"piiDetection,omitempty"`

	// Encryption defines encryption requirements.
	// +optional
	Encryption *ComplianceEncryptionConfig `json:"encryption,omitempty"`
}

// DataRetentionConfig defines data retention policies.
type DataRetentionConfig struct {
	// AuditLogRetention defines how long to keep audit logs.
	// +kubebuilder:default="90d"
	// +optional
	AuditLogRetention *metav1.Duration `json:"auditLogRetention,omitempty"`

	// ContextRetention defines how long to keep context data.
	// +kubebuilder:default="30d"
	// +optional
	ContextRetention *metav1.Duration `json:"contextRetention,omitempty"`

	// SessionRetention defines how long to keep session data.
	// +kubebuilder:default="7d"
	// +optional
	SessionRetention *metav1.Duration `json:"sessionRetention,omitempty"`
}

// PIIDetectionConfig defines PII detection and handling.
type PIIDetectionConfig struct {
	// Enabled controls whether PII detection is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Patterns define PII detection patterns.
	// +optional
	// +listType=atomic
	Patterns []PIIPattern `json:"patterns,omitempty"`

	// Action defines the action to take when PII is detected.
	// +kubebuilder:validation:Enum=Log;Block;Redact
	// +kubebuilder:default=Log
	// +optional
	Action PIIAction `json:"action,omitempty"`
}

// PIIPattern defines a PII detection pattern.
type PIIPattern struct {
	// Type defines the PII type.
	// +kubebuilder:validation:Enum=SSN;CreditCard;Email;Phone;Custom
	// +kubebuilder:validation:Required
	Type PIIType `json:"type"`

	// Pattern is a regex pattern for custom PII detection.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Description provides a human-readable description.
	// +optional
	Description string `json:"description,omitempty"`
}

// PIIType defines PII types.
type PIIType string

const (
	PIITypeSSN        PIIType = "SSN"
	PIITypeCreditCard PIIType = "CreditCard"
	PIITypeEmail      PIIType = "Email"
	PIITypePhone      PIIType = "Phone"
	PIITypeCustom     PIIType = "Custom"
)

// PIIAction defines PII handling actions.
type PIIAction string

const (
	PIIActionLog    PIIAction = "Log"
	PIIActionBlock  PIIAction = "Block"
	PIIActionRedact PIIAction = "Redact"
)

// ComplianceEncryptionConfig defines compliance encryption configuration.
type ComplianceEncryptionConfig struct {
	// RequireEncryption controls whether encryption is required.
	// +kubebuilder:default=false
	// +optional
	RequireEncryption *bool `json:"requireEncryption,omitempty"`

	// EncryptionAtRest controls whether data must be encrypted at rest.
	// +kubebuilder:default=false
	// +optional
	EncryptionAtRest *bool `json:"encryptionAtRest,omitempty"`

	// EncryptionInTransit controls whether data must be encrypted in transit.
	// +kubebuilder:default=true
	// +optional
	EncryptionInTransit *bool `json:"encryptionInTransit,omitempty"`
}

// CircuitBreakerConfig defines circuit breaker configuration.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures to open the circuit.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes to close the circuit.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`

	// OpenStateTimeout is the time to wait before transitioning to half-open.
	// +kubebuilder:default="30s"
	// +optional
	OpenStateTimeout *metav1.Duration `json:"openStateTimeout,omitempty"`

	// MaxRequestsHalfOpen is the maximum requests allowed in half-open state.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxRequestsHalfOpen *int32 `json:"maxRequestsHalfOpen,omitempty"`

	// RequestTimeout defines the timeout for requests in various states.
	// +kubebuilder:default="30s"
	// +optional
	RequestTimeout *metav1.Duration `json:"requestTimeout,omitempty"`

	// ErrorRateThreshold defines the error rate percentage to open the circuit.
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=100.0
	// +kubebuilder:default=50.0
	// +optional
	ErrorRateThreshold *float64 `json:"errorRateThreshold,omitempty"`

	// MinRequestsThreshold is the minimum requests before error rate is calculated.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=20
	// +optional
	MinRequestsThreshold *int32 `json:"minRequestsThreshold,omitempty"`
}
