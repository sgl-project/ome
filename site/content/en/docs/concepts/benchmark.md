---
title: "Benchmark"
date: 2023-03-14
weight: 35
description: >
  BenchmarkJob is a resource that manages automated performance benchmarking of inference services.
---

A _BenchmarkJob_ is a resource in OME that automates the performance benchmarking of inference service or OCI Generative AI Service endpoints. It allows you to evaluate model serving performance under various traffic patterns and load conditions.

## Core Components

A BenchmarkJob consists of several key components:

1. **Endpoint Configuration**: Specifies the target inference service to benchmark
2. **Traffic Patterns**: Defines the load testing scenarios
3. **Resource Configuration**: Controls the benchmark execution environment
4. **Output Management**: Handles benchmark results storage

## Example Configuration

Here's an example of a BenchmarkJob configuration:

```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
   name: llama-3-1-70b-benchmark
   namespace: llama-3-1-70b
spec:
   podOverride:
      image: "ghcr.io/sgl-project/genai-bench:0.1.127"
      resources:
         requests:
            cpu: "4"
            memory: "16Gi"
         limits:
            cpu: "4"
            memory: "16Gi"
   endpoint:
      inferenceService:
         name: llama-3-1-70b-instruct
         namespace: llama-3-1-70b-instruct
   task: text-to-text
   trafficScenarios:
      - "N(480,240)/(300,150)"
      - "D(100,100)"
      - "D(100,1000)"
      - "D(2000,200)"
      - "D(7800,200)"
   numConcurrency:
      - 1
      - 2
      - 4
      - 8
      - 16
      - 32
      - 64
      - 128
      - 256
   maxTimePerIteration: 15
   maxRequestsPerIteration: 100
   additionalRequestParams:
      temperature: "0.0"
   outputLocation:
      storageUri: "oci://n/idqj093njucb/b/ome-benchmark-results/o/llama-3-1-70b-benchmark"
      parameters:
         auth: "instance_principal"  # Authentication type
         config_file: "/path/to/config"  # Optional: Config file for user_principal auth
         profile: "DEFAULT"  # Optional: Profile name for user_principal auth
         security_token: "token"  # Optional: Token for security_token auth
         region: "us-phoenix-1"  # Optional: Region for security_token auth
```

## Spec Attributes

Available attributes in the BenchmarkJob spec:

| Attribute                 | Description                                              |
|---------------------------|----------------------------------------------------------|
| `endpoint`                | Required. Target inference service configuration         |
| `task`                    | Required. Type of task to benchmark (e.g., text-to-text) |
| `trafficScenarios`        | Optional. List of traffic patterns to test               |
| `numConcurrency`          | Optional. List of concurrency levels to test             |
| `maxTimePerIteration`     | Required. Maximum time per test iteration                |
| `maxRequestsPerIteration` | Required. Maximum requests per iteration                 |
| `serviceMetadata`         | Optional. Backend service information                    |
| `outputLocation`          | Required. Where to store benchmark results               |
| `podOverride`             | Optional. Benchmark pod configuration                    |

## Endpoint Configuration

BenchmarkJob supports two types of endpoints:

1. **InferenceService Reference**:
```yaml
endpoint:
  inferenceService:
    name: my-model
    namespace: default
```

2. **Direct URL Endpoint**:
```yaml
endpoint:
  endpoint:
    url: "http://my-model-service:8080/v1/completions"
    apiFormat: "openai"
    modelName: "my-model"
```

## Storage Configuration

BenchmarkJob supports storing benchmark results in OCI Object Storage. The storage configuration is specified in the `outputLocation` field:

```yaml
outputLocation:
  storageUri: "oci://n/my-namespace/b/my-bucket/o/benchmark-results"
  parameters:
    auth: "instance_principal"  # Authentication type
    config_file: "/path/to/config"  # Optional: Config file for user_principal auth
    profile: "DEFAULT"  # Optional: Profile name for user_principal auth
    security_token: "token"  # Optional: Token for security_token auth
    region: "us-phoenix-1"  # Optional: Region for security_token auth
```

### Storage URI Format

The storage URI must follow this format:
```
oci://n/{namespace}/b/{bucket}/o/{object_path}
```

Where:
- `{namespace}`: Your OCI object storage namespace
- `{bucket}`: The bucket name
- `{object_path}`: Path prefix for benchmark results

### Authentication Options

The following authentication methods are supported:

- `user_principal`: Uses OCI config file credentials
  - Requires `config_file` and optionally `profile`
- `instance_principal`: Uses instance credentials
- `security_token`: Uses security token authentication
  - Requires `security_token` and `region`
- `instance_obo_user`: Uses instance principal on behalf of user

## Reconciliation Process

The BenchmarkJob controller performs several steps during reconciliation:

1. **Resource Preparation**:
   - Creates necessary PersistentVolumes and PersistentVolumeClaims
   - Sets up storage for model and benchmark data

2. **Job Creation**:
   - Generates benchmark pod specification
   - Configures resource requirements
   - Sets up environment variables

3. **Execution Management**:
   - Monitors job progress
   - Handles job completion and failures
   - Updates status with results

4. **Cleanup**:
   - Manages resource cleanup on completion
   - Handles proper deletion of resources

## Status

The BenchmarkJob status provides information about the benchmark execution:

```yaml
status:
  state: Running
  startTime: "2023-12-27T02:30:00Z"
  lastReconcileTime: "2023-12-27T02:35:00Z"
  details: "Running iteration 2/6: concurrency=5"
```

## Best Practices

1. **Resource Planning**:
   - Ensure benchmark pods have sufficient resources
   - Consider network bandwidth requirements

2. **Test Scenarios**:
   - Start with low concurrency and gradually increase
   - Use realistic traffic patterns
   - Test both average and peak loads

3. **Results Analysis**:
   - Monitor latency percentiles
   - Track throughput metrics
   - Analyze resource utilization

4. **Storage Management**:
   - Use appropriate storage classes for results
   - Clean up old benchmark data regularly


## Usage Guide
1. Make sure an InferenceService is running in the cluster. 

```shell
# Follow CONTRIBUTING.md to start OME manager
# Create a Llama-3.1-70b-Instruct iscv if there is not one
kubectl apply -f config/samples/iscv/meta/llama3-1-70b-instruct.yaml 
```

2. Start a benchmark

```shell
# If there is a secret reference, apply the sceret resource first
kubectl apply -f config/samples/benchmark/huggingface-secret.yaml 
kubectl apply -f config/samples/benchmark/llama3-1-70b-instruct.yaml 
```