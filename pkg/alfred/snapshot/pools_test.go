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

// poolNode is a snapshot Node carrying just what reconcilePoolKeys reads.
func poolNode(name string, gpus int64, labels map[string]string) *Node {
	return &Node{
		Name:      name,
		Labels:    labels,
		TotalGPUs: gpus,
		GPUPool:   GPUPoolForNode(nodeWithLabels(labels), "nvidia.com/gpu"),
	}
}

func h100(product string) map[string]string {
	labels := map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.H100.8"}
	if product != "" {
		labels["nvidia.com/gpu.product"] = product
	}
	return labels
}

func poolsOf(nodes map[string]*Node) map[string]int {
	out := map[string]int{}
	for _, n := range nodes {
		out[n.GPUPool]++
	}
	return out
}

// TestReconcilePoolKeysHealsPartialGFD reproduces a partial GPU-feature-
// discovery rollout: fourteen identical BM.GPU.H100.8 nodes where two are
// missing the product label. Per-node derivation splits them 12/2, which
// halves every score and makes migration between the halves impossible.
func TestReconcilePoolKeysHealsPartialGFD(t *testing.T) {
	const product = "NVIDIA-H100-80GB-HBM3"
	nodes := map[string]*Node{}
	for i := 0; i < 12; i++ {
		name := "labelled-" + string(rune('a'+i))
		nodes[name] = poolNode(name, 8, h100(product))
	}
	nodes["bare-1"] = poolNode("bare-1", 6, h100(""))
	nodes["bare-2"] = poolNode("bare-2", 8, h100(""))

	if got := poolsOf(nodes); len(got) != 2 || got["H100"] != 2 {
		t.Fatalf("precondition: want the 12/2 split, got %v", got)
	}

	reconcilePoolKeys(nodes)

	got := poolsOf(nodes)
	if len(got) != 1 || got[product] != 14 {
		t.Fatalf("want all 14 nodes in %q, got %v", product, got)
	}
}

// TestReconcilePoolKeysKeepsGenuinePartition: several distinct products under
// one shape is real hardware variance (MIG geometries, a mixed refresh). An
// unlabelled node must not be guessed into either — merging distinct hardware
// would produce a migration onto the wrong accelerator, while leaving it
// split only costs opportunity.
func TestReconcilePoolKeysKeepsGenuinePartition(t *testing.T) {
	nodes := map[string]*Node{
		"mig-1":  poolNode("mig-1", 8, h100("NVIDIA-H100-80GB-HBM3")),
		"mig-2":  poolNode("mig-2", 8, h100("NVIDIA-H100-80GB-HBM3-MIG-1g.10gb")),
		"bare-1": poolNode("bare-1", 8, h100("")),
	}

	reconcilePoolKeys(nodes)

	if nodes["bare-1"].GPUPool != "H100" {
		t.Fatalf("ambiguous group must leave the unlabelled node on its shape key, got %q",
			nodes["bare-1"].GPUPool)
	}
	if nodes["mig-1"].GPUPool == nodes["mig-2"].GPUPool {
		t.Fatalf("distinct products must stay distinct pools")
	}
}

// TestReconcilePoolKeysNoOps covers the two states that are already correct —
// GFD everywhere, and GFD nowhere — plus nodes it must not touch.
func TestReconcilePoolKeysNoOps(t *testing.T) {
	const product = "NVIDIA-A10"
	a10 := func(p string) map[string]string {
		labels := map[string]string{"node.kubernetes.io/instance-type": "BM.GPU.A10.4"}
		if p != "" {
			labels["nvidia.com/gpu.product"] = p
		}
		return labels
	}

	t.Run("gfd everywhere", func(t *testing.T) {
		nodes := map[string]*Node{
			"a": poolNode("a", 4, a10(product)),
			"b": poolNode("b", 4, a10(product)),
		}
		reconcilePoolKeys(nodes)
		if got := poolsOf(nodes); len(got) != 1 || got[product] != 2 {
			t.Fatalf("want one pool, got %v", got)
		}
	})

	t.Run("gfd nowhere", func(t *testing.T) {
		nodes := map[string]*Node{
			"a": poolNode("a", 4, a10("")),
			"b": poolNode("b", 4, a10("")),
		}
		reconcilePoolKeys(nodes)
		if got := poolsOf(nodes); len(got) != 1 || got["A10"] != 2 {
			t.Fatalf("want one shape-keyed pool, got %v", got)
		}
	})

	t.Run("no shape label is left alone", func(t *testing.T) {
		nodes := map[string]*Node{
			"labelled": poolNode("labelled", 4, a10(product)),
			"bare":     poolNode("bare", 4, nil),
		}
		reconcilePoolKeys(nodes)
		if nodes["bare"].GPUPool != "nvidia.com/gpu" {
			t.Fatalf("a node with no shape cannot join a shape group, got %q", nodes["bare"].GPUPool)
		}
	})

	t.Run("gpu-less nodes are ignored", func(t *testing.T) {
		nodes := map[string]*Node{
			"labelled": poolNode("labelled", 4, a10(product)),
			"cpu":      poolNode("cpu", 0, a10("")),
		}
		reconcilePoolKeys(nodes)
		if nodes["cpu"].GPUPool == product {
			t.Fatalf("a GPU-less node must not be promoted into a GPU pool")
		}
	})
}
