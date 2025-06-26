---
title: "Overview"
linkTitle: "Overview"
weight: 1
description: >
  Why OME?
---

OME (Open Model Engine) is a Kubernetes operator designed to simplify and optimize the deployment and management of machine learning models in production environments. It provides a comprehensive solution for model lifecycle management, runtime optimization, service deployment, and intelligent resource scheduling.

## Core Capabilities

## Architecture

<img src="/ome/images/ome-architecture.svg" alt="OME Architecture" style="width: 100%; max-width: 2000px;" />

### 1. Model Management

OME offers a unified platform for managing various types of models, providing comprehensive lifecycle management from storage to deployment:

- **Multi-Format Support**: Handles diverse model formats including Hugging Face models, ONNX, TensorRT, and custom formats
- **Storage Backend Integration**: Seamlessly integrates with multiple storage solutions including OCI Object Storage, local file systems, and distributed storage systems
- **Security Features**: Built-in encryption for model artifacts, secure model distribution, and access control
- **Cross-Hardware Compatibility**: Automatically handles model distribution and optimization across different hardware accelerators (GPUs, TPUs, CPUs)
- **Version Control**: Comprehensive model versioning with rollback capabilities and A/B testing support

### 2. Runtime Configuration Management

OME intelligently selects and configures the optimal runtime environment based on model characteristics:

- **Automatic Runtime Selection**: Analyzes model properties (size, architecture, quantization) to choose the best serving runtime
- **Runtime Optimization**: Pre-configured optimizations for popular runtimes:
  - **SGLang**: First-class support with cache-aware load balancing, RadixAttention for prefix caching, and optimized kernel selection
- **Dynamic Configuration**: Adjusts runtime parameters based on workload patterns and resource availability
- **Custom Runtime Support**: Extensible framework for integrating custom model serving runtimes
- **Performance Profiling**: Continuous monitoring and optimization of runtime performance

### 3. Service Deployment and Management

OME automates the complex process of deploying ML models as scalable Kubernetes services:

- **Kubernetes Native**: Creates and manages all necessary Kubernetes resources (Deployments, Services, Ingresses, ConfigMaps)
- **Advanced Deployment Patterns**:
  - **Prefill-Decode Disaggregation**: Separates compute-intensive prefill operations from memory-bound decode operations for optimal resource utilization
  - **Multi-Node Inference**: Distributes large models across multiple GPUs/nodes with efficient communication
  - Canary deployments with traffic splitting
  - Blue-green deployments for zero-downtime updates
  - A/B testing with metric-based routing
- **Auto-scaling**: Intelligent scaling based on request patterns, GPU utilization, and custom metrics
- **Service Mesh Integration**: Native integration with Istio for advanced traffic management and security
- **Multi-Model Serving**: Efficient serving of multiple models on the same infrastructure with resource isolation
- **Multi-LoRA Support**: Efficiently serves multiple LoRA adapters on the same base model

### 4. Intelligent Scheduling and Resource Optimization

OME implements sophisticated scheduling algorithms to maximize resource utilization:

- **Bin-Packing Algorithm**: Optimally packs model workloads onto available GPUs to maximize utilization
- **Dynamic Rescheduling**: Continuously rebalances workloads based on real-time usage patterns
- **GPU Sharing**: Enables multiple models to share GPU resources with performance isolation
- **Heterogeneous Hardware Support**: Intelligently schedules across different GPU types and generations
- **Priority-Based Scheduling**: Ensures critical models get resources while maximizing overall cluster efficiency
- **Spot Instance Support**: Leverages spot/preemptible instances for cost optimization with automatic failover

## Additional Features

- 💰 **Cost Optimization**: Automatic resource right-sizing and spot instance utilization
- 🔒 **Enterprise Security**: mTLS, RBAC, and audit logging for compliance requirements
- 📊 **Comprehensive Observability**: Integrated metrics, logging, and tracing for all components
- 🌐 **Multi-Region Support**: Deploy and manage models across multiple Kubernetes clusters
- 🛠️ **Extensible Architecture**: Plugin system for custom schedulers, runtimes, and storage backends
- 🚀 **Automated Benchmarking**: Built-in BenchmarkJob resource for systematic performance evaluation
- 🔄 **Kubernetes Ecosystem Integration**: Deep integration with Kueue, LeaderWorkerSet, KEDA, Gateway API, and K8s Inference Service
