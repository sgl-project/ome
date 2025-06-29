#!/bin/bash

# Storage Provider Test Runner Script
# This script demonstrates how to test different storage providers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Storage Provider Test Runner"
echo "============================"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.19 or later."
    exit 1
fi

# Create test file
TEST_FILE="test-data.txt"
echo "This is test data for storage provider testing" > $TEST_FILE
echo "Created at: $(date)" >> $TEST_FILE
echo "Random data: $(openssl rand -base64 32)" >> $TEST_FILE

print_info "Created test file: $TEST_FILE"

# Function to test a provider
test_provider() {
    local provider=$1
    local auth=$2
    local uri=$3
    
    print_info "Testing $provider with $auth authentication..."
    
    if go run main.go -provider "$provider" -auth "$auth" -uri "$uri" -file "$TEST_FILE" -verbose; then
        print_info "✓ $provider/$auth test passed"
        return 0
    else
        print_error "✗ $provider/$auth test failed"
        return 1
    fi
}

# Test OCI
if [[ -n "$OCI_COMPARTMENT_ID" ]]; then
    print_info "Testing OCI Storage..."
    
    # You need to update these values
    OCI_NAMESPACE="your-namespace"
    OCI_BUCKET="your-bucket"
    OCI_REGION="${OCI_REGION:-us-ashburn-1}"
    
    test_provider "oci" "user-principal" "oci://${OCI_NAMESPACE}@${OCI_REGION}/${OCI_BUCKET}/test"
else
    print_warn "Skipping OCI tests (OCI_COMPARTMENT_ID not set)"
fi

# Test AWS S3
if [[ -n "$AWS_ACCESS_KEY_ID" ]]; then
    print_info "Testing AWS S3..."
    
    # You need to update this value
    S3_BUCKET="your-s3-bucket"
    
    test_provider "aws" "access-key" "s3://${S3_BUCKET}/test"
else
    print_warn "Skipping AWS tests (AWS_ACCESS_KEY_ID not set)"
fi

# Test Google Cloud Storage
if [[ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
    print_info "Testing Google Cloud Storage..."
    
    # You need to update this value
    GCS_BUCKET="your-gcs-bucket"
    
    test_provider "gcp" "service-account" "gs://${GCS_BUCKET}/test"
else
    print_warn "Skipping GCP tests (GOOGLE_APPLICATION_CREDENTIALS not set)"
fi

# Test Azure Blob Storage
if [[ -n "$AZURE_CLIENT_ID" ]] && [[ -n "$AZURE_STORAGE_ACCOUNT" ]]; then
    print_info "Testing Azure Blob Storage..."
    
    # You need to update this value
    AZURE_CONTAINER="your-container"
    
    test_provider "azure" "service-principal" "azure://${AZURE_CONTAINER}@${AZURE_STORAGE_ACCOUNT}/test"
else
    print_warn "Skipping Azure tests (AZURE_CLIENT_ID or AZURE_STORAGE_ACCOUNT not set)"
fi

# Test GitHub LFS
if [[ -n "$GITHUB_TOKEN" ]] && [[ -n "$GITHUB_OWNER" ]] && [[ -n "$GITHUB_REPO" ]]; then
    print_info "Testing GitHub LFS..."
    
    test_provider "github" "personal-access-token" "github://${GITHUB_OWNER}/${GITHUB_REPO}@main/test"
else
    print_warn "Skipping GitHub tests (GITHUB_TOKEN, GITHUB_OWNER, or GITHUB_REPO not set)"
fi

# Clean up
rm -f $TEST_FILE

print_info "Test run completed!"

# Show available options
echo ""
echo "To run specific tests:"
echo "  go run main.go -list                    # List all supported combinations"
echo "  go run main.go -provider <provider> -auth <auth> -uri <uri> -file <file>"
echo ""
echo "To run the simple example:"
echo "  go run simple_example.go"