# Auth Task Group 3: AWS Authentication Provider

## Overview
Implement comprehensive Amazon Web Services (AWS) authentication support including Access Keys, IAM Roles, Instance Profiles, and various AWS-specific authentication methods.

## Tasks

### Task 3.1: Create AWS Authentication Factory

**Description**: Implement the AWS-specific authentication factory that integrates with the core authentication framework. This factory will handle AWS-specific configuration and credential instantiation.

**Time Estimate**: 1 day

**Dependencies**: Core Framework (Task Group 1)

**Deliverables**:
- `pkg/auth/aws/factory.go` with AWSFactory implementation
- Registration with DefaultFactory
- AWS-specific configuration types

**Acceptance Criteria**:
- Factory correctly identifies AWS auth types from configuration
- Validates AWS-specific configuration requirements
- Returns appropriate errors for invalid configurations
- Supports AWS SDK configuration precedence
- Unit tests cover all configuration scenarios

---

### Task 3.2: Implement Access Key Authentication

**Description**: Implement authentication using AWS access keys and secret keys. This includes support for temporary session tokens and proper AWS Signature Version 4 signing.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1

**Deliverables**:
- `pkg/auth/aws/access_keys.go` with implementation
- `pkg/auth/aws/credentials_file.go` for file parsing
- AWS Signature V4 implementation

**Acceptance Criteria**:
- Reads credentials from environment variables (AWS_ACCESS_KEY_ID, etc.)
- Parses ~/.aws/credentials file with profile support
- Handles temporary session tokens
- Implements AWS Signature V4 correctly
- Supports credential_source configurations
- Validates access key format
- Unit tests with various credential formats
- Integration tests against AWS STS

---

### Task 3.3: Implement IAM Role Authentication

**Description**: Implement authentication by assuming IAM roles using AWS Security Token Service (STS). This includes support for role chaining, external IDs, and MFA.

**Time Estimate**: 3 days

**Dependencies**: Task 3.1, Task 3.2

**Deliverables**:
- `pkg/auth/aws/iam_role.go` with role assumption
- `pkg/auth/aws/sts_client.go` for STS operations
- MFA token handling logic

**Acceptance Criteria**:
- Assumes roles using STS AssumeRole API
- Supports external ID for third-party access
- Handles MFA challenges when required
- Implements role session naming
- Supports role chaining (assuming role from role)
- Manages temporary credential lifecycle
- Caches role credentials appropriately
- Unit tests with mocked STS
- Integration tests with actual IAM roles

---

### Task 3.4: Implement EC2 Instance Profile Authentication

**Description**: Implement authentication for EC2 instances using the Instance Metadata Service (IMDS). This includes support for both IMDSv1 and IMDSv2.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1

**Deliverables**:
- `pkg/auth/aws/instance_profile.go` with implementation
- `pkg/auth/aws/imds_client.go` for metadata service
- IMDSv2 session token handling

**Acceptance Criteria**:
- Detects when running on EC2 instance
- Supports IMDSv2 with session tokens
- Falls back to IMDSv1 when appropriate
- Retrieves credentials from instance metadata
- Handles credential rotation automatically
- Implements proper retry logic for IMDS
- Respects IMDSv2 hop limit settings
- Unit tests with mocked IMDS
- Integration tests on EC2 instance

---

### Task 3.5: Implement ECS Task Role Authentication ✅ COMPLETED

**Description**: Implement authentication for ECS tasks using task IAM roles. This retrieves credentials from the ECS credential provider endpoint.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1

**Deliverables**:
- ✅ ECS task role implementation in `factory.go`
- ✅ ECS credential endpoint client via SDK
- ✅ Fargate compatibility

**Implementation Status**:
- ✅ Uses `endpointcreds` provider from AWS SDK v2
- ✅ Supports both relative URI (EC2) and full URI (Fargate)
- ✅ Environment variable support:
  - AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
  - AWS_CONTAINER_CREDENTIALS_FULL_URI
  - AWS_CONTAINER_AUTHORIZATION_TOKEN
- ✅ Authorization token support for enhanced security
- ✅ Automatic credential refresh handled by SDK

**Acceptance Criteria**:
- ✅ Detects ECS task environment via env vars
- ✅ Retrieves credentials from ECS endpoint
- ✅ Handles both EC2 and Fargate launch types
- ✅ Supports custom credential endpoints
- ✅ Automatic credential refresh
- ✅ Proper timeout handling via SDK
- ✅ Unit tests with configuration validation
- ⚠️ Integration tests require ECS environment

---

### Task 3.6: Implement Web Identity Token Authentication

**Description**: Implement authentication using Web Identity tokens (OIDC) for EKS pods and other federated scenarios. This enables pods to assume IAM roles.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1, Task 3.3

**Deliverables**:
- `pkg/auth/aws/web_identity.go` with implementation
- OIDC token file reading
- STS AssumeRoleWithWebIdentity support

