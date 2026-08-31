// Package workload is the source-agnostic per-Component reconciler
// pipeline. It drives the lifecycle of one logical workload — Plan →
// Render → Revision → Ops → Aggregate — without knowing whether the
// workload's spec lives on an InferenceService Component or on an
// InferenceReplica object.
//
// Boundary contract: the workload package owns its own status / phase /
// operation / key types and does NOT import the OME CRD API package
// (`pkg/apis/ome/v1beta1`) from any file exported to callers. Adapters
// at the edge convert between the owner-CRD shape and the
// workload-owned types.
//
// Entry points:
//   - workload.Reconcile (reconcile.go) — the per-Component dispatch
//     loop. Drives Delete / Restart / Migrate / Update / Create.
//   - workload.AggregateStatus (status_aggregate.go) — builds the
//     per-Component aggregate status (counts, conditions, traffic).
//   - workload/gang — per-Instance PodGroup lifecycle for multi-pod
//     Components. Wired before ops.Create so the gang is announced to
//     the scheduler before the first pod lands.
//
// Callers populate a workload.ReconcileInput (input.go) — identity,
// projected DesiredSpec / ObservedState, and callback closures
// (MutateInstance, RemoveInstance, WriteAggregateCondition,
// WarnInstanceFailed) — and hand it to the
// entry point. Workload code reads ReconcileInput and never reaches
// back into the caller's types.
//
// Cross-Component coordination gate decisions reach the dispatcher
// via the UpdateGate callback on ReconcileInput; the workload package
// never imports
// `inferenceservice/reconcilers/omenative/coordination/`.
package workload
