package inferencereplica

// Full-loop gang migration walk through the real Reconcile dispatch
// (plan build, EnsurePodGroups, Migrate op, guarded status seams)
// against a lockstep-simulated environment: scheduler binding, kubelet
// readiness (gate-aware PodReady), the coordination layer's
// per-revision routing Service, and the endpointslice controller.
//
// The walk crosses the window the ops-level fixtures cannot reach: the
// plan releases the source index the moment the surge is promoted
// Ready, while the migration record is still Draining. The completion
// tail must keep computing the gang-shaped desired pod set from the
// surge's own plan entry — losing the shape collapses the surge to the
// single-pod fallback, renders a spurious "default" runner pod that
// can never enter the leader-only routing rotation, and parks the
// record at Draining forever.

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

const (
	gangMigSourceNode = "node-a"
	gangMigOtherNode  = "node-b"
)

// gangMigSimulate advances the simulated environment one step:
// binds unscheduled pods (the source leader to gangMigSourceNode,
// everything else — including the NotIn[source-node] surge — to
// gangMigOtherNode), flips ContainersReady, computes PodReady as
// ContainersReady AND the ome.io/serving gate (kubelet's readiness-gate
// contract), and mirrors pod state into EndpointSlices for the
// per-revision routing Service (leaders only, ready follows PodReady)
// and the component headless Service (all pods, publishNotReadyAddresses
// semantics).
func gangMigSimulate(t *testing.T, c client.Client, ns, isvcName string) {
	t.Helper()
	ctx := context.Background()
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(ns)); err != nil {
		t.Fatalf("sim list pods: %v", err)
	}

	leadersByHash := map[string][]*corev1.Pod{}
	all := []*corev1.Pod{}
	for i := range pods.Items {
		p := &pods.Items[i]
		all = append(all, p)
		if p.DeletionTimestamp != nil {
			continue
		}
		if p.Spec.NodeName == "" {
			node := gangMigOtherNode
			if p.Labels[query.LabelRunner] == "leader" && p.Labels[query.LabelInstanceIdx] == "0" {
				node = gangMigSourceNode
			}
			p.Spec.NodeName = node
			if err := c.Update(ctx, p); err != nil {
				t.Fatalf("sim bind pod: %v", err)
			}
		}
		changed := gangMigSetCond(p, corev1.ContainersReady, corev1.ConditionTrue)
		ready := corev1.ConditionFalse
		if podreadiness.IsServing(p) {
			ready = corev1.ConditionTrue
		}
		changed = gangMigSetCond(p, corev1.PodReady, ready) || changed
		p.Status.Phase = corev1.PodRunning
		if changed {
			if err := c.Status().Update(ctx, p); err != nil {
				t.Fatalf("sim kubelet status: %v", err)
			}
		}
		if p.Labels[query.LabelRunner] == "leader" {
			if hash := p.Labels[query.LabelRevisionHash]; hash != "" {
				leadersByHash[hash] = append(leadersByHash[hash], p)
			}
		}
	}

	for hash, leaders := range leadersByHash {
		svcName := query.PerRevisionServiceName(isvcName, workload.ComponentEngine, hash)
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: svcName}}
		if err := c.Get(ctx, client.ObjectKeyFromObject(svc), svc); apierrors.IsNotFound(err) {
			svc.Spec = corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 8080}}}
			if err := c.Create(ctx, svc); err != nil {
				t.Fatalf("sim create routing svc: %v", err)
			}
		}
		eps := make([]discoveryv1.Endpoint, 0, len(leaders))
		for i, p := range leaders {
			podReady := gangMigHasCond(p, corev1.PodReady, corev1.ConditionTrue)
			eps = append(eps, discoveryv1.Endpoint{
				Addresses: []string{fmt.Sprintf("10.0.0.%d", i+1)},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       ptr.To(podReady && p.DeletionTimestamp == nil),
					Serving:     ptr.To(podReady),
					Terminating: ptr.To(p.DeletionTimestamp != nil),
				},
				TargetRef: &corev1.ObjectReference{Kind: "Pod", Namespace: p.Namespace, Name: p.Name, UID: p.UID},
			})
		}
		gangMigUpsertSlice(t, c, ns, svcName, eps)
	}

	headless := query.HeadlessServiceName(isvcName, workload.ComponentEngine)
	eps := make([]discoveryv1.Endpoint, 0, len(all))
	for i, p := range all {
		eps = append(eps, discoveryv1.Endpoint{
			Addresses: []string{fmt.Sprintf("10.0.1.%d", i+1)},
			Conditions: discoveryv1.EndpointConditions{
				Ready:       ptr.To(p.DeletionTimestamp == nil),
				Serving:     ptr.To(true),
				Terminating: ptr.To(p.DeletionTimestamp != nil),
			},
			TargetRef: &corev1.ObjectReference{Kind: "Pod", Namespace: p.Namespace, Name: p.Name, UID: p.UID},
		})
	}
	gangMigUpsertSlice(t, c, ns, headless, eps)
}

