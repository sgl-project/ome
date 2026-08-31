package gangpack

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/kube-scheduler/framework"
)

// BenchmarkFreeByDomain measures the all-domain projection paid when an unplaced
// gang chooses or replans a domain. Pinned members match only within their domain.
func BenchmarkFreeByDomain(b *testing.B) {
	pod := gpuPod("4")
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", n), func(b *testing.B) {
			nodes := make([]framework.NodeInfo, 0, n)
			for i := 0; i < n; i++ {
				// ~18 nodes per NVL72-style domain.
				dom := fmt.Sprintf("dom-%d", i/18)
				nodes = append(nodes, nodeInfo(gpuNode(fmt.Sprintf("n-%d", i), dom, "4")))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = freeByDomain(nodes, testKey, pod)
			}
		})
	}
}

// benchWaitingPod is a minimal WaitingPod carrying a gang-labelled pod, for the
// Permit iteration benchmark.
type benchWaitingPod struct {
	framework.WaitingPod
	pod *v1.Pod
}

func (w *benchWaitingPod) GetPod() *v1.Pod { return w.pod }
func (w *benchWaitingPod) Allow(string)    {}

// BenchmarkWaitingPodIteration measures the waiting-pod scan used to release or
// unwind same-attempt siblings. Permit readiness itself uses reservation state.
func BenchmarkWaitingPodIteration(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("waiting=%d", n), func(b *testing.B) {
			waiting := make([]framework.WaitingPod, 0, n)
			for i := 0; i < n; i++ {
				// Many distinct gangs waiting concurrently.
				waiting = append(waiting, &benchWaitingPod{pod: gangPod("team", fmt.Sprintf("pg-%d", i%256))})
			}
			h := &fakeHandle{waiting: waiting}
			g := &GangPack{handle: h}
			gangKey := "team/pg-0"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				count := 0
				g.handle.IterateOverWaitingPods(func(wp framework.WaitingPod) {
					if sameGang(wp.GetPod(), gangKey) {
						count++
					}
				})
				_ = count
			}
		})
	}
}
