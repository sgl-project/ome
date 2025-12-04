---
title: "Run Performance Benchmarks"
linkTitle: "Run Benchmarks"
weight: 10
date: 2023-03-14
description: >
  Learn how to benchmark inference services with realistic traffic patterns and comprehensive performance metrics.
---

This page shows you how to run performance benchmarks on your inference services using OME's BenchmarkJob. You'll learn how to test different traffic scenarios, measure performance metrics, and store results for analysis.

## Before you begin

You need to have the following:

- A Kubernetes cluster with OME installed
- `kubectl` configured to communicate with your cluster
- An InferenceService deployed and ready
- Access to storage for benchmark results (OCI Object Storage or PVC)
- OME benchmark tool image available

## Step 1: Verify prerequisites

Check that your inference service is running:

```bash
kubectl get inferenceservice -A
```

Example output:
```
NAMESPACE                   NAME                      READY   URL
e5-mistral-7b-instruct     e5-mistral-7b-instruct    True    http://e5-mistral-7b-instruct.default
llama-1b-demo              llama-3-2-1b-instruct     True    http://llama-3-2-1b-instruct.default
```

Verify the service is healthy:

```bash
# Replace with your service details
curl -X GET "http://e5-mistral-7b-instruct.e5-mistral-7b-instruct:8080/health"
```

## Step 2: Create a simple benchmark

Let's start with a basic benchmark for a text embedding service:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: benchmark-demo
---
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: simple-benchmark
  namespace: benchmark-demo
spec:
  podOverride:
    image: "ghcr.io/sgl-project/genai-bench:0.1.132"
  endpoint:
    inferenceService:
      name: llama-3-2-1b-instruct
      namespace: llama-1b-demo
  task: text-to-text
  trafficScenarios:
    - "constant_load"
    - "burst_load"
  numConcurrency: [1, 5, 10]
  maxTimePerIteration: 15
  maxRequestsPerIteration: 1000
  serviceMetadata:
    engine: "SGLang"
    version: "v0.4.5"
    gpuType: "H100"
    gpuCount: 1
  outputLocation:
    storageUri: "pvc://benchmark-results-pvc/simple-benchmark"
EOF
```

## Step 3: Comprehensive embedding benchmark

For embedding models, use specialized traffic scenarios:

```bash
kubectl apply -f - <<EOF
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: e5-mistral-7b-instruct-benchmark
  namespace: e5-mistral-7b-instruct
spec:
  podOverride:
    image: "ghcr.io/sgl-project/genai-bench:0.1.132"
  endpoint:
    inferenceService:
      name: e5-mistral-7b-instruct
      namespace: e5-mistral-7b-instruct
  task: text-to-embeddings
  trafficScenarios:
    - "E(128)"     # 128 token embeddings
    - "E(512)"     # 512 token embeddings
    - "E(1024)"    # 1024 token embeddings
    - "E(2048)"    # 2048 token embeddings
    - "E(4096)"    # 4096 token embeddings
    - "E(32000)"   # Maximum context length
  maxTimePerIteration: 15
  maxRequestsPerIteration: 15000
  serviceMetadata:
    engine: "SGLang"
    version: "v0.4.0.post1"
    gpuType: "H100"
    gpuCount: 1
  outputLocation:
    storageUri: "oci://n/idqj093njucb/b/ome-benchmark-results/o/e5-mistral-7b-instruct-benchmark"
    parameters:
      auth: "instance_principal"
      region: "eu-frankfurt-1"
EOF
```

## Step 4: Large model benchmark with multi-node

For large models like DeepSeek-R1, benchmark with realistic workloads:

```bash
kubectl apply -f - <<EOF
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: deepseek-r1-benchmark
  namespace: deepseek-r1
spec:
  podOverride:
    image: "ghcr.io/sgl-project/genai-bench:0.1.132"
    resources:
      requests:
        cpu: "8"
        memory: 16Gi
      limits:
        cpu: "8"
        memory: 16Gi
  endpoint:
    inferenceService:
      name: deepseek-r1
      namespace: deepseek-r1
  task: text-to-text
  trafficScenarios:
    - "reasoning_short"     # Short reasoning tasks
    - "reasoning_medium"    # Medium complexity reasoning
    - "reasoning_long"      # Long chain-of-thought
    - "math_problems"       # Mathematical reasoning
    - "code_generation"     # Code generation tasks
  numConcurrency: [1, 2, 4, 8]
  maxTimePerIteration: 30  # Longer for reasoning tasks
  maxRequestsPerIteration: 5000
  serviceMetadata:
    engine: "SGLang"
    version: "v0.4.5"
    gpuType: "H200"
    gpuCount: 16  # Multi-node deployment
    modelSize: "670B"
    deployment: "MultiNode-RDMA"
  outputLocation:
    storageUri: "oci://n/idqj093njucb/b/ome-benchmark-results/o/deepseek-r1-benchmark"
    parameters:
      auth: "instance_principal"
      region: "us-phoenix-1"
