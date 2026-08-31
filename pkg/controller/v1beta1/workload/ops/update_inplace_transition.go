package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
)

var inPlaceImageTransitionAnnotation = constants.InferenceServiceInPlaceImageTransitionAnnotationKey

type inPlaceImageTransition struct {
	TargetImages map[string]string `json:"targetImages"`
}

func inPlaceImageTransitionFromPod(pod *corev1.Pod) (*inPlaceImageTransition, bool, bool) {
	if pod == nil || pod.Annotations == nil {
		return nil, false, false
	}
	raw, present := pod.Annotations[inPlaceImageTransitionAnnotation]
	if !present {
		return nil, false, false
	}
	var transition inPlaceImageTransition
	if raw == "" || json.Unmarshal([]byte(raw), &transition) != nil || len(transition.TargetImages) == 0 {
		return nil, true, false
	}
	for name, image := range transition.TargetImages {
		if name == "" || image == "" {
			return nil, true, false
		}
	}
	return &transition, true, true
}

func imagePatchTargets(pod *corev1.Pod, target *corev1.PodSpec) map[string]string {
	if pod == nil || target == nil {
		return nil
	}
	currentImages := make(map[string]string, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		currentImages[container.Name] = container.Image
	}
	patches := make(map[string]string)
	for _, container := range target.Containers {
		if current, found := currentImages[container.Name]; found && current != container.Image {
			patches[container.Name] = container.Image
		}
	}
	if len(patches) == 0 {
		return nil
	}
	return patches
}

// ensureInPlaceImageTransition persists the exact runtime images that must be
// observed before any image patch is issued. Pending container names survive a
// retarget, with their expected values rewritten from the current target.
func ensureInPlaceImageTransition(ctx context.Context, c client.Client, pod *corev1.Pod, target *corev1.PodSpec, patches map[string]string) (bool, error) {
	if pod == nil || target == nil {
		return false, nil
	}
	current, present, valid := inPlaceImageTransitionFromPod(pod)
	if !present && len(patches) == 0 {
		return true, nil
	}
	targetImages := make(map[string]string, len(target.Containers))
	for _, container := range target.Containers {
		targetImages[container.Name] = container.Image
	}
	desired := &inPlaceImageTransition{TargetImages: make(map[string]string)}
	if present && valid {
		for name := range current.TargetImages {
			image, found := targetImages[name]
			if !found {
				valid = false
				break
			}
			desired.TargetImages[name] = image
		}
	}
	if present && !valid {
		desired.TargetImages = maps.Clone(targetImages)
	}
	for name, image := range patches {
		desired.TargetImages[name] = image
	}
	if present && valid && maps.Equal(current.TargetImages, desired.TargetImages) {
		return true, nil
	}
	if err := patchInPlaceImageTransition(ctx, c, pod, desired); err != nil {
		return false, err
	}
	return false, nil
}

func inPlaceImageTransitionMatchesTarget(transition *inPlaceImageTransition, target *corev1.PodSpec) bool {
	if transition == nil || target == nil {
		return false
	}
	targetImages := make(map[string]string, len(target.Containers))
	for _, container := range target.Containers {
		targetImages[container.Name] = container.Image
	}
	for name, image := range transition.TargetImages {
		if targetImages[name] != image {
			return false
		}
	}
	return true
}

func inPlaceImageTransitionRuntimeMatches(pod *corev1.Pod, transition *inPlaceImageTransition) bool {
	if pod == nil || transition == nil {
		return false
	}
	runtimeImages := make(map[string]string, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		runtimeImages[status.Name] = canonicalImage(status.Image)
	}
	for name, targetImage := range transition.TargetImages {
		if runtimeImages[name] != canonicalImage(targetImage) {
			return false
		}
	}
	return true
}

func patchInPlaceImageTransition(ctx context.Context, c client.Client, pod *corev1.Pod, transition *inPlaceImageTransition) error {
	raw, err := json.Marshal(transition)
	if err != nil {
		return fmt.Errorf("marshal in-place image transition for pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	base := pod.DeepCopy()
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[inPlaceImageTransitionAnnotation] = string(raw)
	patch := client.StrategicMergeFrom(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, pod, patch); err != nil {
		return fmt.Errorf("patch in-place image transition for pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	return nil
}

func removeInPlaceImageTransition(ctx context.Context, c client.Client, pod *corev1.Pod) error {
	if pod == nil || pod.Annotations == nil {
		return nil
	}
	if _, present := pod.Annotations[inPlaceImageTransitionAnnotation]; !present {
		return nil
	}
	base := pod.DeepCopy()
	delete(pod.Annotations, inPlaceImageTransitionAnnotation)
	patch := client.StrategicMergeFrom(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, pod, patch); err != nil {
		return fmt.Errorf("remove in-place image transition from pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	return nil
}
