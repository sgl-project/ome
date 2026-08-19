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
func GPUPoolForNode(node *corev1.Node, gpuResource string) string {
	if product := node.Labels[gfdGPUProductLabel]; product != "" {
		return product
	}
	shape := node.Labels[constants.NodeInstanceShapeLabel]
	if shape == "" {
		shape = node.Labels[constants.DeprecatedNodeInstanceShapeLabel]
	}
	if shape != "" {
		if short, err := utils.GetInstanceTypeShortName(shape); err == nil && short != "" {
			return short
		}
	}
	return gpuResource
}
