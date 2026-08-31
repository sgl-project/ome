// Package coordination implements cross-Component rollout coordination
// for OMENative-managed InferenceServices.
//
// The coordination layer plugs in after the per-Component reconciler
// runs and adds:
//
//   - group resolution from spec.rolloutCoordination.groups[]
//   - per-policy state machines (BlueGreen, Independent, RollingUpdate,
//     Sequential)
//   - RatioBalanced pacing math
//   - per-revision Services (`<isvc>-<component>-rev-<hash>` and the
//     headless variant)
//   - generation-scoped peer-discovery env injection
//   - the Status.Components.<c>.Traffic[] producer the consumer-side
//     HTTPRoute builder reads
//
// All persistence is on the parent InferenceService (per-group status
// in InferenceServiceStatus.RolloutCoordination, per-Component Traffic
// entries in Status.Components.<c>.Traffic). No new CRDs.
package coordination
