package components

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestRawDeploymentPodLabelInfoUsesTruncatedComponentName(t *testing.T) {
	componentName := "amaaaaaabgjpxjqah52sm262rjisgzw3sfp2f4lukfkinrwwoj2v7yy73jrq-engine"
	objectMeta := metav1.ObjectMeta{Name: componentName}
	statusSpec := v1beta1.ComponentStatusSpec{LatestCreatedRevision: "revision-name"}
	expectedValue := constants.TruncateNameWithMaxLength(componentName, 63)

	key, value := getPodLabelInfo(true, objectMeta, statusSpec)
	if key != constants.RawDeploymentAppLabel {
		t.Fatalf("label key = %q, want %q", key, constants.RawDeploymentAppLabel)
	}
	if value != expectedValue {
		t.Fatalf("label value = %q, want %q", value, expectedValue)
	}
}

func TestNonRawPodLabelInfoUsesRevision(t *testing.T) {
	statusSpec := v1beta1.ComponentStatusSpec{LatestCreatedRevision: "revision-name"}

	key, value := getPodLabelInfo(false, metav1.ObjectMeta{Name: "component-name"}, statusSpec)
	if key != constants.RevisionLabel {
		t.Fatalf("label key = %q, want %q", key, constants.RevisionLabel)
	}
	if value != statusSpec.LatestCreatedRevision {
		t.Fatalf("label value = %q, want %q", value, statusSpec.LatestCreatedRevision)
	}
}
