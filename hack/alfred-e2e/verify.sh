#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"
trap stop_nested_tunnel EXIT

scenario_namespace="alfred-e2e"
deadline=$((SECONDS + 120))
last_score="unavailable"
last_count="0"
last_withheld="0"

start_nested_tunnel

while (( SECONDS < deadline )); do
  alfred_pod="$("${host_kubectl[@]}" -n "${host_namespace}" get pods \
    -l app=alfred-under-test -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -z "${alfred_pod}" ]]; then
    sleep 2
    continue
  fi

  metrics="$("${host_kubectl[@]}" get --raw \
    "/api/v1/namespaces/${host_namespace}/pods/${alfred_pod}:8080/proxy/metrics" \
    2>/dev/null || true)"
  last_score="$(awk '$1 == "alfred_cluster_fragmentation_score" {print $2}' <<<"${metrics}")"
  record="$(nested_kubectl -n ome get configmap alfred-recommendations \
    -o jsonpath='{.data.last-cycle\.json}' 2>/dev/null || true)"
  if [[ -n "${record}" ]]; then
    last_count="$(jq -r '.recommendations // [] | length' <<<"${record}")"
    last_withheld="$(jq -r '[.recommendations // [] | .[] | select(.outcome == "withheld")] | length' <<<"${record}")"
  fi

  pending_node="$(nested_kubectl -n "${scenario_namespace}" get pod pending-8gpu \
    -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)"
  if [[ -n "${last_score}" ]] &&
     awk -v score="${last_score}" 'BEGIN { exit !(score > 0.25) }' &&
     (( last_count > 0 )) && (( last_withheld > 0 )) &&
     [[ -z "${pending_node}" ]]; then
    echo "Alfred fragmentation E2E passed"
    echo "  score: ${last_score}"
    echo "  recommendations: ${last_count}"
    echo "  recommend-only withheld: ${last_withheld}"
    echo "  pending 8-GPU demand: unscheduled"
    exit 0
  fi
  sleep 2
done

echo "Alfred fragmentation E2E timed out" >&2
echo "  score: ${last_score}" >&2
echo "  recommendations: ${last_count}" >&2
echo "  recommend-only withheld: ${last_withheld}" >&2
"${host_kubectl[@]}" -n "${host_namespace}" logs deployment/alfred-under-test \
  --tail=80 >&2 || true
exit 1
