#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

alfred_image="${ALFRED_E2E_IMG:-}"
pull_secret_source="${ALFRED_E2E_PULL_SECRET_SOURCE:-}"
kustomize_bin="${KUSTOMIZE_BIN:-${project_dir}/bin/kustomize}"
render_dir=""

cleanup() {
  stop_nested_tunnel
  if [[ -n "${render_dir}" && -d "${render_dir}" ]]; then
    rm -rf "${render_dir}"
  fi
}
trap cleanup EXIT

if [[ -z "${alfred_image}" ]]; then
  echo "ALFRED_E2E_IMG must name the Alfred image to test" >&2
  exit 1
fi
if [[ ! -x "${kustomize_bin}" ]]; then
  echo "required executable not found: ${kustomize_bin}" >&2
  exit 1
fi
if [[ -z "${pull_secret_source}" || "${pull_secret_source}" != */* ]]; then
  echo "ALFRED_E2E_PULL_SECRET_SOURCE must be namespace/name" >&2
  exit 1
fi

source_secret_namespace="${pull_secret_source%%/*}"
source_secret_name="${pull_secret_source#*/}"

echo "Creating isolated host namespace ${host_namespace}"
"${host_kubectl[@]}" create namespace "${host_namespace}" \
  --dry-run=client -o yaml | "${host_kubectl[@]}" apply -f -

# Copy only the explicitly selected cluster secret into the isolated namespace.
"${host_kubectl[@]}" -n "${source_secret_namespace}" get secret "${source_secret_name}" -o json |
  jq --arg namespace "${host_namespace}" \
    'del(.metadata.annotations, .metadata.creationTimestamp, .metadata.managedFields, .metadata.ownerReferences, .metadata.resourceVersion, .metadata.uid) |
     .metadata.name = "alfred-e2e-registry" | .metadata.namespace = $namespace' |
  "${host_kubectl[@]}" apply -f -

render_dir="$(mktemp -d)"
cp -R "${script_dir}/host/." "${render_dir}/"
(
  cd "${render_dir}"
  "${kustomize_bin}" edit set namespace "${host_namespace}"
  "${kustomize_bin}" edit set image "alfred-e2e-placeholder=${alfred_image}"
  "${kustomize_bin}" edit set image \
    "registry.k8s.io/kwok/cluster=${KWOK_CLUSTER_IMAGE:-registry.k8s.io/kwok/cluster:v0.8.0-k8s.v1.33.12}"
)

"${kustomize_bin}" build "${render_dir}" |
  "${host_kubectl[@]}" apply --server-side --force-conflicts -f -
"${host_kubectl[@]}" -n "${host_namespace}" rollout status \
  deployment/kwok-cluster --timeout=240s

start_nested_tunnel
echo "Installing OME CRDs and Alfred configuration in nested KWOK"
nested_kubectl apply --server-side --force-conflicts -k "${project_dir}/config/crd"
crd_ready=false
for _ in $(seq 1 60); do
  established="$(nested_kubectl get crd inferenceservices.ome.io \
    -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' \
    2>/dev/null || true)"
  if [[ "${established}" == "True" ]]; then
    crd_ready=true
    break
  fi
  sleep 2
done
if [[ "${crd_ready}" != "true" ]]; then
  echo "InferenceService CRD did not become established" >&2
  exit 1
fi
nested_kubectl apply -f "${script_dir}/nested/alfred-config.yaml"

"${host_kubectl[@]}" -n "${host_namespace}" rollout restart \
  deployment/alfred-under-test
"${host_kubectl[@]}" -n "${host_namespace}" rollout status \
  deployment/alfred-under-test --timeout=240s
echo "Nested KWOK and Alfred are ready in host namespace ${host_namespace}"
