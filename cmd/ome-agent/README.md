# OME-Agent
OME-Agent is a robust, multi-functional tool designed to manage various tasks related to model inference and training in OME. OME-Agent is built as a command-line tool, implemented in Golang, and structured to handle complex model management operations such as replication, encryption, and decryption. With its modular command structure, OME-Agent consolidates critical tasks into a single, easy-to-use “Swiss Army Knife” solution for OME Operator.

## Features
OME-Agent provides the following capabilities:

1. Model Replication from HuggingFace
   - Nested File Handling: Downloads all files within a model, including those nested within subdirectories.
   - Multithreaded Downloads: Accelerates large file downloads, particularly for files stored in Git LFS.
   - Integrity Verification: Confirms successful downloads by validating SHA256 checksums.
   - Resume Interrupted Downloads: Automatically resumes incomplete downloads.
   - Smart Skipping: Detects and skips files that are already downloaded, saving time and bandwidth.
   - HuggingFace Token Authentication: Supports HuggingFace Access Tokens for access-restricted models and datasets.
   - Branch-Specific Updates: Tracks and updates files based on branch changes.
2. Object Storage Replication Between OCI Buckets
   - Cross-Bucket Replication: Copies models between OCI buckets to support data redundancy and facilitate multi-region deployments.
   - Region/Tenancy Support: Provides flexibility to replicate models across OCI regions and tenancies as needed.
   - Configurable Concurrency: Allows customization of concurrent connections for optimized upload/download speeds. 
3. Model Weight Encryption and Decryption
   - OCI Vault Integration: Uses OCI Vault and Key Management Service (KMS) to decrypt model weights securely.
   - Advanced Encryption Standards: Protects model data by supporting decryption of large, sensitive files for regulated environments.

## Getting Started
### Prerequisites
- Go Version 1.23.0 or later.
- OCI CLI and SDK for interacting with Oracle Cloud Infrastructure resources. (Optional if not using OCI services)
- HuggingFace Access Token if downloading restricted models. (Optional if downloading public models)
- OCI Vault and KMS setup for secure decryption of model weights. (Optional if not decrypting model weights)
- `GOPATH`: If you don't have one, simply pick a directory and add
     `export GOPATH=...`
- `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.
- `GONOPROXY`: Set go proxy to pull the dependencies from the internal Oracle bitbucket repository `oracle.com/oci,bitbucket.oci.oraclecorp.com`.
- `GOPRIVATE`: Set go private to pull the dependencies from the internal Oracle bitbucket repository `oracle.com/oci,bitbucket.oci.oraclecorp.com`.

### Installation
Clone the repository and install the OME-Agent CLI.
```bash
mkdir -p ${GOPATH}/src/bitbucket.oci.oraclecorp.com/gencore
cd ${GOPATH}/src/bitbucket.oci.oraclecorp.com/gencore
git clone ssh://git@bitbucket.oci.oraclecorp.com:7999/gencore/ome.git
cd ome
make ome-agent
```

### Configuration
OME-Agent supports both environment variables and configuration files for setting up the agent.

Sample configuration yaml file:
```yaml
model_store_directory: "<local path to store model weight>"
local_path: "<local path to store model weight>"
model_name: "meta-llama/Meta-Llama-3-8B"
hf_token: "<insert your own token>"
num_connections: 20
skip_sha: false
max_retries: 5
retry_internal_in_seconds: 10

auth_type: &default_auth_type "UserPrincipal"
profile: "DEFAULT"

download_size_limit_gb: 650
enable_size_limit_check: true

source:
  bucket_name: "model-store"
  prefix: "meta/llama-3-2-1b/"
  region: "us-chicago-1"
  namespace: "idqj093njucb"

target:
  bucket_name: "test-bucket"
  prefix: "meta/llama-3-2-1b/"
  region: "eu-frankfurt-1"
  namespace: "idqj093njucb"
