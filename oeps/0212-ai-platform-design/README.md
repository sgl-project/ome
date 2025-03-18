# AI Platform Admin Operator Design

## Overview
The AI Platform Admin Operator manages various AI platform administrative resources in a Kubernetes-native way. It provides custom resources to manage organizations, projects, users, service accounts, and rate limits. Initially designed for OpenAI, but extensible to other AI platforms.

## Custom Resources

### Cluster-Scoped Resources

1. `Organization` (Cluster-scoped)
   - Represents an AI platform organization configuration
   - Contains organization-wide settings and API credentials
   - Supports multiple vendors (OpenAI, etc.)
   - Multiple instances per cluster

2. `Project` (Cluster-scoped)
   - Represents an AI platform project
   - Contains project-specific settings and configurations
   - Links to parent organization
   - Multiple instances per cluster

### Namespaced Resources

1. `ServiceAccount` (Namespace-scoped)
   - Represents a service account within a project
   - Links to specific project
   - Contains API key references and permissions

2. `User` (Namespace-scoped)
   - Represents a user within a project
   - Links to specific project
   - Contains user permissions and settings

3. `RateLimit` (Namespace-scoped)
   - Represents rate limit configurations for a project
   - Links to specific project/service account
   - Contains quota and rate limit settings

## Resource Relationships

```
Organization (Cluster)
    |
    +-- Project (Cluster)
           |
           +-- ServiceAccount (Namespace)
           |
           +-- User (Namespace)
           |
           +-- RateLimit (Namespace)
    
```

## Controller Design

The operator will include the following controllers:

1. Organization Controller
   - Manages organization-level configurations
   - Handles API credential management
   - Validates organization settings
   - Supports vendor-specific implementations

2. Project Controller
   - Creates and manages AI platform projects
   - Handles project lifecycle
   - Manages project settings and configurations
   - Implements vendor-specific project management

3. Service Account Controller
   - Manages service account lifecycle
   - Handles API key rotation
   - Configures service account permissions
   - Adapts to vendor-specific requirements

4. User Controller
   - Manages user lifecycle within projects
   - Handles user permissions
   - Syncs user settings with platform
   - Implements vendor-specific user management

5. Rate Limit Controller
   - Manages rate limit configurations
   - Monitors usage and quotas
   - Handles rate limit adjustments
   - Adapts to vendor-specific rate limiting

## Security Considerations

1. API Key Management
   - API keys stored in Kubernetes secrets
   - Automatic key rotation support
   - Secure credential handling
   - Vendor-specific key management

2. RBAC Integration
   - Integration with Kubernetes RBAC
   - Fine-grained access control
   - Role-based permissions

## Implementation Details

1. API Versions
   - Initial version: v1beta1
   - Future versions will follow semantic versioning

2. Status Subresource
   - All resources include status subresource
   - Real-time sync status with platform
   - Error conditions and state tracking
   - Vendor-specific status reporting

3. Validation
   - Webhook validation for all resources
   - Cross-resource validation
   - Vendor-specific API validation

4. Finalizers
   - Proper cleanup of platform resources
   - Dependency handling
   - Resource protection

5. Vendor Support
   - Initial support for OpenAI
   - Extensible design for additional vendors
   - Vendor-specific implementation interfaces
   - Common abstractions across vendors
