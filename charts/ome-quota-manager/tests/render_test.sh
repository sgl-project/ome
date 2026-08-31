#!/usr/bin/env bash
set -euo pipefail

chart_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
helm_bin="${HELM_BIN:-helm}"

fail() {
  echo "ome-quota-manager chart test: $*" >&2
  exit 1
}

# Release name deliberately unlike the component name: object names come from
# quotaManager.name, and a release-derived name would slip through unnoticed if
# the two matched.
render() {
  "${helm_bin}" template rel "${chart_dir}" --namespace ome "$@"
}

# --- mode is required, and the guard must fire rather than render a bad flag ---

if render >/dev/null 2>&1; then
  fail "an unset mode rendered instead of failing"
fi
for mode in workload management; do
  render --set quotaManager.mode="${mode}" >/dev/null ||
    fail "mode ${mode} did not render"
done
if render --set quotaManager.mode=both >/dev/null 2>&1; then
  fail "an invalid mode rendered instead of failing"
fi

# --- object names track quotaManager.name, never the release ---

default="$(render --set quotaManager.mode=workload)"

grep -Fq 'name: ome-quota-manager' <<<"${default}" ||
  fail "objects are not named from quotaManager.name"
if grep -Eq '^\s+name: rel(-|$)' <<<"${default}"; then
  fail "an object was named from the release instead of quotaManager.name"
fi

renamed="$(render --set quotaManager.mode=workload --set quotaManager.name=aq-mgr)"
grep -Fq 'name: aq-mgr' <<<"${renamed}" ||
  fail "overriding quotaManager.name did not rename objects"
# The RBAC scopes itself by name, so a rename that misses one grant would leave
# the component unable to inject its own caBundle.
grep -Fq 'resourceNames: ["aq-mgr.ome.io"]' <<<"${renamed}" ||
  fail "the webhook-config grant did not follow the rename"
grep -Fq 'resourceNames: ["aq-mgr-webhook-cert"]' <<<"${renamed}" ||
  fail "the secret grant did not follow the rename"
grep -Fq -- '--webhook-config-name=aq-mgr.ome.io' <<<"${renamed}" ||
  fail "the binary flag did not follow the rename"

# --- internal cert management: the out-of-the-box default ---

grep -Fq -- '--internal-cert-management=true' <<<"${default}" ||
  fail "internal cert management is not the default"
grep -Fq -- '--cert-namespace=ome' <<<"${default}" ||
  fail "the cert namespace was not passed"
grep -Fq -- '--cert-secret-name=ome-quota-manager-webhook-cert' <<<"${default}" ||
  fail "the cert secret name was not passed"
grep -Fq -- '--webhook-config-name=ome-quota-manager.ome.io' <<<"${default}" ||
  fail "the webhook config name was not passed"

# The rotator only ever Updates the Secret, so the chart creating it empty is
# load-bearing: without it the component never becomes ready.
grep -Fq 'name: ome-quota-manager-webhook-cert' <<<"${default}" ||
  fail "the placeholder cert Secret was not rendered"
grep -Fq 'helm.sh/resource-policy: keep' <<<"${default}" ||
  fail "the cert Secret is not retained across an uninstall"

# The rotator writes the Secret and the kubelet projects it back down, so the
# mount is the delivery path, not just the cert-manager hand-off.
grep -Fq 'secretName: ome-quota-manager-webhook-cert' <<<"${default}" ||
  fail "the cert Secret is not mounted"

if grep -Fq 'cert-manager.io' <<<"${default}"; then
  fail "the default install still references cert-manager"
fi

# Self-injection is scoped to this component's own webhook config. A grant that
# lost its resourceNames could rewrite ome-manager's fail-closed pod mutator.
grep -Fq 'resourceNames: ["ome-quota-manager.ome.io"]' <<<"${default}" ||
  fail "webhook-config update is not scoped by name"
if grep -Fq 'resources: ["mutatingwebhookconfigurations"]' <<<"${default}"; then
  fail "the quota plane was granted access to mutating webhooks"
