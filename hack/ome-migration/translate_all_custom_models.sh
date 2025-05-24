#!/bin/bash

set -euo pipefail

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*"
}

# Check if a ome.io FineTunedWeight with the given name exists
fine_tuned_weight_exists() {
  local name="$1"
  kubectl get finetunedweight "$name" &> /dev/null
}

# Get all fine tuned models from ome.oracle.com which needs to be migrated
get_to_be_translated_models() {
  kubectl get models -o json | jq -r '.items[] | select(.spec.type == "Custom") | .metadata.name'
}

# Get all fine tuned models from model.generative-ai.oracle.com which needs to be migrated
get_to_be_translated_fine_tuned_models() {
  # Get all Custom models from ome.oracle.com
  local models
  models=$(get_to_be_translated_models)

  # Get all Custom models (FineTunedModel) from model.generative-ai.oracle.com
  fine_tuned_models=$(kubectl get finetunedmodels -o json | jq -r '.items[].metadata.name')

  # Convert the model lists into arrays for comparison
  models_array=($models)
  fine_tuned_models_array=($fine_tuned_models)

  # Create an empty array to store custom models from model.generative-ai.oracle.com but not migrated to ome.oracle.com models
  result=()

  # Loop through each fine-tuned model and check if it is not in the models list
  for fine_tuned_model in "${fine_tuned_models_array[@]}"; do
    if [[ ! " ${models_array[@]} " =~ " ${fine_tuned_model} " ]]; then
      result+=("$fine_tuned_model")
    fi
  done

  # Output the result
  echo "${result[@]}"
}

translate_ome_oracle_com_models() {
  local output_dir="$1"

  local models_already_exist="$output_dir/models_already_exist.yaml"
  : > "$models_already_exist"  # Truncate or create

  # Get all Custom models from ome.oracle.com
  local models
  models=$(get_to_be_translated_models)

  if [[ -z "$models" ]]; then
    log "No ome.oracle.com models with .spec.type == \"Custom\" found"
    return 0
  fi

  log "Starting translating ome.oracle.com Custom models"
  for model_id in $models; do
    log "-------------------------------------------------------------------------"
    log "Processing ome.oracle.com model: $model_id"
    if fine_tuned_weight_exists "$model_id"; then
      log "Skipping ome.oracle.com model '$model_id' - A ome.io FineTuneWeight with '$model_id' name already exists"
      echo "$model_id" >> "$models_already_exist"
    else
      bash translate_one_custom_model_from_model.sh "$model_id"
    fi
  done

  log "Finished translating all ome.oracle.com Custom models. Skipped models written to $models_already_exist"
}

translate_generative-ai_oracle_com_fine_tuned_models() {
  local output_dir="$1"

  local fine_tuned_models_already_exist="$output_dir/fine_tuned_models_already_exist.yaml"
  : > "$fine_tuned_models_already_exist"  # Truncate or create

 # Get all Custom models from model.generative-ai.oracle.com which requires to be translated/migrated
  local fine_tuned_models=$(get_to_be_translated_fine_tuned_models)

  if [[ -z "$fine_tuned_models" ]]; then
    log "No model.generative-ai.oracle.com fine tuned models which need to be translated/migrated"
    return 0
  fi

  log "Starting translating model.generative-ai.oracle.com fine tuned models"
  for model_id in $fine_tuned_models; do
    log "-------------------------------------------------------------------------"
    log "Processing model.generative-ai.oracle.com fine tuned model: $model_id"
    if fine_tuned_weight_exists "$model_id"; then
      log "Skipping model.generative-ai.oracle.com fine tuned model '$model_id' - A ome.io FineTuneWeight with '$model_id' name already exists"
      echo "$model_id" >> "$fine_tuned_models_already_exist"
    else
      bash translate_one_custom_model_from_finetunedmodel.sh "$model_id"
    fi
  done

  log "Finished translating all model.generative-ai.oracle.com fine tuned models. Skipped fine tuned models written to $fine_tuned_models_already_exist"
}


main() {
  # Allow user to configure KUBECONFIG or fallback to default
  : "${KUBECONFIG:=$HOME/.kube/config}"
  export KUBECONFIG
  log "Using KUBECONFIG: $KUBECONFIG"

  # Set REGION_CODE and ENV env variable to update generated yaml saved folder path
  local region_code="${REGION_CODE:=ord}"
  local env="${ENV:=dev}"
  log "Target env: $env, target region: $region_code"

  local target="custom-model"
  local dir="./$env-$region_code-$target"
  mkdir -p "$dir"

  translate_ome_oracle_com_models "$dir"

  translate_generative-ai_oracle_com_fine_tuned_models "$dir"
}


main "$@"