**Acceptance Criteria**:
- Reads web identity token from file
- Assumes role using web identity
- Supports EKS IRSA (IAM Roles for Service Accounts)
- Handles token refresh for long-running pods
- Validates token format and expiry
- Proper error handling for federation failures
- Unit tests with sample tokens
- Integration tests in EKS cluster

---

### Task 3.7: Implement AWS SSO Authentication

**Description**: Implement authentication using AWS Single Sign-On (SSO) credentials. This includes support for cached SSO tokens and automatic token refresh.

**Time Estimate**: 3 days

**Dependencies**: Task 3.1

**Deliverables**:
- `pkg/auth/aws/sso.go` with SSO implementation
- `pkg/auth/aws/sso_cache.go` for token caching
- SSO OIDC client implementation

**Acceptance Criteria**:
- Reads SSO configuration from ~/.aws/config
- Implements device authorization flow
- Caches SSO tokens appropriately
- Handles token refresh automatically
- Supports multiple SSO sessions
- Integrates with AWS CLI SSO cache
- Validates SSO token integrity
- Unit tests with mocked SSO
- Integration tests with AWS SSO

---

### Task 3.8: Implement Process Credentials Provider ✅ COMPLETED

**Description**: Implement support for credential_process configuration which allows external processes to provide AWS credentials dynamically.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1

**Deliverables**:
- ✅ Process provider implementation in `factory.go`
- ✅ Process execution handled by SDK
- ✅ Output parsing and validation by SDK

**Implementation Status**:
- ✅ Uses `processcreds` provider from AWS SDK v2
- ✅ Supports command configuration via Extra["process"]["command"]
- ✅ Environment variable support via AWS_CREDENTIAL_PROCESS
- ✅ Default timeout of 1 minute (configurable)
- ✅ SDK handles JSON parsing and validation
- ✅ SDK prevents shell injection

**Acceptance Criteria**:
- ✅ Executes credential_process commands
- ✅ Parses JSON output correctly (SDK)
- ✅ Handles process timeouts gracefully
- ✅ Supports credential caching based on expiry (SDK)
- ✅ Validates output format (SDK)
- ✅ Secure process execution (no shell injection)
- ✅ Unit tests with configuration validation
- ⚠️ Integration tests require sample providers

---

### Task 3.9: Create AWS Credential Chain

**Description**: Implement the AWS credential chain that matches AWS SDK behavior, trying multiple credential sources in the correct order.

**Time Estimate**: 2 days

**Dependencies**: Tasks 3.2-3.8

**Deliverables**:
- `pkg/auth/aws/chain.go` with credential chain
- Configurable provider precedence
- Performance optimizations

**Acceptance Criteria**:
- Matches AWS SDK credential precedence
- Order: Env → Credentials file → Container → Instance → SSO
- Allows custom provider ordering
- Caches successful providers
- Provides detailed error aggregation
- Fast path for common scenarios
- Unit tests for all provider combinations
- Performance benchmarks

---

### Task 3.10: Add AWS China and GovCloud Support

**Description**: Implement support for AWS China regions and GovCloud with their specific authentication requirements and endpoints.

**Time Estimate**: 2 days

**Dependencies**: Task 3.1

**Deliverables**:
- `pkg/auth/aws/partitions.go` with partition support
- Region to partition mapping
- Partition-specific endpoints

**Acceptance Criteria**:
- Detects AWS partition from region
- Uses correct endpoints for China regions
- Handles GovCloud-specific requirements
- Supports cn-north-1, cn-northwest-1
- Supports us-gov-east-1, us-gov-west-1
- Validates region/partition compatibility
- Unit tests for all partitions
- Integration tests with partition endpoints

---

## Summary

**Total Time Estimate**: 21 days (actual: ~12 days due to SDK v2 integration)

**Current Status**: ✅ Core AWS authentication fully implemented

**Completed Deliverables**:
- ✅ AWS authentication factory implementation
- ✅ Access Key authentication with environment variables
- ✅ IAM Role authentication via AssumeRole
- ✅ EC2 Instance Profile support
- ✅ Web Identity token authentication (EKS IRSA)
- ✅ ECS Task Role authentication (EC2 & Fargate)
- ✅ Process credentials provider
- ✅ Default credential chain via SDK
- ✅ AWS Signature V4 via SDK
- ✅ Credential caching via SDK

**Not Implemented (per requirements)**:
- ❌ AWS SSO authentication (not needed)
- ❌ China and GovCloud regions (not needed)
- ❌ MFA support for AssumeRole (not needed)

**Success Metrics Achieved**:
- ✅ All required AWS credential providers supported
- ✅ Seamless AWS service integration
- ✅ Fast credential resolution via SDK caching
- ✅ Full AWS SDK v2 compatibility
- ✅ Clean separation of concerns
- ✅ Good test coverage for implemented features