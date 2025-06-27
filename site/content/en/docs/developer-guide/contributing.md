---
title: "Contributing to OME"
linkTitle: "Contributing"
weight: 5
date: 2023-03-14
description: >
  Learn how to contribute to the OME project, set up your development environment, and follow our coding guidelines.
---

Thank you for your interest in contributing to OME! This repository is open to everyone and welcomes all kinds of contributions, no matter how small or large. 

## Ways to Contribute

There are several ways you can contribute to the project:

- **Bug Reports**: Identify and report issues or bugs
- **Feature Requests**: Suggest new features or improvements
- **Code Contributions**: Submit bug fixes or implement new features
- **Documentation**: Improve documentation and guides
- **Testing**: Help with testing and quality assurance

## Development Environment Setup

### Prerequisites

Before you begin, you'll need to install these tools:

**Required Tools:**
- [Go 1.22.0+](https://golang.org/doc/install) - OME controller is written in Go
- [Git](https://help.github.com/articles/set-up-git/) - For source control
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) - For managing development environments
- [kustomize v5.0.0+](https://github.com/kubernetes-sigs/kustomize/) - To customize YAMLs for different environments
- [yq 4.x](https://github.com/mikefarah/yq) - Used in project makefiles to parse YAML output

**Container Registry Access:**
- Access to a container registry
- Docker login credentials

### Environment Variables

Set up these environment variables (add them to your `.bashrc` or `.zshrc`):

```bash
# Go workspace
export GOPATH=<your-go-workspace>
export PATH=$GOPATH/bin:$PATH


# Container registry
export REGISTRY="<your-registry-url>"  # Adjust to your registry

# Architecture (for M1/M2 MacBook)
export ARCH="linux/arm64"  # Only needed for Apple Silicon
```

### Clone the Repository

Clone OME to the correct location in your GOPATH:

```bash
mkdir -p ${GOPATH}/src/github.com/sgl-project
cd ${GOPATH}/src/github.com/sgl-project
git clone https://github.com/sgl-project/ome.git
cd ome
```

### Container Registry Login

Set up access to your container registry:

```bash
# Generate an auth token in your cloud provider's console
docker login <your-registry-url> -u <username>
# Enter your auth token as the password
```

## Building and Testing

### Build the Project

```bash
# Format code
make fmt

# Run linting
make vet

# Clean up dependencies
make tidy

# Generate manifests and code
make generate
make manifests

# Build and test
make test
```

### Deploy Your Changes

**Option 1: Use Development Images**
```bash
# Build and push your custom image
make push-manager-image

# Deploy with your custom image
make patch-manager-dev
make install
```

**Option 2: Run Locally**
```bash
# For local development (removes webhooks)
make delete-webhooks
make run-ome-manager
```

### Verify Installation

Check that OME components are running:

```bash
# Check controller manager
kubectl get pods -n ome -l control-plane=ome-controller-manager

# Check model controller
kubectl get pods -n ome -l control-plane=ome-model-controller

# Check model agent daemonset
kubectl get pods -n ome -l control-plane=ome-model-agent-daemonset
```

## IDE Configuration

### VS Code / Cursor Setup

1. **Install Go Extension:**
   - Open Extensions (Ctrl+Shift+X)
   - Search for "Go" and install the official extension

2. **Install Go Tools:**
   - Command Palette (Ctrl+Shift+P) → "Go: Install/Update Tools"
   - Select all tools and install

3. **Configure Launch Settings:**

Create `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "OME Manager",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/manager/main.go",
            "env": {
                "KUBECONFIG": "/path/to/your/kubeconfig"
            },
            "args": [
                "--zap-encoder", "console",
                "--health-probe-addr", "127.0.0.1:8081",
                "--metrics-bind-address", "127.0.0.1:8080",
                "--leader-elect"
            ]
        }
    ]
}
```

### GoLand Setup

1. **Create Run Configuration:**
   - Right-click `cmd/manager/main.go`
   - Select "Run 'go build main.go'"

2. **Edit Configuration:**
   - Name: `ome-controller-manager`
   - Run Kind: `File`
   - Files: `${GOPATH}/src/github.com/sgl-project/ome/cmd/manager/main.go`
   - Environment Variables: `KUBECONFIG=/path/to/kubeconfig`
   - Program Arguments: `--zap-encoder console --health-probe-addr 127.0.0.1:8081 --metrics-bind-address 127.0.0.1:8080 --leader-elect`

## Coding Guidelines

### Style Guide

We follow the [Google Go Style Guide](https://google.github.io/styleguide/go/):

- Use `gofmt` for formatting
- Follow standard Go naming conventions
- Write clear, self-documenting code
- Include appropriate comments for complex logic

### Code Formatting

Run these commands before submitting:

```bash
make fmt    # Format code
make vet    # Run static analysis
make tidy   # Clean up dependencies
```

### Testing

- Add test cases for all new functionality
- For bug fixes, tests should fail without your changes
- Aim for good test coverage of critical paths
- Run tests before submitting: `make test`

## Pull Request Process

### Before Submitting

1. **Rebase on latest main:**
   ```bash
   git rebase origin/main
   ```

2. **Run quality checks:**
   ```bash
   make fmt vet tidy test
   ```

3. **Update documentation** as necessary

4. **Test your changes** thoroughly

### PR Requirements

**Title Format:**
Use appropriate prefixes to classify your PR:

- `[Bugfix]` - Bug fixes
- `[Core]` - Core controller changes, build, version upgrades
- `[API]` - OME API changes
- `[Helm]` - Helm chart changes
- `[Docs]` - Documentation changes
- `[CI/Tests]` - Unit tests and integration tests
- `[Misc]` - Other changes (use sparingly)
- `[OEP]` - OME Enhancement Proposals

**Example:** `[API] Add support for custom model configurations`

**Commit Message Guidelines:**
- Keep titles under 52 characters
- Keep message lines under 72 characters
- Provide clear, descriptive commit messages

**PR Description:**
Include:
- **What**: What changes you made
- **Why**: Motivation for the changes
- **How**: How you implemented the solution
- **Testing**: How you tested the changes

### Review Process

1. **Automated Checks**: All CI/CD checks must pass
2. **Code Review**: At least one approval from a maintainer
3. **Testing**: Manual testing may be required for complex changes
4. **Documentation**: Ensure docs are updated if needed

## OME Enhancement Proposals (OEPs)

For substantial changes, you'll need to create an OEP. This includes:

- Significant architectural changes
- Major feature additions (new CRDs)
- Breaking API changes
- Changes affecting multiple components
- Modifications to core behaviors

### Creating an OEP

1. **Create a new branch** for your OEP

2. **Copy the template:**
   ```bash
   cp -r oeps/NNNN-template oeps/XXXX-descriptive-name
   ```

3. **Fill out the template** with:
   - **Title**: Clear, concise description
   - **Summary**: High-level overview
   - **Motivation**: Why this change is needed
   - **Goals/Non-Goals**: Specific objectives and scope
   - **Proposal**: Detailed description
   - **Design Details**: Technical implementation
   - **Alternatives**: Other approaches considered

4. **Submit as PR** with `[OEP]` prefix

### OEP Review Process

1. **Initial Review** (≤1 week): Feasibility and alignment assessment
2. **Feedback Integration**: Address comments and refine design
3. **Final Approval**: Sign-off from required reviewers

## Making Changes

### API Changes

If you modify API types in `pkg/apis/`, run:

```bash
make generate  # Generate Go clients
make manifests # Generate CRDs and other manifests
```

### Adding Dependencies

**New Dependencies:**
- Add imports to your Go files
- Dependencies are automatically added to `go.mod`

**Upgrading Dependencies:**
```bash
# Latest version
go get golang.org/x/text

# Specific version
go get golang.org/x/text@v0.3.0
```

### Configuration Changes

If you modify files in `config/`, run:

```bash
make manifests
```

## Testing Your Changes

### Unit Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./pkg/controllers/...

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Deploy your changes
make push-manager-image
make patch-manager-dev
make install

# Test with sample configurations
kubectl apply -f config/samples/iscv/deepseek-ai/deepseek-r1.yaml

# Verify functionality
kubectl get inferenceservice -A
```

### End-to-End Testing

1. **Deploy a complete example:**
   ```bash
   kubectl apply -f config/samples/benchmark/e5-mistral-7b-instruct.yaml
   ```

2. **Test the full workflow:**
   ```bash
   # Check service deployment
   kubectl get pods -n e5-mistral-7b-instruct
   
   # Test inference
   curl -X POST "http://e5-mistral-7b-instruct.e5-mistral-7b-instruct:8080/v1/embeddings" \
     -H "Content-Type: application/json" \
     -d '{"input": "Hello world", "model": "e5-mistral-7b-instruct"}'
   ```

## Getting Help

### Community Resources

- **Documentation**: [OME Documentation](https://ome.docs.example.com)
- **Issue Tracker**: Report bugs and request features
- **Discussions**: Ask questions and share ideas

### Common Issues

**Build Problems:**
```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download
```

**Registry Access:**
```bash
# Re-authenticate
docker login <your-registry-url>

# Check access
docker pull <your-registry-url>/official-sgl:v0.4.5.5e34fb5-cu124
```

**Webhook Issues:**
```bash
# Remove webhooks for local development
make delete-webhooks

# Reinstall cert-manager if needed
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

## Best Practices

### Development Workflow

1. **Small, Focused Changes**: Keep PRs small and focused
2. **Test Early and Often**: Test changes as you develop
3. **Document Changes**: Update docs with your changes
4. **Follow Conventions**: Stick to established patterns
5. **Seek Feedback**: Don't hesitate to ask for help

### Code Quality

1. **Clear Naming**: Use descriptive variable and function names
2. **Error Handling**: Properly handle and wrap errors
3. **Logging**: Use structured logging with appropriate levels
4. **Comments**: Comment complex logic and public APIs
5. **Performance**: Consider performance implications

### Security

1. **Secret Management**: Never commit secrets or credentials
2. **Input Validation**: Validate all user inputs
3. **RBAC**: Follow principle of least privilege
4. **Container Security**: Use secure base images

## Next Steps

After setting up your development environment:

1. **Start Small**: Look for "good first issue" labels
2. **Read the Code**: Familiarize yourself with the codebase
3. **Join Discussions**: Participate in community discussions
4. **Ask Questions**: Don't hesitate to ask for clarification

Happy contributing! 🚀 