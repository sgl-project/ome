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

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
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

func TestAcceleratorClass_Reconcile_MatchDeclaredResources(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	ac := &v1beta1.AcceleratorClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ac-resources"},
		Spec: v1beta1.AcceleratorClassSpec{
			Discovery: v1beta1.AcceleratorDiscovery{NodeSelector: map[string]string{"accel": "nvidia"}},
			Resources: []v1beta1.AcceleratorResource{
				{Name: constants.NvidiaGPUResourceType, Quantity: resource.MustParse("1")},
			},
		},
	}

	// node-a exposes the declared GPU resource; node-b matches the discovery
	// labels but exposes no GPU capacity (mislabeled or drained hardware).
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
				corev1.ResourceMemory: resource.MustParse("8Gi"),
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
