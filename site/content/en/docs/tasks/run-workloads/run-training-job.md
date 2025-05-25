---
title: "Run a Simple Training Job"
linkTitle: "Run Training Job"
weight: 1
date: 2023-03-14
description: >
  Learn how to run your first training job to fine-tune a model with OME.
---

This page shows you how to run a simple training job using OME. You'll learn how to create a TrainingJob that fine-tunes a pre-trained model with your custom dataset using distributed training across multiple GPUs.

## Before you begin

You need to have the following:

- A Kubernetes cluster with OME installed
- `kubectl` configured to communicate with your cluster
- GPU nodes available in your cluster (preferably A100, H100, or H200)
- Access to training datasets (stored in OCI Object Storage or PVC)
- A base model to fine-tune
- Training runtime configured

## Step 1: Verify prerequisites

Check that OME is installed and running:

```bash
kubectl get pods -n ome
```

Check available training runtimes:

```bash
kubectl get clustertrainingruntimes
```

Example output:
```
NAME                               AGE
trt-pytorch-distributed           1d
trt-deepspeed-zero3               1d
trt-accelerate-multi-gpu          1d
```

Verify GPU availability for training:

```bash
kubectl get nodes -o custom-columns="NAME:.metadata.name,GPU:.status.allocatable.nvidia\.com/gpu"
```

## Step 2: Prepare your training data

### Option A: Using OCI Object Storage

Create a secret with OCI credentials:

```bash
kubectl create secret generic oci-credentials \
  --from-file=config=${HOME}/.oci/config \
  --from-file=key=${HOME}/.oci/private-key.pem \
  -n training-demo
```

### Option B: Using Persistent Volume

Create a PVC for training data:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: training-data-pvc
  namespace: training-demo
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 500Gi
  storageClassName: oci-fss
```

## Step 3: Create a simple fine-tuning job

Let's start with a single-node training job for a smaller model:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: training-demo
---
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: llama-3-2-3b-finetune
  namespace: training-demo
spec:
  runtime: trt-pytorch-distributed
  baseModel: llama-3-2-3b-instruct
  trainingSpec:
    trainer:
      framework: pytorch
      distributedStrategy: ddp
      parameters:
        learning_rate: "2e-5"
        batch_size: "4"
        gradient_accumulation_steps: "8"
        num_train_epochs: "3"
        max_seq_length: "2048"
        warmup_steps: "100"
        save_steps: "500"
        eval_steps: "500"
        logging_steps: "100"
    dataset:
      train:
        type: "text-generation"
        source:
          storageUri: "oci://your-namespace/your-bucket/training-data/train.jsonl"
          secretRef:
            name: oci-credentials
      validation:
        source:
          storageUri: "oci://your-namespace/your-bucket/training-data/validation.jsonl"
          secretRef:
            name: oci-credentials
    outputLocation:
      storageUri: "oci://your-namespace/your-bucket/model-outputs/llama-3-2-3b-finetune/"
      secretRef:
        name: oci-credentials
    resources:
      requests:
        cpu: "32"
        memory: 128Gi
        nvidia.com/gpu: 2
      limits:
        cpu: "32"
        memory: 128Gi
        nvidia.com/gpu: 2
    nodeSelector:
      node.kubernetes.io/instance-type: BM.GPU.A100-v2.8
    tolerations:
      - key: "nvidia.com/gpu"
        operator: "Exists"
        effect: "NoSchedule"
EOF
```

## Step 4: Monitor training progress

Check the training job status:

```bash
kubectl get trainingjob -n training-demo
```

Monitor the training pods:

```bash
kubectl get pods -n training-demo -w
```

View training logs:

```bash
kubectl logs -n training-demo -l job-name=llama-3-2-3b-finetune -f
```

Check detailed job status:

```bash
kubectl describe trainingjob -n training-demo llama-3-2-3b-finetune
```

## Step 5: Multi-node distributed training

For larger models or datasets, use multi-node distributed training:

