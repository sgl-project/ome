# Integration Testing Design for OME

## Overview

This document proposes a comprehensive integration testing framework for Oracle Machine Learning Engine (OME). The goal is to create a robust testing environment that can validate OME's functionality without requiring actual GPU resources, while still ensuring that all critical components work correctly together.

## Motivation

Currently, OME lacks a comprehensive integration testing framework. While there are unit tests for individual components, we need a way to test the interactions between different components and validate the end-to-end behavior of the system. This is particularly important for:

1. Validating InferenceService (isvc) behavior
2. Testing runtime selection and pod merge functionality
3. Verifying webhook behaviors (mutation and validation)
4. Testing training job behavior
5. Validating capacity reservation and DAC functionality

Additionally, we need a way to test these components without requiring actual GPU resources, which can be expensive and difficult to provision in a CI/CD environment.

## Goals

- Create a Kubernetes control plane-based integration testing framework that doesn't require a full cluster
- Test all critical OME components and their interactions
- Validate CRD behaviors and controller logic
- Enable testing of webhook functionality
- Support testing of runtime selection and pod merge
- Provide a foundation for future end-to-end testing with actual GPU resources

## Future-Goals

- Testing actual GPU performance or functionality
- Testing external dependencies (e.g., object storage)
- Implementing end-to-end testing with real clusters
- Performance and load testing in production-like environments
- Chaos testing for resilience validation

## Design

### Approach

We will adopt an approach similar to the one used by the Kubernetes SIG Kueue project, which uses the Kubernetes controller-runtime's envtest package to spin up a local Kubernetes API server without requiring a full cluster. This allows us to:

1. Install and test our CRDs
2. Deploy and test our controllers
3. Validate webhook functionality
4. Test the interactions between different components

### Components

The integration testing framework will consist of the following components:

1. **Test Environment Setup**: Using controller-runtime's envtest to create a local Kubernetes API server
2. **CRD Installation**: Installing OME CRDs into the test environment
3. **Controller Deployment**: Deploying OME controllers in the test environment
4. **Test Fixtures**: Creating test fixtures for different scenarios
5. **Assertion Utilities**: Utilities for validating the state of resources

### Implementation Details

#### Test Environment Setup

We will use the controller-runtime's envtest package to set up a local Kubernetes API server. This will be configured in a test suite setup function:

```go
var (
    testEnv    *envtest.Environment
    k8sClient  client.Client
    ctx        context.Context
    cancel     context.CancelFunc
)

func TestMain(m *testing.M) {
    ctx, cancel = context.WithCancel(context.Background())
    
    // Set up the test environment
    testEnv = &envtest.Environment{
        CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
        WebhookInstallOptions: envtest.WebhookInstallOptions{
            Paths: []string{filepath.Join("..", "..", "config", "webhook")},
        },
    }
    
    // Start the test environment
    cfg, err := testEnv.Start()
    if err != nil {
        log.Fatalf("Failed to start test environment: %v", err)
    }
    
    // Set up the client
    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
    if err != nil {
        log.Fatalf("Failed to create k8s client: %v", err)
    }
    
    // Run the tests
    code := m.Run()
    
    // Clean up
    cancel()
    err = testEnv.Stop()
    if err != nil {
        log.Fatalf("Failed to stop test environment: %v", err)
    }
    
    os.Exit(code)
}
```

#### CRD Installation

The CRDs will be installed automatically by the envtest environment based on the CRDDirectoryPaths configuration.

#### Controller Deployment

We will deploy the OME controllers in the test environment:

```go
func setupControllers(ctx context.Context, k8sClient client.Client) error {
    // Set up the manager
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        Scheme:             scheme.Scheme,
        MetricsBindAddress: "0",
    })
    if err != nil {
        return fmt.Errorf("failed to set up manager: %w", err)
    }
    
    // Set up the controllers
    if err := setupInferenceServiceController(mgr); err != nil {
        return fmt.Errorf("failed to set up inference service controller: %w", err)
    }
    
    if err := setupTrainingJobController(mgr); err != nil {
        return fmt.Errorf("failed to set up training job controller: %w", err)
    }
    
    // Add more controllers as needed
    
    // Start the manager
    go func() {
        if err := mgr.Start(ctx); err != nil {
            log.Fatalf("Failed to start manager: %v", err)
        }
    }()
    
    return nil
}
```

#### Test Fixtures

We will create test fixtures for different scenarios:

```go
func createInferenceServiceFixture(k8sClient client.Client, name, namespace string) error {
    isvc := &v1beta1.InferenceService{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
        Spec: v1beta1.InferenceServiceSpec{
            // Set up the spec
        },
    }
    
    return k8sClient.Create(context.Background(), isvc)
}

func createTrainingJobFixture(k8sClient client.Client, name, namespace string) error {
    job := &v1beta1.TrainingJob{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
        Spec: v1beta1.TrainingJobSpec{
            // Set up the spec
        },
    }
    
    return k8sClient.Create(context.Background(), job)
}
```

