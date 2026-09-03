package ops

import (
	"testing"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// TestRender_PairingProtocolLabel pins the pod-side cohort marker: a plan
// carrying a pairing protocol stamps ome.io/pairing-protocol on the rendered
// pod, and a protocol-free plan stamps nothing (absent = pairs with anything).
func TestRender_PairingProtocolLabel(t *testing.T) {
	plan, inst, runner := singlePodPlan()
	plan.PairingProtocol = "nixl-v2"
	pod, err := testRender(basicISVC(), basicPodSpec(), plan, inst, runner, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pod.Labels[query.LabelPairingProtocol]; got != "nixl-v2" {
		t.Errorf("pairing label: got %q want nixl-v2 (labels %+v)", got, pod.Labels)
	}

	plan.PairingProtocol = ""
	pod, err = testRender(basicISVC(), basicPodSpec(), plan, inst, runner, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, present := pod.Labels[query.LabelPairingProtocol]; present {
		t.Errorf("protocol-free plan must not label pods, got %q", got)
	}
}
