#!/bin/bash
#
# Script to fetch a Model CR, convert it to a ClusterBaseModel CR, and apply it.
# Requires: kubectl, yq, jq
#
set -euo pipefail

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*"
}

error() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $*" >&2
  exit 1
}

validate_dependencies() {
  for cmd in kubectl jq yq; do
    if ! command -v "$cmd" &>/dev/null; then
      error "Missing required command: $cmd"
    fi
  done
}

fetch_model_yaml() {
  local name=$1
  local out_file=$2
  log "Fetching model YAML for $name"
  kubectl get model "$name" -o yaml > "$out_file"
}

convert_model_to_clusterbasemodel() {
  local input_yaml=$1
  local output_yaml=$2
  log "Converting Model CR to ClusterBaseModel CR"
  # Convert input YAML to JSON
  local json
  json=$(yq eval -o=json "$input_yaml")
  # Extract values from Model CR
  local model_name
  model_name=$(jq -r '.metadata.name' <<< "$json")
  local compartment_id
  compartment_id=$(jq -r '.metadata.labels["compartment-id"] // empty' <<< "$json")
  local tenancy_id
  tenancy_id=$(jq -r '.metadata.labels.["tenancy-id"] // empty' <<< "$json")
  local vendor
  vendor=$(jq -r '.spec.vendor' <<< "$json")
  local version
  version=$(jq -r '.spec.version' <<< "$json")
  local display_name
  display_name=$(jq -r '.spec.baseModelSpec.displayName' <<< "$json")
  local capability
  capability=$(jq -r '.spec.capability' <<< "$json")
  local storage_url
  storage_url=$(jq -r '.spec.storageUrl' <<< "$json")
  # Parse namespace, bucket, object from storageUrl (format: os://namespace/bucket/path...)
  local namespace
  namespace=$(echo "$storage_url" | sed -E 's|os://([^/]+)/.*|\1|')
  local bucket
  bucket=$(echo "$storage_url" | sed -E 's|os://[^/]+/([^/]+)/.*|\1|')
  local object
  object=$(echo "$storage_url" | sed -E 's|os://[^/]+/[^/]+/(.*)|\1|')
  # Construct storageUri and local path
  local storage_uri="oci://n/${namespace}/b/${bucket}/o/${object}"
  local local_path="/mnt/data/models/${bucket}/${model_name}"
  # Use global $REGION variable from select_stage_and_region
  local region="$REGION"
  # Compose the ClusterBaseModel CR as JSON
  local converted
  converted=$(jq -n \
    --arg model_name "$model_name" \
    --arg compartment_id "$compartment_id" \
    --arg tenancy_id "$tenancy_id" \
    --arg vendor "$vendor" \
    --arg version "$version" \
    --arg display_name "$display_name" \
    --arg capability "$capability" \
    --arg region "$region" \
    --arg path "$local_path" \
    --arg storageUri "$storage_uri" \
    '{
      apiVersion: "ome.io/v1beta1",
      kind: "ClusterBaseModel",
      metadata: {
        name: $model_name,
        labels: {
          "compartment-id": $compartment_id,
          "tenancy-id": $tenancy_id
        }
      },
      spec: {
        disabled: false,
        displayName: $display_name,
        storage: {
          parameters: { region: $region },
          path: $path,
          storageUri: $storageUri
        },
        vendor: $vendor,
        version: $version,
        modelCapabilities: [ $capability ]
      }
    }'
  )
  # Output CR as YAML
  echo "$converted" | yq eval -P > "$output_yaml"
  log "Wrote ClusterBaseModel CR to $output_yaml"
}

apply_clusterbasemodel() {
  local file=$1
  log "Applying ClusterBaseModel YAML from $file"
  kubectl apply -f "$file"
}

main() {
  if [[ $# -ne 4 ]]; then
    echo "Usage: $0 <region> <stage> <output_dir> <model-id>" >&2
    exit 1
  fi

  local REGION="$1"
  local STAGE="$2"
  local output_dir="$3"
  local name="$4"
  local dir="$output_dir/$name"

  mkdir -p "$dir"
  local original_yaml="$dir/original.yaml"
  local converted_yaml="$dir/clusterbasemodel.yaml"

  validate_dependencies
  fetch_model_yaml "$name" "$original_yaml"
  convert_model_to_clusterbasemodel "$original_yaml" "$converted_yaml"
  apply_clusterbasemodel "$converted_yaml"
  log "Model to ClusterBaseModel conversion complete for $name"
}
main "$@"
