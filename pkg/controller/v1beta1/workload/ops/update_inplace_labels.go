package ops

import (
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/evidence"
)

// applyTemplateLabels moves the pod's labels toward the target revision's
// PodMeta in memory: target labels written, keys only the previous revision
// authored removed, foreign and workload-stamped labels left alone.
func applyTemplateLabels(pod *corev1.Pod, previousTargetLabels, newTargetLabels map[string]string) bool {
	return applyMetadata(&pod.Labels, evidence.LabelsDiff(pod.Labels, previousTargetLabels, newTargetLabels))
}
