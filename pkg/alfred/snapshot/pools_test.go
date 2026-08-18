package snapshot

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func nodeWithLabels(labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: labels}}
}

func TestGPUPoolForNode(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "gpu product label wins",
			labels: map[string]string{"nvidia.com/gpu.product": "NVIDIA-H100-80GB-HBM3", "node.kubernetes.io/instance-type": "BM.GPU.H100.8"},
			want:   "NVIDIA-H100-80GB-HBM3",
		},
		{
			name:   "known instance shape maps to the GPU model",
			labels: map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.H100.8"},
			want:   "H100",
		},
		{
			name:   "deprecated shape label honored",
			labels: map[string]string{"beta.kubernetes.io/instance-type": "BM.GPU.H200.8"},
			want:   "H200",
		},
		{
			// The shape catalog falls back to the shape string itself:
			// still one consistent pool per identical hardware shape.
			name:   "unknown shape keys by the shape",
			labels: map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.FUTURE.8"},
			want:   "BM.GPU.FUTURE.8",
		},
		{
			name:   "no labels falls back to the GPU resource name",
			labels: nil,
			want:   "nvidia.com/gpu",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GPUPoolForNode(nodeWithLabels(tc.labels), "nvidia.com/gpu"); got != tc.want {
				t.Fatalf("GPUPoolForNode = %q, want %q", got, tc.want)
			}
		})
	}
}

// Overlapping AcceleratorClass claims (H100x1..x8 all listing one node) are
// exactly why the pool key must come from the node, not the class CRD: two
// nodes of identical hardware must resolve to the same pool no matter what
// classes claim them.
func TestGPUPoolIgnoresClassOverlap(t *testing.T) {
	a := GPUPoolForNode(nodeWithLabels(map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.H100.8"}), "nvidia.com/gpu")
	b := GPUPoolForNode(nodeWithLabels(map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.H100-NC.8"}), "nvidia.com/gpu")
	if a != b || a != "H100" {
		t.Fatalf("identical hardware must share one pool: %q vs %q", a, b)
	}
}
