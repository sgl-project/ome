// Package status implements `kubectl ome status`: the full readiness story
// of one InferenceService in a single view.
package status

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/apierror"
	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/cli/paging"
	"sigs.k8s.io/ome/pkg/constants"
)

type report struct {
	ISVC   *v1beta1.InferenceService
	Pods   map[v1beta1.ComponentType][]corev1.Pod
	Events []corev1.Event
}

func gather(ctx context.Context, f factory.Factory, ns, name string) (*report, error) {
	ome, err := f.OMEClient()
	if err != nil {
		return nil, err
	}
	isvc, err := ome.OmeV1beta1().InferenceServices(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, apierror.Friendly(err)
	}
	kube, err := f.KubeClient()
	if err != nil {
		return nil, err
	}
	podSelector := fmt.Sprintf("%s=%s", constants.InferenceServiceLabel, name)
	podObjs, err := paging.ListAllPaged(ctx, func(pageOpts metav1.ListOptions) ([]runtime.Object, string, error) {
		pageOpts.LabelSelector = podSelector
		l, err := kube.CoreV1().Pods(ns).List(ctx, pageOpts)
		if err != nil {
			return nil, "", err
		}
		items := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			items = append(items, &l.Items[i])
		}
		return items, l.Continue, nil
	})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0, len(podObjs))
	for _, obj := range podObjs {
		pods = append(pods, *obj.(*corev1.Pod))
	}

	r := &report{ISVC: isvc, Pods: map[v1beta1.ComponentType][]corev1.Pod{}}
	for _, p := range pods {
		component := v1beta1.ComponentType(p.Labels[constants.OMEComponentLabel])
		r.Pods[component] = append(r.Pods[component], p)
	}

	// Warning events, scoped per involved object (the kubectl-describe
	// pattern) instead of one namespace-wide "type=Warning" list filtered
	// client-side: one tiny field-selector query for the InferenceService,
	// plus one for each of its pods. seen dedupes by event UID across those
	// queries.
	seen := map[types.UID]bool{}
	isvcEvents, err := eventsForObject(ctx, kube, ns, name, "InferenceService")
	if err != nil {
		return nil, err
	}
	r.Events = mergeWarningEvents(r.Events, seen, isvcEvents, name, "InferenceService")
	for _, p := range pods {
		podEvents, err := eventsForObject(ctx, kube, ns, p.Name, "Pod")
		if err != nil {
			return nil, err
		}
		r.Events = mergeWarningEvents(r.Events, seen, podEvents, p.Name, "Pod")
	}
	return r, nil
}

// eventsForObject fetches the Warning events for a single involved object
// (kubectl-describe's pattern): a field-selector query keeps each response
// tiny instead of listing every Warning event in the namespace.
func eventsForObject(ctx context.Context, kube kubernetes.Interface, ns, name, kind string) ([]corev1.Event, error) {
	sel := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s,type=%s", name, kind, corev1.EventTypeWarning)
	l, err := kube.CoreV1().Events(ns).List(ctx, metav1.ListOptions{FieldSelector: sel})
	if err != nil {
		return nil, err
	}
	return l.Items, nil
}

// mergeWarningEvents appends the events from one eventsForObject query onto
// acc, keeping only Warning events whose InvolvedObject actually matches
// (name, kind) -- defense in depth, since a field-selector-blind source
// (the fake clientset in tests, or a misbehaving API server) would
// otherwise leak unrelated events through -- and deduping by UID against
// seen, which callers thread across every per-object query so the same
// event is never counted twice.
func mergeWarningEvents(acc []corev1.Event, seen map[types.UID]bool, events []corev1.Event, wantName, wantKind string) []corev1.Event {
	for _, e := range events {
		if e.Type != corev1.EventTypeWarning || e.InvolvedObject.Name != wantName || e.InvolvedObject.Kind != wantKind {
			continue
		}
		if seen[e.UID] {
			continue
		}
		seen[e.UID] = true
		acc = append(acc, e)
	}
	return acc
}
