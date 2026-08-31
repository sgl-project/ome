package analysis

import (
	"context"
	"errors"
	"strconv"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Outcome is the verdict for one analysis sample (all metrics evaluated once).
type Outcome int

const (
	// Pass: every metric was evaluated and satisfied its condition.
	Pass Outcome = iota
	// Fail: at least one metric was evaluated and breached its condition. A known
	// breach is definitive, so Fail takes precedence over Inconclusive.
	Fail
	// Inconclusive: nothing breached, but at least one metric could not be
	// evaluated (Prometheus unreachable, query/template error, or no data). The
	// reconciler holds and, past the stall timeout, escalates — it does NOT count
	// this toward the failure budget unless OnInconclusive is Rollback.
	Inconclusive
)

// Result is the evaluated outcome of one sample plus the per-metric detail that
// status surfaces.
type Result struct {
	Outcome Outcome
	Metrics []MetricResult
}

// MetricResult is one metric's evaluation within a sample.
type MetricResult struct {
	Name      string
	Value     string // formatted result; empty when the metric was not evaluated
	Threshold string
	Operator  v1beta1.ComparisonOperator
	Passed    bool
	Message   string // reason when not passed or not evaluated ("no data", an error)
}

// Evaluate runs every metric once via q and combines them with AND semantics and
// Fail > Inconclusive > Pass precedence. It never returns an error: a
// query/template failure becomes an Inconclusive metric, surfaced in
// MetricResult.Message, so the caller always gets a usable per-sample verdict
// rather than having to distinguish "sample failed" from "Evaluate errored."
func Evaluate(ctx context.Context, q Querier, a *v1beta1.RolloutAnalysis, tc TemplateContext) Result {
	res := Result{Outcome: Pass, Metrics: make([]MetricResult, 0, len(a.Metrics))}
	anyFail, anyInconclusive := false, false
	for i := range a.Metrics {
		m := &a.Metrics[i]
		mr := MetricResult{Name: m.Name, Threshold: m.Threshold, Operator: m.Operator}

		query, err := RenderQuery(m.Query, tc)
		if err != nil {
			mr.Message = "template error: " + err.Error()
			anyInconclusive = true
			res.Metrics = append(res.Metrics, mr)
			continue
		}
		vals, err := q.Query(ctx, query)
		if err != nil || len(vals) == 0 {
			if err == nil {
				err = ErrNoData
			}
			mr.Message = queryErrMessage(err)
			anyInconclusive = true
			res.Metrics = append(res.Metrics, mr)
			continue
		}
		threshold, perr := strconv.ParseFloat(m.Threshold, 64)
		if perr != nil {
			// Admission validates Threshold is numeric; treat a slip as inconclusive
			// rather than panicking the reconcile.
			mr.Message = "invalid threshold: " + m.Threshold
			anyInconclusive = true
			res.Metrics = append(res.Metrics, mr)
			continue
		}

		val := worstValue(vals, m.Operator)
		mr.Value = strconv.FormatFloat(val, 'g', 6, 64)
		mr.Passed = compare(val, m.Operator, threshold)
		if !mr.Passed {
			anyFail = true
		}
		res.Metrics = append(res.Metrics, mr)
	}

	switch {
	case anyFail:
		res.Outcome = Fail
	case anyInconclusive:
		res.Outcome = Inconclusive
	default:
		res.Outcome = Pass
	}
	return res
}

// worstValue selects the series value hardest to pass for op, so a multi-series
// vector passes only when EVERY series passes: the maximum for upper-bound
// conditions (LT/LTE — e.g. the pod with the highest error rate), the minimum
// for lower-bound conditions (GT/GTE — e.g. the pod with the lowest success
// rate). An unknown operator never passes compare, so its pick is immaterial;
// it takes the maximum. vals must be non-empty.
func worstValue(vals []float64, op v1beta1.ComparisonOperator) float64 {
	lowerBound := op == v1beta1.ComparisonGT || op == v1beta1.ComparisonGTE
	worst := vals[0]
	for _, v := range vals[1:] {
		if lowerBound {
			if v < worst {
				worst = v
			}
		} else if v > worst {
			worst = v
		}
	}
	return worst
}

// compare reports whether "val <op> threshold" holds, i.e. the metric is healthy.
func compare(val float64, op v1beta1.ComparisonOperator, threshold float64) bool {
	switch op {
	case v1beta1.ComparisonLT:
		return val < threshold
	case v1beta1.ComparisonLTE:
		return val <= threshold
	case v1beta1.ComparisonGT:
		return val > threshold
	case v1beta1.ComparisonGTE:
		return val >= threshold
	default:
		return false
	}
}

func queryErrMessage(err error) string {
	if errors.Is(err, ErrNoData) {
		return "no data"
	}
	return err.Error()
}
