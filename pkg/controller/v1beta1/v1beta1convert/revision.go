package v1beta1convert

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/revision"
)

// PodTemplates is what a replica's runners render per Instance: the single
// or leader pod template with its metadata, the worker template and the
// worker count. The webhook holds stored runners to the {default} and
// {leader, worker} shapes; rendered runners take the same shapes.
type PodTemplates struct {
	Primary    *corev1.PodSpec
	Meta       *metav1.ObjectMeta
	Worker     *corev1.PodSpec
	WorkerSize int32
	MultiPod   bool
}

// PodTemplatesOf reads the per-Instance pod templates off a runner list.
// The default or leader runner is the primary template; a leader or worker
// runner makes the Instance multi-pod; the worker count is the first worker
// runner's size.
func PodTemplatesOf(runners []v1beta1.Runner) PodTemplates {
	var out PodTemplates
	for i := range runners {
		runner := &runners[i]
		switch runner.Name {
		case v1beta1.RunnerNameDefault:
			spec := runner.Template.Spec
			out.Primary = &spec
			meta := runner.Template.ObjectMeta
			out.Meta = &meta
		case v1beta1.RunnerNameLeader:
			spec := runner.Template.Spec
			out.Primary = &spec
			meta := runner.Template.ObjectMeta
			out.Meta = &meta
			out.MultiPod = true
		case v1beta1.RunnerNameWorker:
			spec := runner.Template.Spec
			out.Worker = &spec
			out.MultiPod = true
			if out.WorkerSize == 0 {
				out.WorkerSize = runner.Size
			}
		}
	}
	return out
}

// RevisionTemplateOf is what a replica's revision hashes: the pod templates,
// the template metadata without the excluded annotation keys (which the
// rendered pods keep), the topology key, the pairing protocol and the worker
// count.
func RevisionTemplateOf(templates PodTemplates, topologyKey, pairingProtocol string, excluded map[string]struct{}) revision.Template {
	return revision.Template{
		PodSpec:         templates.Primary,
		WorkerPodSpec:   templates.Worker,
		Meta:            StripExcludedAnnotations(templates.Meta, excluded),
		TopologyKey:     topologyKey,
		PairingProtocol: pairingProtocol,
		WorkerSize:      templates.WorkerSize,
	}
}

// TemplateHash is the identity of the pod templates a replica's spec holds:
// the revision template read off the spec and the given runners, hashed with
// no collision count and no owner scope. It moves exactly when the spec asks
// for a new revision and stays put under a change the revision leaves out.
// A runner list with no pod template hashes to "".
func TemplateHash(ir *v1beta1.InferenceReplica, runners []v1beta1.Runner) string {
	var topologyKey, pairingProtocol string
	if ir.Spec.TopologyKey != nil {
		topologyKey = *ir.Spec.TopologyKey
	}
	if ir.Spec.PairingProtocol != nil {
		pairingProtocol = *ir.Spec.PairingProtocol
	}
	tmpl := RevisionTemplateOf(PodTemplatesOf(runners), topologyKey, pairingProtocol, ExcludedAnnotationKeys(ir))
	hash, _, err := revision.HashTemplate(tmpl, nil, "")
	if err != nil {
		return ""
	}
	return hash
}

// ExcludedAnnotationKeys is the set of inherited InferenceService annotation
// keys the projector recorded on a replica as outside its revision; nil when
// it recorded none.
func ExcludedAnnotationKeys(ir *v1beta1.InferenceReplica) map[string]struct{} {
	raw := ir.Annotations[constants.RevisionExcludedAnnotationKeysAnnotationKey]
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

// StripExcludedAnnotations returns meta without the excluded annotation keys
// and never mutates the input; the same pointer comes back when there is
// nothing to drop.
func StripExcludedAnnotations(meta *metav1.ObjectMeta, excluded map[string]struct{}) *metav1.ObjectMeta {
	if meta == nil || len(excluded) == 0 || len(meta.Annotations) == 0 {
		return meta
	}
	filtered := make(map[string]string, len(meta.Annotations))
	for k, v := range meta.Annotations {
		if _, drop := excluded[k]; drop {
			continue
		}
		filtered[k] = v
	}
	if len(filtered) == len(meta.Annotations) {
		return meta
	}
	out := *meta
	out.Annotations = filtered
	return &out
}
