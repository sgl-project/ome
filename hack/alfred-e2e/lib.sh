#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd "${script_dir}/../.." && pwd)"
host_kubeconfig="${ALFRED_E2E_HOST_KUBECONFIG:-}"
host_context="${ALFRED_E2E_HOST_CONTEXT:-}"
host_namespace="${ALFRED_E2E_NAMESPACE:-alfred-e2e}"
nested_port="${ALFRED_E2E_NESTED_PORT:-18080}"
nested_server="http://127.0.0.1:${nested_port}"

host_kubectl=(kubectl)
if [[ -n "${host_kubeconfig}" ]]; then
  host_kubectl+=(--kubeconfig "${host_kubeconfig}")
fi
if [[ -n "${host_context}" ]]; then
  host_kubectl+=(--context "${host_context}")
fi

require_command() {
  local cmd
  for cmd in "$@"; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      echo "required command not found: ${cmd}" >&2
      exit 1
    fi
  done
}

tunnel_pid=""
tunnel_log=""

stop_nested_tunnel() {
  if [[ -n "${tunnel_pid}" ]]; then
    kill "${tunnel_pid}" >/dev/null 2>&1 || true
    wait "${tunnel_pid}" >/dev/null 2>&1 || true
    tunnel_pid=""
  fi
  if [[ -n "${tunnel_log}" && -f "${tunnel_log}" ]]; then
    rm -f "${tunnel_log}"
    tunnel_log=""
  fi
}

start_nested_tunnel() {
  local outcome
  # A handoff between scripts can reach here before the previous forward has
  # released the fixed local port, which kills the new one on bind. That is
  # transient, so retry it; a forward that stays up but never answers is not.
  for _ in 1 2 3; do
    tunnel_log="$(mktemp)"
    "${host_kubectl[@]}" -n "${host_namespace}" port-forward \
      service/kwok-cluster "${nested_port}:8080" >"${tunnel_log}" 2>&1 &
    tunnel_pid=$!

    outcome="unready"
    for _ in $(seq 1 30); do
      if ! kill -0 "${tunnel_pid}" >/dev/null 2>&1; then
        outcome="died"
        break
      fi
      if nested_kubectl get --raw=/readyz >/dev/null 2>&1; then
        return 0
      fi
      sleep 1
    done

    cat "${tunnel_log}" >&2
    if [[ "${outcome}" != "died" ]]; then
      return 1
    fi
    stop_nested_tunnel
    sleep 2
  done
  return 1
}

# The nested API is anonymous plain HTTP. --server overrides only the cluster
# server, so without an explicit empty kubeconfig the ambient context's user
# still applies — and every cloud kubeconfig runs an exec credential plugin,
# which would make these calls fail for reasons unrelated to the nested cluster.
nested_kubectl() {
  kubectl --kubeconfig /dev/null --server "${nested_server}" "$@"
}