#### Assertion Utilities

We will create utilities for validating the state of resources:

```go
func waitForInferenceServiceReady(ctx context.Context, k8sClient client.Client, name, namespace string, timeout time.Duration) error {
    return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
        isvc := &v1beta1.InferenceService{}
        if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, isvc); err != nil {
            return false, err
        }
        
        return isvc.Status.IsReady(), nil
    })
}

func waitForTrainingJobComplete(ctx context.Context, k8sClient client.Client, name, namespace string, timeout time.Duration) error {
    return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
        job := &v1beta1.TrainingJob{}
        if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, job); err != nil {
            return false, err
        }
        
        return job.Status.Phase == v1beta1.TrainingJobPhaseSucceeded, nil
    })
}
```

### Test Cases

Based on the OME API structure and controller implementations, we will implement comprehensive test cases for all major components. The following sections detail the specific test cases for each component, focusing on their reconciliation logic:

#### 1. InferenceService Controller Tests

The InferenceService controller is responsible for reconciling InferenceService resources, which involves creating and managing the underlying Kubernetes resources needed for model serving.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from creation to ready state
- Test reconciliation when the InferenceService spec is updated
- Test reconciliation when underlying resources (e.g., Deployments, Services) are modified
- Test reconciliation when the InferenceService is deleted
- Test reconciliation error handling and recovery

**Status Updates:**
- Verify status updates correctly reflect the state of underlying resources
- Test status conditions are properly set based on resource readiness
- Test URL and address fields are correctly populated
- Test status updates when transitioning between different states

**Resource Creation and Management:**
- Test creation of Knative Service resources
- Test creation of Istio VirtualService resources
- Test creation of Kubernetes Service resources
- Test creation of Deployment resources
- Test creation of HorizontalPodAutoscaler resources
- Test creation of ConfigMap resources for model configuration

**Component Integration:**
- Test integration with ServingRuntime selection logic
- Test integration with model loading and initialization
- Test integration with autoscaling components
- Test integration with networking components (Istio)

**Advanced Scenarios:**
- Test handling of canary deployments with traffic splitting
- Test blue/green deployment scenarios
- Test multi-model serving configurations
- Test handling of resource constraints and limits
- Test handling of custom predictor configurations
- Test handling of custom transformer configurations

#### 2. Training Job Controller Tests

The Training Job controller is responsible for reconciling TrainingJob resources, which involves selecting an appropriate runtime and creating the necessary resources for model training.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from job creation to completion
- Test reconciliation when the TrainingJob spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the TrainingJob is deleted
- Test reconciliation error handling and recovery

**Runtime Selection and Integration:**
- Test selection of the appropriate runtime based on the TrainingJob spec
- Test fallback mechanisms when the preferred runtime is not available
- Test integration with different runtime implementations
- Test handling of runtime-specific configurations

**Status Updates:**
- Test status updates correctly reflect the job's phase (Pending, Running, Succeeded, Failed)
- Test status conditions are properly set based on job state
- Test start time and completion time fields are correctly populated
- Test status updates when transitioning between different phases

**Resource Creation and Management:**
- Test creation of Pod resources for training
- Test creation of PersistentVolumeClaim resources for data and model storage
- Test creation of ConfigMap resources for training configuration
- Test creation of Secret resources for credentials

**Error Handling:**
- Test handling of invalid runtime references
- Test handling of resource creation failures
- Test handling of job failures
- Test handling of timeout scenarios

**Advanced Scenarios:**
- Test distributed training configurations
- Test multi-node training setups
- Test hyperparameter tuning workflows
- Test job resumption from checkpoints
- Test handling of resource constraints and limits

#### 3. DAC (Dedicated AI Cluster) Controller Tests

The DAC controller is responsible for reconciling DedicatedAICluster resources, which involves creating and managing namespaces, Volcano queues, and reservation jobs.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from DAC creation to ready state
- Test reconciliation when the DAC spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the DAC is deleted
- Test reconciliation error handling and recovery

**Profile Integration:**
- Test merging of DAC spec with profile spec
- Test handling of missing profiles
- Test profile updates and their effect on DAC reconciliation

**Resource Creation and Management:**
- Test creation of Namespace resources
- Test creation of Volcano Queue resources
- Test creation of Volcano Job resources for reservations
- Test management of resource quotas and limits

**Status Updates:**
- Test status updates correctly reflect the DAC's state
- Test status conditions are properly set based on resource readiness
- Test status updates when transitioning between different states
- Test failure detection and reporting

