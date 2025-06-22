---
title: "Ingress Administration"
linkTitle: "Ingress Administration"
weight: 40
description: >
  Cluster administrator guide for configuring OME ingress and networking in production environments.
---

## Overview

OME (Open Model Engine) provides flexible ingress configuration to support various Kubernetes ingress controllers and deployment scenarios. This guide covers the configuration options available to cluster administrators.

## Supported Ingress Controllers

OME supports the following ingress solutions:

### Standard Kubernetes Ingress Controllers
- **NGINX Ingress Controller** - [Installation Guide](https://kubernetes.github.io/ingress-nginx/deploy/)
- **Traefik** - [Installation Guide](https://doc.traefik.io/traefik/getting-started/install-traefik/)
- **Kong Ingress Controller** - [Installation Guide](https://docs.konghq.com/kubernetes-ingress-controller/latest/install/)
- **HAProxy Ingress** - [Installation Guide](https://haproxy-ingress.github.io/docs/getting-started/)
- **Contour** - [Installation Guide](https://projectcontour.io/getting-started/)

### Service Mesh Ingress
- **Istio** (VirtualService + Gateway) - [Installation Guide](https://istio.io/latest/docs/setup/install/)

### Gateway API
- **Gateway API Controllers** - [Installation Guide](https://gateway-api.sigs.k8s.io/guides/)

## OME Ingress Configuration

OME ingress behavior is configured through the `inferenceservice-config` ConfigMap in the OME controller namespace.

### ConfigMap Structure

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: inferenceservice-config
  namespace: ome  # OME controller namespace
data:
  ingress: |
    {
      "ingressClassName": "nginx",
      "ingressDomain": "api.company.com",
      "domainTemplate": "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
      "urlScheme": "https",
      "disableIngressCreation": false,
      "enableGatewayAPI": false
    }
```

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `ingressClassName` | string | `"istio"` | Ingress controller class name |
| `ingressDomain` | string | `"example.com"` | Base domain for generated hostnames |
| `domainTemplate` | string | `"{{.Name}}.{{.Namespace}}.{{.IngressDomain}}"` | Template for generating hostnames |
| `urlScheme` | string | `"http"` | URL scheme (http/https) |
| `disableIngressCreation` | bool | `false` | Disable ingress creation entirely |
| `enableGatewayAPI` | bool | `false` | Use Gateway API instead of Kubernetes Ingress |

### Advanced Parameters

For Istio service mesh configurations:

| Parameter | Type | Description |
|-----------|------|-------------|
| `ingressGateway` | string | Istio Gateway resource name |
| `ingressService` | string | Istio ingress gateway service name |
| `disableIstioVirtualHost` | bool | Disable Istio VirtualService creation |

## Advanced Configuration

### Disabling Ingress Creation

When you need to use external load balancers or custom networking:

```yaml
data:
  ingress: |
    {
      "disableIngressCreation": true
    }
```

When ingress is disabled, OME automatically creates external services for cluster access.

### Custom Domain Templates

Configure custom hostname patterns:

```yaml
data:
  ingress: |
    {
      "domainTemplate": "{{.Name}}.{{.Namespace}}.ml.company.com",
      "ingressDomain": "ml.company.com"
    }
```

Available template variables:
- `{{.Name}}` - InferenceService name
- `{{.Namespace}}` - Kubernetes namespace
- `{{.IngressDomain}}` - Base domain from config

### Gateway API Configuration

For clusters using Gateway API instead of standard Ingress:

```yaml
data:
  ingress: |
    {
      "enableGatewayAPI": true,
      "ingressClassName": "gateway-controller"
    }
```

## Per-Service Configuration Overrides

While the ConfigMap provides cluster-wide defaults, individual InferenceServices can override specific ingress settings using annotations. This enables flexible per-service customization without changing global configuration.

### Supported Override Annotations

The following ingress settings can be overridden per service:

| Annotation Key | ConfigMap Equivalent | Description | Type |
|----------------|---------------------|-------------|------|
| `ome.io/ingress-domain-template` | `domainTemplate` | Custom domain pattern | String template |
| `ome.io/ingress-domain` | `ingressDomain` | Fixed base domain | String |
| `ome.io/ingress-additional-domains` | `additionalIngressDomains` | Extra domains (comma-separated) | String list |
| `ome.io/ingress-url-scheme` | `urlScheme` | HTTP/HTTPS scheme | String |
| `ome.io/ingress-path-template` | `pathTemplate` | URL path pattern | String template |
| `ome.io/ingress-disable-istio-virtualhost` | `disableIstioVirtualHost` | Skip VirtualService creation | Boolean ("true"/"false") |
| `ome.io/ingress-disable-creation` | `disableIngressCreation` | Skip all ingress creation | Boolean ("true"/"false") |

### Configuration Priority

The resolution order follows this priority (highest to lowest):

1. **Service Annotations** - Per-service overrides via `ome.io/ingress-*` annotations
2. **ConfigMap Defaults** - Cluster-wide settings in `inferenceservice-config`
3. **Built-in Defaults** - OME controller fallback values

## Deployment Mode Behavior

OME creates different ingress resources based on deployment mode:

| Deployment Mode | Ingress Type | Configuration |
|-----------------|--------------|---------------|
| **Serverless** | Istio VirtualService | Uses `ingressGateway` and `ingressService` parameters |
| **RawDeployment** | Kubernetes Ingress or Gateway API | Uses `ingressClassName` parameter |
| **MultiNode** | Kubernetes Ingress or Gateway API | Uses `ingressClassName` parameter |

## Configuration Validation

Apply and validate your configuration:

```bash
# Update the configuration
kubectl apply -f inferenceservice-config.yaml

# Restart OME controller to pick up changes
kubectl rollout restart deployment/ome-controller-manager -n ome

# Verify configuration
kubectl get configmap inferenceservice-config -n ome -o yaml
```

## Monitoring and Troubleshooting

### Check Ingress Creation

```bash
# List all ingresses created by OME
kubectl get ingress -A -l ome.io/inference-service

# Check ingress for specific service
kubectl get ingress <inference-service-name> -n <namespace> -o yaml

# Check VirtualService (Istio)
kubectl get virtualservice <inference-service-name> -n <namespace> -o yaml

# Check HTTPRoute (Gateway API)
kubectl get httproute <inference-service-name> -n <namespace> -o yaml
```

### Verify Service Resolution

```bash
# Check if services exist
kubectl get svc -l ome.io/inference-service=<service-name>

# Test internal resolution
kubectl run debug --image=busybox --rm -it -- nslookup <service-name>.<namespace>.svc.cluster.local

# Test external resolution
nslookup <hostname-from-ingress>
```

### OME Controller Logs

```bash
# Check ingress reconciler logs
kubectl logs -n ome deployment/ome-controller-manager | grep "ingress"

# Look for specific error patterns
kubectl logs -n ome deployment/ome-controller-manager | grep -E "(ingress|IngressReady|ComponentNotReady)"
```

### Common Issues

**Ingress not created:**
- Check if ingress controller class exists: `kubectl get ingressclass`
- Verify component readiness: `kubectl get inferenceservice <name> -o yaml`
- Check controller logs for "ComponentNotReady" messages

**DNS resolution fails:**
- Verify ingress controller is running: `kubectl get pods -n <ingress-namespace>`
- Check ingress resource exists and has correct hostname
- Confirm DNS points to ingress controller LoadBalancer IP

**Service mesh issues (Istio):**
- Verify Istio VirtualService exists: `kubectl get virtualservice`
- Check Gateway configuration: `kubectl get gateway -A`
- Confirm service mesh sidecar injection: `kubectl get pods -o yaml | grep istio-proxy`

## Production Considerations

### Resource Limits

Configure appropriate resource limits for ingress controllers handling AI workloads:

```yaml
# Example for NGINX ingress controller
resources:
  requests:
    cpu: 1000m
    memory: 1Gi
  limits:
    cpu: 4000m
    memory: 4Gi
```

### Security Best Practices

1. **TLS Configuration**: Always use HTTPS in production
2. **Network Policies**: Restrict traffic to inference services
3. **Authentication**: Implement API key or OAuth validation at ingress level
4. **Rate Limiting**: Configure request rate limits for AI inference endpoints

### High Availability

For production clusters:

1. **Multiple Replicas**: Run ingress controller with multiple replicas
2. **Node Affinity**: Distribute ingress pods across availability zones
3. **Health Checks**: Configure proper liveness and readiness probes
4. **Monitoring**: Set up alerts for ingress controller health and performance

### Performance Tuning

AI inference workloads have specific requirements:

1. **Request Timeouts**: Set longer timeouts for model inference
2. **Body Size Limits**: Increase limits for large input payloads
3. **Connection Pooling**: Optimize connection reuse for persistent connections
4. **Buffer Configuration**: Disable response buffering for streaming responses

## Next Steps

1. **Choose Ingress Controller**: Select based on your infrastructure and requirements
2. **Install Ingress Controller**: Follow official installation guides linked above
3. **Configure OME**: Update the `inferenceservice-config` ConfigMap with appropriate settings
4. **Deploy Inference Service**: Create an InferenceService and verify external access
5. **Monitor and Tune**: Implement monitoring and adjust configuration as needed

For user-facing guidance on accessing inference services, see the [Ingress Concepts](/docs/concepts/ingress/) documentation.
