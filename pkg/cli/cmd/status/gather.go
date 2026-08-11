// Package status implements `kubectl ome status`: the full readiness story
// of one InferenceService in a single view.
package status

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/apierror"
	"sigs.k8s.io/ome/pkg/cli/factory"
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
	pods, err := kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", constants.InferenceServiceLabel, name),
	})
	if err != nil {
		return nil, err
	}
	r := &report{ISVC: isvc, Pods: map[v1beta1.ComponentType][]corev1.Pod{}}
	ours := map[string]bool{name: true} // involvedObject names we care about
	for _, p := range pods.Items {
		component := v1beta1.ComponentType(p.Labels[constants.OMEComponentLabel])
		r.Pods[component] = append(r.Pods[component], p)
		ours[p.Name] = true
	}
	events, err := kube.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: "type=" + corev1.EventTypeWarning,
	})
	if err != nil {
		return nil, err
	}
	for _, e := range events.Items {
		if e.Type == corev1.EventTypeWarning && ours[e.InvolvedObject.Name] {
			r.Events = append(r.Events, e)
		}
	}
	return r, nil
}
