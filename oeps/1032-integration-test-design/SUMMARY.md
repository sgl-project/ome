# Integration Testing Framework for OME

## Overview

This design document outlines a comprehensive integration testing framework for the Oracle Machine Learning Engine (OME) system. The framework is designed to validate the interactions between various components of the OME system and ensure that the system behaves correctly as a whole, without requiring actual GPU resources.

## Key Components

The integration testing framework consists of the following key components:

1. **Test Environment Setup**: Using the Kubernetes controller-runtime's envtest package to create a local Kubernetes API server for testing.
2. **CRD Installation**: Installing the OME Custom Resource Definitions (CRDs) in the test environment.
3. **Controller Deployment**: Deploying the OME controllers in the test environment.
4. **Test Fixtures**: Creating test fixtures for various OME resources.
5. **Assertion Utilities**: Developing utilities for asserting the expected behavior of the OME system.

## Test Cases

The integration tests cover the following areas:

### InferenceService Controller Tests
- Full reconciliation cycle tests
- Status update tests
- Resource creation and management tests
- Error handling tests
- Integration with serving runtime selection logic

### Training Job Controller Tests
- Reconciliation logic tests
- Runtime selection tests
- Status update tests
- Resource management tests
- Error handling tests

### DAC Controller Tests
- Reconciliation logic tests
- Profile integration tests
- Resource management tests
- Status update tests
- Error handling tests

### Capacity Reservation Controller Tests
- Reservation logic tests
- Status update tests
- Integration with DAC tests
- Error handling tests

### Benchmark Controller Tests
- Reconciliation logic tests
- Status update tests
- Resource management tests
- Error handling tests

### Model Controller Tests
- Reconciliation logic tests
- Status update tests
- Storage integration tests
- Error handling tests

### AI Platform Controller Tests
- Reconciliation logic tests
- Component integration tests
- Status update tests
- Error handling tests

### Webhook Tests
- Validation webhook tests
- Defaulting webhook tests
- Mutation webhook tests

## Implementation Approach

The implementation of the integration testing framework will follow a phased approach:

### Phase 1: Basic Framework Setup
- Set up the test environment
- Install CRDs
- Implement basic test fixtures
- Create simple tests for InferenceService and Training Job controllers

### Phase 2: Comprehensive Controller Tests
- Implement tests for all controllers
- Add error handling tests
- Enhance test fixtures

### Phase 3: Advanced Scenarios
- Implement tests for complex scenarios
- Add performance tests
- Enhance error handling tests

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

## Sample Implementation

The design includes sample implementations for:

1. **Sample Test File**: A sample integration test file that demonstrates how to set up the test environment and write tests for the InferenceService and Training Job controllers.
2. **DAC Controller Test Example**: A detailed example of integration tests for the DAC controller, demonstrating how to test the reconciliation logic, status updates, and error handling.
3. **Makefile Sample**: A sample Makefile that includes targets for running integration tests and generating coverage reports.

## Future Extensions

The integration testing framework can be extended in the following ways:

1. **Performance Testing**: Adding performance tests to ensure that the system meets performance requirements.
2. **Chaos Testing**: Introducing chaos to test the system's resilience.
3. **Load Testing**: Testing the system under load to ensure it can handle the expected load.

## Conclusion

The integration testing framework provides a solid foundation for testing the OME system. It ensures that all components work correctly together and that the system behaves as expected. The framework is designed to be extensible, allowing for the addition of new test cases as the system evolves. With the addition of end-to-end testing capabilities, the framework will provide comprehensive validation of the OME system in both simulated and real-world environments. 