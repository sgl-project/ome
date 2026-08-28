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
  tunnel_log="$(mktemp)"
  "${host_kubectl[@]}" -n "${host_namespace}" port-forward \
    service/kwok-cluster "${nested_port}:8080" >"${tunnel_log}" 2>&1 &
  tunnel_pid=$!

  for _ in $(seq 1 30); do
    if ! kill -0 "${tunnel_pid}" >/dev/null 2>&1; then
      cat "${tunnel_log}" >&2
      return 1
    fi
    if kubectl --server "${nested_server}" get --raw=/readyz >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  cat "${tunnel_log}" >&2
  return 1
}

nested_kubectl() {
  kubectl --server "${nested_server}" "$@"
}
