# Inference Router

A high-performance Rust implementation of an inference routing service that routes requests through a declarative graph of inference nodes. This service enables flexible composition of AI models and services into complex workflows.

## Overview

The Inference Router is designed to orchestrate complex AI inference pipelines by:
- Routing requests through a directed graph of processing nodes
- Supporting various routing patterns and conditions
- Enabling composition of multiple models and services
- Handling both synchronous and asynchronous processing
- Maintaining request context and headers across services
- Providing robust error handling and fallback mechanisms

## Routing Architecture

The Inference Router allows you to create sophisticated routing networks where each request flows through a series of routing nodes. Each node in this network can process the request and direct it to appropriate services based on configurable logic.

The router is deployed behind an HTTP endpoint and can be scaled dynamically based on request volume.

### Router Types

The Inference Router supports four different types of routing strategies:

#### 1. Sequence Router

Routes requests through a series of steps in sequential order. The output from one step can be passed as input to the next step, enabling data transformations and multi-stage processing.

**Use cases**: Multi-stage inference pipelines, preprocessing → inference → postprocessing flows, chained models.

#### 2. Switch Router

Routes requests based on conditional expressions evaluated against the request content. The request is sent to the first matching route, making it ideal for content-based routing decisions.

**Use cases**: Content-based routing, user segmentation, feature-based model selection.

#### 3. Ensemble Router

Sends the same request to multiple services in parallel and combines their responses. This enables model ensembles and multi-modal processing where multiple models contribute to a final result.

**Use cases**: Model ensembles, multi-modal inference, confidence boosting.

#### 4. Splitter Router

Distributes incoming traffic across multiple services according to configured weights. This enables A/B testing and canary deployments without changing client code.

**Use cases**: A/B testing, canary deployments, traffic shadowing.

## Features

- **Flexible Routing Architecture**:
  - **Sequence**: Executes steps in sequential order, passing output from one step as input to the next
  - **Switch**: Routes based on conditional expressions evaluated against the request
  - **Ensemble**: Executes multiple steps in parallel and combines their responses
  - **Splitter**: Routes requests based on configured weights for traffic splitting

- **Flexible Configuration**:
  - JSON-based graph configuration
  - Dynamic routing based on request content
  - Support for both file and string-based configuration

- **Advanced Request Handling**:
  - Request/response header propagation
  - Distributed tracing with request ID tracking
  - Hard and soft dependencies between services
  - Comprehensive error handling with appropriate status codes

- **Operational Features**:
  - Health check endpoints
  - Configuration validation utilities
  - Graceful shutdown with configurable grace period
  - Structured JSON logging

## Environment Setup

### Installing Rust

1. Install Rust using rustup (the Rust toolchain installer):

   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   ```

   For Windows users, visit [rustup.rs](https://rustup.rs/) for installation instructions.

2. Follow the on-screen instructions to complete the installation.

3. After installation, restart your terminal or run:

   ```bash
   source $HOME/.cargo/env
   ```

4. Verify the installation:

   ```bash
   rustc --version
   cargo --version
   ```

### Development Dependencies

The Inference Router requires the following system packages:

- C compiler (gcc/clang)
- OpenSSL development libraries
- pkg-config

#### Ubuntu/Debian:

```bash
sudo apt update
sudo apt install build-essential pkg-config libssl-dev
```

#### macOS:

```bash
brew install openssl pkg-config
```

### Project Setup

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd <repository-directory>/cmd/inference_router
   ```

2. Install project dependencies:

   ```bash
   cargo fetch
   ```

3. Build the project:

   ```bash
   cargo build
   ```

## Building

To build the project:

```bash
make build
```

## Testing

The router includes comprehensive test coverage:

```bash
make test
```

The test suite includes:
- Unit tests for router components
- Integration tests for full request flows
- Mock servers for testing backend services

## Running

### Validation Mode

Validate a routing configuration without starting the server:

```bash
cargo run -- validate --graph-json path/to/graph.json --verbose
```

### Router Mode

Start the router with a specific configuration:

```bash
# From a JSON file
cargo run -- --graph-json /path/to/graph.json

# From a JSON string
cargo run -- --graph-string '{...}'
```

Additional options:
- `--port <PORT>`: Specify the port (default: 8080)
- `--host <HOST>`: Specify the host (default: 0.0.0.0)
- `--log-level <LEVEL>`: Set log level (default: info)
- `--json-logs`: Enable JSON-formatted logs
- `--parse-headers <HEADERS>`: List of headers to propagate

