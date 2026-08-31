package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// canonicalImage normalizes Docker Hub references for the runtime-vs-spec
// equality compare. Container runtimes may qualify a short reference with
// docker.io while the Pod spec retains the short form.
func canonicalImage(img string) string {
	const dockerHubPrefix = "docker.io/"
	if !strings.HasPrefix(img, dockerHubPrefix) {
		return img
	}
	img = strings.TrimPrefix(img, dockerHubPrefix)
	return strings.TrimPrefix(img, "library/")
}

// podRuntimeImagesMatch is the runtime-truth signal complementing
// podImagesMatch's spec check: spec.image flips immediately on patch but
// kubelet may not have rolled the container yet, leaving the old image
// in Status.ContainerStatuses[*].Image. Only after both match has the
// in-place update actually taken effect.
func podRuntimeImagesMatch(pod *corev1.Pod, target *corev1.PodSpec) bool {
	if pod == nil || target == nil {
		return false
	}
	got := make(map[string]string, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		got[cs.Name] = canonicalImage(cs.Image)
	}
	// Every target container needs a matching status before declaring
	// done. Statuses for containers absent from target (webhook-injected
	// sidecars) are not OMENative-owned and are ignored — comparing them
	// would block convergence forever.
	for _, c := range target.Containers {
		img, ok := got[c.Name]
		if !ok || img != canonicalImage(c.Image) {
			return false
		}
	}
	return true
}

// podRuntimeImageChangesMatch requires runtime confirmation only for
// containers whose image field differs between the running and target
// revisions. ContainerStatus.Image may name any repository tag for the
// running image, so unchanged containers are not a transition signal.
func podRuntimeImageChangesMatch(pod *corev1.Pod, running, target *corev1.PodSpec) bool {
	if pod == nil || target == nil {
		return false
	}
	if running == nil {
		return podRuntimeImagesMatch(pod, target)
	}
	runningImages := make(map[string]string, len(running.Containers))
	for _, c := range running.Containers {
		runningImages[c.Name] = c.Image
	}
	changed := corev1.PodSpec{}
	for _, c := range target.Containers {
		if image, ok := runningImages[c.Name]; ok && image == c.Image {
			continue
		}
		changed.Containers = append(changed.Containers, corev1.Container{Name: c.Name, Image: c.Image})
	}
	return len(changed.Containers) == 0 || podRuntimeImagesMatch(pod, &changed)
}

// podImagesMatch returns true when every container in target has a
// same-named pod container with the same .Image. Pod containers absent
// from target (webhook-injected sidecars) are not OMENative-owned and
// are ignored — failing on them would leave the in-place update
// declaring "patch needed" forever while patchPodImages has nothing to
// patch. Target containers absent from the pod fail the match (the pod
// can never converge to that spec in place).
func podImagesMatch(pod *corev1.Pod, target *corev1.PodSpec) bool {
	if pod == nil || target == nil {
		return false
	}
	got := make(map[string]string, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		got[c.Name] = c.Image
	}
	for _, c := range target.Containers {
		img, ok := got[c.Name]
		if !ok || img != c.Image {
			return false
		}
	}
	return true
}

// patchPodImages strategic-merge-patches each changed container image with an
// optimistic resource-version precondition. Strategic merge keys by container
// name, and the precondition rejects a concurrent pod or status mutation after
// the caller's live read.
// Returns whether a patch was actually issued so callers only requeue
// for a kubelet roll when one is coming.
func patchPodImages(ctx context.Context, c client.Client, pod *corev1.Pod, target *corev1.PodSpec) (bool, error) {
	if pod == nil || target == nil {
		return false, fmt.Errorf("patchPodImages: nil pod or target")
	}
	wantByName := make(map[string]string, len(target.Containers))
	for _, t := range target.Containers {
		wantByName[t.Name] = t.Image
	}
	base := pod.DeepCopy()
	changed := false
	for i := range pod.Spec.Containers {
		current := &pod.Spec.Containers[i]
		newImage, ok := wantByName[current.Name]
		if !ok || newImage == current.Image {
			continue
		}
		current.Image = newImage
		changed = true
	}
	if !changed {
		return false, nil
	}
	patch := client.StrategicMergeFrom(base, client.MergeFromWithOptimisticLock{})
	if err := c.Patch(ctx, pod, patch); err != nil {
		return false, fmt.Errorf("patch pod %s/%s images: %w", pod.Namespace, pod.Name, err)
	}
	return true, nil
}

