package status

import (
	"fmt"
	"io"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/printers"
)

// componentOrder fixes section ordering; conditions sort component-first,
// aggregate Ready last.
var componentOrder = []v1beta1.ComponentType{
	v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent,
}

func render(r *report, w io.Writer) error {
	isvc := r.ISVC
	fmt.Fprintf(w, "Name:       %s\n", isvc.Name)
	fmt.Fprintf(w, "Namespace:  %s\n", isvc.Namespace)
	fmt.Fprintf(w, "Ready:      %t\n", isvc.Status.IsReady())
	if isvc.Status.URL != nil {
		fmt.Fprintf(w, "URL:        %s\n", isvc.Status.URL.String())
	}
	if isvc.Spec.Model != nil {
		fmt.Fprintf(w, "Model:      %s\n", isvc.Spec.Model.Name)
	}
	if isvc.Spec.Runtime != nil {
		fmt.Fprintf(w, "Runtime:    %s\n", isvc.Spec.Runtime.Name)
	} else {
		fmt.Fprintf(w, "Runtime:    (auto-selected)\n")
	}

	fmt.Fprintf(w, "\nConditions:\n")
	conds := append([]apis.Condition{}, isvc.Status.Conditions...)
	sort.SliceStable(conds, func(i, j int) bool { return condRank(conds[i].Type) < condRank(conds[j].Type) })
	condTable := printers.Table{Headers: []string{"  TYPE", "STATUS", "REASON", "MESSAGE"}}
	for _, c := range conds {
		condTable.Rows = append(condTable.Rows, []string{
			"  " + string(c.Type), string(c.Status), printers.OrDash(c.Reason), printers.OrDash(c.Message),
		})
	}
	if err := condTable.Write(w); err != nil {
		return err
	}

	fmt.Fprintf(w, "\nComponents:\n")
	seen := make(map[v1beta1.ComponentType]bool, len(componentOrder))
	for _, ct := range componentOrder {
		seen[ct] = true
		spec, inStatus := isvc.Status.Components[ct]
		pods := r.Pods[ct]
		if !inStatus && len(pods) == 0 {
			continue
		}
		if err := writeComponent(w, componentLabel(ct), spec, pods); err != nil {
			return err
		}
	}
	// gather() buckets pods by their component label verbatim, so a pod
	// whose label is missing (ComponentType("")) or names something other
	// than engine/decoder/router lands here instead of in componentOrder.
	// Surface it rather than silently dropping it from the report; sort
	// for deterministic output.
	var remaining []v1beta1.ComponentType
	for ct := range r.Pods {
		if !seen[ct] {
			remaining = append(remaining, ct)
		}
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i] < remaining[j] })
	for _, ct := range remaining {
		if err := writeComponent(w, componentLabel(ct), isvc.Status.Components[ct], r.Pods[ct]); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "\nModel Status:\n  Transition: %s\n", printers.OrDash(string(isvc.Status.ModelStatus.TransitionStatus)))
	writeOptionalSection(w, "Traffic", isvc.Status.Traffic != nil)
	writeOptionalSection(w, "Canary", isvc.Status.Canary != nil)
	writeOptionalSection(w, "Placement", isvc.Status.Placement != nil)
	writeOptionalSection(w, "RolloutCoordination", isvc.Status.RolloutCoordination != nil)

	if len(r.Events) > 0 {
		fmt.Fprintf(w, "\nRecent Warning Events:\n")
		evTable := printers.Table{Headers: []string{"  OBJECT", "REASON", "MESSAGE"}}
		for _, e := range r.Events {
			evTable.Rows = append(evTable.Rows, []string{
				"  " + e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name, e.Reason, e.Message,
			})
		}
		if err := evTable.Write(w); err != nil {
			return err
		}
	}
	return nil
}

// componentLabel names a Components section heading. Pods whose component
// label was missing land under ComponentType("") (see gather.go); render
// that as an explicit placeholder rather than a blank heading.
func componentLabel(ct v1beta1.ComponentType) string {
	if ct == "" {
		return "(unlabeled)"
	}
	return string(ct)
}

// writeComponent renders one Components section: the revision header line
// plus a pod table (or a "(no pods)" placeholder).
func writeComponent(w io.Writer, label string, spec v1beta1.ComponentStatusSpec, pods []corev1.Pod) error {
	fmt.Fprintf(w, "  %s   revision %s\n", label, printers.OrDash(spec.LatestCreatedRevision))
	if len(pods) == 0 {
		fmt.Fprintf(w, "    (no pods)\n")
		return nil
	}
	podTable := printers.Table{Headers: []string{"    POD", "PHASE", "READY", "RESTARTS", "NODE"}}
	for _, p := range pods {
		ready, restarts := 0, int32(0)
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}
		podTable.Rows = append(podTable.Rows, []string{
			"    " + p.Name, string(p.Status.Phase),
			fmt.Sprintf("%d/%d", ready, len(p.Spec.Containers)),
			fmt.Sprintf("%d", restarts), printers.OrDash(p.Spec.NodeName),
		})
	}
	return podTable.Write(w)
}

// writeOptionalSection prints a one-line presence indicator for a
// v1-deferred status section (Traffic/Canary/Placement/RolloutCoordination):
// full rendering is follow-up work, but presence must never be silently
// dropped from the report.
func writeOptionalSection(w io.Writer, name string, present bool) {
	if present {
		fmt.Fprintf(w, "  %s: present (inspect with kubectl get inferenceservice -o yaml)\n", name)
	}
}

func condRank(t apis.ConditionType) int {
	switch t {
	case "EngineReady":
		return 0
	case "DecoderReady":
		return 1
	case "RouterReady":
		return 2
	case "IngressReady":
		return 3
	case apis.ConditionReady:
		return 5
	default:
		return 4
	}
}
