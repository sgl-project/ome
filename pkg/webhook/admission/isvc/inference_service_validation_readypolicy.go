package isvc

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// validateMultiPodReadyPolicyNone rejects writes that put
// lifecycle.readyPolicy=None on a Component whose effective shape at
// admission time is multi-pod OMENative. Instance readiness for those
// Components is always the AllPodReady aggregation — per-pod readiness
// reporting is not supported — so admitting None would silently behave
// like AllPodReady. Single-pod Components keep accepting None (identical
// to AllPodReady at size 1), and shapes that only become multi-pod
// through runtime resolution are not judged here: the check considers
// only the Leader+Worker pair the InferenceService itself declares.
//
// The check ratchets on UPDATE: a Component is rejected only when the
// violation is newly introduced by this write. A stored object already
// violating for that Component keeps admitting writes so it continues to
// reconcile. A nil oldIsvc (CREATE) rejects every violating Component.
func validateMultiPodReadyPolicyNone(oldIsvc, isvc *v1beta1.InferenceService) error {
	violations := multiPodReadyPolicyNoneComponents(isvc)
	if len(violations) == 0 {
		return nil
	}
	tolerated := map[string]bool{}
	if oldIsvc != nil {
		for _, name := range multiPodReadyPolicyNoneComponents(oldIsvc) {
			tolerated[name] = true
		}
	}
	for _, name := range violations {
		if tolerated[name] {
			continue
		}
		return fmt.Errorf(
			"%s.lifecycle.readyPolicy %q is not allowed on a multi-pod OMENative component: per-pod readiness reporting is not yet supported; set readyPolicy to %q or remove the field",
			name, v1beta1.InstanceReadyPolicyNone, v1beta1.InstanceReadyPolicyAllPodReady)
	}
	return nil
}

// multiPodReadyPolicyNoneComponents returns the names of Components that
// set lifecycle.readyPolicy=None while resolving to a multi-pod OMENative
// shape from the InferenceService spec alone (a declared Leader+Worker
// pair with positive Worker.Size). It applies the same shape and mode
// resolution as the lifecycle defaulter, so the set of Components
// rejected here is exactly the set the defaulter would default to
// AllPodReady. Router is always single-pod and never reported.
func multiPodReadyPolicyNoneComponents(isvc *v1beta1.InferenceService) []string {
	mode := effectiveDeploymentModeForValidation(isvc)
	var names []string
	if isvc.Spec.Engine != nil && engineIsMultiPod(isvc.Spec.Engine) &&
		componentReadyPolicyIsNone(&isvc.Spec.Engine.ComponentExtensionSpec) &&
		componentResolvesToOMENative(isvc.Spec.Engine.Annotations, mode) {
		names = append(names, "engine")
	}
	if isvc.Spec.Decoder != nil && decoderIsMultiPod(isvc.Spec.Decoder) &&
		componentReadyPolicyIsNone(&isvc.Spec.Decoder.ComponentExtensionSpec) &&
		componentResolvesToOMENative(isvc.Spec.Decoder.Annotations, mode) {
		names = append(names, "decoder")
	}
	return names
}

// componentReadyPolicyIsNone reports whether the Component explicitly
// sets lifecycle.readyPolicy=None.
func componentReadyPolicyIsNone(ext *v1beta1.ComponentExtensionSpec) bool {
	return ext != nil && ext.Lifecycle != nil && ext.Lifecycle.ReadyPolicy != nil &&
		*ext.Lifecycle.ReadyPolicy == v1beta1.InstanceReadyPolicyNone
}

// effectiveDeploymentModeForValidation resolves the deployment mode the
// same way the mutating webhook does: the canonical top-level
// ome.io/deploymentMode annotation when present, then the structural
// heuristics (Engine+Decoder ⇒ PDDisaggregated; Engine Leader+Worker
// with positive Size ⇒ OMENative), then the typed spec.deploymentMode.
// Replicating the heuristics keeps validation correct when an object
// reaches the validator without the defaulter having stamped the
// annotation.
func effectiveDeploymentModeForValidation(isvc *v1beta1.InferenceService) *constants.DeploymentModeType {
	if m := isvc.Annotations[constants.DeploymentMode]; m != "" {
		mm := constants.DeploymentModeType(m)
		return &mm
	}
	if isvc.Spec.Engine != nil && isvc.Spec.Decoder != nil {
		mm := constants.PDDisaggregated
		return &mm
	}
	if engineIsMultiPod(isvc.Spec.Engine) {
		mm := constants.OMENative
		return &mm
	}
	return isvc.Spec.DeploymentMode
}
