#!/bin/bash
# Usage: image_patch_dev.sh [OVERLAY]
set -u
set -e
set -o pipefail

IMG=$1
echo "IMG: ${IMG}"
if [ -z ${IMG} ]; then exit; fi
cat > config/default/manager_image_patch.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ome-controller-manager
  namespace: aok
spec:
  template:
    spec:
      containers:
        - name: manager
          image: ${IMG}
EOF
