#!/bin/bash
# Script to update container image references in configuration files

set -e

# Default values
REGISTRY=""
TAG="latest"
CONFIG_DIR="config"
BACKUP=true
DRY_RUN=false
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --registry)
      REGISTRY="$2"
      shift
      shift
      ;;
    --tag)
      TAG="$2"
      shift
      shift
      ;;
    --config-dir)
      CONFIG_DIR="$2"
      shift
      shift
      ;;
    --no-backup)
      BACKUP=false
      shift
      ;;
    --dry-run)
      DRY_RUN=true
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
if [[ -z "$REGISTRY" ]]; then
  echo "Error: --registry parameter is required"
  exit 1
fi

# Function to check if a command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

# Check for required tools
if ! command_exists yq; then
  echo "Error: yq is not installed. Please install it first."
  echo "See: https://github.com/mikefarah/yq#install"
  exit 1
fi

# Function to update image references in YAML files
update_yaml_images() {
  local file=$1
  local registry=$2
  local tag=$3
  
  if [[ "$VERBOSE" == "true" ]]; then
    echo "Processing file: $file"
  fi
  
  # Create backup if requested
  if [[ "$BACKUP" == "true" ]]; then
    cp "$file" "${file}.bak"
    if [[ "$VERBOSE" == "true" ]]; then
      echo "Created backup: ${file}.bak"
    fi
  fi
  
  # Get all image paths in the YAML file
  local image_paths=$(yq eval 'paths | select(. | last == "image") | .[0:-1] | join(".")' "$file")
  
  if [[ -z "$image_paths" ]]; then
    if [[ "$VERBOSE" == "true" ]]; then
      echo "No image references found in $file"
    fi
    return
  fi
  
  # Process each image path
  while IFS= read -r path; do
    # Get current image value
    local current_image=$(yq eval ".$path.image" "$file")
    
    # Skip if the image is empty
    if [[ -z "$current_image" || "$current_image" == "null" ]]; then
      continue
    fi
    
    # Extract image name without registry and tag
    local image_name=$(echo "$current_image" | sed -E 's|^.*\/([^:]+)(:.+)?$|\1|')
    
    # Construct new image reference
    local new_image="${registry}/${image_name}:${tag}"
    
    if [[ "$VERBOSE" == "true" ]]; then
      echo "Updating image at path $path:"
      echo "  From: $current_image"
      echo "  To:   $new_image"
    fi
    
    if [[ "$DRY_RUN" == "false" ]]; then
      # Update the image reference
      yq eval ".$path.image = \"$new_image\"" -i "$file"
    fi
  done <<< "$image_paths"
}

# Find all YAML files in the config directory
echo "Searching for YAML files in $CONFIG_DIR..."
yaml_files=$(find "$CONFIG_DIR" -type f \( -name "*.yaml" -o -name "*.yml" \))

if [[ -z "$yaml_files" ]]; then
  echo "No YAML files found in $CONFIG_DIR"
  exit 0
fi

# Count of files to process
file_count=$(echo "$yaml_files" | wc -l)
echo "Found $file_count YAML files to process"

if [[ "$DRY_RUN" == "true" ]]; then
  echo "Running in dry-run mode, no files will be modified"
fi

# Process each YAML file
updated_count=0
for file in $yaml_files; do
  update_yaml_images "$file" "$REGISTRY" "$TAG"
  updated_count=$((updated_count + 1))
  
  if [[ "$VERBOSE" != "true" ]]; then
    # Show progress
    echo -ne "Processing files: $updated_count/$file_count\r"
  fi
done

echo ""
if [[ "$DRY_RUN" == "true" ]]; then
  echo "Dry run completed. $file_count files would be processed."
else
  echo "Update completed. Processed $file_count files."
fi

exit 0 