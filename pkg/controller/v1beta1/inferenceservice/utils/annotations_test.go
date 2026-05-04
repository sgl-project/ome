package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/constants"
)

func TestSetPodLabelsFromAnnotations_UsesDedicatedAIClusterForKueueQueue(t *testing.T) {
	metadata := &metav1.ObjectMeta{
		Annotations: map[string]string{
			constants.DedicatedAICluster:   "dac-a",
			constants.KueueEnabledLabelKey: "true",
		},
		Labels: map[string]string{},
	}

	SetPodLabelsFromAnnotations(metadata)

	if got, want := metadata.Labels[constants.KueueQueueLabelKey], "dac-a"; got != want {
		t.Fatalf("unexpected kueue queue label: got %q, want %q", got, want)
	}
}

func TestSetPodLabelsFromAnnotations_UsesDACQueueOverrideWhenPresent(t *testing.T) {
	metadata := &metav1.ObjectMeta{
		Annotations: map[string]string{
			constants.DedicatedAICluster:   "dac-a",
			constants.KueueEnabledLabelKey: "true",
			constants.DACQueueNameLabelKey: "reservation-queue-1",
		},
		Labels: map[string]string{},
	}

	SetPodLabelsFromAnnotations(metadata)

	if got, want := metadata.Labels[constants.KueueQueueLabelKey], "reservation-queue-1"; got != want {
		t.Fatalf("unexpected kueue queue label: got %q, want %q", got, want)
	}
}
