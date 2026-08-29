#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

"${host_kubectl[@]}" delete namespace "${host_namespace}" \
  --ignore-not-found --wait=true
