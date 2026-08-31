package ops

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// hostnameNotInValues collects the values of every required
// kubernetes.io/hostname NotIn requirement on the rendered pod.
func hostnameNotInValues(pod *corev1.Pod) []string {
	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil ||
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return nil
	}
	var out []string
	for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		for _, req := range term.MatchExpressions {
			if req.Key == "kubernetes.io/hostname" && req.Operator == corev1.NodeSelectorOpNotIn {
				out = append(out, req.Values...)
			}
		}
	}
	return out
}

// Relocation-directive exclusions ride the same required NotIn
// machinery as the migration overlay: every node on
// InstancePlan.ExcludedNodes lands as a required hostname NotIn on the
// rendered pod, so a disposed instance's rebuild is steered off its
// recorded suspect nodes. An empty list changes nothing.
func TestRender_AppliesExcludedNodes(t *testing.T) {
	plan := workload.ComponentPlan{Component: workload.ComponentEngine}
	runner := workload.RunnerPlan{Name: "default", Size: 1}

	inst := workload.InstancePlan{
		Index: 0, Incarnation: 1,
		Runners:       []workload.RunnerPlan{runner},
		ExcludedNodes: []string{"node-bad-1", "node-bad-2"},
	}
	pod, err := testRender(basicISVC(), basicPodSpec(), plan, inst, runner, 0)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := hostnameNotInValues(pod)
	want := map[string]bool{"node-bad-1": false, "node-bad-2": false}
	for _, v := range got {
		if _, ok := want[v]; ok {
			want[v] = true
		}
	}
	for node, seen := range want {
		if !seen {
			t.Errorf("excluded node %s missing from required NotIn terms: got %v", node, got)
		}
	}

	// Empty exclusion list → no NodeAffinity added.
	plain := workload.InstancePlan{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{runner}}
	pod, err = testRender(basicISVC(), basicPodSpec(), plan, plain, runner, 0)
	if err != nil {
		t.Fatalf("Render (no exclusions): %v", err)
	}
	if vals := hostnameNotInValues(pod); len(vals) != 0 {
		t.Errorf("no-exclusion render must not add NotIn terms: got %v", vals)
	}
}
