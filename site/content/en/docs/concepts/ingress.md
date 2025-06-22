---
title: "Ingress and External Access"
date: 2024-06-21
weight: 6
description: >
  Configure external access to your AI inference services through ingress controllers, load balancers, and service routing.
---

## What is Ingress in OME?

Ingress in OME provides external access to your AI inference services running inside the Kubernetes cluster. When you deploy an InferenceService, OME automatically creates the appropriate ingress resources based on your deployment mode and cluster configuration, allowing external clients to make API calls to your models.

Think of ingress as the "front door" to your AI services - it handles incoming HTTP requests from outside the cluster and routes them to the correct model endpoints inside your cluster.

## Deployment Modes and Ingress Types

OME supports three different ingress strategies depending on how you deploy your inference services:

### Serverless Mode (Knative)
**Best for**: Variable workloads, auto-scaling, cost optimization

When deploying in serverless mode, OME integrates with Knative and creates **Istio VirtualService** resources for routing:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
spec:
  engine:
    model: llama-3-70b-instruct
    resources:
      requests:
        nvidia.com/gpu: 1
```

This automatically creates external access at:
```
https://llama-chat.your-namespace.example.com
```

### Raw Deployment Mode
**Best for**: Consistent workloads, dedicated resources, custom configurations

Raw deployments use standard Kubernetes ingress controllers:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/deployment-mode: "RawDeployment"
spec:
  engine:
    model: llama-3-70b-instruct
    resources:
      requests:
        nvidia.com/gpu: 2
```

This creates a **Kubernetes Ingress** resource accessible at:
```
https://llama-chat.your-namespace.your-cluster.com
```

### MultiNode Mode
**Best for**: Large models requiring multiple GPUs, distributed inference

MultiNode deployments support the same ingress options as Raw deployments:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-405b
  annotations:
    ome.io/deployment-mode: "MultiNode"
spec:
  engine:
    model: llama-3-405b-instruct
    resources:
      requests:
        nvidia.com/gpu: 8
    parallelism: 4  # Distributed across 4 nodes
```

## Component-Based Routing

OME's inference services can include multiple components that work together. The ingress system automatically creates the right routing rules based on which components you deploy:

### Engine Only (Basic Inference)
```yaml
spec:
  engine:
    model: llama-3-70b-instruct
```
**Routing**: All requests → Engine service

### Engine + Router (Advanced Inference)
```yaml
spec:
  engine:
    model: llama-3-70b-instruct
  router:
    template:
      spec:
        containers:
        - name: router
          image: custom-router:latest
```
**Routing**: 
- Top-level requests → Router service
- Router processes and forwards → Engine service

### Engine + Decoder (Post-Processing)
```yaml
spec:
  engine:
    model: llama-3-70b-instruct
  decoder:
    template:
      spec:
        containers:
        - name: decoder
          image: custom-decoder:latest
```
**Routing**:
- Top-level requests → Engine service  
- Decoder-specific requests (`/v1/decoder/...`) → Decoder service

### Full Pipeline (Engine + Router + Decoder)
```yaml
spec:
  engine:
    model: llama-3-70b-instruct
  router:
    template:
      spec:
        containers:
        - name: router
          image: custom-router:latest
  decoder:
    template:
      spec:
        containers:
        - name: decoder
          image: custom-decoder:latest
```
**Routing**:
- Top-level requests → Router service
- Router-specific requests (`/v1/router/...`) → Router service  
- Decoder-specific requests (`/v1/decoder/...`) → Decoder service

## Making API Calls

Once your inference service is deployed and ingress is configured, you can make API calls using standard HTTP clients:

### Basic Text Generation
```bash
# Get the ingress URL
kubectl get inferenceservice llama-chat -o jsonpath='{.status.url}'

# Make a completion request
curl -X POST https://llama-chat.your-namespace.example.com/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b-instruct",
    "prompt": "Explain quantum computing in simple terms:",
    "max_tokens": 150,
    "temperature": 0.7
  }'
```

### Chat Completions
```bash
curl -X POST https://llama-chat.your-namespace.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b-instruct",
    "messages": [
      {"role": "user", "content": "What is machine learning?"}
    ],
    "max_tokens": 200
  }'
```

### Component-Specific Endpoints

When using services with multiple components, you can access specific endpoints:

```bash
# Router-specific endpoint
curl -X POST https://llama-chat.your-namespace.example.com/v1/router/route \
  -H "Content-Type: application/json" \
  -d '{"request": "route this to the best model"}'

# Decoder-specific endpoint  
curl -X POST https://llama-chat.your-namespace.example.com/v1/decoder/decode \
  -H "Content-Type: application/json" \
  -d '{"tokens": [1, 2, 3, 4], "decode_format": "text"}'

# Health checks
curl -H "Accept: application/json" \
  https://llama-chat.your-namespace.example.com/health