**Reservation Management:**
- Test creation and management of reservation jobs
- Test scaling of reservation replicas based on DAC spec
- Test handling of reservation job failures
- Test cleanup of reservation resources

**Advanced Scenarios:**
- Test handling of resource constraints and limits
- Test integration with workload scheduling
- Test handling of priority and preemption
- Test multi-tenant scenarios with namespace isolation

#### 4. Capacity Reservation Controller Tests

The Capacity Reservation controller is responsible for reconciling CapacityReservation resources, which involves creating and managing resources to reserve compute capacity.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from creation to ready state
- Test reconciliation when the CapacityReservation spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the CapacityReservation is deleted
- Test reconciliation error handling and recovery

**Resource Creation and Management:**
- Test creation of reservation resources (e.g., Pods, Jobs)
- Test management of resource quotas and limits
- Test handling of node selectors and affinity rules

**Status Updates:**
- Test status updates correctly reflect the reservation's state
- Test status conditions are properly set based on resource readiness
- Test status updates when transitioning between different states

**Scheduling Integration:**
- Test integration with Kubernetes scheduler
- Test integration with custom schedulers (e.g., Volcano)
- Test handling of priority and preemption

**Advanced Scenarios:**
- Test handling of resource constraints and limits
- Test handling of reservation expiration and renewal
- Test handling of reservation sharing across namespaces
- Test handling of dynamic adjustment based on workload

#### 5. Benchmark Controller Tests

The Benchmark controller is responsible for reconciling BenchmarkJob resources, which involves creating and managing resources for model benchmarking.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from job creation to completion
- Test reconciliation when the BenchmarkJob spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the BenchmarkJob is deleted
- Test reconciliation error handling and recovery

**Resource Creation and Management:**
- Test creation of Pod resources for benchmarking
- Test creation of ConfigMap resources for benchmark configuration
- Test creation of Service resources for benchmark endpoints

**Status Updates:**
- Test status updates correctly reflect the job's phase
- Test status conditions are properly set based on job state
- Test metrics collection and reporting in status
- Test status updates when transitioning between different phases

**Integration with InferenceService:**
- Test benchmarking of InferenceService resources
- Test handling of different model formats and serving configurations
- Test collection of performance metrics

**Advanced Scenarios:**
- Test benchmarking of multiple model versions
- Test comparison of different serving configurations
- Test handling of resource constraints and limits
- Test integration with monitoring systems

#### 6. Model Controller Tests

The Model controller is responsible for reconciling Model resources, which involves managing model artifacts and their metadata.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from model creation to ready state
- Test reconciliation when the Model spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the Model is deleted
- Test reconciliation error handling and recovery

**Resource Creation and Management:**
- Test creation of PersistentVolumeClaim resources for model storage
- Test creation of ConfigMap resources for model metadata
- Test handling of model versioning

**Status Updates:**
- Test status updates correctly reflect the model's state
- Test status conditions are properly set based on resource readiness
- Test status updates when transitioning between different states

**Integration with Storage:**
- Test handling of different storage backends
- Test handling of model artifacts in different formats
- Test handling of model metadata

**Advanced Scenarios:**
- Test handling of model versioning and updates
- Test handling of model dependencies
- Test integration with model registry systems
- Test handling of model validation and verification

#### 7. AI Platform Controller Tests

The AI Platform controller is responsible for reconciling AIPlatform resources, which involves creating and managing the components of the AI platform.

**Reconciliation Logic Tests:**
- Test the full reconciliation cycle from platform creation to ready state
- Test reconciliation when the AIPlatform spec is updated
- Test reconciliation when underlying resources are modified
- Test reconciliation when the AIPlatform is deleted
- Test reconciliation error handling and recovery

**Component Integration:**
- Test integration with InferenceService components
- Test integration with TrainingJob components
- Test integration with BenchmarkJob components
- Test integration with Model components

**Resource Creation and Management:**
- Test creation of Namespace resources
- Test creation of Service resources
- Test creation of Deployment resources
- Test creation of ConfigMap resources

**Status Updates:**
- Test status updates correctly reflect the platform's state
- Test status conditions are properly set based on component readiness
- Test status updates when transitioning between different states

**Advanced Scenarios:**
- Test handling of platform-wide configurations
- Test handling of multi-tenant scenarios
- Test handling of resource constraints and limits
- Test handling of platform upgrades and migrations

#### 8. Webhook Tests

**Validation Webhook Tests:**
- Test validation of required fields for each resource type
- Test validation of field formats and values
- Test validation of resource relationships and dependencies
- Test validation of resource constraints and limits
- Test handling of invalid resources and appropriate error messages

