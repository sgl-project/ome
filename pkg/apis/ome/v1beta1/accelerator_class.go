package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AcceleratorClass defines a class of accelerators with similar capabilities
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Vendor",type=string,JSONPath=`.spec.vendor`
// +kubebuilder:printcolumn:name="Family",type=string,JSONPath=`.spec.family`
// +kubebuilder:printcolumn:name="Memory",type=string,JSONPath=`.spec.capabilities.memoryGB`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.availableNodes`

type AcceleratorClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AcceleratorClassSpec   `json:"spec"`
	Status AcceleratorClassStatus `json:"status"`
}

type AcceleratorClassSpec struct {
	// Vendor of the accelerator (nvidia, amd, intel, etc.)
	// +optional
	Vendor string `json:"vendor,omitempty"`

	// Family of the accelerator (ampere, hopper, cdna2, etc.)
	// +optional
	Family string `json:"family,omitempty"`

	// Model name (a100, h100, mi250x, etc.)
	// +optional
	Model string `json:"model,omitempty"`

	// Discovery patterns to identify nodes with this accelerator
	Discovery AcceleratorDiscovery `json:"discovery"`

	// Capabilities of this accelerator class
	Capabilities AcceleratorCapabilities `json:"capabilities"`

	// Resources exposed by this accelerator
	// +optional
	// +listType=atomic
	Resources []AcceleratorResource `json:"resources,omitempty"`

	// Integration with external systems
	// +optional
	Integration *AcceleratorIntegration `json:"integration,omitempty"`

	// Cost information for optimization decisions
	// +optional
	Cost *AcceleratorCost `json:"cost,omitempty"`
}

type AcceleratorCost struct {
	// Cost per hour in dollars
	// +optional
	PerHour *resource.Quantity `json:"perHour,omitempty"`

	// Cost per million tokens (for usage-based pricing)
	// +optional
	PerMillionTokens *resource.Quantity `json:"perMillionTokens,omitempty"`

	// Spot instance pricing if available
	// +optional
	SpotPerHour *resource.Quantity `json:"spotPerHour,omitempty"`

	// Cost tier for simplified selection (low, medium, high)
	// +optional
	Tier string `json:"tier,omitempty"`
}

type AcceleratorDiscovery struct {
	// NodeSelector to identify nodes with this accelerator
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NodeSelectorTerms for more complex node selection
	// +optional
	// +listType=atomic
	NodeSelectorTerms []v1.NodeSelectorTerm `json:"nodeSelectorTerms,omitempty"`

	// PCIVendorID for device discovery (e.g., "10de" for NVIDIA)
	// +optional
	PCIVendorID string `json:"pciVendorID,omitempty"`

	// DeviceIDs list of PCI device IDs
	// +optional
	// +listType=atomic
	DeviceIDs []string `json:"deviceIDs,omitempty"`
}

type AcceleratorCapabilities struct {
	// Memory capacity in GB
	// +optional
	MemoryGB *resource.Quantity `json:"memoryGB,omitempty"`

	// Compute capability (NVIDIA) or equivalent
	// +optional
	ComputeCapability string `json:"computeCapability,omitempty"`

	// Level Zero version (for Intel accelerators)
	// +optional
	LevelZeroVersion string `json:"levelZeroVersion,omitempty"`

	// Clock speeds
	// +optional
	ClockSpeedMHz *int32 `json:"clockSpeedMHz,omitempty"`

	// Memory bandwidth
	// +optional
	MemoryBandwidthGBps *resource.Quantity `json:"memoryBandwidthGBps,omitempty"`

	// Features supported by this accelerator
	// +optional
	// +listType=atomic
	Features []string `json:"features,omitempty"`

	// Performance metrics
	// +optional
	Performance *AcceleratorPerformance `json:"performance,omitempty"`
}

type AcceleratorPerformance struct {
	// FP32 performance in TFLOPS
	// +optional
	Fp32Tflops *int64 `json:"fp32Tflops,omitempty"`

	// FP16 performance in TFLOPS
	// +optional
	Fp16Tflops *int64 `json:"fp16Tflops,omitempty"`

	// INT8 performance in TOPS
	// +optional
	Int8Tops *int64 `json:"int8Tops,omitempty"`

	// INT4 performance in TOPS
	// +optional
	Int4Tops *int64 `json:"int4Tops,omitempty"`

	// Latency metrics
	// +optional
	Latency *AcceleratorLatency `json:"latency,omitempty"`
}

type AcceleratorLatency struct {
	// Average latency in milliseconds
	// +optional
	AverageMillis *int64 `json:"averageMillis,omitempty"`
	// Maximum latency in milliseconds
	// +optional
	MaximumMillis *int64 `json:"maximumMillis,omitempty"`
}

type AcceleratorResource struct {
	// Name of the resource (e.g., nvidia.com/gpu)
	Name string `json:"name"`

	// Quantity per accelerator
	// +kubebuilder:default="1"
	Quantity resource.Quantity `json:"quantity"`

	// Divisible indicates if the resource can be subdivided
	// +optional
	Divisible bool `json:"divisible,omitempty"`
}

type AcceleratorIntegration struct {
	// KueueResourceFlavor name to sync with
	// +optional
	KueueResourceFlavor string `json:"kueueResourceFlavor,omitempty"`

	// VolcanoGPUType for Volcano integration
	// +optional
	VolcanoGPUType string `json:"volcanoGPUType,omitempty"`
}

type AcceleratorClassStatus struct {
	// Nodes that have this accelerator
	// +optional
	// +listType=atomic
	Nodes []string `json:"nodes,omitempty"`

	// Total number of accelerators in the cluster
	// +optional
	TotalAccelerators int32 `json:"totalAccelerators,omitempty"`

	// Available accelerators (not allocated)
	// +optional
	AvailableAccelerators int32 `json:"availableAccelerators,omitempty"`

	// Last update time
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated"`

	// Conditions represent the latest available observations
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AvailableNodes is the number of nodes that have this accelerator available
	// +optional
	AvailableNodes int32 `json:"availableNodes,omitempty"`
}
