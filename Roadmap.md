# OME 2025 Roadmap

## 1. Stability Enhancement

- **Automated Deployment**:
  - Implement auto-deployment for all OME components via Shepherd, Artifact Notification Service (ANS), and Artifact Push Service (APS).
  - Components include runtimes, training modules, serving layers, charts, and controllers.

- **Integration Testing**:
  - Achieve 90% coverage for integration tests during deployment across all components.

- **Observability Improvements**:
  - Enhance observability by incorporating comprehensive metrics, logs, alarms, and Grafana dashboards for all components.

- **Unit Test Coverage**:
  - Increase unit test coverage to a minimum of 90% for all components.

## 2. Model Serving and Runtime Optimization

- **Inference Service API Refinement**:
  - Remove deprecated fields to streamline the API.
  - Enhance API to better support single-node and multi-node model serving, runtime consolidation, and model artifact reconciliation.

- **LoRA Adapter Support**:
  - Introduce support for multiple LoRA adapters within the inference service.

- **API Versioning**:
  - Graduate the inference service API to version 1.0.

## 3. Model Management Enhancements

- **Model Replication**:
  - Implement native support for model replication between HF/PVC/OCI Object Storage and PVC/OCI Object Storage.

- **Version Management**:
  - Develop native capabilities for managing multiple model versions.

- **Encryption and Decryption**:
  - Provide native support for model encryption and decryption integrated with OCI KMS and OCI Vault.

## 4. Serving and Dedicated AI Cluster Autoscaling

- **Scale-to-Zero Capability**:
  - Implement and document support for scaling resources to zero when not in use.

- **KEDA Integration**:
  - Achieve full integration with Kubernetes Event-Driven Autoscaling (KEDA) for efficient resource management.

- **Dedicated AI Cluster Autoscaling**:
  - Enable autoscaling for dedicated AI clusters to optimize resource utilization.

## 5. Distributed Training Capabilities

- **Multi-Node Training**:
  - Facilitate support for training models across multiple nodes.

- **Hyper-Parameter Optimization**:
  - Introduce native support for hyper-parameter optimization to improve model accuracy and efficiency.