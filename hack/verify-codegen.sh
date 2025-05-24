#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC2128
SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
DIFFROOT="${SCRIPT_ROOT}/pkg/client"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/pkg/client"
_tmp="${SCRIPT_ROOT}/_tmp"
# Cleanup script to remove tmp folder
cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT
# Running cleanup
cleanup
# Creating tmp folder to compare generated client code
mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"
# Generating client code to verify
"${SCRIPT_ROOT}/hack/update-codegen.sh"
echo "diffing ${DIFFROOT} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  echo "${DIFFROOT} up to date."
else
  echo "${DIFFROOT} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi
