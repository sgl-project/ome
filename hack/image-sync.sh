#!/bin/bash

set -euo pipefail

# Detect container CLI
CONTAINER_CLI=$(if command -v nerdctl &> /dev/null; then echo nerdctl; else echo docker; fi)

log() {
    local level=$1
    shift
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [${level}] $*"
}

cleanup_image() {
    local image=$1
    ${CONTAINER_CLI} rmi "${image}" &> /dev/null || true
}

# Determine the latest tag
GIT_TAG=$(git describe --tags $(git rev-list --tags --max-count=1))
SOURCE_TAG="${GIT_TAG}-linux-amd64"  # Source tag with platform suffix
TARGET_TAG="${GIT_TAG}"              # Target tag without platform suffix
REGISTRY="ord.ocir.io/idqj093njucb"
PLATFORM="linux/amd64"

log "INFO" "Script started"
log "INFO" "Using container CLI: ${CONTAINER_CLI}"
log "INFO" "Using source tag: ${SOURCE_TAG}"
log "INFO" "Using target tag: ${TARGET_TAG}"
log "INFO" "Target registry: ${REGISTRY}"
log "INFO" "Platform: ${PLATFORM}"

SOURCE_REGISTRY="odo-docker-signed-local.artifactory-builds.oci.oraclecorp.com"

# Define source and target image names
declare -a SOURCE_IMAGES=(
    "genai-ome-model-agent"
    "genai-multinode-prober"
    "genai-ome-model-controller"
    "genai-ome-manager"
    "genai-ome-agent"
)

declare -a TARGET_PATHS=(
    "ome/model-agent"
    "ome/multinode-prober"
    "ome/model-controller"
    "ome/manager"
    "ome/ome-agent"
)

log "INFO" "Starting image sync process..."

# Pull and push each image
for i in "${!SOURCE_IMAGES[@]}"; do
    source_name="${SOURCE_IMAGES[$i]}"
    target_path="${TARGET_PATHS[$i]}"
    source_image="${SOURCE_REGISTRY}/${source_name}:${SOURCE_TAG}"
    target_image="${REGISTRY}/${target_path}:${TARGET_TAG}"
    
    log "INFO" "Processing image: ${source_name} -> ${target_path}"
    
    # Cleanup any existing images first
    cleanup_image "${source_image}"
    cleanup_image "${target_image}"
    
    log "INFO" "Pulling: ${source_image}"
    if ! ${CONTAINER_CLI} pull --quiet --platform="${PLATFORM}" "${source_image}" > /dev/null; then
        log "ERROR" "Failed to pull ${source_image}"
        exit 1
    fi
    log "INFO" "Pull completed successfully"
    
    # For nerdctl, we need to inspect and potentially convert the image
    if [ "${CONTAINER_CLI}" = "nerdctl" ]; then
        # Create a temporary tag for platform-specific image
        tmp_tag="tmp-${TARGET_TAG}"
        tmp_image="${REGISTRY}/${target_path}:${tmp_tag}"
        
        log "INFO" "Creating temporary image: ${tmp_image}"
        if ! ${CONTAINER_CLI} tag "${source_image}" "${tmp_image}"; then
            log "ERROR" "Failed to create temporary tag"
            exit 1
        fi
        
        log "INFO" "Converting multi-platform image to single platform"
        if ! ${CONTAINER_CLI} push --quiet "${tmp_image}" > /dev/null; then
            cleanup_image "${tmp_image}"
            log "ERROR" "Failed to convert image platform"
            exit 1
        fi
        
        if ! ${CONTAINER_CLI} pull --quiet --platform="${PLATFORM}" "${tmp_image}" > /dev/null; then
            cleanup_image "${tmp_image}"
            log "ERROR" "Failed to pull converted image"
            exit 1
        fi
        
        # Tag the platform-specific image as the target
        if ! ${CONTAINER_CLI} tag "${tmp_image}" "${target_image}"; then
            cleanup_image "${tmp_image}"
            log "ERROR" "Failed to tag final image"
            exit 1
        fi
        cleanup_image "${tmp_image}"
    else
        log "INFO" "Tagging: ${source_image} -> ${target_image}"
        if ! ${CONTAINER_CLI} tag "${source_image}" "${target_image}"; then
            log "ERROR" "Failed to tag ${source_image} as ${target_image}"
            exit 1
        fi
    fi
    
    log "INFO" "Pushing: ${target_image}"
    if ! ${CONTAINER_CLI} push --quiet "${target_image}" > /dev/null; then
        log "ERROR" "Failed to push ${target_image}"
        exit 1
    fi
    log "INFO" "Push completed successfully"
    
    # Cleanup after successful push
    cleanup_image "${source_image}"
    cleanup_image "${target_image}"
    
    log "INFO" "Successfully processed ${source_name}"
    log "INFO" "----------------------------------------"
done

log "INFO" "Image sync process completed successfully!"
