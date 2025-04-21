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
    local output_file=$3

    log "Fetching YAML for $resource_type '$resource_name'."
    kubectl get "$resource_type" "$resource_name" -o yaml > "$output_file"
}

# translate old unitShape to dacProfile
convertDacShape() {
	local type=$1
	local unitshape=$2
	if [[ "$type" == "Hosting" || "$type" == "Hosting - beta" ]] ; then
		if [[ "$unitshape" == "Small_Flex" ]]; then
			echo "xsmall-a100"
		elif [[ "$unitshape" == "Small_Flex_V2" ]]; then
			echo "xsmall-h100"
		elif [[ "$unitshape" == "Small_Flex_4" ]]; then
			echo "medium-a100v1"
		elif [[ "$unitshape" == "Large_Flex" ]]; then
			echo "medium-a100"
		elif [[ "$unitshape" == "Large_Flex_V2" ]]; then
			echo "medium-h100"
		elif [[ "$unitshape" == "XSmall_Flex" ]]; then
			echo "xsmall-a10-a100-h100"
		elif [[ "$unitshape" == "Medium_Flex" ]]; then
			echo "small-a100-h100"
		elif [[ "$unitshape" == "Large_Flex_V3" ]]; then
			echo "medium-a100v2-h100"
		elif [[ "$unitshape" == "Medium_Flex_V2" ]]; then
			echo "small-h100"
		fi
	elif [[ "$type" == "Fine-tuning" || "$type" == "Fine-tuning - beta" ]]; then
		if [[ "$unitshape" == "Small_Flex" ]]; then
			echo "medium-a100-h100"
		elif [[ "$unitshape" == "Small_Flex_V2" ]]; then
			echo "large-h100-count-one"
		elif [[ "$unitshape" == "Large_Flex" ]]; then
			echo "large-a100v2-h100"
		elif [[ "$unitshape" == "Large_Flex_V2" ]]; then
			echo "large-h100-count-one"
		elif [[ "$unitshape" == "Medium_Flex" ]]; then
			echo "large-a100v2-h100"
		elif [[ "$unitshape" == "Large_Flex_V3" ]]; then
			echo "large-a100v2-h100"
        fi
	fi
}

# translate old size to count
convertCount() {
	local type=$1
	local count=$2
	if [[ "$type" == "Hosting" || "$type" == "Hosting - beta" ]] ; then
		echo $count
	elif [[ "$type" == "Fine-tuning" || "$type" == "Fine-tuning - beta" ]]; then
		echo 1
	fi
}

removeNamespaceOwnerrefernce() {
	# Define the resource and namespace
	RESOURCE_TYPE="namespace"
	local namespace=$1
	local old_namespace_json=$2
	local new_namespace_json=$3

	# Retrieve current resource metadata
	kubectl get $RESOURCE_TYPE "$namespace" -o json > "$old_namespace_json"

	# Delete ownerReferences
	jq 'del(.metadata.ownerReferences[])' "$old_namespace_json" > "$new_namespace_json"

	# Apply resource back to cluster
	kubectl apply -f "$new_namespace_json"
}

main() {
	if [[ $# -ne 1 ]]; then
        error "Usage: $0 <dac-name>"
  fi

  local dac_name=$1
  # Allow user to configure KUBECONFIG or fallback to default
  : "${KUBECONFIG:=$HOME/.kube/gais-clusters/dp-us-chicago-1-dev-config}"
  export KUBECONFIG
  log "Using KUBECONFIG: $KUBECONFIG"

  local region="us-chicago-1"
  local env="dev"
  local target="dac"
  local file_dir="./$env-$target-$region/$dac_name"

  validate_dependencies

  mkdir -p "$file_dir"
  # File paths
  local old_yaml_file="$file_dir/old_dac_yaml.yaml"
  local new_yaml_file="$file_dir/new_dac_yaml.yaml"
  local old_namespace_json="$file_dir/old_namespace_json.json"
  local new_namespace_json="$file_dir/new_namespce_json.json"
  local old_pv_json="$file_dir/old_pv_json.json"
  local new_pv_json="$file_dir/new_pv_json.json"
  local old_pvc_json="$file_dir/old_pvc_json.json"
  local new_pvc_json="$file_dir/new_pvc_json.json"
  local old_deployment_json="$file_dir/old_deployment_json.json"
  local new_deployment_json="$file_dir/new_deployment_json.json"

  fetch_resource_yaml "DedicatedAiCluster.ome.oracle.com" "$dac_name" "$old_yaml_file"
  removeNamespaceOwnerrefernce "$dac_name" "$old_namespace_json" "$new_namespace_json"

  dac_json=$(yq eval -o=json -I=0 "$file_dir/old_dac_yaml.yaml")

  echo "$dac_json" > "$file_dir/old_json.json"

  # Extract fields
  size=$(jq -r '.spec.size' <<< "$dac_json")
  type=$(jq -r '.spec.type' <<< "$dac_json")
  unitshape=$(jq -r '.spec.unitShape' <<< "$dac_json")

  # Delete fields
  dac_json=$(jq '
    .apiVersion ="ome.io/v1beta1" |
    .kind = "DedicatedAICluster" |
    del(.status, .metadata.creationTimestamp, .metadata.finalizers, .metadata.generation, .metadata.resourceVersion,
          .metadata.uid, .metadata.labels.isCapacityReserved) |
      del(.spec.size, .spec.type, .spec.unitShape)' <<< "$dac_json")

  # convert unitshape and count
  dacprofile=$(convertDacShape "$type" "$unitshape")
  count=$(convertCount "$type" "$size")

  #update count and dacprofile
  dac_json=$(jq --argjson count "$count" '.spec.count=$count' <<< "$dac_json")
  dac_json=$(jq --arg dacprofile "$dacprofile" '.spec.profile=$dacprofile' <<< "$dac_json")

  result=$(yq -o=yaml <<< "$dac_json")
  echo $result > "$file_dir/new_dac_yaml.yaml"

  kubectl apply -f "$file_dir/new_dac_yaml.yaml"
  log "Successfully applied updated resource: $dac_name"
}

main "$@"