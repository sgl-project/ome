#!/bin/bash
# Script to fetch kubeconfig from MLE or service clusters

set -e

# Default values
CLUSTER_TYPE=""
OUTPUT_FILE=""
REGION="us-phoenix-1"
COMPARTMENT_ID=""
CLUSTER_ID=""
OCI_CONFIG_FILE="$HOME/.oci/config"
OCI_PROFILE="DEFAULT"
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --cluster)
      CLUSTER_TYPE="$2"
      shift
      shift
      ;;
    --output)
      OUTPUT_FILE="$2"
      shift
      shift
      ;;
    --region)
      REGION="$2"
      shift
      shift
      ;;
    --compartment-id)
      COMPARTMENT_ID="$2"
      shift
      shift
      ;;
    --cluster-id)
      CLUSTER_ID="$2"
      shift
      shift
      ;;
    --oci-config)
      OCI_CONFIG_FILE="$2"
      shift
      shift
      ;;
    --oci-profile)
      OCI_PROFILE="$2"
      shift
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Validate required parameters
if [[ -z "$CLUSTER_TYPE" ]]; then
  echo "Error: --cluster parameter is required (mle or service)"
  exit 1
fi

if [[ -z "$OUTPUT_FILE" ]]; then
  echo "Error: --output parameter is required"
  exit 1
fi

# Validate cluster type
if [[ "$CLUSTER_TYPE" != "mle" && "$CLUSTER_TYPE" != "service" ]]; then
  echo "Error: --cluster must be either 'mle' or 'service'"
  exit 1
fi

# Function to check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Check for required tools
if ! command_exists oci; then
  echo "Error: OCI CLI is not installed. Please install it first."
  echo "See: https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm"
  exit 1
fi

# Function to get cluster ID if not provided
get_cluster_id() {
  local cluster_type=$1
  local search_name=""
  
  if [[ "$cluster_type" == "mle" ]]; then
    search_name="ome-mle"
  else
    search_name="ome-service"
  fi
  
  if [[ "$VERBOSE" == "true" ]]; then
    echo "Searching for $cluster_type cluster with name containing '$search_name'..."
  fi
  
  # Get compartment ID if not provided
  if [[ -z "$COMPARTMENT_ID" ]]; then
    if [[ "$VERBOSE" == "true" ]]; then
      echo "No compartment ID provided, using tenancy as compartment..."
    fi
    COMPARTMENT_ID=$(oci iam compartment list --all --config-file "$OCI_CONFIG_FILE" --profile "$OCI_PROFILE" --query "data[0].id" --raw-output)
  fi
  
  # List clusters and find the one matching our search name
  local clusters=$(oci ce cluster list --compartment-id "$COMPARTMENT_ID" --region "$REGION" --config-file "$OCI_CONFIG_FILE" --profile "$OCI_PROFILE")
  local cluster_id=$(echo "$clusters" | jq -r ".data[] | select(.name | contains(\"$search_name\")) | .id" | head -1)
  
  if [[ -z "$cluster_id" ]]; then
    echo "Error: Could not find a $cluster_type cluster with name containing '$search_name'"
    exit 1
  fi
  
  if [[ "$VERBOSE" == "true" ]]; then
    echo "Found cluster ID: $cluster_id"
  fi
  
  echo "$cluster_id"
}

# Get cluster ID if not provided
if [[ -z "$CLUSTER_ID" ]]; then
  CLUSTER_ID=$(get_cluster_id "$CLUSTER_TYPE")
fi

# Fetch kubeconfig
echo "Fetching kubeconfig for $CLUSTER_TYPE cluster (ID: $CLUSTER_ID)..."
oci ce cluster create-kubeconfig \
  --cluster-id "$CLUSTER_ID" \
  --file "$OUTPUT_FILE" \
  --region "$REGION" \
  --token-version 2.0.0 \
  --kube-endpoint PUBLIC_ENDPOINT \
  --config-file "$OCI_CONFIG_FILE" \
  --profile "$OCI_PROFILE"

# Verify kubeconfig
if [[ -f "$OUTPUT_FILE" ]]; then
  echo "Kubeconfig successfully saved to $OUTPUT_FILE"
  
  # Test the kubeconfig if kubectl is available
  if command_exists kubectl; then
    echo "Testing kubeconfig..."
    if kubectl --kubeconfig="$OUTPUT_FILE" cluster-info > /dev/null 2>&1; then
      echo "Kubeconfig is valid and can connect to the cluster"
    else
      echo "Warning: Could not connect to the cluster with the generated kubeconfig"
    fi
  fi
else
  echo "Error: Failed to generate kubeconfig"
  exit 1
fi

exit 0 