fi
grep -Fq 'resourceNames: ["ome-quota-manager-webhook-cert"]' <<<"${default}" ||
  fail "secret update is not scoped by name"

# --- capacity derivation carries its own RBAC, and only in workload mode ---

# The grants and the flags have to appear together: flags without the grants is
# a crash-looping RBAC error, grants without the flags is a cluster-wide Node
# read nothing uses.
grep -Fq 'resources: ["nodes"]' <<<"${default}" ||
  fail "workload mode did not grant the Node read capacity derivation needs"
grep -Fq 'resources: ["resourceflavors"]' <<<"${default}" ||
  fail "workload mode did not grant the ResourceFlavor read"
grep -Fq -- '--accelerator-resources=google.com/tpu,nvidia.com/gpu' <<<"${default}" ||
  fail "the accelerator suffixes were not passed"
grep -Fq -- '--capacity-hysteresis-percent=10' <<<"${default}" ||
  fail "the hysteresis band was not passed"

# A management-mode install holds the authored fleet tree and has no local
# silicon, so a cluster-wide Node read there would be unused privilege.
management="$(render --set quotaManager.mode=management)"
for unwanted in 'resources: ["nodes"]' 'resources: ["resourceflavors"]' '--accelerator-resources'; do
  if grep -Fq -- "${unwanted}" <<<"${management}"; then
    fail "management mode rendered ${unwanted}"
  fi
done

# Turning derivation off must drop the grants with it, or the privilege
# outlives the feature that justified it. Every spelling an operator might
# reach for, because Helm deletes a defaulted map on a null rather than merging
# it and an unguarded lookup then aborts the whole render.
for off in \
  'quotaManager.capacity.resources=null' \
  'quotaManager.capacity=null' \
  'quotaManager.capacity.resources={}' \
  ; do
  nocapacity="$(render --set quotaManager.mode=workload --set "${off}")" ||
    fail "--set ${off} failed to render at all"
  for unwanted in 'resources: ["nodes"]' 'resources: ["resourceflavors"]' '--accelerator-resources'; do
    if grep -Fq -- "${unwanted}" <<<"${nocapacity}"; then
      fail "--set ${off} still rendered ${unwanted}"
    fi
  done
done

# A blank entry is not a resource. The chart must agree with the binary, which
# trims and drops blanks — otherwise a list of blanks grants cluster-wide Node
# reads for a feature the binary treats as switched off.
blanks="$(render --set quotaManager.mode=workload --set 'quotaManager.capacity.resources[0]=')"
if grep -Fq -- 'resources: ["nodes"]' <<<"${blanks}"; then
  fail "a list of blank resources still granted the Node read"
fi

# A flag rendered with no value crash-loops the pod on parse.
allrendered="$(render --set quotaManager.mode=workload --set quotaManager.capacity.hysteresisPercent=null)"
if grep -Eq -- '--[a-z-]+=(")?$' <<<"${allrendered}"; then
  fail "a flag rendered with an empty value"
fi

# --- materialization carries its own Kueue write RBAC, workload mode only ---

# Flags and grants have to appear together: flags without the grants is a
# crash-looping RBAC error, grants without the flags is cluster-wide write on
# Kueue that nothing uses.
grep -Fq -- '--cover-resources=cpu=16M,memory=16Pi' <<<"${default}" ||
  fail "the default cover resources were not passed"

# An operator overrides one key and keeps the other: Helm merges the map rather
# than replacing it, and the whole point of a default is that omitting a key
# leaves it alone.
onekey="$(render --set quotaManager.mode=workload \
  --set quotaManager.materialize.coverResources.cpu=2k)"
grep -Fq -- '--cover-resources=cpu=2k,memory=16Pi' <<<"${onekey}" ||
  fail "overriding one cover resource did not leave the other at its default"

# A Cohort's subtree quota sums the cover across its children, and the Kueue this
# repo compiles against wraps rather than clamps on int64 overflow. Ei-scale
# defaults leave no room for that sum, and a wrapped ceiling is a wrong admission
# decision rather than a failure.
if grep -Eq -- '--cover-resources=[^ ]*[0-9]+(Ei|E)([,"]|$)' <<<"${default}"; then
  fail "the default cover resources reach Ei/E scale, leaving no room for a cohort subtree sum"
