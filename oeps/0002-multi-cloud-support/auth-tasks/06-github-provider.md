# Auth Task Group 6: GitHub Authentication Provider

## Overview
Implement GitHub authentication support for accessing GitHub repositories, releases, and artifacts. This includes Personal Access Tokens, GitHub Apps, and OAuth authentication methods.

## Tasks

### Task 6.1: Create GitHub Authentication Factory ✅ COMPLETED

**Description**: Implement the GitHub-specific authentication factory that integrates with the core authentication framework. This factory will handle GitHub-specific configuration and credential instantiation.

**Time Estimate**: 1 day

**Dependencies**: Core Framework (Task Group 1)

**Deliverables**:
- ✅ `pkg/auth/github/factory.go` with Factory implementation
- ✅ Registration handled externally
- ✅ GitHub-specific configuration types

**Implementation Status**:
- ✅ Factory struct with Create() and SupportedAuthTypes()
- ✅ Supports PersonalAccessToken, GitHubApp, OAuth auth types
- ✅ Configuration extraction from auth.Config.Extra
- ✅ OAuth2 client creation
- ⚠️ GitHub Enterprise endpoints via base URL config

**Acceptance Criteria**:
- ✅ Factory identifies GitHub auth types
- ✅ Validates provider type
- ⚠️ Enterprise support possible via config
- ✅ Returns appropriate errors
- ✅ Unit tests implemented

---

### Task 6.2: Implement Personal Access Token Authentication ✅ COMPLETED

**Description**: Implement authentication using GitHub Personal Access Tokens (PATs), including both classic and fine-grained tokens.

**Time Estimate**: 1 day

**Dependencies**: Task 6.1

**Deliverables**:
- ✅ PAT logic in factory.go
- ✅ PersonalAccessTokenConfig in credentials.go
- ✅ Static token source implementation

**Implementation Status**:
- ✅ PersonalAccessTokenConfig with validation
- ✅ Environment variable support (GITHUB_TOKEN, GH_TOKEN)
- ✅ Configuration via auth.Config.Extra
- ✅ NewStaticTokenSource for OAuth2
- ✅ Token format validation
- ⚠️ Scope verification via API calls
- ⚠️ Rate limiting handled by clients

**Acceptance Criteria**:
- ✅ Reads from environment variables
- ✅ Configuration support
- ✅ Validates token not empty
- ⚠️ Scope verification at API level
- ⚠️ Rate limiting by HTTP client
- ✅ Enterprise via base URL
- ⚠️ Scope errors from API
- ✅ Unit tests implemented
- ⚠️ Integration tests in storage

---

### Task 6.3: Implement GitHub App Authentication ⚠️ PARTIALLY COMPLETED

**Description**: Implement authentication as a GitHub App using JWT tokens and installation access tokens. This provides higher rate limits and better security.

**Time Estimate**: 3 days

**Dependencies**: Task 6.1

**Deliverables**:
- ⚠️ Basic GitHub App support in factory.go
- ❌ JWT generation not implemented
- ❌ Installation token management not implemented

**Implementation Status**:
- ✅ GitHubAppConfig structure defined
- ✅ Factory method createGitHubAppTokenSource
- ❌ JWT token generation missing
- ❌ Installation access token logic missing
- ❌ Private key loading not implemented
- ❌ Token refresh not implemented

**Acceptance Criteria**:
- ❌ Private key loading
- ❌ JWT generation
- ❌ Installation listing
- ❌ Installation tokens
- ❌ Token expiry handling
- ❌ Repository selection
- ❌ Permission scopes
- ⚠️ Basic structure tested
- ❌ Integration tests needed

---

### Task 6.4: Implement OAuth App Authentication ⚠️ PARTIALLY COMPLETED

**Description**: Implement OAuth application flow for user authorization, supporting both web application flow and device flow.

**Time Estimate**: 2 days

**Dependencies**: Task 6.1

**Deliverables**:
- ⚠️ Basic OAuth support in factory.go
- ❌ Device flow not implemented
- ❌ Token storage not implemented

**Implementation Status**:
- ✅ OAuthConfig structure defined
- ✅ Factory method createOAuthTokenSource
- ✅ Static token support for existing tokens
- ❌ Web application flow not implemented
- ❌ Device flow not implemented
- ❌ Authorization callbacks missing
- ❌ Refresh token management missing

**Acceptance Criteria**:
- ❌ Web application flow
- ❌ Device flow support
- ❌ Authorization callbacks
- ❌ Refresh tokens
- ❌ PKCE implementation
- Stores tokens securely
- Handles scope negotiation
- Unit tests for OAuth flows
- Integration tests with GitHub OAuth

