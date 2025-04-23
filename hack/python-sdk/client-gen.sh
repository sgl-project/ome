#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Configuration variables
OPENAPI_GENERATOR_VERSION="4.3.1"
SWAGGER_JAR_URL="https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/${OPENAPI_GENERATOR_VERSION}/openapi-generator-cli-${OPENAPI_GENERATOR_VERSION}.jar"
SWAGGER_CODEGEN_JAR="hack/python-sdk/openapi-generator-cli-${OPENAPI_GENERATOR_VERSION}.jar"
SWAGGER_CODEGEN_CONF="hack/python-sdk/swagger_config.json"
SWAGGER_CODEGEN_FILE="pkg/openapi/swagger.json"
SDK_OUTPUT_PATH="python/ome"

# Function to handle errors
handle_error() {
    echo >&2 "Error: $1"
    exit 1
}

# Redirect stdout to /dev/null but keep stderr
exec 3>&1 # Save the current stdout to file descriptor 3
exec 1>/dev/null # Redirect stdout to /dev/null

# Function to download the OpenAPI generator
download_openapi_generator() {
    echo >&2 "Downloading the swagger-codegen JAR package ..."
    if [ ! -f ${SWAGGER_CODEGEN_JAR} ]; then
        wget -O ${SWAGGER_CODEGEN_JAR} ${SWAGGER_JAR_URL} 2>&1 || handle_error "Failed to download swagger-codegen JAR"
    fi
}

# Function to generate Python SDK
generate_python_sdk() {
    echo >&2 "Generating Python SDK for OME ..."
    java -jar ${SWAGGER_CODEGEN_JAR} generate -i ${SWAGGER_CODEGEN_FILE} -g python -o ${SDK_OUTPUT_PATH} -c ${SWAGGER_CODEGEN_CONF} 2>&1 || handle_error "Failed to generate Python SDK"
    
    # Revert following files since they are diverged from generated ones
    git checkout python/ome/README.md || handle_error "Failed to checkout README.md"
    git checkout python/ome/.gitignore || handle_error "Failed to checkout .gitignore"
    git checkout python/ome/pyproject.toml || handle_error "Failed to checkout pyproject.toml"
}

# Function to update Kubernetes docs links
update_k8s_doc_links() {
    echo >&2 "Updating Kubernetes documentation links ..."
    K8S_IMPORT_LIST=$(cat hack/python-sdk/swagger_config.json | grep "V1" | awk -F"\"" '{print $2}')
    K8S_DOC_LINK="https://github.com/kubernetes-client/python/blob/master/kubernetes/docs"
    
    for item in $K8S_IMPORT_LIST; do
        sed -i'.bak' -e "s@($item.md)@($K8S_DOC_LINK/$item.md)@g" python/ome/docs/* || handle_error "Failed to update Kubernetes docs link"
        rm -rf python/ome/docs/*.bak
    done
}

# Function to ensure npm is installed
ensure_npm_installed() {
    echo >&2 "Checking if npm is installed..."
    if ! command -v npm &> /dev/null; then
        echo >&2 "npm is not installed. Attempting to install..."
        if command -v brew &> /dev/null; then
            echo >&2 "Installing npm using Homebrew..."
            brew install node 2>&1 || handle_error "Failed to install Node.js using Homebrew"
        elif command -v apt-get &> /dev/null; then
            echo >&2 "Installing npm using apt-get..."
            sudo apt-get update 2>&1 || handle_error "Failed to update apt repositories"
            sudo apt-get install -y nodejs npm 2>&1 || handle_error "Failed to install Node.js using apt-get"
        elif command -v yum &> /dev/null; then
            echo >&2 "Installing npm using yum..."
            sudo yum install -y nodejs npm 2>&1 || handle_error "Failed to install Node.js using yum"
        else
            handle_error "Could not install npm. Please install it manually and run this script again."
        fi
    fi
    
    # Configure npm registry
    echo >&2 "Configuring npm registry..."
    npm config set registry https://artifactory.oci.oraclecorp.com/api/npm/global-dev-npm 2>&1 || handle_error "Failed to set npm registry"
    npm config set strict-ssl false 2>&1 || handle_error "Failed to set npm strict-ssl option"
}

# Function to install and setup npm tools
setup_npm_tools() {
    ensure_npm_installed
    
    echo >&2 "Checking if prettier and markdown-table-formatter are installed..."
    if ! npm list -g prettier &> /dev/null; then
        echo >&2 "Installing prettier..."
        npm install -g prettier 2>&1 || handle_error "Failed to install prettier"
    fi
    
    if ! npm list -g markdown-table-formatter &> /dev/null; then
        echo >&2 "Installing markdown-table-formatter..."
        npm install -g markdown-table-formatter 2>&1 || handle_error "Failed to install markdown-table-formatter"
    fi
}

# Function to format markdown files
format_markdown_files() {
    echo >&2 "Formatting Markdown files in ${SDK_OUTPUT_PATH}/docs..."
    if [ -d "${SDK_OUTPUT_PATH}/docs" ]; then
        # Use find to get all Markdown files and format them one by one
        find "${SDK_OUTPUT_PATH}/docs" -name "*.md" -type f | while read -r file; do
            echo >&2 "Formatting $file..."
            prettier --write "$file" 2>&1 || handle_error "Failed to format $file"
            markdown-table-formatter "$file" 2>&1 || handle_error "Failed to format $file"
        done
    else
        echo >&2 "No docs directory found at ${SDK_OUTPUT_PATH}/docs"
    fi
}

# Function to install uv
install_uv() {
    echo >&2 "Installing uv package manager..."
    if ! command -v uv &> /dev/null; then
        echo >&2 "uv is not installed. Attempting to install..."
        if command -v pip &> /dev/null; then
            pip install uv 2>&1 || handle_error "Failed to install uv using pip"
        elif command -v brew &> /dev/null; then
            brew install astral-sh/tap/uv 2>&1 || handle_error "Failed to install uv using Homebrew"
        else
            curl -LsSf https://astral.sh/uv/install.sh | sh 2>&1 || handle_error "Failed to install uv"
        fi
    fi
}

# Function to format Python code
format_python_code() {
    local current_dir=$(pwd)
    
    # First sync dependencies
    echo >&2 "Syncing dependencies with uv..."
    cd ${SDK_OUTPUT_PATH} || handle_error "Failed to change to ${SDK_OUTPUT_PATH} directory"
    uv sync 2>&1 || handle_error "Failed to sync dependencies with uv"
    
    # Format code
    echo >&2 "Running ruff format on Python code..."
    uv run ruff format . 2>&1 || handle_error "Failed to run ruff format on Python code"
    
    echo >&2 "Running isort on Python code..."
    uv run isort . 2>&1 || handle_error "Failed to run isort on Python code"
    
    # Return to original directory
    cd ${current_dir} || handle_error "Failed to return to original directory"
}

# Main execution
main() {
    download_openapi_generator
    generate_python_sdk
    update_k8s_doc_links
    setup_npm_tools
    format_markdown_files
    install_uv
    format_python_code
    
    echo >&2 "OME Python SDK is generated successfully to folder ${SDK_OUTPUT_PATH}/."
    echo >&2 "All Markdown files have been formatted with Prettier."
    echo >&2 "Python code has been formatted with ruff and isort."
}

# Run the main function
main

# Restore stdout
exec 1>&3
