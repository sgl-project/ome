---
title: "Benchmark"
date: 2023-03-14
weight: 6
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
  name: llama-chat-benchmark
spec:
  endpoint:
    inferenceService:
      name: llama-chat
      namespace: default
  task: text-to-text
  trafficScenarios:
    - chat-completion
    - text-completion
  numConcurrency:
    - 1
    - 5
    - 10
  maxTimePerIteration: 5
  maxRequestsPerIteration: 1000
  serviceMetadata:
    engine: vllm
    version: "0.5.3"
    gpuType: "A100"
    gpuCount: 1
  outputLocation:
    path: "oci://my-namespace/my-bucket/benchmark-results"
  podOverride:
    resources:
      requests:
        cpu: "4"
        memory: "16Gi"
      limits:
        cpu: "4"
        memory: "16Gi"
```

## Spec Attributes

Available attributes in the BenchmarkJob spec:

| Attribute | Description |
|-----------|-------------|
| `endpoint` | Required. Target inference service configuration |
| `task` | Required. Type of task to benchmark (e.g., text-to-text) |
| `trafficScenarios` | Optional. List of traffic patterns to test |
| `numConcurrency` | Optional. List of concurrency levels to test |
| `maxTimePerIteration` | Required. Maximum time per test iteration |
| `maxRequestsPerIteration` | Required. Maximum requests per iteration |
| `serviceMetadata` | Optional. Backend service information |
| `outputLocation` | Required. Where to store benchmark results |
| `podOverride` | Optional. Benchmark pod configuration |

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