```

Supported environment variables:
All environment variables ***must*** start the prefix `OME_AGENT_` to be recognized by the OME-Agent.

| YAML Key                    | Environment Variable                | Default | Required |
|-----------------------------|-------------------------------------|---------|----------|
| `auth_type`                 | `OME_AGENT_AUTH_TYPE`               |         | yes      |
| `profile`                   | `OME_AGENT_PROFILE`                 | DEFAULT | no       |
| `local_path`                | `OME_AGENT_LOCAL_PATH`              |         | yes      |
| `skip_sha`                  | `OME_AGENT_SKIP_SHA`                | false   | no       |
| `max_retry`                 | `OME_AGENT_MAX_RETRY`               | 5       | no       |
| `retry_internal_in_seconds` | `OME_AGENT_RETRY_INTERVAL`          | 10      | no       |
| `model_name`                | `OME_AGENT_MODEL_NAME`              |         | yes      |
| `hf_token`                  | `OME_AGENT_HF_TOKEN`                |         | no       |
| `num_connections`           | `OME_AGENT_NUM_CONNECTIONS`         | 10      | no       |
| `download_size_limit_gb`    | `OME_AGENT_DOWNLOAD_SIZE_LIMIT_GB`  | 650     | no       |
| `enable_size_limit_check`   | `OME_AGENT_ENABLE_SIZE_LIMIT_CHECK` | true    | no       |
| `source.bucket_name`        | `OME_AGENT_SOURCE_BUCKET_NAME`      |         | yes      |
| `source.prefix`             | `OME_AGENT_SOURCE_PREFIX`           |         | no       |
| `source.region`             | `OME_AGENT_SOURCE_REGION`           |         | yes      |
| `source.namespace`          | `OME_AGENT_SOURCE_NAMESPACE`        |         | yes      |
| `target.bucket_name`        | `OME_AGENT_TARGET_BUCKET_NAME`      |         | yes      |
| `target.prefix`             | `OME_AGENT_TARGET_PREFIX`           |         | no       |
| `target.region`             | `OME_AGENT_TARGET_REGION`           |         | yes      |
| `target.namespace`          | `OME_AGENT_TARGET_NAMESPACE`        |         | yes      |

### Usage
OME-Agent uses subcommands to run specific tasks. Use the following commands:=
```bash
./ome-agent hf-download --config <path-to-config.yaml> --debug
```
```bash
./ome-agent replica --config <path-to-config.yaml> --debug
```
```bash
./ome-agent enigma --config <path-to-config.yaml> --debug
```



## Development Guide

### Make Commands

OME-Agent comes with several Makefile commands to simplify development, building, and deployment tasks. Below is a summary of the main commands available:

```bash
# Builds the OME-Agent CLI.
make ome-agent
# Builds the Docker image for OME-Agent, tagging it with the specified REGISTRY and TAG variables. 
make ome-agent-image
# Pushes the OME-Agent Docker image to the specified REGISTRY.
make push-ome-agent-image
# Runs the OME-Agent CLI with the specified subcommand (e.g., hf-download, replica, enigma).
make run-ome-agent-enigma
make run-ome-agent-hf-download
make run-ome-agent-os-replica
```

### Code Structure
OME-Agent follows a modular and scalable design, which makes it easy to extend with new features. The core structure relies on:
1.	[Cobra](https://cobra.dev/): For command-line interface (CLI) management, organizing tasks into distinct subcommands (hf-download, enigma, and replica).
2.	[Fx](https://uber.github.io/fx/): A dependency injection framework that manages the lifecycle of each component, injecting necessary dependencies and ensuring clean startup/shutdown of services.
3.	[Viper](https://pkg.go.dev/github.com/spf13/viper): For configuration management, allowing settings to be loaded from a configuration file, environment variables, or command-line flags.

The main directory layout might look something like this:
```
ome-agent/
├── cmd/                    # Contains Cobra subcommands
│   ├──main.go              # Main entry point for the CLI
│   ├── hf_download.go      # Subcommand for HuggingFace model downloads
│   ├── enigma.go           # Subcommand for model encryption/decryption
│   └── replica.go          # Subcommand for object storage replication
├── internal/               # Contains the core business logic for each feature
│   ├── hf_download/        # Logic for HuggingFace downloading
│   ├── enigma/             # Logic for encryption/decryption
│   └── replica/            # Logic for replication across OCI buckets
├── pkg/                    # Shared libraries and utility functions
│   ├── configutils/        # Utility functions for handling configuration files
│   ├── constants/          # Common constants used across the project
│   ├── logging/            # Logging module for consistent logging across commands
│   └── secrets/            # Secret management (e.g., KMS integration for decryption)
```

### Key Components Explained

#### Main Entry (main.go)
   The main.go file initializes the root Cobra command (ome-agent) and executes it. All subcommands are registered here using rootCmd.AddCommand. This file acts as the command dispatcher, directing commands like hf-download, enigma, and replica to their respective handlers.
### Command Files (hf_download.go, enigma.go, replica.go)
   Each subcommand file defines:
   - Command Metadata: Such as Use, Short, and Long descriptions.
   - Run Function: This is where the logic for each command begins. When a user invokes a command, the Run function creates an Fx application, passing along necessary dependencies and modules.
   - Flags: Each command can define its own flags (e.g., --config, --debug) to customize execution.

For example, in hf_download.go, the Run function invokes runHFDownload, which sets up and starts the HuggingFace Download Agent.

#### Config Provider (configProvider function)

The configProvider function is responsible for loading configurations into a Viper instance, which is then injected into the Fx app. It does the following:

1. Sets up default values and environment variable mappings.
2. Loads configurations from a file (specified by the --config flag).
3. Allows configurations to be overridden by environment variables.

Using Viper enables flexible configuration management, as it can support different deployment environments with minimal change.

#### Dependency Injection with Fx

Fx handles the orchestration of dependencies, lifecycle management, and dependency injection for OME-Agent. Each subcommand creates an Fx application with an fx.New() call that includes:

- Configuration: configProvider injects Viper with configurations for each agent.
- Modules: These represent the core functionalities and external dependencies, like:
  - Environment Management (env.Module): Manages environment variables and system-level configurations.
  - File System Abstraction (afero.Module): Provides a file abstraction layer for operations like file download, allowing for easier testing and extensibility.
  - Logging (logging.Module): Sets up logging using Zap, enabling structured logging across the application.
  - Secrets (keymanagement and secretretrieval): Manages interactions with OCI’s Vault and Key Management Service.

#### Main Application Module

Each subcommand has its own main application module, implemented as an Fx Option. For example:

- HuggingFace Download Agent (hf_download.Module): Handles the logic for downloading models from HuggingFace, validating checksums, resuming interrupted downloads, and managing access tokens.
- Replica Agent (replica.Module): Manages replication of model weights across OCI buckets, handling cross-region and cross-tenancy replication.
- Enigma Agent (enigma.Module): Manages encryption and decryption of model weights using OCI Vault and KMS.

Each module includes the main agent logic and is registered as a dependency in the fx.Options list for that subcommand.

#### Lifecycle Hooks

Each agent uses an Fx Lifecycle Hook to manage the OnStart and OnStop events. This ensures:

- OnStart: The agent starts and performs its primary function. For example, the HuggingFace Download Agent begins downloading the model files.
- OnStop: The agent shuts down gracefully when the CLI terminates.

#### Example code for managing lifecycle hooks:
```go
fx.Invoke(func(lc fx.Lifecycle, agent *hf_download.HFDownloadAgent, logger *zap.Logger, shutdown fx.Shutdowner) {
    lc.Append(fx.Hook{
        OnStart: func(context.Context) error {
            go func() {
                if err := agent.Start(); err != nil {
                    logger.Error("Error starting agent", zap.Error(err))
                }
                shutdown.Shutdown()
            }()
            return nil
        },
        OnStop: func(ctx context.Context) error {
            return nil
        },
    })
})
```
This hook architecture allows each agent to handle startup tasks asynchronously and exit cleanly, providing flexibility and stability during operation.


#### Adding New Commands
New functionality can be added by creating a new command using Cobra and integrating it with Fx as shown below:

 1.	Define a new command.
 2.	Set up dependencies and configuration.
 3.	Add the command to the main command tree.

Example:
```go
var cmdNewFeature = &cobra.Command{
    Use:   "new-feature",
    Short: "Description of new feature",
    Run:   runNewFeature,
}

func init() {
    rootCmd.AddCommand(cmdNewFeature)
}
```
