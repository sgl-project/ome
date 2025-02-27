# OpenAI Admin Utility

A production-ready CLI tool for managing OpenAI resources and configurations. The utility provides various administrative commands for managing OpenAI organizations, API keys, and other resources.

## Architecture

### Components

1. **Organization Watcher**
   - Periodically scans for organizations with OpenAI vendor configuration
   - Filters organizations based on vendor type and secret reference presence
   - Sends organizations to the key rotator via a channel
   - Maintains metrics for total organizations being monitored

2. **Key Rotator**
   - Processes organizations from the watcher
   - Checks API key age using OpenAI Admin API
   - Creates new admin API keys when rotation is needed
   - Updates both Kubernetes secrets and organization resources
   - Tracks rotation status and timing through metrics

3. **Health and Metrics**
   - Exposes health check endpoint at `/healthz`
   - Exposes readiness check endpoint at `/readyz`
   - Provides Prometheus metrics at `/metrics`
   - Tracks key metrics:
     - Total number of organizations
     - Key rotation status (success/error/skipped)
     - Last rotation timestamp per organization

### Workflow

1. **Organization Discovery**
   - The watcher scans for organizations at configured intervals
   - Identifies organizations with OpenAI vendor configuration
   - Validates presence of secret references

2. **Key Management**
   - For each organization:
     1. Retrieves current API key from secret
     2. Checks key age via OpenAI Admin API
     3. If rotation needed:
        - Creates new admin API key
        - Updates secret with new key
        - Updates organization's secret reference
     4. Updates metrics based on operation status

3. **Monitoring**
   - Exposes Prometheus metrics for monitoring
   - Provides health and readiness check endpoints
   - Logs errors and operation status

## User Guide

### Installation

```bash
go build -o openai-admin-util .
```

### Available Commands

```bash
openai-admin-util [command] [flags]
```

Commands:
- `rotate-admin-keys`: Rotate OpenAI admin API keys for organizations

Run `openai-admin-util help` for a list of all available commands.

### Key Rotation Configuration

The `rotate-admin-keys` command supports the following flags:

```bash
-w, --watch-interval      Duration between organization scans (default: 5m)
-r, --rotation-interval   Duration before rotating API keys (default: 720h/30d)
-m, --metrics-port       Port for Prometheus metrics (default: 9090)
-p, --health-port        Port for health check endpoints (default: 8080)
-k, --kubeconfig         Path to kubeconfig file (optional, uses in-cluster config if not set)
```

### Example Usage

1. **Start key rotation with default settings**:
   ```bash
   ./openai-admin-util rotate-admin-keys
   ```

2. **Custom rotation interval (7 days)**:
   ```bash
   ./openai-admin-util rotate-admin-keys --rotation-interval=168h
   ```

3. **Custom watch interval and ports**:
   ```bash
   ./openai-admin-util rotate-admin-keys \
     --watch-interval=10m \
     --metrics-port=8081 \
     --health-port=8082
   ```

4. **Using external kubeconfig**:
   ```bash
   ./openai-admin-util rotate-admin-keys --kubeconfig=/path/to/kubeconfig
   ```

### Monitoring

1. **Health Check**:
   ```bash
   curl http://localhost:8080/healthz
   ```

2. **Readiness Check**:
   ```bash
   curl http://localhost:8080/readyz
   ```

3. **Metrics**:
   ```bash
   curl http://localhost:9090/metrics
   ```

   Available metrics:
   - `openai_admin_total_organizations`: Total number of OpenAI organizations
   - `openai_admin_key_rotation_status`: Count of key rotation operations by status
   - `openai_admin_last_key_rotation_time`: Timestamp of last rotation attempt

### Organization Configuration

Organizations must have:
1. Vendor type set to "openai"
2. Valid secret reference with:
   - Name
   - Namespace
   - Key

Example organization YAML:
```yaml
apiVersion: ome.oracle.com/v1beta1
kind: Organization
metadata:
  name: example-org
spec:
  vendor: openai
  secretRef:
    name: openai-keys
    namespace: default
    key: admin-key
```

## Troubleshooting

Common issues and solutions:

1. **Key Rotation Failures**:
   - Check OpenAI API permissions
   - Verify secret write permissions
   - Ensure valid organization configuration

2. **Service Not Ready**:
   - Check `/readyz` endpoint
   - Verify all components are initialized
   - Check for required permissions

3. **Organization Not Found**:
   - Verify vendor configuration
   - Check secret reference
   - Validate RBAC permissions
