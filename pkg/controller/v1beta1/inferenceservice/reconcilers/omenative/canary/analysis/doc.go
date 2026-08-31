// Package analysis implements metric-gated canary promotion: the
// first controller-side Prometheus reader in OME. It renders an AnalysisMetric's
// PromQL against the rollout's revision context, runs an instant query, selects
// the worst series value for the metric's comparison operator (maximum for
// LT/LTE, minimum for GT/GTE — a multi-series vector passes only when every
// series passes), and compares it to the metric's threshold. The
// canary reconciler turns the combined per-sample Outcome (Pass, Fail,
// Inconclusive) into a decision: advance, hold, or rollback.
//
// The package is deliberately free of Kubernetes client wiring. The caller
// resolves any bearer token from a Secret and passes it to NewQuerier, and
// populates TemplateContext from live rollout state. That keeps Querier and
// Evaluator unit-testable with a fake Querier and no API server — the whole point
// of isolating the one network-touching unit (Querier) behind an interface.
package analysis
