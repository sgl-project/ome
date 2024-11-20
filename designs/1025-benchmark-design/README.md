
# Benchmark CRD to Support Automatic Benchmark for Model Serving

## Overview
The Benchmark CRD allows users to define and create a benchmark task using [genai-bench](https://bitbucket.oci.oraclecorp.com/projects/GENAICORE/repos/genai-bench/browse) within a Kubernetes environment. This CRD is designed to automate the benchmarking process for LLM (Large Language Model) inference services, streamlining performance evaluations of deployed models. By defining a Benchmark custom resource, users can configure key parameters for a benchmark task.

## Motivation
1. Automate Benchmarking Workflows: Simplifies the process of configuring a benchmark task within a Kubernetes cluster. Use namespace-scoped resources to independently manage and run benchmark tasks.
2. Reduce Configuration Complexity: Abstracts the complexity of manually setting up and managing benchmark tasks, allow users to focus on high-level benchmark settings. 

## Design Details
The Benchmark CRD provides an interface for configuring and managing benchmarking tasks for model inference services. Below are key sections of the CRD design:

### Key Fields in spec
1. **BenchmarkJobSpec**: Defines the benchmark job specification, containing fields such as the reference to the inference service, traffic scenarios, concurrency levels, runtime settings, and output location.
2. **InferenceServiceRef**: A reference specifying the name and namespace of the inference service to be benchmarked. This allows the benchmark controller to locate and interact with the correct inference service in the Kubernetes cluster, automatically providing model serving and API-related options for the `genai-bench` CLI.
3. **OutputLocation**: Specifies where the benchmark results should be stored.
4. **ObjectStorage**: Defines the object storage details for storing benchmark results.

## CRD Specification

### BenchmarkJobSpec

```go
type BenchmarkJobSpec struct {
    // HuggingFaceAPIKey is the API key required for Hugging Face authentication.
	// +required
    HuggingFaceAPIKey        string                   `json:"huggingFaceAPIKey"`
	
    // InferenceServiceReference is the reference to the inference service to benchmark.
    // +required
    InferenceServiceReference InferenceServiceReference  `json:"inferenceServiceReference"`

	// ServiceMetadata records metadata about the backend model server or service being benchmarked.
	// This includes details such as server engine, version, and GPU configuration for filtering experiments.
	// +optional
	ServiceMetadata          *ServiceMetadata          `json:"serviceMetadata,omitempty"`
    
    // Task specifies the task to benchmark (e.g., "chat", "vision", "embeddings").
    // +kubebuilder:validation:Enum=chat;vision;embeddings
    // +required
    Task                     string                   `json:"task"`

	// TrafficScenarios contains a list of traffic scenarios to simulate during the benchmark.
	// If not provided, defaults will be assigned via genai-bench.
	// +optional
	TrafficScenarios          []string                 `json:"trafficScenarios,omitempty"`
    
    // NumConcurrency defines a list of concurrency levels to test during the benchmark.
    // If not provided, defaults will be assigned via genai-bench.
    // +optional
    NumConcurrency           []int                    `json:"numConcurrency,omitempty"`
    
    // MaxTimePerIteration specifies the maximum time (in minutes) for a single iteration.
    // Each iteration runs for a specific combination of TrafficScenario and NumConcurrency.
    // +required
    MaxTimePerIteration      *int                      `json:"maxTimePerIteration"`
    
    // MaxRequestsPerIteration specifies the maximum number of requests for a single iteration.
    // Each iteration runs for a specific combination of TrafficScenario and NumConcurrency.
    // +required
    MaxRequestsPerIteration  *int                      `json:"maxRequestsPerIteration"`
    
    // AdditionalRequestParams contains additional request parameters as a map.
    // +optional
    AdditionalRequestParams  map[string]interface{}    `json:"additionalRequestParams,omitempty"`
    
    // Dataset is the dataset used for benchmarking.
    // It is optional and only required for tasks other than "chat".
    // +optional
    Dataset                  *Storage                 `json:"dataset,omitempty"`
    
    // OutputLocation specifies where the benchmark results will be stored (e.g., object storage).
    // +required
    OutputLocation           Storage                  `json:"outputLocation"`
}
```
- **HuggingFaceAPIKey**: The API key uses Hugging Face's transformers library to load or download the model tokenizer. This is required for `genai-bench` as it needs to tokenize the input.
- **InferenceServiceReference**: A reference to the deployed model service providing the necessary model serving environment. This removes the need for manual configuration of model inference protocol, model API fields, task for the benchmark, etc.

The following parameters are adapted from `genai-bench benchmark --help`.
- **ServiceMetadata**: Contains metadata about the backend model server or inference environment. Useful for tracking the server configuration and filtering experiments based on specific server properties.
- **Task**: Specifies the task being benchmarked (e.g., “chat”, “vision”, “embeddings”). A single model can support multiple tasks. For example, the [`meta-llama/Llama-3.2-90B-Vision-Instruct`](https://huggingface.co/meta-llama/Llama-3.2-90B-Vision-Instruct) model can be used for text-only chat tasks and image-based vision tasks. The task determines the model API used.
- **TrafficScenarios**: A list of traffic scenario strings specifying how input and output tokens will be sampled. Different Task corresponds to different TrafficScenario format. The format of each scenario may vary depending on the **Task** being executed. For example, a `chat` task might have traffic scenarios that simulate different conversation lengths, while a `vision` task could simulate scenarios with varying image sizes.
    * Example: "N(480,240)/(300,150)" might simulate a normal distribution of input tokens with mean 480 and standard deviation 240, and output tokens with mean 300 and standard deviation 150.
    * If no scenarios are provided, default traffic scenarios will be automatically assigned by genai-bench based on **Task**.
    * For more details on the format of traffic scenarios, please refer to [genai_bench/cli/validation.py](https://bitbucket.oci.oraclecorp.com/projects/GENAICORE/repos/genai-bench/browse/genai_bench/cli/validation.py#20-45).
- **NumConcurrency**: A list of concurrency levels to test during the benchmark. For each traffic scenario, the benchmark will run at multiple concurrency levels to evaluate the model’s performance under different loads. 
- **MaxTimePerIteration**: Maximum allowed runtime per iteration (in minutes). An experiment iteration will stop once the maximum runtime is reached. Each iteration consists of a specific combination of a TrafficScenario and a NumConcurrency level.
- **MaxRequestsPerIteration**: The maximum number of requests per experiment iteration. An iteration stops once this limit is reached.
- **AdditionalRequestParams**: JSON-encoded parameters for additional parameters in the request sent to the model server, such as `temperature`, `ignore_eos`, etc.
- **Dataset**: A single *Storage type that defines the dataset used for benchmarking. Only one dataset needed for one benchmark job. Optional for `chat` task. A validating webhook logic will be added to check if Task is not "chat" and then enforces that Dataset must be set.
- **OutputLocation**: Specifies where the benchmark results will be stored (e.g., object storage).

#### How the Experiment Iterates Through Scenarios and Concurrency Levels:
The benchmark is executed in a nested loop where:
	1.	For each traffic scenario in TrafficScenarios,
	2.	It will test the scenario with each concurrency level in NumConcurrency.
This means that if there are 3 traffic scenarios and 5 concurrency levels, the benchmark will perform 15 iterations (3 x 5). Each iteration corresponds to a unique combination of one traffic scenario and one concurrency level. During each iteration, the experiment will run for up to MaxTimePerIteration or MaxRequestsPerIteration, whichever is reached first. For detailed code implementation, please refer to [genai_bench/cli/cli.py](https://bitbucket.oci.oraclecorp.com/projects/GENAICORE/repos/genai-bench/browse/genai_bench/cli/cli.py#227-232,254-258).

### InferenceServiceReference
The `InferenceServiceReference` struct defines a reference to an inference service. It supports either an InferenceService or an endpoint with API-specific configuration (Endpoint).
```go
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
```
- **Name**: The name of the `InferenceService` that the benchmark job will target.
- **Namespace**: The Kubernetes namespace where the `InferenceService` is deployed. Support cross-namespace references allows centralization of inference services in one namespace while running benchmark jobs in another. This provides flexibility but requires careful management of RBAC policies to ensure secure access control and prevent unauthorized access.

```go
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
```

### ServiceMetadata
```go
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
```
- **Engine**: Specifies the backend server engine, restricted to values:
    * "vLLM": vLLM engine.
    * "SGLang": SGLang engine.
    * "TGI": TGI engine.
- **Version**: Specifies the version of the server.
- **GpuType**: Specifies the GPU type.
- **GpuCount**: Number of GPU cards the server uses.

### OutputLocation
Adopted from [`training_common.go`](`pkg/apis/serving/v1beta1/training_common.go`).

```go
type StorageSource string

const (
    ObjectStorage StorageSource = "OBJECT_STORAGE"
)

type Storage struct {
    // Represents the type of the storage
    StorageType string `json:"storageType,omitempty"`
}

// OSStorage defines the arguments for object storage
type OSStorage struct {
    Storage    `json:",inline"`
    BucketName string `json:"bucketName"`
    Namespace  string `json:"namespace"`
    ObjectName string `json:"objectName,omitempty"`
    Prefix     string `json:"prefix,omitempty"`
    OboToken   string `json:"oboToken,omitempty"`
}
```
- **StorageType**: Specifies the storage type (e.g., Object Storage).
- **OSStorage**: Object storage configuration, including bucket name, namespace, optional object name, and storage path 
prefix. An optional token can also be provided for object storage access.

### BenchmarkJobStatus
The status of the benchmark pod. It reflects the state and results of the benchmark job. It will be set and updated
by the controller.

```go
type BenchmarkJobStatus struct {
    // State represents the current state of the benchmark job: "Pending", "Running", "Completed", "Failed".
    // +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	// +required
    State           string      `json:"state"`
	
    // StartTime is the timestamp for when the benchmark job started.
	// +optional
    StartTime       *metav1.Time `json:"startTime,omitempty"`
	
    // CompletionTime is the timestamp for when the benchmark job completed, either successfully or unsuccessfully.
	// +optional
    CompletionTime  *metav1.Time `json:"completionTime,omitempty"`
	
    // LastReconcileTime is the timestamp for the last time the job was reconciled by the controller.
	// +optional
    LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
	
    // FailureMessage contains any error messages if the benchmark job failed.
    // +optional
    FailureMessage  string      `json:"failureMessage,omitempty"`
	
    // Details provide additional information or metadata about the benchmark job.
    // +optional
    Details         string      `json:"details,omitempty"`
}
```
Omit the JobConditions field because the lifecycle of a BenchmarkJob is simple and linear, with well-defined states like Pending, Running, Completed, and Failed, which can be effectively captured using the State field alone.

### Sample YAML
This YAML configuration specifies a benchmark task for the `llama-3-1-70b` inference service, running with multiple 
traffic scenarios and concurrency levels, and storing the results in object storage.

#### Chat
```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: llama3-1-70b-vllm-benchmark
spec:
  huggingFaceAPIKey: "hf-api-token"
  inferenceServiceReference:
    k8sInferenceService:
      name: llama-3-1-70b
      namespace: llama-3-1-70b
  serviceMetadata:
    engine: "vLLM"
    version: "0.6.3.post1"
    gpuType: "H100"
    gpuCount: 4
  task: "chat"
  trafficScenarios:
    - "N(480,240)/(300,150)"
    - "D(16000,100)"
    - "D(7800,200)"
  numConcurrency: [1, 2, 4, 8, 16, 32, 64, 128, 256]
  maxTimePerIteration: 15
  maxRequestsPerIteration: 200
  additionalRequestParams:
    temperature: 0.0
    ignore_eos: true
  dataset:
     storageType: "OBJECT_STORAGE"
     osStorage:
        bucketName: "benchmark-dataset"
        namespace: "datasets-namespace"
        prefix: "chat-dataset/"
  outputLocation:
    storageType: "OBJECT_STORAGE"
    osStorage:
      bucketName: "benchmark-results"
      namespace: "llama3"
      prefix: "llama3-1-70b/"
```
