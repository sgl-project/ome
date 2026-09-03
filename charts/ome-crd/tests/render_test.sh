#!/usr/bin/env bash
set -euo pipefail

chart_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
helm_bin="${HELM_BIN:-helm}"

fail() {
  echo "ome-crd chart test: $*" >&2
  exit 1
}

"${helm_bin}" lint --strict "${chart_dir}" >/dev/null
rendered="$("${helm_bin}" template ome-crd "${chart_dir}" --namespace ome)"

grep -Fq 'helm.sh/chart: ome-crd-0.1.0' <<<"${rendered}" ||
  fail "release marker chart version label was not rendered"
grep -Fq 'name: ome-crd-release' <<<"${rendered}" ||
  fail "release marker ConfigMap was not rendered"
grep -Fq 'chart-version: "0.1.0"' <<<"${rendered}" ||
  fail "release marker chart version was not rendered"
grep -Fq 'app-version: "1.16.0"' <<<"${rendered}" ||
  fail "release marker app version was not rendered"

# The AutoscalerPolicy CRD installs only with the feature gate, so a cluster
# never carries the CRD without the controller + webhook the ome-resources
# chart gates on the same value.
if grep -Fq 'name: autoscalerpolicies.ome.io' <<<"${rendered}"; then
  fail "AutoscalerPolicy CRD was rendered with the feature gate off"
fi

autoscaler_enabled="$("${helm_bin}" template ome-crd "${chart_dir}" \
  --namespace ome \
  --set ome.autoscalerPolicy.enabled=true)"
grep -Fq 'name: autoscalerpolicies.ome.io' <<<"${autoscaler_enabled}" ||
  fail "AutoscalerPolicy CRD was not rendered with the feature gate on"

# The RolloutPolicy CRD installs only with the feature gate, so a cluster
# never carries the CRD without the webhook the ome-resources chart gates on
# the same value.
if grep -Fq 'name: rolloutpolicies.ome.io' <<<"${rendered}"; then
  fail "RolloutPolicy CRD was rendered with the feature gate off"
fi

rollout_enabled="$("${helm_bin}" template ome-crd "${chart_dir}" \
  --namespace ome \
  --set ome.rolloutPolicy.enabled=true)"
grep -Fq 'name: rolloutpolicies.ome.io' <<<"${rollout_enabled}" ||
  fail "RolloutPolicy CRD was not rendered with the feature gate on"
