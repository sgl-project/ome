package integration

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/pkg/scheduler"
	"k8s.io/kubernetes/pkg/scheduler/profile"

	schedclientset "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
)

// domainLabelKey is the node label the tests use as their topology dimension. It
// is arbitrary — the plugin learns it per-workload from the PodGroup's
// topologyKeyAnnotation — which is exactly what these tests prove.
const domainLabelKey = "topology.ome.io/domain"

// topologyKeyAnnotation is the PodGroup annotation that declares which node label
// is the gang's domain. Mirrors gangpack's package-private const; kept in sync by
// this literal (the plugin owns the name, the test asserts the contract).
const topologyKeyAnnotation = "ome.io/topology-key"

// podGroupLabel is the standard scheduler-plugins gang-membership label.
const podGroupLabel = "scheduling.x-k8s.io/pod-group"

// gpuResource is the extended resource the fake accelerator nodes advertise and
// the gang pods request. The plugin infers free-ness from the pod's own
// requests, so the concrete name is irrelevant to it.
const gpuResource v1.ResourceName = "example.com/gpu"

// testContext bundles a running scheduler and its clients, replicated from
// scheduler-plugins' package-private test harness (not importable).
type testContext struct {
	ClientSet    clientset.Interface
	SchedClient  schedclientset.Interface
	KubeConfig   *restclient.Config
	Ctx          context.Context
	CancelFn     context.CancelFunc
	Scheduler    *scheduler.Scheduler
	informers    informers.SharedInformerFactory
	dynInformers dynamicinformer.DynamicSharedInformerFactory
}

// startScheduler stands up an in-process kube-scheduler wired with the given
// options (profiles + out-of-tree registry) against the envtest API server, and
// runs it. It mirrors scheduler-plugins' initTestSchedulerWithOptions +
// syncInformerFactory, which are package-private and cannot be imported.
func startScheduler(t *testing.T, cfg *restclient.Config, opts ...scheduler.Option) *testContext {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	cs := clientset.NewForConfigOrDie(cfg)
	tc := &testContext{
		ClientSet:   cs,
		SchedClient: schedclientset.NewForConfigOrDie(cfg),
		KubeConfig:  cfg,
		Ctx:         ctx,
		CancelFn:    cancel,
	}

	tc.informers = scheduler.NewInformerFactory(cs, 0)
	dynClient := dynamic.NewForConfigOrDie(cfg)
	tc.dynInformers = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, v1.NamespaceAll, nil)

	eventBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: cs.EventsV1()})

	opts = append(opts, scheduler.WithKubeConfig(cfg))
	sched, err := scheduler.New(
		ctx,
		cs,
		tc.informers,
		tc.dynInformers,
		profile.NewRecorderFactory(eventBroadcaster),
		opts...,
	)
	if err != nil {
		cancel()
		t.Fatalf("scheduler.New: %v", err)
	}
	tc.Scheduler = sched

	eventBroadcaster.StartRecordingToSink(ctx.Done())
	tc.informers.Start(ctx.Done())
	tc.dynInformers.Start(ctx.Done())
	tc.informers.WaitForCacheSync(ctx.Done())
	tc.dynInformers.WaitForCacheSync(ctx.Done())

	go sched.Run(ctx)
	return tc
}

// teardown removes the objects this test created and stops the scheduler, so the
// suite can reuse the shared API server without cross-test leakage. Pods and
// PodGroups are deleted (force, across namespaces) alongside nodes — a prior test's
// bound pods would otherwise linger on reused node names as phantom occupancy, and
// leftover pod/PodGroup names collide on a repeated run.
func (tc *testContext) teardown(t *testing.T) {
	t.Helper()
	zero := int64(0)
	force := metav1.DeleteOptions{GracePeriodSeconds: &zero}
	if pods, err := tc.ClientSet.CoreV1().Pods(metav1.NamespaceAll).List(tc.Ctx, metav1.ListOptions{}); err == nil {
		for i := range pods.Items {
			p := &pods.Items[i]
			_ = tc.ClientSet.CoreV1().Pods(p.Namespace).Delete(tc.Ctx, p.Name, force)
		}
	}
	if pgs, err := tc.SchedClient.SchedulingV1alpha1().PodGroups(metav1.NamespaceAll).List(tc.Ctx, metav1.ListOptions{}); err == nil {
		for i := range pgs.Items {
			pg := &pgs.Items[i]
			_ = tc.SchedClient.SchedulingV1alpha1().PodGroups(pg.Namespace).Delete(tc.Ctx, pg.Name, metav1.DeleteOptions{})
		}
	}
	if err := tc.ClientSet.CoreV1().Nodes().DeleteCollection(tc.Ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		t.Errorf("cleanup nodes: %v", err)
	}
	tc.CancelFn()
}

