#!/bin/bash
set -euo pipefail

# Default values
AUTH="instance_principal"
REGION="us-chicago-1"
VENDOR="meta"


print_usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --bucket-name     OCI Object Storage bucket name (required)"
  echo "  --namespace       OCI Object Storage namespace (required)"
  echo "  --dest-dir        Local destination directory (required)"
  echo "  --prefix          Prefix path inside the bucket (required)"
  echo "  --region          OCI region (default: us-chicago-1)"
  echo "  --auth            OCI auth type (default: instance_principal)"
  echo "  --vendor          Model vendor name (default: meta)"
  echo "  --help            Show this help message"
}

# Parse long options
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bucket-name)
      BUCKET_NAME="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --dest-dir)
      DEST_DIR="$2"
      shift 2
      ;;
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --region)
      REGION="$2"
      shift 2
      ;;
    --auth)
      AUTH="$2"
      shift 2
      ;;
    --vendor)
      VENDOR="$2"
      shift 2
      ;;
    --help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done


# Ensure required parameters are set
: "${BUCKET_NAME:?--bucket-name is required}"
: "${NAMESPACE:?--namespace is required}"
: "${DEST_DIR:?--dest-dir is required}"
: "${PREFIX:?--prefix is required}"

echo "== Configuration =="
echo "  Bucket Name : $BUCKET_NAME"
echo "  Namespace   : $NAMESPACE"
echo "  Prefix      : $PREFIX"
echo "  Destination : $DEST_DIR"
echo "  Region      : $REGION"
echo "  Auth        : $AUTH"
echo "  Vendor      : $VENDOR"
echo ""


# Step 1: Download model files
echo "== Downloading model files from OCI Object Storage..."
DOWNLOAD_CMD=(
  oci os object bulk-download
  --bucket-name "$BUCKET_NAME"
  --namespace "$NAMESPACE"
  --dest-dir "$DEST_DIR"
  --prefix "$PREFIX"
  --auth "$AUTH"
  --exclude ".cache/*"
  --region "$REGION"
)

if [[ "$VENDOR" == "meta" ]]; then
  echo "Excluding folder 'original/*'"
  DOWNLOAD_CMD+=(--exclude 'original/*')
fi

echo "Executing: ${DOWNLOAD_CMD[*]}"
"${DOWNLOAD_CMD[@]}"
echo "Download completed."
echo ""


# Step 2: Iterate and update metadata
DOWNLOAD_PATH="$DEST_DIR/$PREFIX"
echo "== Updating metadata for downloaded files..."
echo "Scanning directory: $DOWNLOAD_PATH"

COUNT=0
TOTAL=$(find "$DOWNLOAD_PATH" -type f | wc -l)

find "$DOWNLOAD_PATH" -type f -print0 | while IFS= read -r -d '' FILE; do
  COUNT=$((COUNT + 1))
  echo "[$COUNT/$TOTAL] Processing: $FILE"

  REL_PATH="${FILE#$DEST_DIR/}"  # Path relative to root of model directory
  MD5_HASH=$(md5sum "$FILE" | awk '{print $1}' | xxd -r -p | base64)
  OBJECT_NAME=$(echo "$REL_PATH" | sed 's| |%20|g')  # URL-encode spaces if any

  echo "Updating metadata for: $OBJECT_NAME (md5: $MD5_HASH)"

  TARGET_URI="https://objectstorage.${REGION}.oraclecloud.com/n/${NAMESPACE}/b/${BUCKET_NAME}/actions/mergeObjectMetadata/${OBJECT_NAME}"

  oci raw-request \
    --http-method POST \
    --target-uri "$TARGET_URI" \
    --request-body "{\"metadata\":{\"opc-meta-md5\":\"${MD5_HASH}\"}}" \
    --auth "$AUTH" \
    --region "$REGION"
done

echo ""
echo "✅ Metadata update completed for all files."