```bash
kubectl apply -f - <<EOF
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: llama-3-3-70b-finetune
  namespace: training-demo
  annotations:
    ome.io/trainingMode: "MultiNode"
spec:
  runtime: trt-deepspeed-zero3
  baseModel: llama-3-3-70b-instruct
  trainingSpec:
    trainer:
      framework: deepspeed
      distributedStrategy: zero3
      worldSize: 16  # Total number of GPUs across all nodes
      parameters:
        learning_rate: "1e-5"
        micro_batch_size: "1"
        gradient_accumulation_steps: "16"
        num_train_epochs: "2"
        max_seq_length: "4096"
        deepspeed_config: |
          {
            "train_batch_size": "auto",
            "train_micro_batch_size_per_gpu": "auto",
            "gradient_accumulation_steps": "auto",
            "zero_optimization": {
              "stage": 3,
              "offload_optimizer": {
                "device": "cpu",
                "pin_memory": true
              },
              "offload_param": {
                "device": "cpu",
                "pin_memory": true
              }
            },
            "fp16": {
              "enabled": true
            },
            "gradient_clipping": 1.0,
            "wall_clock_breakdown": false
          }
    dataset:
      train:
        type: "text-generation"
        source:
          storageUri: "oci://your-namespace/your-bucket/large-dataset/train/"
          secretRef:
            name: oci-credentials
    outputLocation:
      storageUri: "oci://your-namespace/your-bucket/model-outputs/llama-3-3-70b-finetune/"
      secretRef:
        name: oci-credentials
    resources:
      requests:
        cpu: "64"
        memory: 256Gi
        nvidia.com/gpu: 8
      limits:
        cpu: "64"
        memory: 256Gi
        nvidia.com/gpu: 8
    replicas: 2  # Number of nodes
    nodeSelector:
      node.kubernetes.io/instance-type: BM.GPU.H100.8
    tolerations:
      - key: "nvidia.com/gpu"
        operator: "Exists"
        effect: "NoSchedule"
EOF
```

## Step 6: Hyperparameter optimization

Run hyperparameter tuning to find optimal training settings:

```bash
kubectl apply -f - <<EOF
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: llama-hpo-experiment
  namespace: training-demo
spec:
  runtime: trt-pytorch-distributed
  baseModel: llama-3-2-3b-instruct
  hyperparameterOptimization:
    enabled: true
    algorithm: "bayesian"
    maxTrials: 20
    parallelTrials: 4
    objective:
      type: "minimize"
      metric: "validation_loss"
    parameters:
      - name: "learning_rate"
        type: "double"
        min: 1e-6
        max: 1e-3
      - name: "batch_size"
        type: "discrete"
        values: [2, 4, 8, 16]
      - name: "warmup_ratio"
        type: "double"
        min: 0.01
        max: 0.2
  trainingSpec:
    trainer:
      framework: pytorch
      distributedStrategy: ddp
      parameters:
        num_train_epochs: "1"  # Shorter for HPO
        max_seq_length: "2048"
        gradient_accumulation_steps: "4"
    dataset:
      train:
        source:
          storageUri: "oci://your-namespace/your-bucket/training-data/train.jsonl"
          secretRef:
            name: oci-credentials
      validation:
        source:
          storageUri: "oci://your-namespace/your-bucket/training-data/validation.jsonl"
          secretRef:
            name: oci-credentials
    resources:
      requests:
        nvidia.com/gpu: 1
EOF
```

## Advanced Training Configurations

### LoRA Fine-tuning

For efficient fine-tuning with Low-Rank Adaptation:

```yaml
spec:
  trainingSpec:
    trainer:
      framework: pytorch
      adaptationMethod: lora
      parameters:
        lora_rank: "16"
        lora_alpha: "32"
        lora_dropout: "0.1"
        target_modules: ["q_proj", "v_proj", "k_proj", "o_proj"]
        learning_rate: "3e-4"
        batch_size: "8"
```

### QLoRA with 4-bit Quantization

For memory-efficient training:

```yaml
spec:
  trainingSpec:
    trainer:
      framework: pytorch
      adaptationMethod: qlora
      parameters:
        load_in_4bit: "true"
        bnb_4bit_compute_dtype: "bfloat16"
        bnb_4bit_use_double_quant: "true"
        bnb_4bit_quant_type: "nf4"
        lora_rank: "64"
        lora_alpha: "128"
```

