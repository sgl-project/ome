#!/bin/bash
set -euo pipefail

# Get all models and clusterbasemodels at once
models_json=$(kubectl get models -o json)
cbms_json=$(kubectl get clusterbasemodels -o json)

echo -e "MODEL_ID\tMODEL_STATUS\tCLUSTERBASEMODEL_STATUS\tMATCH"

# We'll iterate directly over models in one go
echo "$models_json" | jq -r '
  .items[] 
  | select(.spec.vendor == "DUMMY_VENDOR")
  | .metadata.name' | while read -r model_id; do

  model_state=$(echo "$models_json" | jq -r --arg id "$model_id" '
    .items[] | select(.metadata.name == $id) | .status.state // "N/A"
  ')

  cbm_state=$(echo "$cbms_json" | jq -r --arg id "$model_id" '
    .items[] | select(.metadata.name == $id) | .status.state // "NOT_FOUND"
  ')

  if [[ "$model_state" == "$cbm_state" ]]; then
    match="YES"
  else
    match="NO"
  fi

  echo -e "$model_id\t$model_state\t$cbm_state\t$match"
done