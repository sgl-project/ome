#!/bin/bash
set -eu -o pipefail

cd "$(dirname "$0")/.."

# Process opensource CRDs
for dir in opensource-ome ome; do
  if [ -d "config/crd/full/${dir}" ]; then
    mkdir -p "config/crd/minimal/${dir}"
    find "config/crd/full/${dir}" -name 'ome.io*.yaml' | while read -r file; do
      minimal="config/crd/minimal/${dir}/$(basename "$file")"
      echo "Creating minimal CRD file: ${minimal}"
      cp "$file" "$minimal"
      go run ./cmd/crd-gen removecrdvalidation "$minimal"
    done
  fi
done
