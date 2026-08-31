package gangpack

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kube-scheduler/framework"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

func newCycleState() framework.CycleState { return schedulerframework.NewCycleState() }

// fakeWaitingPod is a minimal framework.WaitingPod for tests: it records whether
// Allow/Reject was called. The embedded nil interface satisfies the methods we
// don't exercise (they panic if called, which surfaces accidental use).
type fakeWaitingPod struct {
	framework.WaitingPod
	pod      *v1.Pod
	allowed  bool
	rejected bool
}

func (w *fakeWaitingPod) GetPod() *v1.Pod       { return w.pod }
func (w *fakeWaitingPod) Allow(string)          { w.allowed = true }
func (w *fakeWaitingPod) Reject(string, string) { w.rejected = true }

// fakeHandle is a minimal framework.Handle exposing the waiting-pod iteration the
// gate/unwind use, plus an optional node snapshot for the gate's bound-sibling
// scan. The embedded nil Handle satisfies the rest.
type fakeHandle struct {
	framework.Handle
	waiting   []framework.WaitingPod
	snapshot  []framework.NodeInfo // nodes (with their pods) the bound scan sees
	activated map[string]*v1.Pod
}

func (h *fakeHandle) Activate(_ klog.Logger, pods map[string]*v1.Pod) {
	if h.activated == nil {
		h.activated = make(map[string]*v1.Pod)
	}
	for key, pod := range pods {
		h.activated[key] = pod
	}
}

func (h *fakeHandle) IterateOverWaitingPods(f func(framework.WaitingPod)) {
	for _, wp := range h.waiting {
		f(wp)
	}
}

func (h *fakeHandle) SnapshotSharedLister() framework.SharedLister {
	return &fakeSharedLister{nodes: h.snapshot}
}

// fakeSharedLister / fakeNodeInfoLister expose a fixed node slice; only List is
// exercised (by boundSiblings). The rest return empty/not-found.
type fakeSharedLister struct{ nodes []framework.NodeInfo }

func (l *fakeSharedLister) NodeInfos() framework.NodeInfoLister {
	return &fakeNodeInfoLister{nodes: l.nodes}
}
func (l *fakeSharedLister) StorageInfos() framework.StorageInfoLister { return nil }

type fakeNodeInfoLister struct{ nodes []framework.NodeInfo }

func (l *fakeNodeInfoLister) List() ([]framework.NodeInfo, error) { return l.nodes, nil }
func (l *fakeNodeInfoLister) HavePodsWithAffinityList() ([]framework.NodeInfo, error) {
	return nil, nil
}
func (l *fakeNodeInfoLister) HavePodsWithRequiredAntiAffinityList() ([]framework.NodeInfo, error) {
	return nil, nil
}
func (l *fakeNodeInfoLister) Get(string) (framework.NodeInfo, error) { return nil, nil }