---

### Task 6.5: Add GitHub Actions OIDC Support

**Description**: Implement authentication from GitHub Actions using OpenID Connect tokens, enabling secure keyless authentication.

**Time Estimate**: 2 days

**Dependencies**: Task 6.1

**Deliverables**:
- `pkg/auth/github/actions.go` with Actions support
- OIDC token retrieval
- Token validation

**Acceptance Criteria**:
- Detects GitHub Actions environment
- Retrieves OIDC token from Actions
- Validates token claims
- Supports custom audiences
- Handles job-specific tokens
- Verifies token signatures
- Unit tests with mock tokens
- Integration tests in Actions

---

### Task 6.6: Implement GitHub Enterprise Support

**Description**: Extend all authentication methods to support GitHub Enterprise Server with custom endpoints and potentially different API versions.

**Time Estimate**: 2 days

**Dependencies**: Tasks 6.2-6.5

**Deliverables**:
- Enterprise endpoint configuration
- API version detection
- Certificate handling

**Acceptance Criteria**:
- Supports custom Enterprise URLs
- Handles self-signed certificates
- Detects API version differences
- Validates Enterprise endpoints
- Supports LDAP-backed auth
- Handles Enterprise-specific features
- Unit tests with Enterprise endpoints
- Integration tests with Enterprise

---

### Task 6.7: Add Rate Limit Management

**Description**: Implement comprehensive rate limit tracking and management, including primary and secondary rate limits.

**Time Estimate**: 2 days

**Dependencies**: Task 6.1

**Deliverables**:
- `pkg/auth/github/ratelimit.go` with rate limiting
- Automatic retry with backoff
- Rate limit metrics

**Acceptance Criteria**:
- Tracks primary rate limits
- Handles secondary rate limits
- Implements smart backoff
- Provides rate limit status
- Supports conditional requests
- Optimizes API calls
- Unit tests for rate limiting
- Load tests with rate limits

---

### Task 6.8: Create GitHub Credential Chain

**Description**: Implement a GitHub-specific credential chain that tries multiple authentication methods in optimal order.

**Time Estimate**: 1 day

**Dependencies**: Tasks 6.2-6.5

**Deliverables**:
- `pkg/auth/github/chain.go` with credential chain
- Smart ordering based on context
- Fallback strategies

**Acceptance Criteria**:
- Detects execution environment
- Orders methods by rate limits
- Actions OIDC → App → PAT → OAuth
- Caches successful methods
- Provides clear error messages
- Unit tests for all paths
- Performance benchmarks

---

### Task 6.9: Implement Token Scope Validation

**Description**: Add comprehensive token scope validation to ensure tokens have required permissions before attempting operations.

**Time Estimate**: 1 day

**Dependencies**: Task 6.2, Task 6.3

**Deliverables**:
- Scope validation utilities
- Operation to scope mapping
- Helpful error messages

**Acceptance Criteria**:
- Maps operations to required scopes
- Validates token scopes upfront
- Provides clear scope error messages
- Suggests minimum required scopes
- Handles fine-grained permissions
- Supports Enterprise permissions
- Unit tests for scope logic
- Integration tests with API

---

### Task 6.10: Add Credential Helper Integration

**Description**: Implement integration with Git credential helpers for seamless authentication with Git operations.

**Time Estimate**: 1 day

**Dependencies**: Task 6.2

**Deliverables**:
- Git credential helper protocol
- Credential storage integration
- Helper registration

**Acceptance Criteria**:
- Implements credential helper protocol
- Stores credentials securely
- Integrates with system keychain
- Supports multiple accounts
- Handles credential updates
- Works with Git LFS
- Unit tests for protocol
- Integration tests with Git

---

## Summary

**Total Time Estimate**: 16 days

**Current Status**: Basic GitHub authentication implemented with PAT support

**Completed Deliverables**:
- ✅ GitHub authentication factory
- ✅ Personal Access Token (PAT) authentication
- ✅ OAuth2 token source integration
- ✅ Environment variable support
- ✅ HTTP client with OAuth2 transport

**Pending Deliverables**:
- ❌ GitHub App JWT authentication
- ❌ OAuth web/device flows
- ❌ GitHub Actions OIDC
- ❌ Enterprise Server endpoints
- ❌ Rate limit handling
- ❌ Token scope validation
- ❌ Git credential helper

**Success Metrics Achieved**:
- ✅ PAT authentication works
- ❌ App/OAuth flows incomplete
- ❌ Actions OIDC not implemented
- ⚠️ Basic error messages
- ✅ Suitable for basic operations