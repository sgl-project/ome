package snapshot

import (
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils"
)

// gfdGPUProductLabel is written by NVIDIA GPU feature discovery
// (e.g. "NVIDIA-H100-80GB-HBM3") and is the most precise pool signal when
// present.
const gfdGPUProductLabel = "nvidia.com/gpu.product"

// GPUPoolForNode derives the hardware-pool key scoring partitions on, from
// the node itself — deliberately NOT from AcceleratorClass.Status.Nodes.
// AcceleratorClass is a workload-selection surface, and several shape-scoped
// classes (H100x1, H100x2, H100x4, H100x8) may all claim the same node, so
// class membership does not partition hardware; any first-wins tie-break
// would split one physical pool across classes and corrupt per-pool scoring.
//
// Precedence:
//  1. The GPU product label (GPU feature discovery), when installed.
//  2. The instance-type label mapped through the shape catalog
//     (utils.GetInstanceTypeShortName; BM.GPU.H100.8 -> H100; unknown
//     shapes key by the shape string itself, which still groups identical
//     hardware).
//  3. The GPU resource name (nvidia.com/gpu, amd.com/gpu, a MIG profile) —
//     coarse, but never mixes vendors or MIG geometries.
//
// The product label is the most precise rung but the least universally
// present — GPU feature discovery is a DaemonSet, not something the hardware
// announces — so per-node derivation alone can split one physical pool.
// reconcilePoolKeys repairs that across the whole node set.
func GPUPoolForNode(node *corev1.Node, gpuResource string) string {
	if product := node.Labels[gfdGPUProductLabel]; product != "" {
		return product
	}
	if shape := shapePoolKey(node.Labels); shape != "" {
		return shape
	}
	return gpuResource
}

// shapePoolKey resolves the instance-shape rung on its own, returning "" when
// the node carries no usable instance-type label.
func shapePoolKey(labels map[string]string) string {
	shape := labels[constants.NodeInstanceShapeLabel]
	if shape == "" {
		shape = labels[constants.DeprecatedNodeInstanceShapeLabel]
	}
	if shape == "" {
		return ""
	}
	if short, err := utils.GetInstanceTypeShortName(shape); err == nil && short != "" {
		return short
	}
	return ""
}

// reconcilePoolKeys makes the partition consistent across nodes of identical
// hardware, which per-node derivation cannot guarantee. GPU feature discovery
// runs as a DaemonSet, so a node it has not reached keys by instance shape
// while its identical neighbours key by GPU product — one physical pool
// silently becomes two. The damage is not cosmetic: each half scores against
// only its own bins, and because pool is a hard boundary in candidate
// selection, no workload can ever migrate across the split even between
// adjacent nodes in the same rack.
//
// The rule is per shape group. A group whose labelled members agree on a
// single product adopts that product for every member, unlabelled ones
// included — precision preserved, split gone. A group carrying several
// distinct products keeps its per-node keys, because that is a genuine
// partition (MIG geometries, a mixed hardware refresh) and precisely what the
// product label exists to express; an unlabelled node there stays on its own
// shape key rather than being guessed into one of them, since splitting
// identical hardware only costs opportunity while merging distinct hardware
// would produce migrations onto the wrong accelerator.
func reconcilePoolKeys(nodes map[string]*Node) {
	// byShape collects the distinct products observed on each shape
	// group's labelled nodes.
	byShape := map[string]map[string]struct{}{}
	for _, n := range nodes {
		if n.TotalGPUs == 0 {
			continue
		}
		product := n.Labels[gfdGPUProductLabel]
		shape := shapePoolKey(n.Labels)
		if product == "" || shape == "" {
			continue
		}
		if byShape[shape] == nil {
			byShape[shape] = map[string]struct{}{}
		}
		byShape[shape][product] = struct{}{}
	}

	for _, n := range nodes {
		if n.TotalGPUs == 0 || n.Labels[gfdGPUProductLabel] != "" {
			continue
		}
		shape := shapePoolKey(n.Labels)
		if shape == "" {
			continue
		}
		products := byShape[shape]
		if len(products) != 1 {
			continue
		}
		for product := range products {
			n.GPUPool = product
		}
	}
}