EOF
```

## Step 5: Monitor benchmark progress

Check the benchmark job status:

```bash
kubectl get benchmarkjob -n benchmark-demo
```

Monitor the benchmark pod:

```bash
kubectl get pods -n benchmark-demo -w
```

View benchmark logs:

```bash
kubectl logs -n benchmark-demo -l job-name=simple-benchmark -f
```

Check detailed progress:

```bash
kubectl describe benchmarkjob -n benchmark-demo simple-benchmark
```

## Advanced Benchmark Configurations

### Custom Traffic Patterns

Define custom traffic scenarios:

```yaml
spec:
  trafficScenarios:
    - "warmup(100)"           # Warmup with 100 requests
    - "constant(50,300)"      # 50 RPS for 300 seconds
    - "ramp(10,100,60)"       # Ramp from 10 to 100 RPS over 60s
    - "spike(200,30)"         # Spike to 200 RPS for 30 seconds
    - "burst(100,5,10)"       # 100 RPS burst every 10s for 5s
```

### Multi-Model Comparison

Benchmark multiple models simultaneously:

```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: model-comparison
spec:
  endpoints:
    - name: "llama-3-2-1b"
      inferenceService:
        name: llama-3-2-1b-instruct
        namespace: llama-models
    - name: "llama-3-2-3b"
      inferenceService:
        name: llama-3-2-3b-instruct
        namespace: llama-models
    - name: "mistral-7b"
      inferenceService:
        name: mistral-7b-instruct
        namespace: mistral-models
  task: text-to-text
  trafficScenarios:
    - "constant_load"
    - "variable_load"
  comparisonMetrics:
    - "throughput"
    - "latency_p50"
    - "latency_p95"
    - "latency_p99"
    - "cost_per_token"
```

### External API Benchmarking

Test external APIs for comparison:

```yaml
spec:
  endpoint:
    external:
      url: "https://api.openai.com/v1/chat/completions"
      headers:
        Authorization: "Bearer ${OPENAI_API_KEY}"
        Content-Type: "application/json"
      secretRef:
        name: openai-credentials
  task: text-to-text
  serviceMetadata:
    engine: "OpenAI"
    model: "gpt-4"
    provider: "external"
```

### Custom Benchmark Metrics

Define additional metrics to collect:

```yaml
spec:
  customMetrics:
    - name: "gpu_utilization"
      type: "prometheus"
      query: "avg(nvidia_gpu_utilization_percentage)"
    - name: "memory_usage"
      type: "prometheus"
      query: "avg(container_memory_usage_bytes)"
    - name: "cost_per_request"
      type: "calculated"
      formula: "(gpu_hours * gpu_cost) / total_requests"
```

## Benchmark Traffic Scenarios

### Text Generation Scenarios

**Basic Text Generation:**
```yaml
trafficScenarios:
  - "short_generation(128)"     # 128 output tokens
  - "medium_generation(512)"    # 512 output tokens
  - "long_generation(2048)"     # 2048 output tokens
```

**Chat Completion:**
```yaml
trafficScenarios:
  - "chat_single_turn"          # Single user message
  - "chat_multi_turn(5)"        # 5-turn conversation
  - "chat_context_long"         # Long context conversations
```

**Code Generation:**
```yaml
trafficScenarios:
  - "code_completion"           # Code completion tasks
  - "code_explanation"          # Code explanation requests
  - "code_refactoring"          # Code refactoring tasks
```

### Embedding Scenarios

**Document Embedding:**
```yaml
trafficScenarios:
  - "E(128)"    # Short text embedding
  - "E(512)"    # Paragraph embedding
  - "E(2048)"   # Document embedding
  - "E(8192)"   # Long document embedding
```

**Batch Processing:**
```yaml
trafficScenarios:
  - "batch_small(10)"           # 10 texts per batch
  - "batch_medium(50)"          # 50 texts per batch
  - "batch_large(100)"          # 100 texts per batch
```

## Result Analysis

### Access Benchmark Results

**OCI Object Storage:**
```bash
# List benchmark results
oci os object list -bn ome-benchmark-results --prefix e5-mistral-7b-instruct-benchmark

# Download results
oci os object get -bn ome-benchmark-results \
  --name e5-mistral-7b-instruct-benchmark/results.json \
  --file ./benchmark-results.json
```

**Persistent Volume:**
```bash
# Mount PVC and view results
kubectl run results-viewer --rm -i --tty \
  --image=alpine:latest \
  --overrides='{"spec":{"volumes":[{"name":"results","persistentVolumeClaim":{"claimName":"benchmark-results-pvc"}}],"containers":[{"name":"viewer","image":"alpine:latest","volumeMounts":[{"name":"results","mountPath":"/results"}],"command":["sh"]}]}}' \
  -- sh

