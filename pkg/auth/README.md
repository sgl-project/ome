# Auth Package

The auth package provides a unified authentication framework for multiple cloud providers. It follows the same pattern as the existing principals package but extends support to AWS, GCP, Azure, and GitHub.

## Features

- **Multi-Provider Support**: Unified interface for OCI, AWS, GCP, Azure, and GitHub authentication
- **Multiple Auth Types**: Support for various authentication methods per provider
- **Credential Chaining**: Try multiple credential sources in sequence
- **Fallback Support**: Configure fallback authentication methods
- **HTTP Transport**: Built-in HTTP transport with automatic request signing
- **Extensible Design**: Easy to add new providers and auth types

## Supported Providers and Auth Types

### OCI (Oracle Cloud Infrastructure)
- User Principal (API Key)
- Instance Principal
- Resource Principal
- OKE Workload Identity

### AWS (Amazon Web Services)
- Access Key
- Instance Profile (EC2 metadata)
- Assume Role (STS)
- Web Identity (OIDC)

### GCP (Google Cloud Platform)
- Service Account
- Application Default Credentials
- Workload Identity (GKE)

### Azure (Microsoft Azure)
- Service Principal
- Managed Identity
- Device Flow

### GitHub
- Personal Access Token
- GitHub App

## Installation

```go
import "github.com/sgl-project/ome/pkg/auth"
```

## Usage

### Creating Credentials

```go
// Create auth factory
logger := logging.NewLogger()
factory := auth.NewDefaultFactory(logger)

// Configure authentication
config := auth.Config{
    Provider: auth.ProviderOCI,
    AuthType: auth.OCIInstancePrincipal,
    Region:   "us-ashburn-1",
}

// Create credentials
ctx := context.Background()
credentials, err := factory.Create(ctx, config)
```

### Using Fallback Authentication

```go
config := auth.Config{
    Provider: auth.ProviderAWS,
    AuthType: auth.AWSWebIdentity,
    Fallback: &auth.Config{
        Provider: auth.ProviderAWS,
        AuthType: auth.AWSInstanceProfile,
    },
}

// Will try Web Identity first, fall back to Instance Profile if it fails
credentials, err := factory.Create(ctx, config)
```

### Credential Chaining

```go
// Create chain of credential providers
chain := &auth.ChainProvider{
    Providers: []auth.CredentialsProvider{
        envProvider,      // Try environment variables
        fileProvider,     // Try config file
        instanceProvider, // Try instance metadata
    },
}

// Get credentials from first successful provider
credentials, err := chain.GetCredentials(ctx)
```

### Signing HTTP Requests

```go
// Create HTTP client with automatic signing
transport := &auth.HTTPTransport{
    Base:        http.DefaultTransport,
    Credentials: credentials,
}

client := &http.Client{
    Transport: transport,
}

// Requests will be automatically signed
resp, err := client.Get("https://api.example.com/data")
```

## Configuration

### OCI Configuration

```go
config := auth.Config{
    Provider: auth.ProviderOCI,
    AuthType: auth.OCIUserPrincipal,
    Extra: map[string]interface{}{
        "user_principal": map[string]interface{}{
            "config_path":       "~/.oci/config",
            "profile":           "DEFAULT",
            "use_session_token": false,
        },
    },
}
```

### AWS Configuration

```go
config := auth.Config{
    Provider: auth.ProviderAWS,
    AuthType: auth.AWSAccessKey,
    Region:   "us-east-1",
    Extra: map[string]interface{}{
        "access_key_id":     "AKIAIOSFODNN7EXAMPLE",
        "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    },
}
```

## Implementing Custom Providers

To add a new authentication provider:

1. Create a new package under `auth/yourprovider`
2. Implement the `auth.Credentials` interface
3. Implement a factory that creates credentials
4. Register the factory in the default factory

Example:

```go
type YourProviderFactory struct {
    logger logging.Interface
}

func (f *YourProviderFactory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
    // Implementation
}

func (f *YourProviderFactory) SupportedAuthTypes() []auth.AuthType {
    // Return supported auth types
}
```

## Testing

The package includes comprehensive unit tests. Run tests with:

```bash
go test ./pkg/auth/...
```

## Security Considerations

- Never log or expose credentials
- Use appropriate credential rotation strategies
- Follow the principle of least privilege
- Use managed identities/instance principals when possible
- Store long-term credentials securely (e.g., using secret managers)

## Future Enhancements

- Complete implementation of AWS, GCP, Azure, and GitHub providers