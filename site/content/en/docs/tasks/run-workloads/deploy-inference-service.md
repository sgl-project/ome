---
title: "Deploy a Simple Inference Service"
linkTitle: "Deploy Inference Service"
weight: 5
date: 2023-03-14
description: >
  Learn how to deploy your first inference service with OME.
---

This page shows you how to deploy a simple inference service using OME. You'll learn how to create an InferenceService that serves a pre-trained model for real-time inference using SGLang and OpenAI-compatible APIs.

## Before you begin

You need to have the following:

- A Kubernetes cluster with OME installed
- `kubectl` configured to communicate with your cluster
- GPU nodes available in your cluster (A100, H100, H200, or B4)
- Access to OME container registry (`ghcr.io/sgl-project/`)

## Step 1: Verify prerequisites

Check that OME is installed and running:

```bash
kubectl get pods -n ome
```

Expected output:
```
NAME                                     READY   STATUS    RESTARTS   AGE
ome-controller-manager-xxx               2/2     Running   0          5m
ome-model-controller-xxx                 1/1     Running   0          5m
ome-model-agent-daemonset-xxx            1/1     Running   0          5m
```

Check available serving runtimes:

```bash
kubectl get clusterservingruntimes
```

Example output:
```
NAME                               AGE
srt-llama-3-2-1b-instruct         1d
srt-llama-3-2-3b-instruct         1d
srt-llama-3-3-70b-instruct        1d
srt-deepseek-r1                   1d
srt-mistral-7b-instruct           1d
```

Verify GPU availability:

```bash
kubectl get nodes -o custom-columns="NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu"
```

## Step 2: Deploy a small model (1B parameters)

Let's start with a small model that requires only one GPU:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: llama-1b-demo
---
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-3-2-1b-instruct
  namespace: llama-1b-demo
spec:
  predictor:
    model:
      baseModel: llama-3-2-1b-instruct
      protocolVersion: openAI
    minReplicas: 1
    maxReplicas: 1