// patchPodRevisionHashLabel strategic-merge-patches the pod's
// ome.io/revision-hash label to targetHash. An in-place update changes a
// pod's revision (image / annotations) WITHOUT recreating it, but that
// label is stamped only at pod create time (render.go). Without restamping
// it here, an in-place-rolled pod keeps its OLD revision-hash label while
// InstanceStatus.RunningRevision advances — so per-revision Service
// routing, drain (drain.IsPodDrained), and stuck-pod detection
// (HasWedgedPodAgainstCurrent), which all key on this label, treat the
// rolled pod as the PREVIOUS revision. Returns whether a patch was issued;
// matching labels and empty target hashes are no-ops.
func patchPodRevisionHashLabel(ctx context.Context, c client.Client, pod *corev1.Pod, targetHash string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("patchPodRevisionHashLabel: nil pod")
	}
	if targetHash == "" || pod.Labels[query.LabelRevisionHash] == targetHash {
		return false, nil
	}
	raw, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"labels": map[string]string{query.LabelRevisionHash: targetHash}},
	})
	if err != nil {
		return false, fmt.Errorf("marshal revision-hash label patch for pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	if err := c.Patch(ctx, pod, client.RawPatch(types.StrategicMergePatchType, raw)); err != nil {
		return false, fmt.Errorf("patch pod %s/%s revision-hash label: %w", pod.Namespace, pod.Name, err)
	}
	return true, nil
}

// annotationsDiff computes the patch values that reconcile a pod's
// metadata.annotations toward the target revision's PodMeta:
//
//   - keys in target that are missing or have a different value on pod
//     map to their target value (add/update).
//   - keys that existed on the previous revision's PodMeta but are gone
//     from target map to nil (delete via strategic-merge JSON null).
//   - keys on pod that are foreign to both previous and target (sidecar
//     webhook injections, CNI attachments, kubelet-stamped values) are
//     left alone — they are not OMENative-owned.
//
// Returns an empty map when nothing needs to change. Order of map
// iteration doesn't affect the patch payload.
//
// Why we don't blindly replace the whole annotation set: third-party
// controllers (linkerd, istio, the Pod mutator webhook) routinely add
// annotations that aren't on any ControllerRevision. Erasing them on
// every in-place pass would break those integrations on every spec
// edit. The previous-CR diff lets OMENative remove ONLY keys it once
// authored that the user has since deleted from spec.
func annotationsDiff(podAnnotations, previousTargetAnnotations, newTargetAnnotations map[string]string) map[string]any {
	out := map[string]any{}
	// Add/update: every key in the new target whose pod value differs.
	for k, want := range newTargetAnnotations {
		if k == inPlaceImageTransitionAnnotation {
			continue
		}
		if got, ok := podAnnotations[k]; !ok || got != want {
			out[k] = want
		}
	}
	// Delete: keys that were OMENative-owned in the previous revision
	// but the user removed from spec. Skip keys that already-don't-exist
	// on the pod (no need to issue a delete for a no-op) and skip keys
	// still present in the new target (they were handled by the
	// add/update loop above).
	for k := range previousTargetAnnotations {
		if k == inPlaceImageTransitionAnnotation {
			continue
		}
		if _, stillTarget := newTargetAnnotations[k]; stillTarget {
			continue
		}
		if _, onPod := podAnnotations[k]; !onPod {
			continue
		}
		out[k] = nil
	}
	return out
}

// patchPodAnnotations applies a strategic-merge patch on the pod's
// metadata.annotations to reconcile it toward the target revision's
// PodMeta. previousTargetAnnotations is the running revision's PodMeta
// (used to compute which keys OMENative owns and is allowed to
// delete). Returns whether a patch was issued; an empty diff is a no-op.
// Without this reconcile, in-place updates would leave
// pod.metadata.annotations stuck at the value the pod was created
// with, so spec.{component}.annotations edits during an
// in-place-eligible rollout would never reach the pod.
func patchPodAnnotations(ctx context.Context, c client.Client, pod *corev1.Pod, previousTargetAnnotations, newTargetAnnotations map[string]string) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("patchPodAnnotations: nil pod")
	}
	diff := annotationsDiff(pod.Annotations, previousTargetAnnotations, newTargetAnnotations)
	if len(diff) == 0 {
		return false, nil
	}
	raw, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": diff},
	})
	if err != nil {
		return false, fmt.Errorf("marshal annotation patch for pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	if err := c.Patch(ctx, pod, client.RawPatch(types.StrategicMergePatchType, raw)); err != nil {
		return false, fmt.Errorf("patch pod %s/%s annotations: %w", pod.Namespace, pod.Name, err)
	}
	return true, nil
}
