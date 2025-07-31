# ome-predefined-models

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.16.0](https://img.shields.io/badge/AppVersion-1.16.0-informational?style=flat-square)

OME Predefined Models and Serving Runtimes

## Description

This Helm chart provides a collection of predefined models and serving runtimes for OME (Open Model Engine). Instead of manually managing these resources through kustomize, users can now deploy them natively using Helm with fine-grained control over which models and runtimes to enable.

## Features

- **Predefined Models**: Deploy popular models from various vendors (Meta, DeepSeek, Intfloat, Microsoft, Moonshot AI, NVIDIA)
- **Serving Runtimes**: Support for both vLLM and SRT (SGLang Runtime) configurations
- **Selective Deployment**: Enable/disable specific models and runtimes through values configuration
- **Production Ready**: Includes proper resource limits, health checks, and monitoring configurations

## Installation

### Prerequisites

- Kubernetes cluster with GPU nodes
- OME CRDs already installed (`ome-crd` chart)
- OME controller running (`ome-resources` chart)

### Install the chart

```bash
# Add the repository (if using a Helm repository)
helm repo add ome <repository-url>
helm repo update

# Install with default values
helm install ome-predefined-models ome/ome-predefined-models

# Or install from local chart
helm install ome-predefined-models ./charts/ome-predefined-models
```

### Custom Configuration

Create a `custom-values.yaml` file to customize which models and runtimes to enable:

```yaml
# Enable all resources
global:
  enableAll: false

# Enable specific models
models:
  meta:
    enabled: true
    llama_3_3_70b_instruct:
      enabled: true
    llama_4_maverick_17b_128e_instruct_fp8:
      enabled: false
      
  deepseek:
    enabled: true
    deepseek_v3:
      enabled: true
    deepseek_r1:
      enabled: false

  intfloat:
    enabled: true
    e5_mistral_7b_instruct:
      enabled: true

# Enable specific runtimes
runtimes:
  vllm:
    enabled: true
    e5_mistral_7b_instruct:
      enabled: true
    llama_3_3_70b_instruct:
      enabled: true
      
  srt:
    enabled: true
    deepseek_rdma:
      enabled: true
    e5_mistral_7b_instruct:
      enabled: true
```

Then install with your custom values:

```bash
helm install ome-predefined-models ./charts/ome-predefined-models -f custom-values.yaml
```

## Supported Models

### Meta/Llama Models

- `llama-3-3-70b-instruct` - Llama 3.3 70B Instruct model
- `llama-4-maverick-17b-128e-instruct-fp8` - Llama 4 Maverick 17B model (FP8)
- `llama-4-scout-17b-16e-instruct` - Llama 4 Scout 17B model

### DeepSeek Models

- `deepseek-v3` - DeepSeek V3 model
- `deepseek-r1` - DeepSeek R1 model

### Intfloat Models

- `e5-mistral-7b-instruct` - E5 Mistral 7B Instruct model

### Microsoft Models

- `phi-3-vision-128k-instruct` - Phi-3 Vision 128K Instruct model

### Moonshot AI Models

- `kimi-k2-instruct` - Kimi K2 Instruct model

### NVIDIA Models

- `llama-3-1-nemotron-ultra-253b-v1` - Llama 3.1 Nemotron Ultra 253B
- `llama-3-3-nemotron-super-49b-v1` - Llama 3.3 Nemotron Super 49B
- `llama-3-1-nemotron-nano-8b-v1` - Llama 3.1 Nemotron Nano 8B

## Supported Runtimes

### vLLM Runtimes

- Optimized for inference workloads
- Built-in OpenAI-compatible API server
- Efficient memory utilization

### SRT (SGLang Runtime) Runtimes

- Advanced serving capabilities
- Support for complex multi-node deployments
- RDMA support for high-performance networking

## Configuration Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| global.enableAll | bool | `false` | Enable all predefined resources |
| models.meta.enabled | bool | `true` | Enable Meta/Llama models |
| models.deepseek.enabled | bool | `true` | Enable DeepSeek models |
| models.intfloat.enabled | bool | `true` | Enable Intfloat models |
| models.microsoft.enabled | bool | `false` | Enable Microsoft models |
| models.moonshotai.enabled | bool | `false` | Enable Moonshot AI models |
| models.nvidia.enabled | bool | `false` | Enable NVIDIA models |
| runtimes.vllm.enabled | bool | `true` | Enable vLLM runtimes |
| runtimes.srt.enabled | bool | `true` | Enable SRT runtimes |

## Usage Examples

### Deploy Only Essential Models

```yaml
global:
  enableAll: false

models:
  meta:
    enabled: true
    llama_3_3_70b_instruct:
      enabled: true
  
  intfloat:
    enabled: true
    e5_mistral_7b_instruct:
      enabled: true

runtimes:
  vllm:
    enabled: true
    llama_3_3_70b_instruct:
      enabled: true
    e5_mistral_7b_instruct:
      enabled: true
```

### High-Performance Setup with RDMA

```yaml
models:
  deepseek:
    enabled: true
    deepseek_v3:
      enabled: true

runtimes:
  srt:
    enabled: true
    deepseek_rdma:
      enabled: true
```

## Monitoring

All deployed runtimes include Prometheus metrics endpoints configured for monitoring:

- Metrics endpoint: `/metrics`
- Health check endpoint: `/health`  
- Generate health check: `/health_generate` (for SRT runtimes)

## Troubleshooting

### Common Issues

1. **Models not downloading**: Ensure proper Hugging Face token is configured
2. **GPU resources**: Verify GPU nodes have sufficient resources
3. **RDMA configuration**: For RDMA-enabled runtimes, ensure proper network setup

### Debugging Commands

```bash
# Check deployed models
kubectl get clusterbasemodels

# Check deployed runtimes
kubectl get clusterservingruntimes

# Check pod logs
kubectl logs -l app=ome-predefined-models
```

## Contributing

To add new models or runtimes:

1. Add the configuration to the appropriate template file
2. Update the `values.yaml` with the new configuration options
3. Update this README with the new resource information
