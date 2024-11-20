package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// BenchmarkJob is the schema for the BenchmarkJobs API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type BenchmarkJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkJobSpec   `json:"spec,omitempty"`
	Status BenchmarkJobStatus `json:"status,omitempty"`
}

// BenchmarkJobSpec defines the specification for a benchmark job.
// All fields within this specification collectively represent the desired
// state and configuration of a BenchmarkJob.
type BenchmarkJobSpec struct {
	// HuggingFaceAPIKey is the API key required for Hugging Face authentication.
	// +required
	// TODO: integrate HF token with k8s secret
	HuggingFaceAPIKey string `json:"huggingFaceAPIKey"`

	// InferenceServiceReference is the reference to the inference service to benchmark.
	// +required
	InferenceServiceReference InferenceServiceReference `json:"inferenceServiceReference"`

	// ServiceMetadata records metadata about the backend model server or service being benchmarked.
	// This includes details such as server engine, version, and GPU configuration for filtering experiments.
	// +optional
	ServiceMetadata *ServiceMetadata `json:"serviceMetadata,omitempty"`

	// Task specifies the task to benchmark (e.g., "chat", "vision", "embeddings").
	// +kubebuilder:validation:Enum=chat;vision;embeddings
	// +required
	Task string `json:"task"`

	// TrafficScenarios contains a list of traffic scenarios to simulate during the benchmark.
	// If not provided, defaults will be assigned via genai-bench.
	// +listType=set
	// +optional
	TrafficScenarios []string `json:"trafficScenarios,omitempty"`

	// NumConcurrency defines a list of concurrency levels to test during the benchmark.
	// If not provided, defaults will be assigned via genai-bench.
	// +listType=set
	// +optional
	NumConcurrency []int `json:"numConcurrency,omitempty"`

	// MaxTimePerIteration specifies the maximum time (in minutes) for a single iteration.
	// Each iteration runs for a specific combination of TrafficScenario and NumConcurrency.
	// +required
	MaxTimePerIteration *int `json:"maxTimePerIteration"`

	// MaxRequestsPerIteration specifies the maximum number of requests for a single iteration.
	// Each iteration runs for a specific combination of TrafficScenario and NumConcurrency.
	// +required
	MaxRequestsPerIteration *int `json:"maxRequestsPerIteration"`

	// AdditionalRequestParams contains additional request parameters as a map.
	// +optional
	AdditionalRequestParams map[string]string `json:"additionalRequestParams,omitempty"`

	// Dataset is the dataset used for benchmarking.
	// It is optional and only required for tasks other than "chat".
	// +optional
	Dataset *Storage `json:"dataset,omitempty"`

	// OutputLocation specifies where the benchmark results will be stored (e.g., object storage).
	// +required
	OutputLocation Storage `json:"outputLocation"`
}

// InferenceServiceReference defines a reference to an inference service.
// It supports either a Kubernetes-style reference (K8sInferenceService) or an Endpoint struct for a direct URL.
type InferenceServiceReference struct {
	// K8sInferenceService holds a Kubernetes reference to an internal inference service.
	// +optional
	K8sInferenceService *K8sInferenceServiceReference `json:"k8sInferenceService,omitempty"`

	// Endpoint holds the details of a direct endpoint for an external inference service, including URL and API details.
	// +optional
	Endpoint *Endpoint `json:"endpoint,omitempty"`
}

// K8sInferenceServiceReference defines the reference to a Kubernetes inference service.
type K8sInferenceServiceReference struct {
	// Name specifies the name of the inference service to benchmark.
	// +required
	Name string `json:"name"`

	// Namespace specifies the Kubernetes namespace where the inference service is deployed.
	// +required
	Namespace string `json:"namespace"`
}

// Endpoint defines a direct URL-based inference service with additional API configuration.
type Endpoint struct {
	// URL represents the endpoint URL for the inference service.
	// +kubebuilder:validation:Pattern=`^(http|https)://`
	URL string `json:"url"`

	// APIFormat specifies the type of API, such as "openai" or "genai".
	// +kubebuilder:validation:Enum=openai;genai
	APIFormat string `json:"apiFormat"`

	// ModelName specifies the name of the model being served at the endpoint.
	// Useful for endpoints that require model-specific configuration. For instance,
	// for openai API, this is a required field in the payload
	ModelName string `json:"modelName,omitempty"`
}

// ServiceMetadata contains metadata fields for recording the backend model server's configuration and version details.
// This information helps track experiment context, enabling users to filter and query experiments based on server properties.
type ServiceMetadata struct {
	// Engine specifies the backend model server engine.
	// Supported values: "vLLM", "SGLang", "TGI".
	// +kubebuilder:validation:Enum=vLLM;SGLang;TGI
	Engine string `json:"engine"`

	// Version specifies the version of the model server (e.g., "0.5.3").
	Version string `json:"version"`

	// GpuType specifies the type of GPU used by the model server.
	// Supported values: "H100", "A100", "MI300", "A10".
	// +kubebuilder:validation:Enum=H100;A100;MI300;A10
	GpuType string `json:"gpuType"`

	// GpuCount indicates the number of GPU cards available on the model server.
	GpuCount int `json:"gpuCount"`
}

// BenchmarkJobStatus reflects the state and results of the benchmark job. It
// will be set and updated by the controller.
type BenchmarkJobStatus struct {
	// State represents the current state of the benchmark job: "Pending", "Running", "Completed", "Failed".
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	// +required
	State string `json:"state"`

	// StartTime is the timestamp for when the benchmark job started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the timestamp for when the benchmark job completed, either successfully or unsuccessfully.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// LastReconcileTime is the timestamp for the last time the job was reconciled by the controller.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// FailureMessage contains any error messages if the benchmark job failed.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// Details provide additional information or metadata about the benchmark job.
	// +optional
	Details string `json:"details,omitempty"`
}

// BenchmarkJobList contains a list of BenchmarkJob
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type BenchmarkJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BenchmarkJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BenchmarkJob{}, &BenchmarkJobList{})
}
