#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

OPENAPI_GENERATOR_VERSION="7.9.0"
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

echo >&2 "Downloading the swagger-codegen JAR package ..."
if [ ! -f ${SWAGGER_CODEGEN_JAR} ]
then
    wget -O ${SWAGGER_CODEGEN_JAR} ${SWAGGER_JAR_URL} 2>&1 || handle_error "Failed to download swagger-codegen JAR"
fi

echo >&2 "Generating Python SDK for OME ..."
java -jar ${SWAGGER_CODEGEN_JAR} generate -i ${SWAGGER_CODEGEN_FILE} -g python -o ${SDK_OUTPUT_PATH} -c ${SWAGGER_CODEGEN_CONF} 2>&1 || handle_error "Failed to generate Python SDK"

# revert following files since they are diverged from generated ones
git checkout python/ome/README.md || handle_error "Failed to checkout README.md"
git checkout python/ome/.gitignore || handle_error "Failed to checkout .gitignore"
git checkout python/ome/pyproject.toml || handle_error "Failed to checkout pyproject.toml"

# Update kubernetes docs link.
K8S_IMPORT_LIST=$(cat hack/python-sdk/swagger_config.json|grep "V1" | awk -F"\"" '{print $2}')
K8S_DOC_LINK="https://github.com/kubernetes-client/python/blob/master/kubernetes/docs"
for item in $K8S_IMPORT_LIST; do
    sed -i'.bak' -e "s@($item.md)@($K8S_DOC_LINK/$item.md)@g" python/ome/docs/* || handle_error "Failed to update Kubernetes docs link"
    rm -rf python/ome/docs/*.bak
done

# Check if npm is installed, if not, try to install it
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

# Install prettier if not already installed
echo >&2 "Checking if prettier and markdown-table-formatter are installed..."
if ! npm list -g prettier &> /dev/null; then
    echo >&2 "Installing prettier..."
    npm install -g prettier 2>&1 || handle_error "Failed to install prettier"
fi
if ! npm list -g markdown-table-formatter &> /dev/null; then
    echo >&2 "Installing markdown-table-formatter..."
    npm install -g prettier 2>&1 || handle_error "Failed to install markdown-table-formatter"
fi

# Format all Markdown files in the docs directory
echo >&2 "Formatting Markdown files in ${SDK_OUTPUT_PATH}/docs..."
if [ -d "${SDK_OUTPUT_PATH}/docs" ]; then
    # Use find to get all Markdown files and format them one by one
    find "${SDK_OUTPUT_PATH}/docs" -name "*.md" -type f | while read -r file; do
        echo >&2 "Formatting $file..."
        prettier --write "$file" 2>&1 || handle_error "Failed to format $file"
    done
    markdown-table-formatter "${SDK_OUTPUT_PATH}/docs/*.md" 2>&1 || handle_error "Failed to format $file"
else
    echo >&2 "No docs directory found at ${SDK_OUTPUT_PATH}/docs"
fi

echo >&2 "OME Python SDK is generated successfully to folder ${SDK_OUTPUT_PATH}/."
echo >&2 "All Markdown files have been formatted with Prettier."

# Restore stdout
exec 1>&3
