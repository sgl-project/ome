#!/bin/bash 
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

is_merged_weight() {
  local runtime=$1

 if [[ "$runtime" == "cohere-finetuning" ]]; then
   echo "false"
 else
   echo "true"
 fi
}

convert_model_to_finetunedweight() {
  local input_yaml=$1
  local output_yaml=$2

  log "Converting Model CR to FineTunedWeight CR"

  local json=$(yq eval -o=json "$input_yaml")

  # Extract values from Model CR
  local model_name=$(jq -r '.metadata.name' <<< "$json")
  local training_job_name=$(jq -r '.status.activeJobRef.name' <<< "$json")
  local training_job_namespace=$(jq -r '.status.activeJobRef.namespace' <<< "$json")
  local base_model_ref=$(jq -r '.spec.fineTunedModelSpec.baseModelRef' <<< "$json")
  local storage_url=$(jq -r '.spec.storageUrl' <<< "$json")
  local vendor=$(jq -r '.spec.vendor' <<< "$json")
  local version=$(jq -r '.spec.version' <<< "$json")
  local runtime=$(jq -r '.spec.runtime' <<< "$json")

  # Extract hyperparameters directly from the Model CR
  declare -A hyperparameters
  local strategy=""
  for param in $(jq -r '.spec.fineTunedModelSpec.hyperparameters[] | @base64' <<< "$json"); do
    key=$(echo "$param" | base64 --decode | jq -r '.key')
    value=$(echo "$param" | base64 --decode | jq -r '.value')
    if [[ "$key" == "trainingConfigType" ]]; then
      strategy="$value"
      hyperparameters["strategy"]="$value"
    else
      hyperparameters["$key"]="$value"
    fi
  done

  # If strategy is still empty, and vendor is "meta", default strategy to "lora"
  if [[ -z "$strategy" ]]; then
    if [[ "$vendor" == "meta" ]]; then
      strategy="lora"
      hyperparameters["strategy"]="lora"
      log "No strategy/trainingConfigType set in hyperparameters. Defaulting strategy to 'lora' for vendor 'meta'"
    fi
  fi

  # Convert storage URL
  local namespace=$(echo "$storage_url" | sed -E 's|os://([^/]+)/.*|\1|')
  local bucket=$(echo "$storage_url" | sed -E 's|os://[^/]+/([^/]+)/.*|\1|')
  local object=$(echo "$storage_url" | sed -E 's|os://[^/]+/[^/]+/(.*)|\1|')
  local storage_uri="oci://n/${namespace}/b/${bucket}/o/${object}"

  # Check if fine-tuned weight is a merged one
  local merged_weights=$(is_merged_weight "$runtime")

  # Prepare dynamic hyperParameters JSON fragment
  local hyperparams_json="{"
  local first=1
  for key in "${!hyperparameters[@]}"; do
    [[ $first -eq 0 ]] && hyperparams_json+=","
    hyperparams_json+="\"$key\":\"${hyperparameters[$key]}\""
    first=0
  done
  hyperparams_json+="}"

  # Create FineTunedWeight CR structure
  local converted=$(jq -n \
    --arg model_name "$model_name" \
    --arg base_model_ref "$base_model_ref" \
    --arg training_job_name "$training_job_name" \
    --arg training_job_namespace "$training_job_namespace" \
    --arg storage_uri "$storage_uri" \
    --arg vendor "$vendor" \
    --arg version "$version" \
    --arg strategy "$strategy" \
    --argjson hyperparams "$hyperparams_json" \
    --arg merged_weights "$merged_weights" '
    {
      apiVersion: "ome.io/v1beta1",
      kind: "FineTunedWeight",
      metadata: {
        name: $model_name
      },
      spec: {
        baseModelRef: {
          name: $base_model_ref
        },
        configuration: {
          merged_weights: ($merged_weights | test("true"))
        },
        hyperParameters: $hyperparams,
        modelType: $strategy,
        storage: {
          storageUri: $storage_uri
        },
        vendor: $vendor,
        version: $version,
        disabled: false,
        trainingJobRef: {
          name: $training_job_name,
          namespace: $training_job_namespace
        }
      }
    }')

  echo "$converted" | yq eval -P > "$output_yaml"
}


apply_finetunedweight() {
  local file=$1
  log "Applying FineTunedWeight YAML from $file"
  kubectl apply -f "$file"
}

patch_finetunedweight_status() {
  local model_name=$1
  local input_yaml=$2

  local json=$(yq eval -o=json "$input_yaml")

  # Extract values from Model CR
  local model_state=$(jq -r '.status.state' <<< "$json")

  if [[ -n "$model_state" ]]; then
    log "Patching FineTunedWeight status.state = $model_state"
    kubectl patch finetunedweight "$model_name" \
      --type merge \
      --subresource status \
      -p "{\"status\": {\"state\": \"$model_state\"}}"

    log "Successfully patch the FineTunedWeight $model_name"
  else
    log "Model status.state is empty, skipping status patch"
  fi
}

main() {
  if [[ $# -ne 1 ]]; then
    error "Usage: $0 <model-name>"
  fi

  local name=$1

  # Allow user to configure KUBECONFIG or fallback to default
  : "${KUBECONFIG:=$HOME/.kube/config}"
  export KUBECONFIG
  log "Using KUBECONFIG: $KUBECONFIG"

  # Set REGION_CODE and ENV env variable to update generated yaml saved folder path
  local region_code="${REGION_CODE:=ord}"
  local env="${ENV:=dev}"
  #log "Target env: $env, target region: $region_code"

  local target="custom-model"
  local dir="./$env-$region_code-$target/$name"
  mkdir -p "$dir"

  local original_yaml="$dir/original.yaml"
  local converted_yaml="$dir/finetunedweight.yaml"

  validate_dependencies
  fetch_model_yaml "$name" "$original_yaml"
  convert_model_to_finetunedweight "$original_yaml" "$converted_yaml"
  apply_finetunedweight "$converted_yaml"
  log "Model to FineTunedWeight conversion complete for $name"

  patch_finetunedweight_status "$name" "$original_yaml"
}

main "$@"
