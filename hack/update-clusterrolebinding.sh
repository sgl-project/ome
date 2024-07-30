#!/bin/bash

CONFIG_DIR=$1

set -o errexit
set -o nounset
set -o pipefail

TMPFILE="/tmp/tmp_$$"

if [ ! -d "${TMPFILE}" ] 
then
    mkdir ${TMPFILE}
fi

kubectl kustomize ${CONFIG_DIR} -o ${TMPFILE}
for i in ${TMPFILE}/rbac.authorization.k8s.io*clusterrolebinding*.yaml; do kubectl auth reconcile -f $i; done
rm -rf ${TMPFILE}