// namespaceObj builds a bare Namespace object, for setup done before a
// testContext/scheduler exists.
func namespaceObj(name string) *v1.Namespace {
	return &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// createNamespace creates a namespace, tolerating AlreadyExists.
func createNamespace(t *testing.T, tc *testContext, ns string) {
	t.Helper()
	_, err := tc.ClientSet.CoreV1().Namespaces().Create(tc.Ctx,
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create namespace %s: %v", ns, err)
	}
}

// makeGPUNode builds a Ready, schedulable node in the given domain advertising
// `gpus` units of gpuResource. TaintNodesByCondition is disabled in TestMain so
// the kubelet-less node is not tainted unschedulable.
func makeGPUNode(name, domain string, gpus int64) *v1.Node {
	cap := v1.ResourceList{
		gpuResource:       *resource.NewQuantity(gpus, resource.DecimalSI),
		v1.ResourcePods:   *resource.NewQuantity(110, resource.DecimalSI),
		v1.ResourceCPU:    *resource.NewQuantity(64, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(256<<30, resource.BinarySI),
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{domainLabelKey: domain},
		},
		Status: v1.NodeStatus{
			Capacity:    cap,
			Allocatable: cap,
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionTrue, Reason: "KubeletReady"},
			},
		},
	}
}

// makeGangPod builds a gang member: labelled into pgName, requesting one gpu.
func makeGangPod(name, ns, pgName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{podGroupLabel: pgName},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "app",
				Image: "registry.k8s.io/pause:3.10",
				Resources: v1.ResourceRequirements{
					// Extended resources require limits == requests.
					Requests: v1.ResourceList{gpuResource: *resource.NewQuantity(1, resource.DecimalSI)},
					Limits:   v1.ResourceList{gpuResource: *resource.NewQuantity(1, resource.DecimalSI)},
				},
			}},
		},
	}
}

// domainOfNode returns a node's domain label (domainLabelKey).
func domainOfNode(t *testing.T, tc *testContext, node string) string {
	t.Helper()
	n, err := tc.ClientSet.CoreV1().Nodes().Get(tc.Ctx, node, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get node %s: %v", node, err)
	}
	return n.Labels[domainLabelKey]
}

// keys returns a set's members as a slice (for readable failure messages).
func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// sameDomain reports whether two single-domain sets refer to the same domain.
func sameDomain(a, b map[string]bool) bool {
	for d := range a {
		if b[d] {
			return true
		}
	}
	return false
}

// ensureNotBound asserts the pod stays unbound (no node assigned) for the whole
// window — used to prove the gang gate holds an incomplete gang rather than
// letting a member bind. A not-found pod counts as unbound.
func ensureNotBound(t *testing.T, tc *testContext, ns, name string, window time.Duration) {
	t.Helper()
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		p, err := tc.ClientSet.CoreV1().Pods(ns).Get(tc.Ctx, name, metav1.GetOptions{})
		if err == nil && p.Spec.NodeName != "" {
			t.Fatalf("pod %s/%s bound to %s while its gang was incomplete (gate should hold)", ns, name, p.Spec.NodeName)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// waitForPodBound polls until the pod has a node assigned, returning that node.
func waitForPodBound(t *testing.T, tc *testContext, ns, name string, timeout time.Duration) string {
	t.Helper()
	var node string
	err := wait.PollUntilContextTimeout(tc.Ctx, 200*time.Millisecond, timeout, true,
		func(ctx context.Context) (bool, error) {
			p, err := tc.ClientSet.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if p.Spec.NodeName != "" {
				node = p.Spec.NodeName
				return true, nil
			}
			return false, nil
		})
	if err != nil {
		t.Fatalf("pod %s/%s never bound: %v", ns, name, err)
	}
	return node
}