**Mutation Webhook Tests:**
- Test default value injection for each resource type
- Test field normalization and formatting
- Test handling of deprecated fields and versions
- Test handling of backward compatibility

**Webhook Lifecycle Tests:**
- Test webhook registration and activation
- Test webhook version compatibility
- Test webhook failure handling and recovery
- Test webhook performance under load

#### 9. Controller Integration Tests

**Controller Coordination:**
- Test coordination between InferenceService and ServingRuntime controllers
- Test coordination between TrainingJob and TrainingRuntime controllers
- Test coordination between BenchmarkJob and InferenceService controllers
- Test coordination between CapacityReservation and scheduler components
- Test coordination between DAC and scheduler components
- Test coordination between Model and storage components

**Resource Ownership and Garbage Collection:**
- Test proper setting of owner references
- Test garbage collection of dependent resources
- Test handling of orphaned resources
- Test cleanup of resources during deletion

**Error Handling and Recovery:**
- Test controller recovery from API server errors
- Test controller recovery from resource conflicts
- Test controller handling of invalid resources
- Test controller handling of resource creation failures
- Test controller handling of resource update failures
- Test controller handling of resource deletion failures

**Performance and Scalability:**
- Test controller performance with many resources
- Test controller memory usage
- Test controller CPU usage
- Test controller throughput and latency
- Test controller behavior under high load

## End-to-End Testing Implementation Options

To enable comprehensive end-to-end testing that can run against actual clusters, the following high-level implementation options are proposed:

### 1. Configuration Change Detection
- Implement mechanisms to detect changes in runtime, model, or InferenceService configurations
- Create a diff utility that can identify changes between versions
- Set up triggers to automatically run tests when configurations change

### 2. Cluster Access Management
- Develop utilities to fetch kubeconfig from both MLE cluster and potentially service cluster
- Implement secure credential management for accessing clusters
- Create context switching capabilities to run tests against different clusters

### 3. Container Image Management
- Add Makefile targets to build and push container images to registries
- Implement versioning for container images to track changes
- Set up CI/CD pipelines to automatically build and push images on code changes

### 4. Configuration Updates
- Create utilities to update container image references in configuration files
- Implement templating for configuration files to easily swap image references
- Develop validation to ensure configuration integrity after updates

### 5. Cluster Installation
- Add Makefile targets to install against a particular cluster with latest CRDs and container images
- Implement rollback mechanisms in case of installation failures
- Create validation steps to verify successful installation

### 6. Parallel Test Execution
- Design test framework to support parallel test execution
- Implement resource isolation to prevent test interference
- Create test result aggregation to provide comprehensive reports
- Develop timeout and failure handling for parallel tests

These implementation options will enable a robust end-to-end testing framework that can validate the OME system in real-world environments, complementing the integration tests that run in the simulated environment.

### Future Extensions

In the future, we can extend this framework to include:

1. **End-to-End Tests with GPU Resources**: Using actual GPU resources to validate performance and functionality
2. **Model Benchmark Tests**: Testing model performance and accuracy
3. **Model Evaluation Tests**: Validating model evaluation functionality
4. **Model Quantization Tests**: Testing model quantization and optimization

## Implementation Plan

### Phase 1: Basic Integration Testing Framework

1. Set up the envtest environment
2. Install OME CRDs
3. Implement basic test fixtures and assertion utilities
4. Implement basic InferenceService and Training Job tests

### Phase 2: Advanced Integration Tests

1. Implement webhook tests
2. Implement Capacity Reservation tests
3. Implement DAC tests
4. Add more complex test scenarios

### Phase 3: End-to-End Testing

1. Implement a framework for testing with actual GPU resources
2. Implement model benchmark tests
3. Implement model evaluation tests
4. Implement model quantization tests

### Phase 4: CI/CD Integration

1. Implement configuration change detection
2. Set up cluster access management
3. Implement container image management
4. Create configuration update utilities
5. Add cluster installation targets
6. Enable parallel test execution

## Alternatives Considered

### Using a Full Kubernetes Cluster

We considered using a full Kubernetes cluster for integration testing, but this approach has several drawbacks:

1. It requires more resources and is slower to set up
2. It is more difficult to automate in a CI/CD environment
3. It requires managing cluster state between tests

### Using Mocks

We considered using mocks for integration testing, but this approach doesn't adequately test the interactions between components and can miss issues that only appear in a real environment.

## References

1. [Kubernetes controller-runtime envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
2. [Kueue Integration Tests](https://github.com/kubernetes-sigs/kueue/tree/main/test)
3. [Kubernetes Integration Testing Best Practices](https://kubernetes.io/blog/2020/01/08/testing-of-kubernetes-controllers/) 