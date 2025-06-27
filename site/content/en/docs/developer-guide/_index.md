---
title: "Developer Guide"
linkTitle: "Developer Guide"
weight: 8
description: >
  Resources for developers contributing to and extending OME.
no_list: true
---

This section provides comprehensive resources for developers who want to contribute to OME, extend its functionality, or integrate it with other systems.

## Getting Started

### [Contributing to OME](/ome/docs/developer-guide/contributing/)

Learn how to set up your development environment, follow coding guidelines, and submit contributions to the OME project.

**Key Topics:**
- Development environment setup
- Building and testing OME
- IDE configuration (VS Code, GoLand)
- Pull request process
- OME Enhancement Proposals (OEPs)

## Architecture & Internals

### [Controller Architecture](/ome/docs/developer-guide/controller-architecture/)

Deep dive into OME's controller architecture, including the controller manager, model controller, and model agent components.

### [API Design](/ome/docs/developer-guide/api-design/)

Understanding OME's API design principles, CRD structure, and how to extend the API.

### [Serving Runtime Development](/ome/docs/developer-guide/serving-runtime-development/)

Learn how to create custom serving runtimes for new inference engines or specialized deployment patterns.

## Development Tools

### [Local Development Setup](/ome/docs/developer-guide/local-development/)

Set up a local development environment for rapid iteration and testing.

### [Testing Strategies](/ome/docs/developer-guide/testing/)

Comprehensive guide to testing OME components, including unit tests, integration tests, and end-to-end testing.

### [Debugging Guide](/ome/docs/developer-guide/debugging/)

Troubleshooting techniques and debugging tools for OME development.

## Integration & Extension

### [Custom Resource Development](/ome/docs/developer-guide/custom-resources/)

How to extend OME with custom resources and controllers for specialized use cases.

### [Webhook Development](/ome/docs/developer-guide/webhooks/)

Implementing admission webhooks for validation and mutation of OME resources.

### [Plugin Architecture](/ome/docs/developer-guide/plugins/)

Understanding OME's plugin system and how to develop custom plugins.

## API Reference

### [Go Client Libraries](/ome/docs/developer-guide/client-libraries/)

Using OME's Go client libraries for programmatic interaction with the system.

### [REST API Reference](/ome/docs/developer-guide/rest-api/)

Complete REST API documentation for external integrations.

### [Prometheus Metrics](/ome/docs/developer-guide/metrics/)

Available metrics for monitoring and observability integration.

## Advanced Topics

### [Performance Optimization](/ome/docs/developer-guide/performance/)

Guidelines for optimizing OME performance and resource utilization.

### [Security Considerations](/ome/docs/developer-guide/security/)

Security best practices for OME development and deployment.

### [Multi-Node Networking](/ome/docs/developer-guide/networking/)

Deep dive into OME's multi-node networking capabilities and RDMA integration.

## Release & Deployment

### [Release Process](/ome/docs/developer-guide/release-process/)

Understanding OME's release cycle and contribution to releases.

### [Container Image Management](/ome/docs/developer-guide/container-images/)

Building, tagging, and managing OME container images.

### [Helm Chart Development](/ome/docs/developer-guide/helm-charts/)

Contributing to and extending OME's Helm charts.

## Community & Governance

### [Development Roadmap](/ome/docs/developer-guide/roadmap/)

Current development priorities and future plans for OME.

### [Governance Model](/ome/docs/developer-guide/governance/)

Understanding OME's governance structure and decision-making process.

### [Community Guidelines](/ome/docs/developer-guide/community/)

Code of conduct and community interaction guidelines.

## Quick References

### [Environment Variables](/ome/docs/developer-guide/environment-variables/)

Complete list of environment variables used by OME components.

### [Make Targets](/ome/docs/developer-guide/make-targets/)

Reference for all available Makefile targets and their purposes.

### [Configuration Schema](/ome/docs/developer-guide/configuration-schema/)

JSON Schema references for all OME configuration files.

## Examples & Tutorials

### [End-to-End Development Example](/ome/docs/developer-guide/e2e-example/)

Complete walkthrough of implementing a new feature from concept to deployment.

### [Integration Examples](/ome/docs/developer-guide/integration-examples/)

Real-world examples of integrating OME with other systems and tools.

### [Best Practices](/ome/docs/developer-guide/best-practices/)

Collected wisdom and patterns for effective OME development. 