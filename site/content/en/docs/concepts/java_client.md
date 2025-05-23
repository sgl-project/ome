---
title: "Java Client"
date: 2023-03-14
weight: 7
description: >
  Type-safe Java client for interacting with OME custom resources
---

The OME Java client provides a type-safe, idiomatic way to interact with OME custom resources from Java applications. Built on top of the [Fabric8 Kubernetes Client](https://github.com/fabric8io/kubernetes-client), it offers comprehensive support for all OME resources and operations.


The Java client enables you to:
- Create, read, update, and delete OME custom resources
- Watch for resource changes and events
- Handle complex operations like scaling and status updates
- Manage resources across multiple namespaces
- Integrate OME operations into Java applications and services

## Installation

### Maven Dependency

Add the OME Java client to your project:

```xml
<dependency>
    <groupId>com.oracle.pic.ocasgs.ome</groupId>
    <artifactId>ocasgs-ome-fabric8-java-client</artifactId>
    <version>1.0-SNAPSHOT</version>
</dependency>
```

### Additional Dependencies

The client uses these key dependencies:
```xml
<dependency>
    <groupId>io.fabric8</groupId>
    <artifactId>kubernetes-client</artifactId>
    <version>${fabric8.version}</version>
</dependency>
<dependency>
    <groupId>io.fabric8</groupId>
    <artifactId>kubernetes-httpclient-vertx</artifactId>
    <version>${fabric8.version}</version>
</dependency>
```

## Client Configuration

### Basic Setup

Create a client with default configuration:

```java
import com.oracle.pic.ocasgs.ome.client.OmeClient;

Config config = new ConfigBuilder().build();
OmeClient client = new OmeClient(config);
```

### Custom Configuration

Configure the client with specific settings:

```java
Config config = new ConfigBuilder()
    .withMasterUrl("https://your-cluster:6443")
    .withOauthToken("your-token")
    .withNamespace("your-namespace")
    .withTrustCerts(true)
    .withConnectionTimeout(30000)
    .withRequestTimeout(30000)
    .build();

OmeClient client = new OmeClient(config);
```

## Working with InferenceServices

### Creating an InferenceService

```java
// Create a basic inference service
V1beta1InferenceService service = new V1beta1InferenceService()
    .withMetadata(new ObjectMetaBuilder()
        .withName("llama-chat")
        .withNamespace("default")
        .addToAnnotations("ome.io/deploymentMode", "RawDeployment")
        .build())
    .withSpec(new V1beta1InferenceServiceSpecBuilder()
        .withPredictor(new V1beta1PredictorBuilder()
            .withModel(new V1beta1ModelSpecBuilder()
                .withBaseModel("llama-2-70b")
                .withRuntime("vllm-text-generation")
                .build())
            .withResources(new ResourceRequirementsBuilder()
                .addToRequests("nvidia.com/gpu", Quantity.parse("1"))
                .addToLimits("nvidia.com/gpu", Quantity.parse("1"))
                .build())
            .build())
        .build());

client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .create(service);
```

### Advanced InferenceService Configuration

```java
// Create an inference service with advanced configuration
V1beta1InferenceService service = new V1beta1InferenceService()
    .withMetadata(new ObjectMetaBuilder()
        .withName("llama-chat")
        .withNamespace("default")
        .addToAnnotations(Map.of(
            "ome.io/deploymentMode", "MultiNodeRayVLLM",
            "ome.io/autoscalerClass", "kpa",
            "ome.io/metrics", "concurrency"
        ))
        .build())
    .withSpec(new V1beta1InferenceServiceSpecBuilder()
        .withPredictor(new V1beta1PredictorBuilder()
            .withModel(new V1beta1ModelSpecBuilder()
                .withBaseModel("llama-2-70b")
                .withRuntime("vllm-text-generation")
                .build())
            .withWorkerSpec(new V1beta1WorkerSpecBuilder()
                .withWorldSize(4)
                .withResources(new ResourceRequirementsBuilder()
                    .addToRequests("nvidia.com/gpu", Quantity.parse("1"))
                    .addToLimits("nvidia.com/gpu", Quantity.parse("1"))
                    .build())
                .build())
            .build())
        .build());
```

### Managing InferenceService Lifecycle

```java
// List all inference services
List<V1beta1InferenceService> services = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .list()
    .getItems();

// Get a specific service
V1beta1InferenceService service = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withName("llama-chat")
    .get();

// Update a service
service.getSpec()
    .getPredictor()
    .getResources()
    .getRequests()
    .put("memory", Quantity.parse("32Gi"));

client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .createOrReplace(service);

// Delete a service
client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withName("llama-chat")
    .delete();
```

## Working with BaseModels

### Creating a BaseModel

```java
V1beta1BaseModel model = new V1beta1BaseModel()
    .withMetadata(new ObjectMetaBuilder()
        .withName("custom-llama")
        .build())
    .withSpec(new V1beta1BaseModelSpecBuilder()
        .withDisplayName("Custom Llama Model")
        .withModelFormat(new V1beta1ModelFormatBuilder()
            .withName("safetensors")
            .withVersion("1")
            .build())
        .withModelType("transformer")
        .withModelArchitecture("LlamaForCausalLM")
        .withModelParameterSize("70B")
        .withModelCapabilities(Arrays.asList("TEXT_GENERATION", "CHAT"))
        .withStorage(new V1beta1StorageSpecBuilder()
            .withPath("oci://my-bucket/models/llama")
            .build())
        .build());

client.v1beta1()
    .baseModels()
    .create(model);
```

## Working with BenchmarkJobs

### Creating a Benchmark Job

```java
V1beta1BenchmarkJob job = new V1beta1BenchmarkJob()
    .withMetadata(new ObjectMetaBuilder()
        .withName("llama-benchmark")
        .withNamespace("default")
        .build())
    .withSpec(new V1beta1BenchmarkJobSpecBuilder()
        .withEndpoint(new V1beta1EndpointBuilder()
            .withInferenceService(new V1beta1InferenceServiceRefBuilder()
                .withName("llama-chat")
                .withNamespace("default")
                .build())
            .build())
        .withTask("text-to-text")
        .withTrafficScenarios(Arrays.asList(
            "chat-completion",
            "text-completion"
        ))
        .withNumConcurrency(Arrays.asList(1, 5, 10))
        .withMaxTimePerIteration(5)
        .withMaxRequestsPerIteration(1000)
        .withServiceMetadata(new V1beta1ServiceMetadataBuilder()
            .withEngine("vllm")
            .withVersion("0.5.3")
            .withGpuType("A100")
            .withGpuCount(1)
            .build())
        .withOutputLocation(new V1beta1OutputLocationBuilder()
            .withPath("oci://my-bucket/benchmark-results")
            .build())
        .build());

client.v1beta1()
    .benchmarkJobs()
    .inNamespace("default")
    .create(job);
```

## Event Handling and Watching

### Watching Resource Changes

```java
// Watch inference service events
Watch watch = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .watch(new Watcher<V1beta1InferenceService>() {
        @Override
        public void eventReceived(Action action, V1beta1InferenceService resource) {
            System.out.printf("Event: %s, Service: %s, Status: %s%n",
                action,
                resource.getMetadata().getName(),
                Optional.ofNullable(resource.getStatus())
                    .map(status -> status.getConditions())
                    .orElse(Collections.emptyList()));
        }
        
        @Override
        public void onClose(WatcherException e) {
            if (e != null) {
                System.err.println("Watch error: " + e.getMessage());
            }
        }
    });

// Remember to close the watch when done
watch.close();
```

### Using Informers

```java
SharedIndexInformer<V1beta1InferenceService> informer = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .inform(new ResourceEventHandler<V1beta1InferenceService>() {
        @Override
        public void onAdd(V1beta1InferenceService obj) {
            System.out.println("Added: " + obj.getMetadata().getName());
        }

        @Override
        public void onUpdate(V1beta1InferenceService oldObj, V1beta1InferenceService newObj) {
            System.out.println("Updated: " + newObj.getMetadata().getName());
        }

        @Override
        public void onDelete(V1beta1InferenceService obj, boolean deletedFinalStateUnknown) {
            System.out.println("Deleted: " + obj.getMetadata().getName());
        }
    });

// Start the informer
informer.start();
```

## Error Handling

### Comprehensive Error Handling

```java
try {
    client.v1beta1()
        .inferenceServices()
        .inNamespace("default")
        .create(service);
} catch (KubernetesClientException e) {
    switch (e.getCode()) {
        case 409:  // Conflict
            System.out.println("Resource already exists: " + e.getMessage());
            break;
        case 403:  // Forbidden
            System.out.println("Permission denied: " + e.getMessage());
            break;
        case 422:  // Validation error
            System.out.println("Invalid resource specification: " + e.getMessage());
            break;
        default:
            System.out.println("Unexpected error: " + e.getMessage());
    }
} catch (Exception e) {
    System.out.println("General error: " + e.getMessage());
}
```

## Best Practices

### Resource Management

```java
// Use try-with-resources for automatic cleanup
try (OmeClient client = new OmeClient(config)) {
    // Perform operations
    client.v1beta1()
        .inferenceServices()
        .inNamespace("default")
        .list();
}

// For watches, use a separate try-with-resources
try (Watch watch = client.v1beta1()
        .inferenceServices()
        .inNamespace("default")
        .watch(new Watcher<V1beta1InferenceService>() {
            // ... watcher implementation
        })) {
    // Watch is active
    Thread.sleep(60000); // Watch for 1 minute
}
```

### Performance Optimization

```java
// Use label selectors for efficient filtering
Map<String, String> labels = new HashMap<>();
labels.put("app", "llama");
labels.put("environment", "production");

List<V1beta1InferenceService> services = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withLabels(labels)
    .list()
    .getItems();

// Use field selectors for specific queries
List<V1beta1InferenceService> readyServices = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withField("status.conditions[?(@.type=='Ready')].status", "True")
    .list()
    .getItems();
```

### Security Considerations

```java
// Load configuration from kubeconfig file
Config config = Config.autoConfigure(null);

// Or use service account token
String serviceAccountToken = new String(Files.readAllBytes(
    Paths.get("/var/run/secrets/kubernetes.io/serviceaccount/token")));

Config config = new ConfigBuilder()
    .withOauthToken(serviceAccountToken)
    .build();

// Use SSL/TLS
Config config = new ConfigBuilder()
    .withTrustCerts(false)
    .withCaCertFile("/path/to/ca.crt")
    .withClientCertFile("/path/to/client.crt")
    .withClientKeyFile("/path/to/client.key")
    .build();
```

## Common Patterns

### Batch Operations

```java
// Create multiple services
List<V1beta1InferenceService> services = Arrays.asList(service1, service2, service3);
services.forEach(service -> {
    try {
        client.v1beta1()
            .inferenceServices()
            .inNamespace("default")
            .create(service);
    } catch (Exception e) {
        System.err.println("Failed to create service: " + service.getMetadata().getName());
    }
});

// Delete multiple services
client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withLabel("environment", "test")
    .delete();
```

### Status Updates

```java
V1beta1InferenceService service = client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .withName("llama-chat")
    .get();

// Update status conditions
List<V1beta1InferenceServiceCondition> conditions = Arrays.asList(
    new V1beta1InferenceServiceConditionBuilder()
        .withType("Ready")
        .withStatus("True")
        .withReason("ModelLoaded")
        .withMessage("Model is loaded and ready to serve")
        .withLastTransitionTime(new Date())
        .build(),
    new V1beta1InferenceServiceConditionBuilder()
        .withType("Available")
        .withStatus("True")
        .build()
);

service.setStatus(new V1beta1InferenceServiceStatusBuilder()
    .withConditions(conditions)
    .build());

client.v1beta1()
    .inferenceServices()
    .inNamespace("default")
    .updateStatus(service);
```

## Troubleshooting

### Enable Debug Logging

```java
System.setProperty("org.slf4j.simpleLogger.defaultLogLevel", "DEBUG");
System.setProperty("org.slf4j.simpleLogger.log.okhttp3", "DEBUG");
```

### Connection Issues

```java
// Test connectivity
try {
    client.getKubernetesVersion();
    System.out.println("Successfully connected to cluster");
} catch (KubernetesClientException e) {
    System.err.println("Failed to connect: " + e.getMessage());
    // Check configuration
    System.out.println("Current configuration:");
    System.out.println("Master URL: " + client.getConfiguration().getMasterUrl());
    System.out.println("Namespace: " + client.getConfiguration().getNamespace());
}
```