# Inside the pod
ls -la /results/
cat /results/simple-benchmark/summary.json
```

### Key Performance Metrics

**Throughput Metrics:**
- `requests_per_second` - Total RPS handled
- `tokens_per_second` - Token generation rate
- `successful_requests` - Successful request count
- `failed_requests` - Failed request count

**Latency Metrics:**
- `latency_p50` - 50th percentile latency
- `latency_p95` - 95th percentile latency
- `latency_p99` - 99th percentile latency
- `time_to_first_token` - TTFT for streaming

**Resource Metrics:**
- `gpu_utilization_avg` - Average GPU utilization
- `memory_usage_peak` - Peak memory usage
- `cpu_utilization_avg` - Average CPU utilization

**Quality Metrics (if enabled):**
- `bleu_score` - BLEU score for generation quality
- `rouge_score` - ROUGE score for summarization
- `semantic_similarity` - Embedding quality metrics

### Benchmark Report Example

```json
{
  "benchmark_id": "e5-mistral-7b-instruct-benchmark",
  "timestamp": "2024-01-15T10:30:00Z",
  "service_metadata": {
    "engine": "SGLang",
    "version": "v0.4.0.post1",
    "gpu_type": "H100",
    "gpu_count": 1,
    "model": "e5-mistral-7b-instruct"
  },
  "scenarios": [
    {
      "name": "E(128)",
      "duration": 900,
      "total_requests": 15000,
      "successful_requests": 14987,
      "failed_requests": 13,
      "requests_per_second": 16.65,
      "latency_p50": 45.2,
      "latency_p95": 89.7,
      "latency_p99": 156.3,
      "throughput_mbps": 12.4
    }
  ],
  "resource_usage": {
    "gpu_utilization_avg": 87.3,
    "memory_usage_peak": "22.1GB",
    "cpu_utilization_avg": 34.2
  }
}
```

## Best Practices

### Benchmark Design

1. **Warm-up Phase**: Always include a warm-up period
2. **Realistic Workloads**: Use production-like traffic patterns
3. **Multiple Concurrency Levels**: Test various concurrent user loads
4. **Sufficient Duration**: Run for at least 10-15 minutes per scenario
5. **Baseline Comparison**: Establish baseline performance metrics

### Traffic Scenario Selection

1. **Start Simple**: Begin with basic constant load testing
2. **Add Complexity**: Progress to burst and variable loads
3. **Model-Specific**: Choose scenarios appropriate for your model type
4. **Production Patterns**: Mirror expected production traffic

### Resource Considerations

1. **Dedicated Resources**: Use dedicated benchmark nodes when possible
2. **Network Isolation**: Minimize network interference
3. **Storage Performance**: Ensure fast storage for result collection
4. **Monitoring**: Enable comprehensive monitoring during benchmarks

## Troubleshooting

### Benchmark Job Not Starting

```bash
# Check job status
kubectl describe benchmarkjob -n benchmark-demo simple-benchmark

# Check pod issues
kubectl get events -n benchmark-demo --sort-by=.metadata.creationTimestamp

# Verify image pull
kubectl describe pod -n benchmark-demo <benchmark-pod>
```

### Low Performance Results

**Common Issues:**
- **Resource Constraints**: Check GPU/CPU/memory limits
- **Network Bottlenecks**: Verify network connectivity
- **Storage Latency**: Ensure fast storage for logs
- **Inference Service Issues**: Check target service health

**Debugging Commands:**
```bash
# Check target service performance
kubectl top pods -l serving.ome.io/inferenceservice=<service-name>

# Monitor GPU usage during benchmark
kubectl exec -it <inference-pod> -- nvidia-smi -l 1

# Check network connectivity
kubectl exec -it <benchmark-pod> -- ping <inference-service>
```

### Storage Issues

**OCI Object Storage:**
```bash
# Test OCI credentials
kubectl exec -it <benchmark-pod> -- oci os ns get

# Check bucket permissions
kubectl exec -it <benchmark-pod> -- oci os bucket get --bucket-name ome-benchmark-results
```

**Persistent Volume:**
```bash
# Check PVC status
kubectl get pvc -n benchmark-demo

# Verify mount points
kubectl exec -it <benchmark-pod> -- df -h
```

## Next Steps

- [Analyze Performance Results](/ome/docs/tasks/developer-tools/analyze-performance/) - Deep dive into benchmark data
- [Setup Continuous Benchmarking](/ome/docs/tasks/manage-ome/setup-continuous-benchmarking/) - Automate performance testing
- [Optimize Model Performance](/ome/docs/tasks/run-workloads/optimize-performance/) - Improve model efficiency
- [Compare Model Variants](/ome/docs/tasks/run-workloads/compare-models/) - A/B test different models

## Cleanup

To remove benchmark resources:

```bash
kubectl delete benchmarkjob -n benchmark-demo simple-benchmark
kubectl delete namespace benchmark-demo
``` 
