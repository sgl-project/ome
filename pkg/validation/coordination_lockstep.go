// RollingUpdate lockstep admission compare (spec.rollout.groups[]).
//
// A rollingUpdate group promises its Components bump together. The update-time
// check therefore has to detect ANY change that mints a new workload revision
// for a Component — not just an image bump. This file holds the
// revision-affecting view of a Component's ISVC spec and the change predicate
// validateRollingUpdateLockstep consumes.
package validation

import (
	"k8s.io/apimachinery/pkg/api/equality"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// componentRevisionSpec is the subset of one Component's ISVC spec that shapes
// its rendered pod template — and therefore its workload revision hash. The
// pod spec, runner/leader/worker overrides, gang topology key, router config,
// and the pod-template labels/annotations all land on the rendered template;
// changing any of them rolls the Component exactly like an image bump.
//
// Scaling, pacing, and autoscaling fields (replica bounds, timeout, PDB
// budgets, deployment strategy, lifecycle, autoscaler, service port
// protocols) are deliberately absent: they change behavior without
// re-rendering pods, so a one-sided change to them must not trip lockstep.
type componentRevisionSpec struct {
	PodSpec     *v1beta1.PodSpec
	Runner      *v1beta1.RunnerSpec
	Leader      *v1beta1.LeaderSpec
	Worker      *v1beta1.WorkerSpec
	TopologyKey *string
	Config      map[string]string
	Labels      map[string]string
	Annotations map[string]string
}

// componentRevisionSpecFor extracts the revision-affecting view for one
// Component. nil when the Component is not declared on the spec, so
// add/remove of a whole Component also reads as a change.
func componentRevisionSpecFor(spec *v1beta1.InferenceServiceSpec, c v1beta1.ComponentType) *componentRevisionSpec {
	if spec == nil {
		return nil
	}
	switch c {
	case v1beta1.EngineComponent:
		if e := spec.Engine; e != nil {
			return &componentRevisionSpec{
				PodSpec:     &e.PodSpec,
				Runner:      e.Runner,
				Leader:      e.Leader,
				Worker:      e.Worker,
				TopologyKey: e.TopologyKey,
				Labels:      e.Labels,
				Annotations: e.Annotations,
			}
		}
	case v1beta1.DecoderComponent:
		if d := spec.Decoder; d != nil {
			return &componentRevisionSpec{
				PodSpec:     &d.PodSpec,
				Runner:      d.Runner,
				Leader:      d.Leader,
				Worker:      d.Worker,
				TopologyKey: d.TopologyKey,
				Labels:      d.Labels,
				Annotations: d.Annotations,
			}
		}
	case v1beta1.RouterComponent:
		if r := spec.Router; r != nil {
			return &componentRevisionSpec{
				PodSpec:     &r.PodSpec,
				Runner:      r.Runner,
				Config:      r.Config,
				Labels:      r.Labels,
				Annotations: r.Annotations,
			}
		}
	}
	return nil
}

// componentRevisionSpecChanged reports whether the Component's
// revision-affecting spec differs between the old and new ISVC specs.
// Semantic equality so resource quantities compare by value ("1" == "1000m").
func componentRevisionSpecChanged(oldSpec, newSpec *v1beta1.InferenceServiceSpec, c v1beta1.ComponentType) bool {
	return !equality.Semantic.DeepEqual(
		componentRevisionSpecFor(oldSpec, c),
		componentRevisionSpecFor(newSpec, c))
}
