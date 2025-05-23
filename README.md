# Welcome to OME

<a name="readme-top"></a>


<!-- PROJECT LOGO -->
<br />
<div align="center">
  <a href="https://github.com/github_username/repo_name">
    <img src="site/assets/ome-logo-hd.png" alt="Logo" width="" height="300">
  </a>

<h3 align="center">Open Model Engine</h3>

  <p align="center">
Enterprise-Grade AI/ML Workload Management Platform on Kubernetes
    <br />
    <a href="https://PENDING"><strong>Explore the docs »</strong></a>
    <br />
  </p>
</div>


OME is a comprehensive operator for managing the lifecycle of Large Language Models (LLMs) and other AI workloads in Kubernetes environments. It orchestrates model serving, training, benchmarking, and dedicated AI clusters with enterprise-grade resource management.

## Architecture
<p align="center"><img src="site/static/images/architecture.drawio.svg" alt="Architecture Diagram" width="" height=""></p>

### Infrastructure Layers

1. **Hardware Layer**: Built on OCI infrastructure with high-performance RDMA networks, GPUs, CPUs, and NVMe storage.

2. **Kubernetes Layer**: Leverages OCI Kubernetes Engine (OKE) for container orchestration with hardware-level optimizations.

3. **OME Core Components**: Provides specialized Kubernetes operators for managing AI/ML workflows:

   - **Model Management**: Manages model lifecycle, from import to versioning, with support for various model formats and architectures including large-scale models (LLaMA, Mistral, Mixtral, etc.)
     - **Dynamic Model Parsing**: Automatically analyzes model files to extract architecture, parameter count, and capabilities directly from safetensors files
   
   - **Inference Services**: Deploys models as inference services with flexible scaling options, from serverless to dedicated resources
   
   - **Training System**: Orchestrates distributed training jobs with support for popular frameworks
   
   - **Resource Management**: Controls GPU allocation through Capacity Reservations and Dedicated AI Clusters with cache-aware resource allocation

   - **Performance Analysis**: Provides benchmarking tools for model evaluation and optimization

### Runtime Support

- **Serving Runtimes**: Integrates with popular inference engines including vLLM, SGLang, TGI, NIM, Triton, and more
   - **Intelligent Runtime Selection**: Automatically selects the optimal runtime based on model architecture, model type, format, quantization, and parameter size
   - **Advanced Model Parsing**: Built-in support for parsing configurations from popular model families including LLaMA, Mistral, Mixtral, DeepSeek, Phi, and Qwen

- **Training Frameworks**: Supports Accelerate, DeepSpeed, PyTorch, TensorFlow, and MPI-based systems 

- **Deployment Patterns**:
   - **PD Deployment**: Prefill-Decode disaggregated serving for efficient token generation
   - **Multi-Node Serving**: Distributed inference across multiple nodes
   - **Serverless**: On-demand scaling with zero idle resources
   - **Multi-Node Training**: Distributed training with gang scheduling
   - **Cache-Aware Load Balancing**: Intelligent routing to optimize model cache utilization

## Key Features

- 🚀 **Model Management**: Comprehensive model registry with support for different formats, architectures, and model types. Supports both OCI Object Storage and Hugging Face Hub as model sources. Features sophisticated model file parsing to automatically determine model architecture, size, and capabilities.

- 🔀 **Inference Graphs**: Create complex inference workflows with routing patterns (sequence, splitter, ensemble, switch) for chaining models together.

- 🔌 **Flexible Deployment Options**: From serverless to dedicated clusters with fine-grained resource control.

- 💰 **Advanced Autoscaling**: Support for serverless workloads with scale-to-zero capabilities and KEDA integration.

- 🔒 **Enterprise Security**: Built-in mTLS, RBAC, and compartment isolation for secure multi-tenant deployments.

- 🌐 **Distributed Computing**: First-class support for multi-node model serving and distributed training.

- 📏 **Resource Management**: Capacity reservation system for dedicated resource allocation and efficient resource sharing.

- 📊 **Benchmarking**: Built-in performance testing for model evaluation and comparison.

- 💾 **Optimized Storage**: Efficient model storage and loading with distributed model caching leveraging high-performance NVMe storage for optimal performance.

## Core Components

- **Model System**: Manages base models and fine-tuned weights with rich metadata

- **Inference Service**: Deploys models with customizable configurations and auto-scaling

- **Serving Runtime**: Integrates various serving technologies (vLLM, TGI, Cohere, TensorRT-LLM, etc.)

- **Training Job**: Orchestrates model training with framework-specific optimizations

- **Benchmark Job**: Evaluates model performance with standardized metrics

- **Capacity Reservation**: Allocates and manages compute resources

- **Dedicated AI Cluster**: Creates isolated environments for specialized workloads

## Documentation

- [Contributing Guidelines](CONTRIBUTING.md) - Learn how to contribute to OME
- [Roadmap](Roadmap.md) - Check out our future plans and upcoming features

[Back to top](#readme-top)