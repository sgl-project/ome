---
title: "Installation"
linkTitle: "Installation"
weight: 2
description: >
  Installing Kueue to a Kubernetes Cluster
---

<!-- toc -->
- [Before you begin](#before-you-begin)
  - [1. Install Istio](#1-install-istio)
  - [2. Install Cert Manager](#2-install-cert-manager)
  - [3. Install Knative Serving](#3-install-knative-serving)
  - [4. Install KEDA through Helm](#4-install-keda-through-helm)
  - [5. Install Prometheus](#5-install-prometheus)
  - [6. Clone OME repository](#6-clone-ome-repository)
- [Install the latest development version](#install-the-latest-development-version)
  - [Uninstall](#uninstall)
<!-- /toc -->

## Before you begin

OME supports `RawDeployment`, `Serverless`,
and `MultiNodeRayVLLM` mode to enable `InferenceService` deployment with Kubernetes resources Deployment,
Service, Ingress and Horizontal Pod Autoscaler.
- `RawDeployment`, comparing to serverless deployment, unlocks Knative limitations such as mounting multiple volumes,
on the other hand Scale down and from Zero is not supported in RawDeployment mode. `RawDeployment` mode also supports scaling based on custom metrics with KEDA and Prometheus.
- `Serverless` installation enables autoscaling based on request volume and supports scale down to and from zero. It also supports revision management
  and canary rollout based on revisions.
- `MultiNodeRayVLLM` mode enables deploying a Ray cluster with multiple nodes and a VLLM model serving with InferenceService. This mode does not support auto-scaling or canary deployment.

Kubernetes 1.27.1 is the minimally required version, and please check the following recommended Istio versions for the corresponding Kubernetes version.

Make sure the following conditions are met:

- A Kubernetes cluster with version 1.27 or newer is running. Learn how to [install the Kubernetes tools](https://kubernetes.io/docs/tasks/tools/).
- The kubectl command-line tool has communication with your cluster.
- The cluster has a [cert-manager](https://cert-manager.io/docs/installation/) installed.
- The cluster has knative-serving and Istio installed for Serverless mode.
- The cluster has a [KEDA](https://keda.sh/docs/2.6/deploy/) and [Prometheus](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) installed for custom metrics scaling.
- The cluster has a [Ray](https://docs.ray.io/en/latest/serve/deployment/kubernetes.html) installed for MultiNodeRayVLLM mode.

### 1. Install Istio
This is required for Serverless mode.
The minimally required Istio version is `1.19` and you can refer to the [Istio install guide](https://istio.io/latest/docs/setup/install).

Once Istio is installed, create `IngressClass` resource for istio.
```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: istio
spec:
  controller: istio.io/ingress-controller
```

!!! Note
If you are running on a managed Kubernetes service like OKE, you can use the managed Istio service provided by the cloud provider.

!!! Note
Istio ingress is recommended, but you can choose to install with other [Ingress controllers](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) and create `IngressClass` resource for your Ingress option.



### 2. Install Cert Manager
The minimally required Cert Manager version is 1.9.0, and you can refer to [Cert Manager installation guide](https://cert-manager.io/docs/installation/).

!!! Note
Cert manager is required to provision webhook certs for production grade installation. Alternatively, you can run a self-signed certs generation script.

### 3. Install Knative Serving
Please refer to [Knative Serving install guide](https://knative.dev/docs/admin/install/serving/install-serving-with-yaml/).
This is required for Serverless mode mode.
!!! note
If you are looking to use PodSpec fields such as nodeSelector, affinity or tolerations which are now supported in the v1beta1 API spec,
you need to turn on the corresponding [feature flags](https://knative.dev/docs/admin/serving/feature-flags) in your Knative configuration.

!!! note
If you are using private registry for your images, you need to configure the knative to skip resolve image digest.

```bash
kubectl -n knative-serving edit configmap config-deployment
```

add the following to the `data` section:

```yaml
data:
  registriesSkippingTagResolving: ko.local, dev.local, ord.ocir.io, us-chicago-1.ocir.io
```

### 4. Install KEDA through Helm
Please refer to [KEDA install guide](https://keda.sh/docs/2.6/deploy/).


### 5. Install Prometheus
1. Get Helm Repository Information
```shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
````
2. Install kube-prometheus-stack
```shell
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack
```

### 6. Clone OME repository

The Go tools require that you clone the repository to the
`src/bitbucket.oci.oraclecorp.com/genaicore/ome` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [clone this repo](https://support.atlassian.com/bitbucket-cloud/docs/clone-a-git-repository/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/bitbucket.oci.oraclecorp.com/gen
cd ${GOPATH}/src/bitbucket.oci.oraclecorp.com/gen
git clone ssh://git@bitbucket.oci.oraclecorp.com:7999/gen/ome.git
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
