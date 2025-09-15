package acceleratorclass

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func TestAcceleratorClass_Reconcile_AddsFinalizerAndUpdatesStatus(t *testing.T) {
	g := NewWithT(t)

	// Setup scheme
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	// Initial objects: one AcceleratorClass and one matching Node
	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ac",
		},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{
				NodeSelector: map[string]string{"accelerator": "nvidia"},
			},
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"accelerator": "nvidia"},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceMemory:                                resource.MustParse("64Gi"),
				corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("1"),
			},
		},
	}

	// Create fake client with status subresource
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, node).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()

	reconciler := &AcceleratorClassReconciler{
		Client:   c,
		Log:      ctrl.Log.WithName("AcceleratorClassTest"),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	// Reconcile
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	// Validate finalizer added and status updated
	updated := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, updated)).To(Succeed())
	g.Expect(updated.GetFinalizers()).To(ContainElement(constants.AcceleratorClassFinalizer))
	g.Expect(updated.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(updated.Status.Nodes).To(ContainElement("node-1"))
	g.Expect(updated.Status.LastUpdated.IsZero()).To(BeFalse())
}

func TestAcceleratorClass_Reconcile_MatchDiscovery(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-discovery"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{NodeSelector: map[string]string{"accel": "nvidia"}},
		},
	}

	nodeA := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"accel": "nvidia"}},
	}
	nodeB := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b", Labels: map[string]string{"accel": "amd"}},
	}

	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, nodeA, nodeB).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()

	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	g.Expect(curr.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(curr.Status.Nodes).To(ContainElement("node-a"))
	g.Expect(curr.Status.Nodes).NotTo(ContainElement("node-b"))
}

func TestAcceleratorClass_Reconcile_MatchSelectorTerms(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-terms"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: "node.info/kubeletVersion", Operator: corev1.NodeSelectorOpIn, Values: []string{"v1.30"}},
					}},
				},
			},
		},
	}

	nodeA := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"node.info/kubeletVersion": "v1.30"}},
	}
	nodeB := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b", Labels: map[string]string{"node.info/kubeletVersion": "v1.29"}},
	}
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, nodeA, nodeB).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()
	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())
	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	g.Expect(curr.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(curr.Status.Nodes).To(ContainElement("node-a"))
	g.Expect(curr.Status.Nodes).NotTo(ContainElement("node-b"))
}

func TestAcceleratorClass_Reconcile_MatchSelectorFields(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-fields"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchFields: []corev1.NodeSelectorRequirement{
						{Key: "metadata.name", Operator: corev1.NodeSelectorOpIn, Values: []string{"node-a"}},
					}},
				},
			},
		},
	}

	nodeA := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"some": "label"}},
	}
	nodeB := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b", Labels: map[string]string{"some": "label"}},
	}
	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, nodeA, nodeB).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()
	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}
	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())
	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	g.Expect(curr.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(curr.Status.Nodes).To(ContainElement("node-a"))
	g.Expect(curr.Status.Nodes).NotTo(ContainElement("node-b"))
}

func TestAcceleratorClass_Reconcile_MatchMemoryGB(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-memory"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{NodeSelector: map[string]string{"accel": "nvidia"}},
			Capabilities: v1beta1.AcceleratorCapabilities{
				MemoryGB: resource.NewQuantity(16*1024*1024*1024, resource.BinarySI), // 16Gi
			},
		},
	}

	nodeA := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"accel": "nvidia"}},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("1"),
				corev1.ResourceMemory:                                resource.MustParse("32Gi"),
			},
		},
	}
	nodeB := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b", Labels: map[string]string{"accel": "nvidia"}},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("1"),
				corev1.ResourceMemory:                                resource.MustParse("8Gi"),
			},
		},
	}

	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, nodeA, nodeB).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()

	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	g.Expect(curr.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(curr.Status.Nodes).To(ContainElement("node-a"))
	g.Expect(curr.Status.Nodes).NotTo(ContainElement("node-b"))
}

func TestAcceleratorClass_Reconcile_MatchCapabilities(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-capabilities"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{NodeSelector: map[string]string{"accel": "nvidia"}},
			Capabilities: v1beta1.AcceleratorCapabilities{
				ComputeCapability: "7",
			},
		},
	}

	nodeA := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"accel": "nvidia"}},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("8"),
			},
		},
	}
	nodeB := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b", Labels: map[string]string{"accel": "nvidia"}},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("6"),
			},
		},
	}

	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, nodeA, nodeB).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()

	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	g.Expect(curr.Status.AvailableNodes).To(Equal(int32(1)))
	g.Expect(curr.Status.Nodes).To(ContainElement("node-a"))
}

func TestAcceleratorClass_Reconcile_DoesNotUpdateTimestampOnNoChange(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-ts"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{NodeSelector: map[string]string{"accel": "nvidia"}},
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Labels: map[string]string{"accel": "nvidia"}},
	}

	c := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ac, node).
		WithStatusSubresource(&v1beta1.AcceleratorClass{}).
		Build()

	reconciler := &AcceleratorClassReconciler{Client: c, Log: ctrl.Log.WithName("AcceleratorClassTest"), Scheme: scheme, Recorder: record.NewFakeRecorder(5)}

	ctx := context.TODO()
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	curr := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, curr)).To(Succeed())
	firstUpdate := curr.Status.LastUpdated
	g.Expect(firstUpdate.IsZero()).To(BeFalse())

	// Wait briefly to ensure a future Now() would differ if called
	time.Sleep(5 * time.Millisecond)

	// Second reconcile with no changes should not bump LastUpdated
	_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ac.Name}})
	g.Expect(err).NotTo(HaveOccurred())

	post := &v1beta1.AcceleratorClass{}
	g.Expect(c.Get(ctx, types.NamespacedName{Name: ac.Name}, post)).To(Succeed())
	g.Expect(post.Status.LastUpdated.Time.Equal(firstUpdate.Time)).To(BeTrue())
}

func Test_getGPUCapacity_Helper(t *testing.T) {
	g := NewWithT(t)

	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-node"},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"):         resource.MustParse("2"),
				corev1.ResourceName("nvidia.com/mig-1g.10gb"): resource.MustParse("4"),
				corev1.ResourceName("amd.com/gpu"):            resource.MustParse("1"),
				corev1.ResourceName("gpu.intel.com/cards"):    resource.MustParse("3"),
			},
		},
	}

	total, byRes := getGPUCapacity(n)
	g.Expect(total).To(Equal(int64(10))) // 2 + 4 + 1 + 3
	g.Expect(byRes).To(HaveKeyWithValue("nvidia.com/gpu", int64(2)))
	g.Expect(byRes).To(HaveKeyWithValue("nvidia.com/mig-1g.10gb", int64(4)))
	g.Expect(byRes).To(HaveKeyWithValue("amd.com/gpu", int64(1)))
	g.Expect(byRes).To(HaveKeyWithValue("gpu.intel.com/cards", int64(3)))
}

func Test_nodeMatchCapabilities_GPUCount(t *testing.T) {
	g := NewWithT(t)

	ac := &v1beta1.AcceleratorClass{
		Spec: v1beta1.AcceleratorClassSpec{
			Capabilities: v1beta1.AcceleratorCapabilities{ComputeCapability: "1"},
		},
	}

	node := &corev1.Node{Status: corev1.NodeStatus{Capacity: corev1.ResourceList{corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("1")}}}
	g.Expect(nodeMatchCapabilities(ac, node)).To(BeTrue())

	ac.Spec.Capabilities.ComputeCapability = "2"
	g.Expect(nodeMatchCapabilities(ac, node)).To(BeFalse())
}