### Start mock servers for testing

Start the bundled mock servers to simulate various backend services:

```bash
make mockservers
```

This starts:
- Classification server (port 8081)
- Text processing server (port 8082)
- Image processing server (port 8083)

### Run with test configuration

Start both the mock servers and the router with a predefined test configuration:

```bash
make run-test
```

## Testing with curl

Test the router with example requests:

```bash
# Text processing route
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: my-request-id" \
  -d '{"type":{"text":true},"content":"Hello world"}' \
  http://localhost:8080/

# Image processing route
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: my-request-id" \
  -d '{"type":{"image":true},"width":100,"height":100}' \
  http://localhost:8080/

# Run all test scenarios
make test-router
```

## Router Configuration

The router uses a JSON-based configuration format that defines nodes and their routing behavior:

```json
{
  "nodes": {
    "root": {
      "routerType": "switch",
      "steps": [
        {
          "stepName": "process-text",
          "nodeName": "text-processing",
          "condition": "type.text"
        },
        {
          "stepName": "process-image",
          "nodeName": "image-processing",
          "condition": "type.image"
        }
      ]
    },
    "text-processing": {
      "routerType": "sequence",
      "steps": [
        {
          "stepName": "classify-content",
          "serviceUrl": "http://localhost:8081",
          "dependency": "hard"
        },
        {
          "stepName": "process-text",
          "serviceUrl": "http://localhost:8082",
          "data": "$response",
          "dependency": "soft"
        }
      ]
    },
    "image-processing": {
      "routerType": "sequence",
      "steps": [
        {
          "stepName": "classify-content",
          "serviceUrl": "http://localhost:8081",
          "dependency": "hard"
        },
        {
          "stepName": "process-image",
          "serviceUrl": "http://localhost:8083",
          "data": "$response",
          "dependency": "soft"
        }
      ]
    }
  }
}
```

### Configuration Elements

- **nodes**: Map of named routing nodes
- **routerType**: Type of router (sequence, splitter, ensemble, switch)
- **steps**: Array of processing steps for the node
  - **stepName**: Unique name for the step
  - **nodeName**: Reference to another node in the routing network
  - **serviceUrl**: URL of an external service to call
  - **condition**: JSON path condition for switch routers
  - **weight**: Numeric weight for splitter routers
  - **data**: Data transformation expression (supports $request and $response variables)
  - **dependency**: Whether the step is a "hard" (required) or "soft" (optional) dependency

## Router Examples

### Sequence Router Example

Sequential processing pipeline that chains multiple steps together:

```json
{
  "nodes": {
    "root": {
      "routerType": "sequence",
      "steps": [
        {
          "stepName": "extract-features",
          "serviceUrl": "http://feature-extractor:8080"
        },
        {
          "stepName": "classify",
          "serviceUrl": "http://classifier:8080",
          "data": "$response"
        },
        {
          "stepName": "enrich",
          "serviceUrl": "http://enrichment:8080",
          "data": "$response"
        }
      ]
    }
  }
}
```

### Splitter Router Example

A/B testing with weighted traffic distribution:

```json
{
  "nodes": {
    "root": {
      "routerType": "splitter",
      "steps": [
        {
          "stepName": "model-a",
          "serviceUrl": "http://model-a:8080",
          "weight": 80
        },
        {
          "stepName": "model-b",
          "serviceUrl": "http://model-b:8080",
          "weight": 20
        }
      ]
    }
  }
}
```

### Ensemble Router Example

Parallel service execution with response combining:

```json
{
  "nodes": {
    "root": {
      "routerType": "ensemble",
      "steps": [
        {
          "stepName": "text-classifier",
          "serviceUrl": "http://text-classifier:8080"
        },
        {
          "stepName": "sentiment-analyzer",
          "serviceUrl": "http://sentiment:8080"
        },
        {
          "stepName": "entity-recognition",
          "serviceUrl": "http://ner:8080"
        }
      ]
    }
  }
}
```

### Switch Router Example

Conditional routing based on request content:

```json
{
  "nodes": {
    "root": {
      "routerType": "switch",
      "steps": [
        {
          "stepName": "premium-route",
          "serviceUrl": "http://premium-model:8080",
          "condition": "user.tier == 'premium'"
        },
        {
          "stepName": "standard-route",
          "serviceUrl": "http://standard-model:8080",
          "condition": "user.tier == 'standard'"
        },
        {
          "stepName": "default-route",
          "serviceUrl": "http://basic-model:8080"
        }
      ]
    }
  }
}
``` 