fi
grep -Fq -- '--field-manager=ome-quota-manager' <<<"${default}" ||
  fail "the field manager did not default to the component name"
grep -Fq 'resources: ["cohorts", "clusterqueues", "localqueues"]' <<<"${default}" ||
  fail "workload mode did not grant the Kueue write materialization needs"
# patch on the resource, not update on the finalizers subresource: a finalizer
# is metadata, so claiming a node is an ordinary patch. Granting the subresource
# instead leaves every reconcile failing on a forbidden patch, which is invisible
# to a render test that only checks a grant is present.
grep -Fq 'verbs: ["patch"]' <<<"${default}" ||
  fail "the finalizer patch grant is missing, so no node could be claimed or reaped"
if grep -Fq 'resources: ["acceleratorquotas/finalizers"]' <<<"${default}"; then
  fail "granted the finalizers subresource, which governs blockOwnerDeletion and is not what a finalizer needs"
fi

# A rename must carry the field manager with it, or two installs would claim
# each other's objects.
grep -Fq -- '--field-manager=aq-mgr' <<<"${renamed}" ||
  fail "the field manager did not follow the rename"

# A management-mode install writes no Kueue object anywhere, so the write grant
# there would be pure unused privilege.
for unwanted in 'resources: ["cohorts", "clusterqueues", "localqueues"]' '--cover-resources' '--field-manager'; do
  if grep -Fq -- "${unwanted}" <<<"${management}"; then
    fail "management mode rendered ${unwanted}"
  fi
done

# Turning materialization off must drop the write grant with it, in every
# spelling an operator might reach for.
for off in \
  'quotaManager.materialize.coverResources=null' \
  'quotaManager.materialize=null' \
  ; do
  nomat="$(render --set quotaManager.mode=workload --set "${off}")" ||
    fail "--set ${off} failed to render at all"
  for unwanted in 'resources: ["cohorts", "clusterqueues", "localqueues"]' '--cover-resources' \
    'verbs: ["patch"]'; do
    if grep -Fq -- "${unwanted}" <<<"${nomat}"; then
      fail "--set ${off} still rendered ${unwanted}"
    fi
  done
done

# A blank quantity is not a cover resource. The chart must agree with the
# binary, which rejects a valueless pair at startup.
blankqty="$(render --set quotaManager.mode=workload \
  --set quotaManager.materialize.coverResources.cpu= \
  --set quotaManager.materialize.coverResources.memory=)"
if grep -Fq -- 'resources: ["cohorts", "clusterqueues", "localqueues"]' <<<"${blankqty}"; then
  fail "blank cover quantities still granted the Kueue write"
fi

# --- the CRD is opt-in, and retained when it is opted into ---

if grep -Fq 'kind: CustomResourceDefinition' <<<"${default}"; then
  fail "the CRD rendered by default, which collides with ome-crd"
fi

withcrd="$(render --set quotaManager.mode=workload --set quotaManager.crd.install=true)"
grep -Fq 'name: acceleratorquotas.ome.io' <<<"${withcrd}" ||
  fail "opting in did not render the CRD"
# Deleting the CRD cascades to every AcceleratorQuota, and each of those deletes
# is gated by this chart's own fail-closed webhook — which by uninstall time has
# no endpoints. Without the retention annotation the CRD wedges in Terminating.
"${helm_bin}" template rel "${chart_dir}" --namespace ome \
  --set quotaManager.mode=workload --set quotaManager.crd.install=true \
  --show-only templates/crd.yaml | grep -Fq 'helm.sh/resource-policy: keep' ||
  fail "the opted-in CRD is not retained on uninstall"

# --- cert-manager remains available as an opt-out ---

certmanager="$(render --set quotaManager.mode=workload \
  --set quotaManager.webhook.internalCertManagement=false)"

