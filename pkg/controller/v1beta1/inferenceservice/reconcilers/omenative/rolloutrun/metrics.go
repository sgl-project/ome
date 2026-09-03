package rolloutrun

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Per-ISVC vectors carry "namespace" and "isvc" labels so same-named ISVCs
// across namespaces do not collide; the drift gauge is dropped when its run
// state clears so cardinality stays bounded to live rollouts.
var (
	runOpenedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ome_rollout_run_opened_total",
		Help: "Count of rollout runs opened, by plan source composition and adopt-in-place.",
	}, []string{"namespace", "source"})

	runClosedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ome_rollout_run_closed_total",
		Help: "Count of rollout runs closed, by outcome.",
	}, []string{"namespace", "outcome"})

	runParkedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ome_rollout_run_parked_total",
		Help: "Count of park transitions (an unresolvable plan held a rollout fail-closed), by reason.",
	}, []string{"namespace", "reason"})

	planDrift = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ome_rollout_plan_drift",
		Help: "1 while the live render of an ISVC's rollout plan differs from its pinned run (the edit is inert until the next run or a repin).",
	}, []string{"namespace", "isvc"})

	repinTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ome_rollout_repin_total",
		Help: "Count of ome.io/rollout-repin verbs processed, by outcome (applied|rejected).",
	}, []string{"namespace", "outcome"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(runOpenedTotal, runClosedTotal, runParkedTotal, planDrift, repinTotal)
}

func recordRunOpened(isvc *v1beta1.InferenceService, plan composedPlan, adopting bool) {
	source := "inline"
	for i := range plan.groups {
		if plan.groups[i].Source == v1beta1.RolloutPlanSourcePolicy {
			source = "policy"
			break
		}
	}
	if adopting {
		source = source + "-adopted"
	}
	runOpenedTotal.WithLabelValues(isvc.Namespace, source).Inc()
}

func recordRunClosed(isvc *v1beta1.InferenceService, outcome v1beta1.RolloutRunOutcome) {
	runClosedTotal.WithLabelValues(isvc.Namespace, string(outcome)).Inc()
}

func recordParked(isvc *v1beta1.InferenceService, reason string) {
	runParkedTotal.WithLabelValues(isvc.Namespace, reason).Inc()
}

func recordDrift(isvc *v1beta1.InferenceService, drifted bool) {
	if drifted {
		planDrift.WithLabelValues(isvc.Namespace, isvc.Name).Set(1)
		return
	}
	planDrift.DeleteLabelValues(isvc.Namespace, isvc.Name)
}

func recordRepin(isvc *v1beta1.InferenceService, applied bool) {
	outcome := "rejected"
	if applied {
		outcome = "applied"
	}
	repinTotal.WithLabelValues(isvc.Namespace, outcome).Inc()
}