func gangMigUpsertSlice(t *testing.T, c client.Client, ns, svcName string, eps []discoveryv1.Endpoint) {
	t.Helper()
	ctx := context.Background()
	name := svcName + "-sim"
	existing := &discoveryv1.EndpointSlice{}
	err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, existing)
	if apierrors.IsNotFound(err) {
		slice := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns, Name: name,
				Labels: map[string]string{discoveryv1.LabelServiceName: svcName},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   eps,
		}
		if cerr := c.Create(ctx, slice); cerr != nil {
			t.Fatalf("sim create slice: %v", cerr)
		}
		return
	}
	if err != nil {
		t.Fatalf("sim get slice: %v", err)
	}
	existing.Endpoints = eps
	if uerr := c.Update(ctx, existing); uerr != nil {
		t.Fatalf("sim update slice: %v", uerr)
	}
}

func gangMigSetCond(p *corev1.Pod, ct corev1.PodConditionType, st corev1.ConditionStatus) bool {
	for i := range p.Status.Conditions {
		if p.Status.Conditions[i].Type == ct {
			if p.Status.Conditions[i].Status == st {
				return false
			}
			p.Status.Conditions[i].Status = st
			p.Status.Conditions[i].LastTransitionTime = metav1.Now()
			return true
		}
	}
	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
		Type: ct, Status: st, LastTransitionTime: metav1.Now(),
	})
	return true
}

func gangMigHasCond(p *corev1.Pod, ct corev1.PodConditionType, st corev1.ConditionStatus) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == ct {
			return cond.Status == st
		}
	}
	return false
}

func TestReconcile_GangMigrationCompletesAfterSourcePlanRelease(t *testing.T) {
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Spec.Runners = []v1beta1.Runner{
		{Name: v1beta1.RunnerNameLeader, Size: 1, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "ome-container", Image: "test:v1",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}}},
		}}},
		{Name: v1beta1.RunnerNameWorker, Size: 1, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "ome-container", Image: "test:v1",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}}}},
		}}},
	}
	r, c := newReconciler(t, ir)
	r.GangSchedulingAvailable = true
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ir)}
	ctx := context.Background()

	get := func() *v1beta1.InferenceReplica {
		fresh := &v1beta1.InferenceReplica{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(ir), fresh); err != nil {
			t.Fatalf("get IR: %v", err)
		}
		return fresh
	}
	// One reconcile + one environment step; expectations are reset first
	// (a fresh cache reads as satisfied — the informer-caught-up state).
	step := func(tag string) {
		r.Expectations = workload.NewExpectations()
		if _, err := r.Reconcile(ctx, request); err != nil {
			t.Fatalf("%s reconcile: %v", tag, err)
		}
		gangMigSimulate(t, c, ir.Namespace, ir.Spec.ParentRef.Name)
	}

	for i := 0; i < 10; i++ {
		step("startup")
	}
	fresh := get()
	if len(fresh.Status.InstanceStatuses) != 1 || fresh.Status.InstanceStatuses[0].Phase != v1beta1.OMENativeInstanceReady {
		t.Fatalf("gang did not reach Ready: %+v", fresh.Status.InstanceStatuses)
	}

	// Accept-shaped record: migrate the gang off the leader's node.
	fresh.Status.Migrations = []v1beta1.MigrationStatus{{
		RequestUUID:    "mig-gang-release",
		Trigger:        v1beta1.MigrationTriggerManual,
		Phase:          v1beta1.MigrationPhaseAccepted,
		SourceInstance: 0,
		FromNode:       gangMigSourceNode,
		Reason:         "test",
		StartedAt:      metav1.Now(),
		Deadline:       metav1.NewTime(time.Now().Add(30 * time.Minute)),
	}}
	if err := c.Status().Update(ctx, fresh); err != nil {
		t.Fatalf("seed migration record: %v", err)
	}

	completed := false
	for i := 0; i < 30 && !completed; i++ {
		step("migration")
		got := get()
		if len(got.Status.Migrations) != 1 {
			t.Fatalf("migration record count: %+v", got.Status.Migrations)
		}
		switch got.Status.Migrations[0].Phase {
		case v1beta1.MigrationPhaseCompleted:
			completed = true
		case v1beta1.MigrationPhaseFailed:
			t.Fatalf("migration failed: %+v", got.Status.Migrations[0])
		}
	}
	if !completed {
		got := get()
		t.Fatalf("migration never completed: record=%+v instances=%+v",
			got.Status.Migrations[0], got.Status.InstanceStatuses)
	}

	// Exactly the promoted surge survives, unpinned.
	got := get()
	if len(got.Status.InstanceStatuses) != 1 {
		t.Fatalf("instance statuses after completion: %+v", got.Status.InstanceStatuses)
	}
	surge := got.Status.InstanceStatuses[0]
	if surge.Index == 0 || surge.Phase != v1beta1.OMENativeInstanceReady || surge.Operation != nil {
		t.Fatalf("promoted surge shape: %+v", surge)
	}

	// The surge kept the gang shape end to end: one leader + one worker,
	// no single-pod-fallback "default" runner pod, and no source pod left.
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(ir.Namespace)); err != nil {
		t.Fatalf("list pods: %v", err)
	}
	runners := map[string]int{}
	for i := range pods.Items {
		p := pods.Items[i]
		if p.Labels[query.LabelInstanceIdx] == "0" {
			t.Fatalf("source gang pod survived completion: %s", p.Name)
		}
		runners[p.Labels[query.LabelRunner]]++
	}
	if runners["default"] != 0 || runners["leader"] != 1 || runners["worker"] != 1 {
		t.Fatalf("surge runner layout: %+v", runners)
	}
}