grep -Fq 'kind: Certificate' <<<"${certmanager}" ||
  fail "opting out did not render a cert-manager Certificate"
grep -Fq 'cert-manager.io/inject-ca-from: ome/ome-quota-manager-serving-cert' <<<"${certmanager}" ||
  fail "opting out did not annotate the webhook config for cainjector"
grep -Fq 'secretName: ome-quota-manager-webhook-cert' <<<"${certmanager}" ||
  fail "opting out did not mount the cert Secret"
if grep -Fq 'resources: ["validatingwebhookconfigurations"]' <<<"${certmanager}"; then
  fail "opting out left the self-injection RBAC in place"
fi

# --- disabling the webhook drops the whole cert story ---

nowebhook="$(render --set quotaManager.mode=workload \
  --set quotaManager.webhook.enabled=false)"

for unwanted in ValidatingWebhookConfiguration cert-manager.io ome-quota-manager-webhook-cert; do
  if grep -Fq "${unwanted}" <<<"${nowebhook}"; then
    fail "disabling the webhook still rendered ${unwanted}"
  fi
done

# --- reaching the fleet is a management-mode grant only ---

mgmt="$(render --set quotaManager.mode=management --show-only templates/rbac.yaml)"
for wanted in '"workloadclusters"' '"secrets"'; do
  grep -Fq "${wanted}" <<<"${mgmt}" ||
    fail "management mode cannot reach the fleet: ${wanted} not granted"
done
# The registry has one writer, in ome-manager, and its grace state is in memory.
# A second writer flaps the condition and loses silently.
if grep -Fq 'workloadclusters/status' <<<"${mgmt}"; then
  fail "management mode grants WorkloadCluster status, which belongs to the registry"
fi

work="$(render --set quotaManager.mode=workload --show-only templates/rbac.yaml)"
if grep -Fq '"workloadclusters"' <<<"${work}"; then
  fail "workload mode was granted the registry it never reads"
fi

# --- a projecting plane can claim the nodes it projects ---

# The projector puts a finalizer on every node it claims, which is an ordinary
# patch of the object. Without the grant each pass fails forbidden before
# anything is projected, and the fleet never leaves its authored state.
mgmtProject="$(render --set quotaManager.mode=management \
  --set quotaManager.projection.origin=hub-1 \
  --set quotaManager.projection.fieldManager=proj \
  --show-only templates/rbac.yaml)"

# One rule spans two lines, and a newline inside a grep pattern makes it two
# alternative patterns rather than one contiguous match — which would pass on
# the resource name alone, whatever the verbs beside it. Flatten instead.
flat() { tr '\n' ' ' | tr -s ' '; }
claim='resources: ["acceleratorquotas"] verbs: ["patch"]'

grep -Fq "${claim}" <<<"$(flat <<<"${mgmtProject}")" ||
  fail "a projecting management plane cannot claim a node with a finalizer"

# ...and one that is not projecting carries no write it cannot use.
if grep -Fq "${claim}" <<<"$(flat <<<"${mgmt}")"; then
  fail "management mode grants a spec write with no projector to use it"
fi

# --- projection is off until it is named, and never leaks into workload mode ---

projectOn="$(render --set quotaManager.mode=management \
  --set quotaManager.projection.origin=hub-1 \
  --set quotaManager.projection.fieldManager=proj \
  --set quotaManager.projection.defaultDistributionPolicy=Proportional \
  --show-only templates/deployment.yaml)"
# Counted, not merely found. A duplicated template block renders every flag
# twice, which Go's flag parsing tolerates -- last wins -- so nothing downstream
# would complain and a `grep -q` would pass while the args list carried each
# projection flag two times over.
for wanted in \
  '--projection-origin=hub-1' \
  '--projection-field-manager=proj' \
  '--default-distribution-policy=Proportional'; do
  seen="$(grep -Fc -- "${wanted}" <<<"${projectOn}" || true)"
  [ "${seen}" = 1 ] ||
    fail "management mode rendered ${wanted} ${seen} times, want exactly 1"
done

