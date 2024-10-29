#!/bin/bash
# Usage: patch_image_dev.sh <IMAGE> <RESOURCE_NAME>

set -u
set -e
set -o pipefail

IMG=${1:-}
RESOURCE_NAME=${2:-}

if [ -z "$IMG" ] || [ -z "$RESOURCE_NAME" ]; then
  echo "Usage: patch_image.sh <IMAGE> <RESOURCE_NAME>"
  exit 1
fi

echo "Patching resource '${RESOURCE_NAME}' with image '${IMG}'"

patch_manager_deployment() {
  local resource_name=$1
  local container_name=$2
  cat > config/default/${resource_name}_image_patch.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ome-controller-manager
  namespace: ome
spec:
  template:
    spec:
      containers:
        - name: ${container_name}
          image: ${IMG}
EOF
  echo "Generated Deployment patch file: config/default/${resource_name}_image_patch.yaml"
}

patch_model_controller_deployment() {
  local resource_name=$1
  local container_name=$2
  cat > config/default/${resource_name}_image_patch.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ome-model-controller
  namespace: ome
spec:
  template:
    spec:
      containers:
        - name: ${container_name}
          image: ${IMG}
EOF
  echo "Generated Deployment patch file: config/default/${resource_name}_image_patch.yaml"
}

patch_model_daemonset() {
  local resource_name=$1
  local container_name=$2
  cat > config/default/${resource_name}_image_patch.yaml << EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ome-model-agent-daemonset
  namespace: ome
spec:
  template:
    spec:
      containers:
        - name: ${container_name}
          image: ${IMG}
EOF
  echo "Generated DaemonSet patch file: config/default/${resource_name}_image_patch.yaml"
}

case "$RESOURCE_NAME" in
  manager)
    patch_manager_deployment "$RESOURCE_NAME" "manager"
    ;;
  model_controller)
    patch_model_controller_deployment "$RESOURCE_NAME" "model-controller"
    ;;
  model_agent)
    patch_model_daemonset "$RESOURCE_NAME" "model-agent"
    ;;
  *)
    echo "Error: Unknown resource name '$RESOURCE_NAME'"
    exit 1
    ;;
esac