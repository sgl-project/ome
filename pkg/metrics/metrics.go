package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Training related metrics vectors
// `baseModelName` (one of dimension used below) refers to the base model CR Name, not the exact/internal base model name
var (
	TrainingJobFailureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "training_job_failure_total",
			Help: "Total number of failed TrainingJob",
		},
		[]string{"dacName", "baseModelName", "ftModelName", "failedReason"},
	)

	TrainingJobSuccessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "training_job_success_total",
			Help: "Total number of successful TrainingJob",
		},
		[]string{"dacName", "baseModelName", "ftModelName"},
	)

	TrainingJobTimeDurationMins = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "training_job_time_duration_mins",
			Help: "Time duration of TrainingJob in unit of minutes",
		},
		[]string{"dacName", "baseModelName", "trainingStatus", "ftModelName"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		TrainingJobSuccessTotal,
		TrainingJobFailureTotal,
		TrainingJobTimeDurationMins)
}
