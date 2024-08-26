## Multi Node Prober

This Golang application is a simple HTTP server designed to handle Kubernetes pod health probes,
specifically for applications running a vLLM within a Ray Cluster.
It provides liveness, readiness,
and startup endpoints that Kubernetes can use to determine the health and status of the multi-node vLLM application.

### Features

- Liveness Probe (/healthz): Check whether the vLLM and its services are running and responsive.
- Readiness Probe (/readyz): Ensures that the vLLM is ready to accept traffic, typically after the model is fully initialized.
- Startup Probe (/startupz): Verifies that the vLLM has started correctly, useful for containers with long startup times. 
- Configurable endpoints for monitoring the health of the vLLM services.

### Requirements

- Golang: This application is written in Golang, so you need to have Go installed to build it.
- Kubernetes: The application is intended to be deployed in a Kubernetes environment where it can respond to probe requests from the Kubelet.

### Configuration

The server accepts the following command-line arguments:

- --addr: The address the server listens on. Default is :8080.
- --vllm-endpoint: The HTTP endpoint used to check the health of the vLLM service. Default is http://localhost:8081/health.
- --read-timeout: The timeout for reading the request from the client. Default is 10 seconds.
- --write-timeout: The timeout for writing the response to the client. Default is 10 seconds.
- --idle-timeout: The maximum amount of time to wait for the next request when keep-alives are enabled. Default is 120 seconds.
- --inference-timeout: The timeout for the inference request to the vLLM service. Default is 100 seconds.

### Usage

#### Running Locally

To run the application locally:
1. Clone the repository.
2. Build the application:
```bash
go build -o multinode-prober cmd/multinode-prober/main.go
```
3. Run the application:
```bash
./multinode-prober --vllm-endpoint=http://vllm-service:8081/health
```
The server will start and listen on the port specified by the --addr flag (default is :8080).

#### Running in Kubernetes
To deploy this application in a Kubernetes pod, you would typically include it as a sidecar or as the main container within your pod spec. Here’s an example pod specification:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-pod
spec:
  containers:
  - name: health-check-container
    image: your-health-checker-image:latest
    args:
      - "--vllm-endpoint=http://vllm-service:8081/health"
    ports:
    - containerPort: 8080
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 5
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080
      initialDelaySeconds: 15
      periodSeconds: 5
    startupProbe:
      httpGet:
        path: /startupz
        port: 8080
      failureThreshold: 30
      periodSeconds: 10
```

#### Kubernetes Probes

1. Liveness Probe: Kubernetes will use this to check if the pod is still running. If it fails, Kubernetes will restart the pod.
2. Readiness Probe: Ensures that the pod is ready to serve traffic.
   If it fails, the pod will not receive any traffic.
3. Startup Probe: Checks if the pod has successfully started. Useful for containers with long initialization times.

#### Unit Tests
The application includes a comprehensive suite of unit tests covering all critical components and scenarios. To run the tests, use:
```bash
go test ./...
```