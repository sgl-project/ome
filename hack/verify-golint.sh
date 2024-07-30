#!/bin/bash

set -o errexit
set -o pipefail

# golangci-lint binary path
golangci_lint_binary="$(go env GOPATH)/bin/golangci-lint"

# Check if golangci-lint is already installed
if ! command -v golangci-lint &> /dev/null; then
    echo "installing golangci-lint"
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
	# Verify golangci-lint installation
	$golangci_lint_binary --version
fi

# Run golangci-lint
$golangci_lint_binary run