# An unmarked copy cannot be told from a node an admin authored on the member,
# so projection must stay off until an origin is chosen deliberately.
bare="$(render --set quotaManager.mode=management --show-only templates/deployment.yaml)"
if grep -Fq -- '--projection-origin' <<<"${bare}"; then
  fail "projection turned itself on without an origin"
fi

# A workload-mode manager holds no transport at all; the flags would be inert
# at best and misleading at worst.
leak="$(render --set quotaManager.mode=workload \
  --set quotaManager.projection.origin=hub-1 --show-only templates/deployment.yaml)"
if grep -Fq -- '--projection-origin' <<<"${leak}"; then
  fail "projection flags leaked into workload mode"
fi

# Exec is opt-in, and naming no command still permits none.
if grep -Fq -- '--allow-exec-credentials' <<<"${projectOn}"; then
  fail "exec credentials were enabled without being asked for"
fi
# --- remote access is off unless asked for, grants narrowly, and mints no
# --- non-expiring credential unless explicitly told to ---

for unwanted in ome-quota-access quota-remote-access; do
  if grep -Fq "${unwanted}" <<<"${default}"; then
    fail "remote access rendered without being enabled (${unwanted})"
  fi
done

# Only this template, so the local manager's own grants cannot be mistaken for
# the remote ones. Comments are stripped: the file explains at length which
# grants it deliberately omits, and prose about a rule must not read as the rule.
remote_only() {
  render --set quotaManager.mode=workload \
    --set quotaManager.remoteAccess.enabled=true "$@" \
    --show-only templates/remote_access.yaml | grep -v '^[[:space:]]*#'
}

remote="$(remote_only)"

for wanted in \
  'name: "ome-quota-access"' \
  'resources: ["acceleratorquotas"]'; do
  grep -Fq "${wanted}" <<<"${remote}" ||
    fail "enabling remote access did not render ${wanted}"
done

# A token that never expires is the thing this chart should not hand out by
# default. It is available for simulators, and only when asked for twice.
if grep -Fq 'service-account-token' <<<"${remote}"; then
  fail "enabling remote access minted a non-expiring token without being asked"
fi
grep -Fq 'service-account-token' \
  <<<"$(remote_only --set quotaManager.remoteAccess.serviceAccount.staticToken=true)" ||
  fail "opting into staticToken did not mint the token Secret"

# The role must be bindable to an identity the platform already rotates, with
# no in-cluster ServiceAccount at all.
external="$(remote_only \
  --set quotaManager.remoteAccess.serviceAccount.create=false \
  --set 'quotaManager.remoteAccess.subjects[0].kind=User' \
  --set 'quotaManager.remoteAccess.subjects[0].name=projector@example.com')"
grep -Fq 'kind: "User"' <<<"${external}" ||
  fail "an external subject was not bound"
if grep -Fq 'kind: ServiceAccount' <<<"${external}"; then
  fail "serviceAccount.create=false still rendered a ServiceAccount"
fi

# A role bound to nobody is a silent no-op, so it must not render at all.
if render --set quotaManager.mode=workload \
  --set quotaManager.remoteAccess.enabled=true \
  --set quotaManager.remoteAccess.serviceAccount.create=false >/dev/null 2>&1; then
  fail "remote access bound to nobody rendered instead of failing"
fi

# The whole point of a separate credential is that it can only write budgets.
# A grant that let it create workloads, read the fleet's hardware, or touch
# Kueue would make it interchangeable with ome-multicluster-access, and a leak
# of either would then cost the same as a leak of both.
#
# Status on a member belongs to the workload-mode manager running there; a
# remote writer cannot see what it would be overwriting.
for forbidden in \
  'acceleratorquotas/status' \
  '"nodes"' \
  '"pods"' \
  '"inferenceservices"' \
  'kueue.x-k8s.io'; do
  if grep -Fq "${forbidden}" <<<"${remote}"; then
    fail "the remote role reaches beyond projecting quota: ${forbidden}"
  fi
done

echo "ome-quota-manager chart contracts OK"
