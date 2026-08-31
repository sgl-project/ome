// Package omenative carries the OMENative carrier-type re-exports the
// ISVC controller and its component builders still depend on after the
// legacy OMENative direct path was retired.
//
// Pod lifecycle for OMENative-mode Components is now driven exclusively
// through the InferenceReplica path (see irprojector + the
// inferencereplica controller); the shared per-Instance state machines
// live in pkg/controller/v1beta1/workload, and the per-Component
// coordination gates live in omenative/coordination. What remains in the
// umbrella package is the Pod-watch wiring (watches.go) plus these two
// re-exports.
package omenative

import (
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/omenative/core"
)

// Expectations is the per-controller pod create/delete expectations
// cache. Re-exported from core/ so the ISVC controller + its component
// builders (components/{builder,base,engine,decoder,router}.go,
// controller.go) reference a single symbol in the umbrella package
// rather than importing core/ directly. The alias shares the underlying
// type with core/ at compile time — it is a package-API declaration,
// not a wrapper.
type Expectations = core.Expectations

// NewExpectations constructs a fresh Expectations cache. Paired with the
// Expectations carrier type above; the ISVC controller seeds one per
// reconciler (controller.go). The DefaultExpectations singleton is
// intentionally NOT re-exported here — a var re-export would copy the
// pointer at init time, and any caller that reassigned the local copy
// would silently desync from the core singleton core/params.go's
// ExpectationsCache fallback uses. Callers that need the singleton
// import core directly.
var NewExpectations = core.NewExpectations
