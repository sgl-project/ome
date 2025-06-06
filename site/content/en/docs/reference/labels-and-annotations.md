---
title: Labels and Annotations
linkTitle: Labels and Annotations
weight: 2
description: >
  Reference of labels and annotations used by OME.
---

This document serves as a reference of the various labels and annotations used throughout OME.

## Annotations

### InferenceService Annotations

These annotations are used to configure InferenceService behavior:

| Annotation                           | Description                                                                                                                                               |
|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ome.io/enable-tag-routing`          | Enables tag-based routing for the InferenceService                                                                                                        |
| `ome.io/autoscalerClass`             | Specifies the autoscaler class to use. Valid values: `hpa`, `keda`, `external`                                                                            |
| `ome.io/metrics`                     | Defines the scaling metric type. Valid values: `cpu`, `memory`                                                                                            |
| `ome.io/targetUtilizationPercentage` | Sets the target utilization percentage for autoscaling                                                                                                    |
| `ome.io/deprecation-warning`         | Displays deprecation warnings for legacy configurations                                                                                                   |
| `ome.io/enable-metric-aggregation`   | Enables metric aggregation for the InferenceService                                                                                                       |
| `ome.io/enable-prometheus-scraping`  | Enables Prometheus scraping for metrics collection                                                                                                        |
| `ome.io/volcano-queue`               | Specifies the Volcano queue name for job scheduling                                                                                                       |

### Model and Runtime Annotations

| Annotation                                      | Description                                          |
|-------------------------------------------------|------------------------------------------------------|
| `ome.io/inject-model-init`                      | Enables injection of model initialization containers |
| `ome.io/inject-fine-tuned-adapter`              | Enables injection of fine-tuned adapter containers   |
| `ome.io/inject-serving-sidecar`                 | Enables injection of serving sidecar containers      |
| `ome.io/fine-tuned-weight-ft-strategy`          | Specifies the fine-tuning strategy for weights       |
| `ome.io/base-model-name`                        | Specifies the base model name                        |
| `ome.io/base-model-vendor`                      | Specifies the base model vendor                      |
| `ome.io/serving-runtime`                        | Specifies the serving runtime to use                 |
| `ome.io/base-model-format`                      | Specifies the base model format                      |
| `ome.io/base-model-format-version`              | Specifies the base model format version              |
| `ome.io/fine-tuned-serving-with-merged-weights` | Enables fine-tuned serving with merged weights       |

### Model Security Annotations

These annotations control model encryption and decryption:

| Annotation                                 | Description                                                 |
|--------------------------------------------|-------------------------------------------------------------|
| `ome.io/base-model-decryption-key-name`    | Specifies the decryption key name for the base model        |
| `ome.io/base-model-decryption-secret-name` | Specifies the secret name containing decryption credentials |
| `ome.io/disable-model-decryption`          | Disables model decryption                                   |

### Service Configuration Annotations

| Annotation                    | Description                           |
|-------------------------------|---------------------------------------|
| `ome.io/service-type`         | Specifies the Kubernetes service type |
| `ome.io/load-balancer-ip`     | Sets the load balancer IP address     |

### RDMA Annotations

| Annotation                   | Description                                         |
|------------------------------|-----------------------------------------------------|
| `rdma.ome.io/auto-inject`    | Enables automatic RDMA injection                    |
| `rdma.ome.io/profile`        | Specifies the RDMA profile to use                   |
| `rdma.ome.io/container-name` | Specifies the container name for RDMA configuration |


### Knative Annotations

| Annotation                                       | Description                         |
|--------------------------------------------------|-------------------------------------|
| `autoscaling.knative.dev/min-scale`              | Sets the minimum number of replicas |
| `autoscaling.knative.dev/max-scale`              | Sets the maximum number of replicas |
| `serving.knative.dev/rollout-duration`           | Specifies the rollout duration      |
| `serving.knative.openshift.io/enablePassthrough` | Enables passthrough on OpenShift    |


## Labels

### Model and Runtime Labels

| Label                                    | Description                                  |
|------------------------------------------|----------------------------------------------|
| `base-model-name`                        | Base model name label                        |
| `base-model-size`                        | Base model size label                        |
| `base-model-type`                        | Base model type label                        |
| `base-model-vendor`                      | Base model vendor label                      |
| `fine-tuned-serving`                     | Fine-tuned serving label                     |
| `fine-tuned-serving-with-merged-weights` | Fine-tuned serving with merged weights label |
| `serving-runtime`                        | Serving runtime label                        |
| `fine-tuned-weight-ft-strategy`          | Fine-tuning strategy label                   |

### Scheduling Labels

| Label                          | Description                       |
|--------------------------------|-----------------------------------|
| `ray.io/scheduler-name`        | Ray scheduler name                |
| `ray.io/priority-class-name`   | Ray priority class name           |
| `raycluster/unavailable-since` | Ray cluster unavailable timestamp |
| `volcano.sh/queue-name`        | Volcano queue name                |
| `volcano.sh/job-name`          | Volcano job name                  |

### Kueue Labels

| Label                           | Description                    |
|---------------------------------|--------------------------------|
| `kueue.x-k8s.io/queue-name`     | Kueue queue name               |
| `kueue.x-k8s.io/priority-class` | Kueue workload priority class  |
| `kueue-enabled`                 | Enables Kueue for the resource |

### Model Agent Labels

| Label                                  | Description                       |
|----------------------------------------|-----------------------------------|
| `node.kubernetes.io/instance-type`     | Node instance shape               |
| `models.ome/{uid}`                     | Model label with UID              |
| `models.ome.io/target-instance-shapes` | Target instance shapes for models |
| `models.ome/basemodel-status`          | Base model status                 |

### Component Labels

| Label                     | Description                             |
|---------------------------|-----------------------------------------|
| `component`               | KService component label                |
| `endpoint`                | KService endpoint label                 |
| `ome.io/inferenceservice` | InferenceService label for TrainedModel |
| `ome.io/inferenceservice` | InferenceService pod label              |

### Network Visibility Labels

| Label                               | Description                      |
|-------------------------------------|----------------------------------|
| `networking.ome.io/visibility`      | Network visibility configuration |
| `networking.knative.dev/visibility` | Knative network visibility       |
| `sidecar.istio.io/inject`           | Istio sidecar injection          |


## Special Values

### Autoscaler Classes

- `hpa`: Horizontal Pod Autoscaler
- `keda`: Kubernetes Event-driven Autoscaling
- `external`: External autoscaler

### Scale Metrics

- `cpu`: CPU utilization
- `memory`: Memory utilization
- `concurrency`: Request concurrency (Knative)
- `rps`: Requests per second (Knative)


### Priority Classes

- `volcano-scheduling-high-priority`: High priority for Volcano scheduling
- `kueue-scheduling-high-priority`: High priority for Kueue workload scheduling 