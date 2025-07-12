---
title: "Benchmark"
date: 2023-03-14
weight: 35
description: >
  BenchmarkJob is a resource that manages automated performance benchmarking of inference services.
---

A _BenchmarkJob_ is a resource in OME that automates the performance benchmarking of inference service or OCI Generative AI Service endpoints. It allows you to evaluate model serving performance under various traffic patterns and load conditions.

BenchmarkJob uses [genai-bench](https://docs.sglang.ai/genai-bench/), a comprehensive benchmarking tool for evaluating generative AI model serving systems. For detailed information about genai-bench features and capabilities, refer to the [official genai-bench documentation](https://docs.sglang.ai/genai-bench/).

## Core Components

A BenchmarkJob consists of several key components:

1. **Endpoint Configuration**: Specifies the target inference service to benchmark
2. **Traffic Patterns**: Defines the load testing scenarios
3. **Resource Configuration**: Controls the benchmark execution environment
4. **Output Management**: Handles benchmark results storage

## Example Configuration

Here's an example of a BenchmarkJob configuration using OCI Object Storage:

```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
   name: llama-3-1-70b-benchmark
   namespace: llama-3-1-70b
spec:
   podOverride:
      image: "ghcr.io/sgl-project/genai-bench:0.1.127"
      resources:
         requests:
            cpu: "4"
            memory: "16Gi"
         limits:
            cpu: "4"
            memory: "16Gi"
   endpoint:
      inferenceService:
         name: llama-3-1-70b-instruct
         namespace: llama-3-1-70b-instruct
   task: text-to-text
   trafficScenarios:
      - "N(480,240)/(300,150)"
      - "D(100,100)"
      - "D(100,1000)"
      - "D(2000,200)"
      - "D(7800,200)"
   numConcurrency:
      - 1
      - 2
      - 4
      - 8
      - 16
      - 32
      - 64
      - 128
      - 256
   maxTimePerIteration: 15
   maxRequestsPerIteration: 100
   additionalRequestParams:
      temperature: "0.0"
   outputLocation:
      storageUri: "oci://n/idqj093njucb/b/ome-benchmark-results/o/llama-3-1-70b-benchmark"
      parameters:
         auth: "instance_principal"
```

### Example with AWS S3 Storage

```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
   name: model-benchmark-s3
   namespace: default
spec:
   endpoint:
      endpoint:
         url: "http://my-model-service:8080/v1/completions"
         apiFormat: "openai"
         modelName: "llama-3"
   task: text-to-text
   numConcurrency: [1, 4, 8, 16]
   maxTimePerIteration: 10
   maxRequestsPerIteration: 100
   outputLocation:
      storageUri: "s3://my-benchmarks@us-east-1/experiments/2024"
      parameters:
         aws_profile: "production"  # Or use aws_access_key_id and aws_secret_access_key
```

## Spec Attributes

Available attributes in the BenchmarkJob spec:

| Attribute                 | Description                                              |
|---------------------------|----------------------------------------------------------|
| `endpoint`                | Required. Target inference service configuration         |
| `task`                    | Required. Type of task to benchmark (e.g., text-to-text) |
| `trafficScenarios`        | Optional. List of traffic patterns to test               |
| `numConcurrency`          | Optional. List of concurrency levels to test             |
| `maxTimePerIteration`     | Required. Maximum time per test iteration                |
| `maxRequestsPerIteration` | Required. Maximum requests per iteration                 |
| `serviceMetadata`         | Optional. Backend service information                    |
| `outputLocation`          | Required. Where to store benchmark results               |
| `podOverride`             | Optional. Benchmark pod configuration                    |

## Endpoint Configuration

BenchmarkJob supports two types of endpoints:

1. **InferenceService Reference**:
```yaml
endpoint:
  inferenceService:
    name: my-model
    namespace: default
```

2. **Direct URL Endpoint**:
```yaml
endpoint:
  endpoint:
    url: "http://my-model-service:8080/v1/completions"
    apiFormat: "openai"
    modelName: "my-model"
```

## Storage Configuration

BenchmarkJob supports storing benchmark results in multiple cloud storage providers. The storage configuration is specified in the `outputLocation` field.

### Supported Storage Providers

#### 1. OCI Object Storage
```yaml
outputLocation:
  storageUri: "oci://n/my-namespace/b/my-bucket/o/benchmark-results"
  parameters:
    auth: "instance_principal"  # Authentication type
    config_file: "/path/to/config"  # Optional: Config file for user_principal auth
    profile: "DEFAULT"  # Optional: Profile name for user_principal auth
    security_token: "token"  # Optional: Token for security_token auth
    region: "us-phoenix-1"  # Optional: Region for security_token auth
```

#### 2. AWS S3
```yaml
outputLocation:
  storageUri: "s3://my-bucket/path/to/results"
  # Or with region: "s3://my-bucket@us-west-2/path/to/results"
  parameters:
    aws_access_key_id: "AKIAIOSFODNN7EXAMPLE"  # Optional: AWS access key
    aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG"  # Optional: AWS secret key
    aws_profile: "production"  # Optional: AWS profile name
    aws_region: "us-east-1"  # Optional: AWS region
```

#### 3. Azure Blob Storage
```yaml
outputLocation:
  storageUri: "az://myaccount/mycontainer/path/to/results"
  # Or: "az://myaccount.blob.core.windows.net/mycontainer/path/to/results"
  parameters:
    azure_account_name: "myaccount"  # Optional: Storage account name
    azure_account_key: "YOUR_KEY"  # Optional: Account key
    azure_connection_string: "DefaultEndpointsProtocol=..."  # Optional: Connection string
    azure_sas_token: "?sv=..."  # Optional: SAS token
```

#### 4. Google Cloud Storage
```yaml
outputLocation:
  storageUri: "gs://my-bucket/path/to/results"
  parameters:
    gcp_project_id: "my-project-123"  # Optional: GCP project ID
    gcp_credentials_path: "/path/to/service-account.json"  # Optional: Service account path
```

#### 5. GitHub Releases
```yaml
outputLocation:
  storageUri: "github://owner/repo@v1.0.0"  # @tag is optional, defaults to "latest"
  parameters:
    github_token: "ghp_xxxxxxxxxxxx"  # Required: GitHub personal access token
```

#### 6. Persistent Volume Claim (PVC)
```yaml
outputLocation:
  storageUri: "pvc://my-pvc/results"
  # No additional parameters needed
```

### Storage URI Formats

| Provider | URI Format | Example |
|----------|------------|---------|
| OCI | `oci://n/{namespace}/b/{bucket}/o/{path}` | `oci://n/myns/b/mybucket/o/results` |
| S3 | `s3://{bucket}[@{region}]/{path}` | `s3://mybucket@us-west-2/results` |
| Azure | `az://{account}/{container}/{path}` | `az://myaccount/mycontainer/results` |
| GCS | `gs://{bucket}/{path}` | `gs://mybucket/results` |
| GitHub | `github://{owner}/{repo}[@{tag}]` | `github://myorg/myrepo@v1.0.0` |
| PVC | `pvc://[{namespace}:]{pvc-name}/{path}` | `pvc://my-pvc/results` or `pvc://default:my-pvc/results` |

### Authentication Options

#### OCI Authentication
- `user_principal`: Uses OCI config file credentials (requires `config_file` and optionally `profile`)
- `instance_principal`: Uses instance credentials
- `security_token`: Uses security token authentication (requires `security_token` and `region`)
- `instance_obo_user`: Uses instance principal on behalf of user

#### AWS Authentication
- IAM credentials via `aws_access_key_id` and `aws_secret_access_key`
- AWS profile via `aws_profile`
- Environment variables or IAM roles (when no parameters specified)

#### Azure Authentication
- Storage account key via `azure_account_key`
- Connection string via `azure_connection_string`
- SAS token via `azure_sas_token`
- Azure AD authentication (when no parameters specified)

#### GCP Authentication
- Service account via `gcp_credentials_path`
- Application default credentials (when no parameters specified)

#### GitHub Authentication
- Personal access token via `github_token` (required)

## Reconciliation Process

The BenchmarkJob controller performs several steps during reconciliation:

1. **Resource Preparation**:
   - Creates necessary PersistentVolumes and PersistentVolumeClaims
   - Sets up storage for model and benchmark data

2. **Job Creation**:
   - Generates benchmark pod specification
   - Configures resource requirements
   - Sets up environment variables

3. **Execution Management**:
   - Monitors job progress
   - Handles job completion and failures
   - Updates status with results

4. **Cleanup**:
   - Manages resource cleanup on completion
   - Handles proper deletion of resources

## Status

The BenchmarkJob status provides information about the benchmark execution:

```yaml
status:
  state: Running
  startTime: "2023-12-27T02:30:00Z"
  lastReconcileTime: "2023-12-27T02:35:00Z"
  details: "Running iteration 2/6: concurrency=5"
```

## Best Practices

1. **Resource Planning**:
   - Ensure benchmark pods have sufficient resources
   - Consider network bandwidth requirements

2. **Test Scenarios**:
   - Start with low concurrency and gradually increase
   - Use realistic traffic patterns
   - Test both average and peak loads

3. **Results Analysis**:
   - Monitor latency percentiles
   - Track throughput metrics
   - Analyze resource utilization

4. **Storage Management**:
   - Use appropriate storage classes for results
   - Clean up old benchmark data regularly
   - Choose storage provider based on your needs:
     - **OCI**: Best for Oracle Cloud deployments
     - **S3**: Ideal for AWS-based infrastructure
     - **Azure Blob**: Optimal for Azure environments
     - **GCS**: Recommended for Google Cloud Platform
     - **GitHub**: Good for public benchmarks and CI/CD integration
     - **PVC**: Best for on-premise or air-gapped environments

5. **Multi-Cloud Considerations**:
   - Store credentials securely using Kubernetes secrets
   - Use service accounts or managed identities when possible
   - Consider data egress costs when choosing storage location
   - Enable encryption at rest for sensitive benchmark data
   - Use consistent naming conventions across cloud providers


## Usage Guide
1. Make sure an InferenceService is running in the cluster. 

```shell
# Follow CONTRIBUTING.md to start OME manager
# Create a Llama-3.1-70b-Instruct iscv if there is not one
kubectl apply -f config/samples/iscv/meta/llama3-1-70b-instruct.yaml 
```

2. Start a benchmark

```shell
# If there is a secret reference, apply the sceret resource first
kubectl apply -f config/samples/benchmark/huggingface-secret.yaml 
kubectl apply -f config/samples/benchmark/llama3-1-70b-instruct.yaml 
```