### Custom Training Script

Use your own training script:

```yaml
spec:
  trainingSpec:
    trainer:
      framework: custom
      image: "your-registry/custom-trainer:latest"
      command: ["python", "/app/train.py"]
      args:
        - "--model-name=$(MODEL_NAME)"
        - "--data-path=$(DATA_PATH)"
        - "--output-dir=$(OUTPUT_DIR)"
    customScript:
      configMap:
        name: training-script
        key: train.py
```

## Monitoring and Debugging

### View Training Metrics

If TensorBoard is enabled:

```bash
kubectl port-forward -n training-demo svc/llama-3-2-3b-finetune-tensorboard 6006:6006
```

Access TensorBoard at `http://localhost:6006`

### Check GPU Utilization

```bash
kubectl exec -n training-demo <training-pod> -- nvidia-smi
```

### Debug Training Issues

**Check resource allocation:**
```bash
kubectl describe node <gpu-node> | grep nvidia.com/gpu
```

**View detailed pod events:**
```bash
kubectl describe pod -n training-demo <training-pod>
```

**Check storage access:**
```bash
kubectl exec -n training-demo <training-pod> -- ls -la /data/
```

## Training Data Formats

### Text Generation (Chat/Instruct)

```json
{"messages": [{"role": "user", "content": "What is AI?"}, {"role": "assistant", "content": "AI stands for Artificial Intelligence..."}]}
{"messages": [{"role": "user", "content": "Explain quantum computing"}, {"role": "assistant", "content": "Quantum computing uses quantum mechanics..."}]}
```

### Text Completion

```json
{"text": "The capital of France is Paris."}
{"text": "Machine learning is a subset of artificial intelligence that..."}
```

### Instruction Following

```json
{"instruction": "Summarize the following text:", "input": "Long text to summarize...", "output": "Brief summary..."}
{"instruction": "Translate to French:", "input": "Hello world", "output": "Bonjour le monde"}
```

## Performance Optimization

### Memory Optimization

```yaml
spec:
  trainingSpec:
    trainer:
      parameters:
        gradient_checkpointing: "true"
        dataloader_pin_memory: "true"
        dataloader_num_workers: "8"
        fp16_full_eval: "true"
```

### Multi-Node Communication

For RDMA-enabled clusters:

```yaml
spec:
  trainingSpec:
    networking:
      backend: "nccl"
      rdmaEnabled: true
    nodeSelector:
      oci.oraclecloud.com/rdma.authenticated: "16"
    hostNetwork: true
    dnsPolicy: ClusterFirstWithHostNet
```

### Checkpointing and Recovery

```yaml
spec:
  trainingSpec:
    checkpointing:
      enabled: true
      saveFrequency: "500"  # Save every 500 steps
      maxCheckpoints: 3
      resumeFromCheckpoint: "auto"  # Auto-resume from latest checkpoint
```

## Supported Training Frameworks

### PyTorch Distributed

- **Single-node multi-GPU**: DistributedDataParallel (DDP)
- **Multi-node**: DDP with NCCL backend
- **Memory optimization**: Gradient checkpointing, mixed precision

### DeepSpeed

- **ZeRO Stage 1**: Optimizer state partitioning
- **ZeRO Stage 2**: Gradient partitioning
- **ZeRO Stage 3**: Parameter partitioning
- **CPU offloading**: For very large models

### Accelerate (Hugging Face)

- **Multi-GPU**: Automatic device placement
- **FSDP**: Fully Sharded Data Parallel
- **Integration**: Seamless with Transformers library

## Next Steps

After your training job completes:

- [Deploy the Fine-tuned Model](/docs/tasks/run-workloads/deploy-custom-model/) - Serve your trained model
- [Run Model Evaluation](/docs/tasks/run-workloads/evaluate-model/) - Assess model quality
- [Setup Model Registry](/docs/tasks/manage-ome/setup-model-registry/) - Manage model versions
- [Performance Benchmarking](/docs/tasks/run-workloads/run-benchmarks/) - Test inference performance

## Cleanup

To remove training resources:

```bash
kubectl delete trainingjob -n training-demo llama-3-2-3b-finetune
kubectl delete namespace training-demo
``` 