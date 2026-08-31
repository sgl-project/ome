package podmonitor

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// RelabelingsFromConfig converts the JSON-friendly config relabelings into the
// Prometheus-operator type used on PodMetricsEndpoint.Relabelings. Returns nil
// for an empty input so callers can append unconditionally.
func RelabelingsFromConfig(in []controllerconfig.RelabelConfig) []monitoringv1.RelabelConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]monitoringv1.RelabelConfig, 0, len(in))
	for _, rc := range in {
		mrc := monitoringv1.RelabelConfig{
			TargetLabel: rc.TargetLabel,
			Regex:       rc.Regex,
			Action:      rc.Action,
		}
		if len(rc.SourceLabels) > 0 {
			src := make([]monitoringv1.LabelName, 0, len(rc.SourceLabels))
			for _, s := range rc.SourceLabels {
				src = append(src, monitoringv1.LabelName(s))
			}
			mrc.SourceLabels = src
		}
		if rc.Separator != "" {
			sep := rc.Separator
			mrc.Separator = &sep
		}
		if rc.Replacement != "" {
			repl := rc.Replacement
			mrc.Replacement = &repl
		}
		out = append(out, mrc)
	}
	return out
}

// ApplyManagedScrapeConfig stamps the cluster-scope PodMonitor defaults onto a
// PodMonitor OME is about to create/update:
//   - cfg.Labels are merged into metadata.labels (so a label-selecting collector
//     such as a label-selecting target allocator will scrape it). They are intentionally
//     NOT added to spec.selector, which must keep selecting pods by the
//     component's own labels.
//   - cfg.Relabelings are appended to every PodMetricsEndpoint's relabelings
//     (pre-scrape) so scraped series carry the dashboard label schema
//     (inferenceservice, component, ...).
//   - cfg.MetricRelabelings are appended to every PodMetricsEndpoint's
//     metricRelabelings (post-scrape) so metric names/labels can be rewritten,
//     e.g. normalizing a router's re-exported vllm_* back to vllm:*.
//
// Existing endpoint relabelings/metricRelabelings are preserved; OME's are
// appended after them.
func ApplyManagedScrapeConfig(pm *monitoringv1.PodMonitor, cfg controllerconfig.PodMonitorConfig) {
	if pm == nil {
		return
	}
	if len(cfg.Labels) > 0 {
		if pm.Labels == nil {
			pm.Labels = make(map[string]string, len(cfg.Labels))
		}
		for k, v := range cfg.Labels {
			pm.Labels[k] = v
		}
	}
	relabelings := RelabelingsFromConfig(cfg.Relabelings)
	metricRelabelings := RelabelingsFromConfig(cfg.MetricRelabelings)
	for i := range pm.Spec.PodMetricsEndpoints {
		if len(relabelings) > 0 {
			pm.Spec.PodMetricsEndpoints[i].RelabelConfigs = append(
				pm.Spec.PodMetricsEndpoints[i].RelabelConfigs, relabelings...)
		}
		if len(metricRelabelings) > 0 {
			pm.Spec.PodMetricsEndpoints[i].MetricRelabelConfigs = append(
				pm.Spec.PodMetricsEndpoints[i].MetricRelabelConfigs, metricRelabelings...)
		}
	}
}
