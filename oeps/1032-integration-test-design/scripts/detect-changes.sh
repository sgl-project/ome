#!/bin/bash
# Script to detect changes in runtime, model, or InferenceService configurations

set -e

# Default values
TYPE="all"
REPO_ROOT=$(git rev-parse --show-toplevel)
PREVIOUS_COMMIT=$(git rev-parse HEAD~1)
CURRENT_COMMIT=$(git rev-parse HEAD)
OUTPUT_FILE=""
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --type)
      TYPE="$2"
      shift
      shift
      ;;
    --previous-commit)
      PREVIOUS_COMMIT="$2"
      shift
      shift
      ;;
    --current-commit)
      CURRENT_COMMIT="$2"
      shift
      shift
      ;;
    --output)
      OUTPUT_FILE="$2"
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

# Validate type
if [[ "$TYPE" != "all" && "$TYPE" != "runtime" && "$TYPE" != "model" && "$TYPE" != "isvc" ]]; then
  echo "Invalid type: $TYPE. Must be one of: all, runtime, model, isvc"
  exit 1
fi

# Function to detect changes in specific paths
detect_changes() {
  local type=$1
  local paths=$2
  local changes=$(git diff --name-only $PREVIOUS_COMMIT $CURRENT_COMMIT -- $paths)
  
  if [[ -n "$changes" ]]; then
    echo "Detected changes in $type configurations:"
    echo "$changes"
    return 0
  else
    echo "No changes detected in $type configurations"
    return 1
  fi
}

# Function to write changes to output file
write_to_output() {
  local type=$1
  local changes=$2
  
  if [[ -n "$OUTPUT_FILE" ]]; then
    echo "# $type changes" >> "$OUTPUT_FILE"
    echo "$changes" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
  fi
}

# Initialize output file if specified
if [[ -n "$OUTPUT_FILE" ]]; then
  echo "# Configuration changes between $PREVIOUS_COMMIT and $CURRENT_COMMIT" > "$OUTPUT_FILE"
  echo "Generated at: $(date)" >> "$OUTPUT_FILE"
  echo "" >> "$OUTPUT_FILE"
fi

# Track if any changes were detected
CHANGES_DETECTED=false

# Detect runtime changes
if [[ "$TYPE" == "all" || "$TYPE" == "runtime" ]]; then
  RUNTIME_PATHS="config/runtimes/ pkg/controller/v1beta1/runtime/ pkg/apis/ome/v1beta1/runtime_*.go"
  if RUNTIME_CHANGES=$(detect_changes "runtime" "$RUNTIME_PATHS"); then
    CHANGES_DETECTED=true
    write_to_output "Runtime" "$RUNTIME_CHANGES"
    
    if [[ "$VERBOSE" == "true" ]]; then
      echo "Detailed runtime changes:"
      git diff $PREVIOUS_COMMIT $CURRENT_COMMIT -- $RUNTIME_PATHS
    fi
  fi
fi

# Detect model changes
if [[ "$TYPE" == "all" || "$TYPE" == "model" ]]; then
  MODEL_PATHS="config/models/ pkg/controller/v1beta1/model/ pkg/apis/ome/v1beta1/model_*.go"
  if MODEL_CHANGES=$(detect_changes "model" "$MODEL_PATHS"); then
    CHANGES_DETECTED=true
    write_to_output "Model" "$MODEL_CHANGES"
    
    if [[ "$VERBOSE" == "true" ]]; then
      echo "Detailed model changes:"
      git diff $PREVIOUS_COMMIT $CURRENT_COMMIT -- $MODEL_PATHS
    fi
  fi
fi

# Detect InferenceService changes
if [[ "$TYPE" == "all" || "$TYPE" == "isvc" ]]; then
  ISVC_PATHS="config/inferenceservice/ pkg/controller/v1beta1/inferenceservice/ pkg/apis/ome/v1beta1/inferenceservice_*.go"
  if ISVC_CHANGES=$(detect_changes "InferenceService" "$ISVC_PATHS"); then
    CHANGES_DETECTED=true
    write_to_output "InferenceService" "$ISVC_CHANGES"
    
    if [[ "$VERBOSE" == "true" ]]; then
      echo "Detailed InferenceService changes:"
      git diff $PREVIOUS_COMMIT $CURRENT_COMMIT -- $ISVC_PATHS
    fi
  fi
fi

# Set exit code based on whether changes were detected
if [[ "$CHANGES_DETECTED" == "true" ]]; then
  echo "Changes detected in configuration files"
  exit 0
else
  echo "No changes detected in configuration files"
  exit 1
fi 