EOF
```

## Step 3: Monitor deployment progress

Check the deployment status:

```bash
kubectl get inferenceservice -n llama-1b-demo
```

Monitor the pods:

```bash
kubectl get pods -n llama-1b-demo -w
```

Check the events for troubleshooting:

```bash
kubectl get events -n llama-1b-demo --sort-by=.metadata.creationTimestamp
```

The deployment is ready when the pod status shows `Running` and the readiness probe passes.

## Step 4: Test the service

### Method 1: Port Forward (for testing)

Forward the service port to your local machine:

```bash
kubectl port-forward -n llama-1b-demo svc/llama-3-2-1b-instruct 8080:8080
```

Test with a simple chat completion:

```bash
curl -X POST "http://localhost:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-2-1b-instruct",
    "messages": [
      {"role": "user", "content": "Hello! Can you introduce yourself?"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Method 2: In-Cluster Access

Create a test pod to access the service:

```bash
kubectl run test-client --rm -i --tty --image=curlimages/curl -- /bin/sh
```

From within the pod:

```bash
curl -X POST "http://llama-3-2-1b-instruct.llama-1b-demo:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-2-1b-instruct",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 50
  }'
```

## Step 5: Deploy a larger model (70B parameters)

For larger models, you'll need multiple GPUs and more resources:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: llama-70b-demo
---
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-3-3-70b-instruct
  namespace: llama-70b-demo
spec:
  predictor:
    model:
      baseModel: llama-3-3-70b-instruct
      protocolVersion: openAI
      runtime: srt-llama-3-3-70b-instruct
    minReplicas: 1
    maxReplicas: 1
EOF
```

This configuration will:
- Use tensor parallelism across 4 GPUs (tp=4)
- Require ~160GB GPU memory
- Target H100/H200 GPU nodes

## Step 6: Deploy a multi-node model (600B+ parameters)

For very large models like DeepSeek-R1, use multi-node deployment with RDMA:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: deepseek-r1
---
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: deepseek-r1
  namespace: deepseek-r1
  annotations:
    ome.io/deploymentMode: "MultiNode"
spec:
  predictor:
    model:
      baseModel: deepseek-r1
      protocolVersion: openAI
      runtime: srt-multi-node-deepseek-r1-rdma
    minReplicas: 1
    maxReplicas: 1
EOF
```

This deployment features:
- Multi-node RDMA networking for optimal performance
- Support for 670B parameter models
- Specialized reasoning capabilities
- Requires cluster network nodes with RDMA support

## Advanced Configuration Options

### Custom Resource Requirements

Override the default resource requirements:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: custom-resources
spec:
  predictor:
    model:
      baseModel: llama-3-2-3b-instruct
    resources:
      requests:
        cpu: "16"
        memory: 64Gi
        nvidia.com/gpu: 1
      limits:
        cpu: "16"
        memory: 64Gi
        nvidia.com/gpu: 1
```

### Environment Variables

Pass custom environment variables to the serving container:

```yaml
spec:
  predictor:
    model:
      baseModel: llama-3-2-1b-instruct
    env:
      - name: CUSTOM_SETTING
        value: "production"
      - name: LOG_LEVEL
        value: "INFO"
```

### Node Selection

Target specific node types:

```yaml
spec:
  predictor:
    nodeSelector:
      node.kubernetes.io/instance-type: BM.GPU.H100.8
    tolerations:
      - key: "nvidia.com/gpu"
        operator: "Exists"
        effect: "NoSchedule"
```

## Monitoring and Debugging

### Check Service Health

All OME serving runtimes include health endpoints:

```bash
# Basic health check
curl http://llama-3-2-1b-instruct.llama-1b-demo:8080/health

# Advanced health check (includes model loading status)
curl http://llama-3-2-1b-instruct.llama-1b-demo:8080/health_generate
```

### View Metrics

OME exposes Prometheus metrics on port 8080:

```bash
curl http://llama-3-2-1b-instruct.llama-1b-demo:8080/metrics
```

Key metrics include:
- `sglang_prompt_tokens_total` - Total prompt tokens processed
- `sglang_generation_tokens_total` - Total tokens generated
- `sglang_request_duration_seconds` - Request latency distribution
- `sglang_concurrent_requests` - Current concurrent requests

### Debug Common Issues

**Pod won't start:**
```bash
kubectl describe pod -n llama-1b-demo <pod-name>
kubectl logs -n llama-1b-demo <pod-name> -c ome-container
```

**Model loading fails:**
```bash
# Check if base model exists
kubectl get clusterbasemodels

# Check serving runtime compatibility
kubectl describe clusterservingruntime srt-llama-3-2-1b-instruct
```

**GPU resource issues:**
```bash
# Check GPU allocation
kubectl describe node <gpu-node-name> | grep nvidia.com/gpu

# View GPU utilization
kubectl exec -it -n llama-1b-demo <pod-name> -- nvidia-smi
```

## Supported Models and Runtimes

### Small Models (1-8 GPUs)
- **LLaMA 3.2 1B/3B**: Single GPU deployment
- **LLaMA 3.3 70B**: 4-GPU tensor parallelism
- **Mistral 7B**: Single GPU with high throughput
- **Mixtral 8x7B**: Mixture of Experts architecture

### Large Models (Multi-Node)
- **DeepSeek-V3 (670B)**: Multi-node RDMA deployment
- **DeepSeek-R1 (670B)**: Reasoning-optimized multi-node
- **LLaMA 3.1 405B**: FP8 quantized multi-node

### Specialized Models
- **E5-Mistral 7B**: Text embedding generation
- **LLaMA Vision**: Multi-modal text and image processing

## Performance Optimization

### Tensor Parallelism

For multi-GPU models, OME automatically configures tensor parallelism:

- **1B models**: tp=1 (single GPU)
- **3B models**: tp=1 with memory optimization
- **70B models**: tp=4 across 4 GPUs
- **400B+ models**: Multi-node distribution

### Memory Management

Configure memory fraction for optimal GPU utilization:

```yaml
# Defined in serving runtime
args:
  - |
    python3 -m sglang.launch_server \
    --mem-frac=0.9 \  # Use 90% of GPU memory
    --model-path="$MODEL_PATH"
```

### Compilation Optimization

Enable PyTorch compilation for better performance:

```yaml
args:
  - |
    python3 -m sglang.launch_server \
    --enable-torch-compile \
    --torch-compile-max-bs 1 \
    --model-path="$MODEL_PATH"
```

## Next Steps

- [Run Performance Benchmarks](/ome/docs/tasks/run-workloads/run-benchmarks/) - Test your model's performance
- [Setup Autoscaling](/ome/docs/tasks/manage-ome/setup-autoscaling/) - Configure dynamic scaling
- [Monitor with Prometheus](/ome/docs/tasks/developer-tools/setup-prometheus/) - Set up comprehensive monitoring
- [Deploy Multiple Models](/ome/docs/tasks/run-workloads/deploy-multiple-models/) - Run multiple models efficiently

## Cleanup

To remove the inference service:

```bash
kubectl delete inferenceservice -n llama-1b-demo llama-3-2-1b-instruct
kubectl delete inferenceservice -n llama-70b-demo llama-3-3-70b-instruct
kubectl delete inferenceservice -n deepseek-r1 deepseek-r1
```

This will clean up all associated resources including deployments, services, and storage. 