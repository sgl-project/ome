#!/bin/bash
set -euo pipefail

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*"
}

# Check if a ClusterBaseModel under ome.io with a given name exists
clusterbasemodel_exists() {
  local name="$1"
  kubectl get clusterbasemodel "$name" &>/dev/null
}

# Get all imported models from ome.oracle.com that need to be migrated
get_to_be_translated_models() {
  kubectl get models -o json | jq -r \
    '.items[]
     | select(
         .spec.vendor == "DUMMY_VENDOR").metadata.name'
}

# Interactive prompt for stage and region, export as STAGE and REGION
select_stage_and_region() {
  local valid_stages=("dev" "ppe" "prod")
  echo "Select stage:"
  for s in "${valid_stages[@]}"; do
    echo "  $s"
  done
  while true; do
    read -rp "Enter stage (dev/ppe/prod): " stage
    if [[ " ${valid_stages[*]} " == *" $stage "* ]]; then
      break
    else
      echo "Invalid stage! Please enter 'dev', 'ppe', or 'prod'."
    fi
  done

  local regions=(
    ap-osaka-1
    me-dubai-1
    sa-saopaulo-1
    us-phoenix-1
    uk-london-1
    us-ashburn-1
    eu-frankfurt-1
    us-chicago-1
  )
  echo "Select region from the following list:"
  for i in "${!regions[@]}"; do
    printf "  %d) %s\n" "$((i+1))" "${regions[$i]}"
  done
  local num
  while true; do
    read -rp "Enter region number (1-${#regions[@]}): " num
    if [[ "$num" =~ ^[1-8]$ ]]; then
      break
    else
      echo "Invalid region number! Please enter a number between 1 and ${#regions[@]}."
    fi
  done
  export STAGE="$stage"
  export REGION="${regions[num-1]}"
  echo "Selected stage: $STAGE"
  echo "Selected region: $REGION"
}

translate_ome_oracle_com_models() {
  local output_dir="$1"
  local models_already_exist="$output_dir/models_already_exist.yaml"
  : > "$models_already_exist"  # Truncate or create

  local models
  models=$(get_to_be_translated_models)

  if [[ -z "$models" ]]; then
    log "No ome.oracle.com models with vendor='DUMMY_VENDOR'' found."
    return 0
  fi

  log "Starting translation of ome.oracle.com imported models."
  for model_id in $models; do
    log "-------------------------------------------------------------------------"
    log "Processing ome.oracle.com model: $model_id"
    if clusterbasemodel_exists "$model_id"; then
      log "Skipping '$model_id' - clusterbasemodel with this name already exists."
      echo "$model_id" >> "$models_already_exist"
    else
       bash translate_one_imported_model_from_model.sh "$REGION" "$STAGE" "$output_dir" "$model_id"
    fi
  done
  log "Finished translating all ome.oracle.com imported models. Skipped models written to $models_already_exist"
}

main() {
  # Allow user to configure KUBECONFIG or fall back to default
  : "${KUBECONFIG:=$HOME/.kube/config}"
  export KUBECONFIG
  log "Using KUBECONFIG: $KUBECONFIG"

  select_stage_and_region

  local target="imported-models"
  local dir="./${STAGE}-${REGION}-${target}"
  mkdir -p "$dir"

  log "Target env: $STAGE, target region: $REGION"
  translate_ome_oracle_com_models "$dir"
}

main "$@"