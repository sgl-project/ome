---
title: "Installation"
linkTitle: "Installation"
weight: 2
description: >
  Installing OME to a Kubernetes Cluster
---

<!-- toc -->
- [Before you begin](#before-you-begin)
  - [Required Components](#required-components)
  - [Optional Components](#optional-components)
  - [1. Install Istio](#1-install-istio)
  - [2. Install Cert Manager](#2-install-cert-manager)
  - [3. Install Knative Serving (Optional - Serverless mode only)](#3-install-knative-serving-optional---serverless-mode-only)
  - [4. Install KEDA (Optional - Custom metrics scaling)](#4-install-keda-optional---custom-metrics-scaling)
  - [5. Install Prometheus (Optional - Custom metrics scaling)](#5-install-prometheus-optional---custom-metrics-scaling)
  - [6. Install LeaderWorkerSet (Optional - MultiNode mode only)](#6-install-leaderworkerset-optional---multinode-mode-only)
  - [7. Install Kueue (Optional - Job scheduling)](#7-install-kueue-optional---job-scheduling)
  - [9. Clone OME repository](#8-clone-ome-repository)
- [Install the latest development version](#install-the-latest-development-version)
  - [Uninstall](#uninstall)
<!-- /toc -->

## Before you begin

OME supports multiple deployment modes to enable `InferenceService` deployment with Kubernetes resources:

- **`RawDeployment`** (Default): Uses standard Kubernetes Deployment, Service, Ingress and HorizontalPodAutoscaler. Supports mounting multiple volumes but does not support scale to/from zero. Optionally supports custom metrics scaling with KEDA and Prometheus.
- **`Serverless`**: Enables autoscaling based on request volume with scale down to and from zero. Supports revision management and canary rollout. **Requires: Knative Serving and Istio**.
- **`MultiNode`**: Enables multi-node deployment for models that require distributed computing. **Requires: LeaderWorkerSet (LWS)**.
- **`PDDisaggregated`**: Enables prefill-decode disaggregated deployment for models that require most optimal performance. **Requires: LeaderWorkerSet (LWS) for larger models that require distributed computing**.

### Required Components

Make sure the following conditions are met:

- A Kubernetes cluster with version 1.27 or newer is running. Learn how to [install the Kubernetes tools](https://kubernetes.io/docs/tasks/tools/).
- The kubectl command-line tool has communication with your cluster.
- The cluster has a [cert-manager](https://cert-manager.io/docs/installation/) installed (minimum version 1.9.0).

### Optional Components

The following components are optional and only required for specific features:

| Component                 | Required For                      | Description                                                |
|---------------------------|-----------------------------------|------------------------------------------------------------|
| **Istio**                 | Serverless mode, Virtual Services | Service mesh for traffic management (minimum version 1.19) |
| **Knative Serving**       | Serverless mode                   | Serverless container deployment and serving                |
| **KEDA**                  | Custom metrics autoscaling        | Kubernetes Event-driven Autoscaling                        |
| **Prometheus**            | Custom metrics autoscaling        | Metrics collection and monitoring                          |
| **LeaderWorkerSet (LWS)** | MultiNode, MultiNodeRayVLLM modes | Kubernetes API for distributed training workloads          |
| **Kueue**                 | Job scheduling                    | Kubernetes-native job queueing                             |

!!! warning
    **Important**: If you plan to use `MultiNode` or `MultiNodeRayVLLM` deployment modes, you MUST install the corresponding optional components (Ray and/or LWS) BEFORE installing OME. The controller may panic if these CRDs are not available when needed.

### 1. Install Istio

**Optional - Required only for Serverless mode and Virtual Service ingress**

The minimally required Istio version is `1.19` and you can refer to the [Istio install guide](https://istio.io/latest/docs/setup/install).

Once Istio is installed, create `IngressClass` resource for istio:
```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: istio
spec:
  controller: istio.io/ingress-controller
```

!!! Note
    If you are running on a managed Kubernetes service, you can use the managed Istio service provided by the cloud provider.

!!! Note
    Istio ingress is recommended for Serverless mode, but you can choose to install with other [Ingress controllers](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) and create `IngressClass` resource for your Ingress option.

### 2. Install Cert Manager

**Required**

The minimally required Cert Manager version is 1.9.0, and you can refer to [Cert Manager installation guide](https://cert-manager.io/docs/installation/).

!!! Note
    Cert manager is required to provision webhook certs for production grade installation. Alternatively, you can run a self-signed certs generation script.

### 3. Install Knative Serving (Optional - Serverless mode only)

**Optional - Required only for Serverless deployment mode**

Please refer to [Knative Serving install guide](https://knative.dev/docs/admin/install/serving/install-serving-with-yaml/).

!!! note
    If you are looking to use PodSpec fields such as nodeSelector, affinity or tolerations which are now supported in the v1beta1 API spec,
    you need to turn on the corresponding [feature flags](https://knative.dev/docs/admin/serving/feature-flags) in your Knative configuration.

!!! note
    If you are using private registry for your images, you need to configure knative to skip resolve image digest.

```bash
kubectl -n knative-serving edit configmap config-deployment
```

Add the following to the `data` section:

```yaml
data:
  registriesSkippingTagResolving: ko.local, dev.local, ghcr.io
```

### 4. Install KEDA (Optional - Custom metrics scaling)

**Optional - Required only for custom metrics autoscaling**

Please refer to [KEDA install guide](https://keda.sh/docs/2.6/deploy/).

### 5. Install Prometheus (Optional - Custom metrics scaling)

**Optional - Required only for custom metrics autoscaling with KEDA**

1. Get Helm Repository Information
```shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```
2. Install kube-prometheus-stack
```shell
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack
```


### 6. Install LeaderWorkerSet (Optional - MultiNode mode only)

**Optional - Required for both MultiNode and MultiNodeRayVLLM deployment modes**

Please refer to [LeaderWorkerSet installation guide](https://github.com/kubernetes-sigs/lws).

Example installation:
```shell
kubectl apply --server-side -f https://github.com/kubernetes-sigs/lws/releases/download/v0.3.0/lws-webhook.yaml
```

### 7. Install Kueue (Optional - Job scheduling)

**Optional - Required only for advanced job scheduling features**

Please refer to [Kueue installation guide](https://kueue.sigs.k8s.io/docs/installation/).

### 8. Clone OME repository

The Go tools require that you clone the repository to the
`src/github.com/sgl-project/ome` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [clone this repo](https://docs.github.com/en/repositories/creating-and-managing-repositories/cloning-a-repository)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/sgl-project
cd ${GOPATH}/src/github.com/sgl-project
git clone https://github.com/sgl-project/ome.git
cd ome
```

Once you reach this point, you are ready to do a full build and deploy as
described below.


## Install the latest development version

To install the latest development version of OME in your cluster, run the
following command:

```shell
make install
```

The controller runs in the `ome` namespace.


### Uninstall

To uninstall OME, run the following command:

```shell
make uninstall
```
