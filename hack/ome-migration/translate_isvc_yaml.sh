#!/bin/bash

set -u
set -e

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $1" >&2
    exit 1
}

# Function to validate prerequisites
validate_dependencies() {
    for cmd in kubectl yq jq; do
        if ! command -v "$cmd" &>/dev/null; then
            error "Required command '$cmd' not found. Please install it."
        fi
    done
}

# Fetch and save the YAML of the resource
fetch_resource_yaml() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    local output_file=$4

    log "Fetching YAML for $resource_type '$resource_name' in namespace '$namespace'."
    kubectl get "$resource_type" "$resource_name" -n "$namespace" -o yaml > "$output_file"
}

removeDeploymentOwnerreference() {
    # Define the resource and namespace
    RESOURCE_TYPE="Deployment"
    local resource_name=$1
    local namespace=$2
    local old_deployment_json=$3
    local new_deployment_json=$4

    # Retrieve current resource metadata
    kubectl get $RESOURCE_TYPE "$resource_name" -n "$namespace" -o json > "$old_deployment_json"

    # Delete ownerReferences
    jq 'del(.metadata.ownerReferences[])' "$old_deployment_json" > "$new_deployment_json"

    # Apply resource back to cluster
    kubectl apply -f "$new_deployment_json"
}

transfer_json() {
	local input_file=$1
	local output_file=$2

	log "Processing JSON from YAML file '$input_file'."
  local isvc_json=$(yq eval -o=json "$input_file")

  # Extract info
  local base_model=$(jq -r '.spec.predictor.model.modelRefs[0].name' <<< "$isvc_json")
  local minReplicas=$(jq -r '.spec.predictor.minReplicas' <<< "$isvc_json")
  local maxReplicas=$(jq -r '.spec.predictor.maxReplicas' <<< "$isvc_json")
  local tenancyId=$(jq -r '.metadata.labels."tenancy-id" // empty' <<< "$isvc_json")
  local dacId=$(jq -r '.spec.predictor.model.dedicatedAIClusterRef.name // empty' <<< "$isvc_json")
  local releaseName=$(jq -r '.metadata.annotations."meta.helm.sh/release-name" // empty' <<< "$isvc_json")
  local releaseNameSpace=$(jq -r '.metadata.annotations."meta.helm.sh/release-namespace" // empty' <<< "$isvc_json")
  local kubeInstance=$(jq -r '.metadata.labels."app.kubernetes.io/instance" // empty' <<< "$isvc_json")
  local managedBy=$(jq -r '.metadata.labels."app.kubernetes.io/managed-by" // empty' <<< "$isvc_json")
  local kubeName=$(jq -r '.metadata.labels."app.kubernetes.io/name" // empty' <<< "$isvc_json")
  local kubeVersion=$(jq -r '.metadata.labels."app.kubernetes.io/version" // empty' <<< "$isvc_json")
  local chart=$(jq -r '.metadata.labels."helm.sh/chart" // empty' <<< "$isvc_json")
  local deploymentMode=$(jq -r '.metadata.annotations."ome.oracle.com/deploymentMode" // empty' <<< "$isvc_json")

  # transform fields
  isvc_json=$(jq '
      .apiVersion = "ome.io/v1beta1" |
      del(.status, .metadata.annotations, .metadata.creationTimestamp,
          .metadata.finalizers, .metadata.generation, .metadata.resourceVersion,
          .metadata.uid, .metadata.labels) |
      del(.spec.predictor) |
      .spec.predictor.minReplicas = $ARGS.named.minReplicas |
      .spec.predictor.maxReplicas = $ARGS.named.maxReplicas |
      .spec.predictor.model.baseModel = $ARGS.named.base_model |
      .spec.predictor.model.protocolVersion = "openAI" |
      .metadata.annotations."kueue-enabled" = "true"' \
      --arg base_model "$base_model" \
      --argjson minReplicas "$minReplicas" \
      --argjson maxReplicas "$maxReplicas" <<< "$isvc_json")

  echo $isvc_json

  # Add tenancyId back if it exists
  if [[ -n "$tenancyId" ]]; then
      isvc_json=$(jq --arg tenancyId "$tenancyId" '.metadata.labels."tenancy-id" = $tenancyId' <<< "$isvc_json")
  fi
  # Add dac reference in annotation
  if [[ -n "$dacId" ]]; then
    isvc_json=$(jq --arg dacId "$dacId" '.metadata.annotations."ome.io/dedicated-ai-cluster" = $dacId' <<< "$isvc_json")
  fi
  # Add helm related info back
  if [[ -n "$releaseName" ]]; then
      isvc_json=$(jq --arg releaseName "$releaseName" '.metadata.annotations."meta.helm.sh/release-name" = $releaseName' <<< "$isvc_json")
  fi
  if [[ -n "$releaseNameSpace" ]]; then
      isvc_json=$(jq --arg releaseNameSpace "$releaseNameSpace" '.metadata.annotations."meta.helm.sh/release-namespace" = $releaseNameSpace' <<< "$isvc_json")
  fi
  if [[ -n "$kubeInstance" ]]; then
      isvc_json=$(jq --arg kubeInstance "$kubeInstance" '.metadata.labels."app.kubernetes.io/instance" = $kubeInstance' <<< "$isvc_json")
  fi
  if [[ -n "$managedBy" ]]; then
      isvc_json=$(jq --arg managedBy "$managedBy" '.metadata.labels."app.kubernetes.io/managed-by" = $managedBy' <<< "$isvc_json")
  fi
  if [[ -n "$kubeName" ]]; then
      isvc_json=$(jq --arg kubeName "$kubeName" '.metadata.labels."app.kubernetes.io/name" = $kubeName' <<< "$isvc_json")
  fi
  if [[ -n "$kubeVersion" ]]; then
      isvc_json=$(jq --arg kubeVersion "$kubeVersion" '.metadata.labels."app.kubernetes.io/version" = $kubeVersion' <<< "$isvc_json")
  fi
  if [[ -n "$chart" ]]; then
      isvc_json=$(jq --arg chart "$chart" '.metadata.labels."helm.sh/chart" = $chart' <<< "$isvc_json")
  fi
  # add deploymentMode back if exist
  if [[ -n "$deploymentMode" ]]; then
      isvc_json=$(jq --arg deploymentMode "$deploymentMode" '.metadata.annotations."ome.io/deploymentMode" = $deploymentMode' <<< "$isvc_json")
  fi

  echo $isvc_json

  # Save the processed JSON as YAML
  log "Saving transformed JSON as YAML to '$output_file'."
  echo "$isvc_json" | yq eval -P > "$output_file"
}

main() {
	if [[ $# -ne 2 ]]; then
        error "Usage: $0 <inference-service-name> <namespace>"
  fi

  local isvc_name=$1
  local namespace=$2

  # Allow user to configure KUBECONFIG or fallback to default
  : "${KUBECONFIG:=$HOME/.kube/gais-clusters/dp-us-chicago-1-dev-config}"
  export KUBECONFIG
  log "Using KUBECONFIG: $KUBECONFIG"

  local region="us-chicago-1"
  local env="dev"
  local target="isvc"
  local file_dir="./$env-$target-$region/$isvc_name"

  # Validate dependencies
  validate_dependencies

  # Prepare directories
  mkdir -p "$file_dir"

  # File paths
  local old_yaml_file="$file_dir/old_isvc_yaml.yaml"
  local new_yaml_file="$file_dir/new_isvc_yaml.yaml"
  local old_deployment_json="$file_dir/old_isvc_yaml.json"
  local new_deployment_json="$file_dir/new_deployment_json.json"

  # Fetch inference service yaml file
  fetch_resource_yaml "InferenceService.ome.oracle.com" "$isvc_name" "$namespace" "$old_yaml_file"

  # Remove deployment ownerreference
  removeDeploymentOwnerreference "$isvc_name" "$namespace" "$old_deployment_json" "$new_deployment_json"

  # transfer isvc yaml from v1alpha1 to v1beta1
  transfer_json "$old_yaml_file" "$new_yaml_file"

  # Apply
  log "Applying new YAML for $isvc_name."
  kubectl apply -f "$new_yaml_file"

  log "Successfully applied updated resource: $isvc_name in namespace: $namespace."
}

main "$@"