curl -H "Accept: application/json" \
  https://llama-chat.your-namespace.example.com/v1/router/health
curl -H "Accept: application/json" \
  https://llama-chat.your-namespace.example.com/v1/decoder/health
```

### Model Information
```bash
# List available models
curl -H "Accept: application/json" \
  https://llama-chat.your-namespace.example.com/v1/models

# Get model details
curl -H "Accept: application/json" \
  https://llama-chat.your-namespace.example.com/v1/models/llama-3-70b-instruct
```

## Ingress Configuration Options

### Custom Ingress Class
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/ingress-class: "nginx"  # Use nginx instead of default istio
spec:
  engine:
    model: llama-3-70b-instruct
```

### Cluster-Local Services (Internal Only)
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: internal-llama
  labels:
    networking.knative.dev/visibility: cluster-local
spec:
  engine:
    model: llama-3-70b-instruct
```

This creates a service accessible only from within the cluster:
```bash
# From inside the cluster
curl -X POST http://internal-llama.your-namespace.svc.cluster.local/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b-instruct",
    "prompt": "Internal cluster request",
    "max_tokens": 100
  }'
```

### Load Balancer Services
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/disable-ingress: "true"
    ome.io/service-type: "LoadBalancer"
spec:
  engine:
    model: llama-3-70b-instruct
```

## Troubleshooting Ingress

### Check Service Status
```bash
# Check inference service status
kubectl get inferenceservice llama-chat -o yaml

# Look for ingress readiness condition
kubectl get inferenceservice llama-chat -o jsonpath='{.status.conditions[?(@.type=="IngressReady")]}'
```

### Verify Ingress Resources
```bash
# Check ingress resources
kubectl get ingress -l ome.io/inferenceservice=llama-chat

# For serverless mode, check VirtualService
kubectl get virtualservice -l ome.io/inferenceservice=llama-chat

# For Gateway API
kubectl get httproute -l ome.io/inferenceservice=llama-chat
```

### Test Internal Connectivity
```bash
# Test engine service directly
kubectl port-forward service/llama-chat-engine 8080:80
curl -H "Accept: application/json" http://localhost:8080/health

# Test router service (if exists)
kubectl port-forward service/llama-chat 8080:80
curl -H "Accept: application/json" http://localhost:8080/health
```

### Common Issues

**Ingress Not Created**: Check that ingress isn't disabled and components are ready:
```bash
kubectl get inferenceservice llama-chat -o yaml | grep -A 5 "conditions:"
```

**503/404 Errors**: Verify target services exist and are healthy:
```bash
kubectl get services -l ome.io/inferenceservice=llama-chat
kubectl get pods -l ome.io/inferenceservice=llama-chat
```

**DNS Issues**: Ensure your ingress controller is properly configured with DNS:
```bash
kubectl get ingress llama-chat -o yaml
nslookup llama-chat.your-namespace.example.com
```

## Per-Service Ingress Configuration

While cluster-wide ingress settings are configured in the OME ConfigMap, you can override specific ingress settings per InferenceService using annotations. This allows flexible customization without changing cluster defaults.

### Available Annotation Overrides

| Annotation | Purpose | Example |
|------------|---------|---------|
| `ome.io/ingress-domain-template` | Custom domain template | `{{.Name}}.{{.Namespace}}.ml.example.com` |
| `ome.io/ingress-domain` | Fixed ingress domain | `ml-services.example.com` |
| `ome.io/ingress-additional-domains` | Additional domains (comma-separated) | `backup.example.com,alt.example.com` |
| `ome.io/ingress-url-scheme` | URL scheme override | `https` |
| `ome.io/ingress-path-template` | Custom path template | `/models/{{.Name}}/{{.Namespace}}` |
| `ome.io/ingress-disable-istio-virtualhost` | Disable Istio VirtualService | `"true"` |
| `ome.io/ingress-disable-creation` | Skip ingress creation entirely | `"true"` |

## Security Considerations

### Authentication
OME ingress supports various authentication methods:

```bash
# Using API keys (if configured)
curl -X POST https://llama-chat.your-namespace.example.com/v1/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b-instruct",
    "prompt": "API key authentication",
    "max_tokens": 100
  }'

# Using basic auth (if configured)  
curl -X POST https://llama-chat.your-namespace.example.com/v1/completions \
  -u username:password \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b-instruct",
    "prompt": "Basic auth request",
    "max_tokens": 100
  }'
```

### TLS/SSL
All external ingress endpoints should use HTTPS in production. Check your ingress controller documentation for TLS certificate configuration.

## Next Steps

- **[Administration Guide](/docs/administration/ingress/)** - Configure ingress controllers and networking
- **[Serving Runtime](/docs/concepts/serving_runtime/)** - Understand the underlying serving infrastructure
- **[InferenceService](/docs/concepts/inference_service/)** - Complete InferenceService configuration reference

For production deployments and cluster configuration, see the [Administration](/docs/administration/) section.
