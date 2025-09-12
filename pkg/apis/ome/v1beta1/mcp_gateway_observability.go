package v1beta1

import "k8s.io/apimachinery/pkg/api/resource"

// ObservabilityConfig defines monitoring, metrics, and tracing configuration.
type MCPGatewayObservabilityConfig struct {
	// Metrics defines metrics collection and export configuration.
	// +optional
	Metrics *MetricsConfig `json:"metrics,omitempty"`

	// Tracing defines distributed tracing configuration.
	// +optional
	Tracing *TracingConfig `json:"tracing,omitempty"`

	// Logging defines structured logging configuration.
	// +optional
	Logging *LoggingConfig `json:"logging,omitempty"`

	// Health defines health check endpoint configuration.
	// +optional
	Health *HealthEndpointConfig `json:"health,omitempty"`
}

// MetricsConfig defines metrics collection and export configuration.
type MetricsConfig struct {
	// Enabled controls whether metrics collection is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port defines the metrics endpoint port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9090
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Path defines the metrics endpoint path.
	// +kubebuilder:default="/metrics"
	// +optional
	Path string `json:"path,omitempty"`

	// Format defines the metrics format.
	// +kubebuilder:validation:Enum=Prometheus;OpenMetrics
	// +kubebuilder:default=Prometheus
	// +optional
	Format MetricsFormat `json:"format,omitempty"`

	// CustomMetrics define additional custom metrics to collect.
	// +optional
	// +listType=atomic
	CustomMetrics []CustomMetric `json:"customMetrics,omitempty"`
}

// TracingConfig defines distributed tracing configuration.
type TracingConfig struct {
	// Enabled controls whether distributed tracing is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Provider defines the tracing backend provider.
	// +kubebuilder:validation:Enum=Jaeger;Zipkin;OpenTelemetry;DataDog
	// +kubebuilder:default=OpenTelemetry
	// +optional
	Provider TracingProvider `json:"provider,omitempty"`

	// Endpoint defines the tracing collector endpoint.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// SamplingRate defines the sampling rate for traces (0.0-1.0).
	// +kubebuilder:validation:Minimum=0.0
	// +kubebuilder:validation:Maximum=1.0
	// +kubebuilder:default=0.1
	// +optional
	SamplingRate *float64 `json:"samplingRate,omitempty"`

	// Headers define additional headers to send with traces.
	// +optional
	// +mapType=atomic
	Headers map[string]string `json:"headers,omitempty"`
}

// LoggingConfig defines structured logging configuration.
type LoggingConfig struct {
	// Level defines the logging level.
	// +kubebuilder:validation:Enum=Debug;Info;Warn;Error
	// +kubebuilder:default=Info
	// +optional
	Level LogLevel `json:"level,omitempty"`

	// Format defines the log format.
	// +kubebuilder:validation:Enum=JSON;Text
	// +kubebuilder:default=JSON
	// +optional
	Format LogFormat `json:"format,omitempty"`

	// Output defines where logs are sent.
	// +kubebuilder:validation:Enum=Stdout;File;Syslog
	// +kubebuilder:default=Stdout
	// +optional
	Output LogOutput `json:"output,omitempty"`

	// File defines file-based logging configuration.
	// +optional
	File *LogFileConfig `json:"file,omitempty"`
}

// HealthEndpointConfig defines health check endpoint configuration.
type HealthEndpointConfig struct {
	// Enabled controls whether health endpoints are exposed.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port defines the health endpoint port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	Port *int32 `json:"port,omitempty"`

	// LivenessPath defines the liveness probe path.
	// +kubebuilder:default="/healthz"
	// +optional
	LivenessPath string `json:"livenessPath,omitempty"`

	// ReadinessPath defines the readiness probe path.
	// +kubebuilder:default="/readyz"
	// +optional
	ReadinessPath string `json:"readinessPath,omitempty"`
}

// MetricsFormat defines supported metrics formats.
type MetricsFormat string

const (
	MetricsFormatPrometheus  MetricsFormat = "Prometheus"
	MetricsFormatOpenMetrics MetricsFormat = "OpenMetrics"
)

// TracingProvider defines supported tracing providers.
type TracingProvider string

const (
	TracingProviderJaeger        TracingProvider = "Jaeger"
	TracingProviderZipkin        TracingProvider = "Zipkin"
	TracingProviderOpenTelemetry TracingProvider = "OpenTelemetry"
	TracingProviderDataDog       TracingProvider = "DataDog"
)

// LogLevel defines logging levels.
type LogLevel string

const (
	LogLevelDebug LogLevel = "Debug"
	LogLevelInfo  LogLevel = "Info"
	LogLevelWarn  LogLevel = "Warn"
	LogLevelError LogLevel = "Error"
)

// LogFormat defines log formats.
type LogFormat string

const (
	LogFormatJSON LogFormat = "JSON"
	LogFormatText LogFormat = "Text"
)

// LogOutput defines log output destinations.
type LogOutput string

const (
	LogOutputStdout LogOutput = "Stdout"
	LogOutputFile   LogOutput = "File"
	LogOutputSyslog LogOutput = "Syslog"
)

// CustomMetric defines a custom metric to collect.
type CustomMetric struct {
	// Name is the metric name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type defines the metric type.
	// +kubebuilder:validation:Enum=Counter;Gauge;Histogram;Summary
	// +kubebuilder:validation:Required
	Type MetricType `json:"type"`

	// Help provides a description of the metric.
	// +optional
	Help string `json:"help,omitempty"`

	// Labels define metric labels.
	// +optional
	// +listType=set
	Labels []string `json:"labels,omitempty"`
}

// MetricType defines metric types.
type MetricType string

const (
	MetricTypeCounter   MetricType = "Counter"
	MetricTypeGauge     MetricType = "Gauge"
	MetricTypeHistogram MetricType = "Histogram"
	MetricTypeSummary   MetricType = "Summary"
)

// LogFileConfig defines file-based logging configuration.
type LogFileConfig struct {
	// Path defines the log file path.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// MaxSize defines the maximum log file size before rotation.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// MaxFiles defines the maximum number of log files to keep.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	MaxFiles *int32 `json:"maxFiles,omitempty"`

	// Compress controls whether rotated logs are compressed.
	// +kubebuilder:default=true
	// +optional
	Compress *bool `json:"compress,omitempty"`
}
