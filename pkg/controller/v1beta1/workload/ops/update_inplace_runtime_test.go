package ops

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestPodRuntimeImageChangesMatch(t *testing.T) {
	spec := func(main, helper string) *corev1.PodSpec {
		containers := []corev1.Container{{Name: "main", Image: main}}
		if helper != "" {
			containers = append(containers, corev1.Container{Name: "helper", Image: helper})
		}
		return &corev1.PodSpec{Containers: containers}
	}
	pod := func(main, helper string) *corev1.Pod {
		statuses := []corev1.ContainerStatus{{
			Name: "main", Image: main, ContainerID: "containerd://main", RestartCount: 3,
		}}
		if helper != "" {
			statuses = append(statuses, corev1.ContainerStatus{Name: "helper", Image: helper})
		}
		return &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: statuses}}
	}

	tests := []struct {
		name    string
		pod     *corev1.Pod
		running *corev1.PodSpec
		target  *corev1.PodSpec
		want    bool
	}{
		{
			name:    "metadata-only ignores runtime alias",
			pod:     pod("mirror.example.com/app:v1", ""),
			running: spec("example.com/app:v1", ""),
			target:  spec("example.com/app:v1", ""),
			want:    true,
		},
		{
			name:    "changed image matches while unchanged alias is ignored",
			pod:     pod("example.com/app:v2", "mirror.example.com/helper:v1"),
			running: spec("example.com/app:v1", "example.com/helper:v1"),
			target:  spec("example.com/app:v2", "example.com/helper:v1"),
			want:    true,
		},
		{
			name:    "changed image remains stale",
			pod:     pod("example.com/app:v1", "mirror.example.com/helper:v1"),
			running: spec("example.com/app:v1", "example.com/helper:v1"),
			target:  spec("example.com/app:v2", "example.com/helper:v1"),
		},
		{
			name:    "one of two changed images remains stale",
			pod:     pod("example.com/app:v2", "example.com/helper:v1"),
			running: spec("example.com/app:v1", "example.com/helper:v1"),
			target:  spec("example.com/app:v2", "example.com/helper:v2"),
		},
		{
			name:   "missing running revision checks every target image",
			pod:    pod("example.com/app:v2", "example.com/helper:v2"),
			target: spec("example.com/app:v2", "example.com/helper:v2"),
			want:   true,
		},
		{
			name:   "missing running revision rejects stale runtime",
			pod:    pod("example.com/app:v1", ""),
			target: spec("example.com/app:v2", ""),
		},
		{
			name:    "retarget does not accept prior target runtime",
			pod:     pod("example.com/app:v2", ""),
			running: spec("example.com/app:v1", ""),
			target:  spec("example.com/app:v3", ""),
		},
		{name: "nil pod", running: spec("example.com/app:v1", ""), target: spec("example.com/app:v1", "")},
		{name: "nil target", pod: pod("example.com/app:v1", ""), running: spec("example.com/app:v1", "")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := podRuntimeImageChangesMatch(test.pod, test.running, test.target); got != test.want {
				t.Fatalf("podRuntimeImageChangesMatch() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPodRuntimeImageChangesMatchDoesNotUseRestartAsTargetProof(t *testing.T) {
	running := &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "example.com/app:v1"}}}
	target := &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "example.com/app:v2"}}}

	for _, observedGeneration := range []int64{0, 6, 7} {
		t.Run(fmt.Sprintf("observed-generation-%d", observedGeneration), func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Generation: 7},
				Spec:       *target.DeepCopy(),
				Status: corev1.PodStatus{
					ObservedGeneration: observedGeneration,
					ContainerStatuses: []corev1.ContainerStatus{{
						Name: "main", Image: "example.com/app:v1",
						ContainerID: "containerd://restarted-old-image", RestartCount: 4,
					}},
				},
			}
			if podRuntimeImageChangesMatch(pod, running, target) {
				t.Fatal("old runtime image was accepted after an unrelated restart")
			}
		})
	}
}

func TestPatchPodImagesRejectsStaleResourceVersion(t *testing.T) {
	isvc, _ := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	pod := legacyPodAtIncarnation(isvc, 0, 1, true, true)
	pod.Spec.Containers = []corev1.Container{{Name: "main", Image: "llama:v1"}}
	c := legacyNewFakeClient(t, pod)

	stale := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stale); err != nil {
		t.Fatalf("get stale pod: %v", err)
	}
	concurrent := stale.DeepCopy()
	concurrent.Annotations = map[string]string{"example.com/concurrent": "write"}
	if err := c.Update(context.Background(), concurrent); err != nil {
		t.Fatalf("concurrent update: %v", err)
	}

	issued, err := patchPodImages(context.Background(), c, stale, &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "main", Image: "llama:v2"}},
	})
	if !apierrors.IsConflict(err) {
		t.Fatalf("patchPodImages error = %v, want conflict", err)
	}
	if issued {
		t.Fatal("conflicted image patch reported issued=true")
	}
}

func TestUpdateInPlaceMetadataRollRepairsImageDriftBeforePromotion(t *testing.T) {
	legacyResetExpectations(t)
	isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	spec := legacyTargetSpecImage("example.com/app:v1")
	pod := legacyRunningPodAtRevision(isvc, 0, 1, "example.com/app:drift")
	pod.Annotations = map[string]string{"release": "one"}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "main", Image: "example.com/app:drift", ContainerID: "containerd://drift",
	}}
	c := legacyNewFakeClient(t, isvc, ir, pod)
	legacySeedRunningRevisionWithMeta(t, c, isvc, workload.ComponentEngine, 0, spec,
		&metav1.ObjectMeta{Annotations: map[string]string{"release": "one"}})
	target := legacyEnsureTargetCRWithMeta(t, c, isvc, spec,
		&metav1.ObjectMeta{Annotations: map[string]string{"release": "two"}})
	markNotReady := false
	plan := legacyComponentPlan(workload.UpdateStrategyInPlaceIfPossible,
		&workload.InPlaceUpdateStrategy{MarkNotReadyDuringLifecycle: &markNotReady})
	run := func(pass string) bool {
		t.Helper()
		done, err := Update(context.Background(), legacyTestDeps(c), legacyTestInput(isvc, c, workload.ComponentEngine), plan, plan.Instances[0], target, spec)
		if err != nil {
			t.Fatalf("%s Update: %v", pass, err)
		}
		return done
	}
	getPod := func() *corev1.Pod {
		t.Helper()
		got := &corev1.Pod{}
		if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
			t.Fatalf("get pod: %v", err)
		}
		return got
	}

	if run("record image transition") {
		t.Fatal("marker pass returned done=true")
	}
	marked := getPod()
	transition, present, valid := inPlaceImageTransitionFromPod(marked)
	if !present || !valid || transition.TargetImages["main"] != "example.com/app:v1" {
		t.Fatalf("transition marker: %+v present=%v valid=%v", transition, present, valid)
	}
	if marked.Spec.Containers[0].Image != "example.com/app:drift" {
		t.Fatal("image changed before the write-ahead marker was observed")
	}

	if run("patch image") {
		t.Fatal("image patch pass returned done=true")
	}
	patched := getPod()
	if patched.Spec.Containers[0].Image != "example.com/app:v1" {
		t.Fatalf("patched image = %q, want example.com/app:v1", patched.Spec.Containers[0].Image)
	}
	if patched.Status.ContainerStatuses[0].Image != "example.com/app:drift" {
		t.Fatal("fake kubelet unexpectedly advanced runtime status")
	}

	if run("stale runtime") {
		t.Fatal("stale ContainersReady and old runtime image promoted the Instance")
	}
	waiting := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if waiting.Phase != v1beta1.OMENativeInstanceUpdating || waiting.RunningRevision == target.Name {
		t.Fatalf("status advanced before runtime confirmation: %+v", waiting)
	}

	patched = getPod()
	patched.Status.ContainerStatuses[0].Image = "example.com/app:v1"
	patched.Status.ContainerStatuses[0].ContainerID = "containerd://target"
	if err := c.Status().Update(context.Background(), patched); err != nil {
		t.Fatalf("advance runtime status: %v", err)
	}
	if run("clear runtime proof") {
		t.Fatal("marker removal pass returned done=true")
	}
	cleared, present, valid := inPlaceImageTransitionFromPod(getPod())
	if present || valid || cleared != nil {
		t.Fatalf("transition remained after runtime proof: %+v present=%v valid=%v", cleared, present, valid)
	}

	if !run("promote") {
		t.Fatal("confirmed transition did not promote")
	}
	final := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if final.Phase != v1beta1.OMENativeInstanceReady || final.RunningRevision != target.Name || final.Operation != nil {
		t.Fatalf("final status: %+v", final)
	}
}

func TestInPlaceImageTransitionRetargetsUnconfirmedImages(t *testing.T) {
	isvc, _ := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	pod := legacyPodAtIncarnation(isvc, 0, 1, true, true)
	pod.Spec.Containers = []corev1.Container{{Name: "main", Image: "example.com/app:v1"}}
	pod.Annotations = map[string]string{inPlaceImageTransitionAnnotation: `{"targetImages":{"main":"example.com/app:v2"}}`}
	c := legacyNewFakeClient(t, pod)
	stored := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stored); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	target := &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "example.com/app:v3"}}}
	ready, err := ensureInPlaceImageTransition(context.Background(), c, stored, target, map[string]string{"main": "example.com/app:v3"})
	if err != nil || ready {
		t.Fatalf("retarget marker: ready=%v err=%v", ready, err)
	}
	got := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
		t.Fatalf("get retargeted pod: %v", err)
	}
	transition, present, valid := inPlaceImageTransitionFromPod(got)
	if !present || !valid || transition.TargetImages["main"] != "example.com/app:v3" {
		t.Fatalf("retargeted transition: %+v present=%v valid=%v", transition, present, valid)
	}
}

func TestInPlaceImageTransitionInvalidMarkersFailClosed(t *testing.T) {
	target := &corev1.PodSpec{Containers: []corev1.Container{
		{Name: "main", Image: "example.com/app:v2"},
		{Name: "helper", Image: "example.com/helper:v1"},
	}}
	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed JSON", raw: "{"},
		{name: "empty target set", raw: `{"targetImages":{}}`},
		{name: "removed container", raw: `{"targetImages":{"removed":"example.com/removed:v1"}}`},
		{name: "stale target value", raw: `{"targetImages":{"main":"example.com/app:v1"}}`},
		{name: "forged confirmed field", raw: `{"targetImages":{"main":"example.com/app:v2"},"confirmed":true}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isvc, _ := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
			pod := legacyPodAtIncarnation(isvc, 0, 1, true, true)
			pod.Spec = *target.DeepCopy()
			pod.Annotations = map[string]string{inPlaceImageTransitionAnnotation: test.raw}
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{Name: "main", Image: "mirror.example.com/app:v2"},
				{Name: "helper", Image: "mirror.example.com/helper:v1"},
			}
			c := legacyNewFakeClient(t, pod)
			stored := &corev1.Pod{}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stored); err != nil {
				t.Fatalf("get pod: %v", err)
			}

			ready, err := ensureInPlaceImageTransition(context.Background(), c, stored, target, nil)
			if err != nil {
				t.Fatalf("ensure transition: %v", err)
			}
			if test.name == "forged confirmed field" && !ready {
				t.Fatal("unknown confirmed field changed an otherwise current pending marker")
			}
			if test.name != "forged confirmed field" && ready {
				t.Fatal("invalid or stale marker was accepted without a corrective write")
			}

			got := &corev1.Pod{}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
				t.Fatalf("get ensured pod: %v", err)
			}
			transition, present, valid := inPlaceImageTransitionFromPod(got)
			if !present || !valid || !inPlaceImageTransitionMatchesTarget(transition, target) {
				t.Fatalf("ensured transition: %+v present=%v valid=%v", transition, present, valid)
			}
			if inPlaceImageTransitionRuntimeMatches(got, transition) {
				t.Fatal("runtime aliases satisfied a pending exact-image proof")
			}
		})
	}
}

func TestRemoveInPlaceImageTransitionRejectsStaleResourceVersion(t *testing.T) {
	isvc, _ := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	pod := legacyPodAtIncarnation(isvc, 0, 1, true, true)
	pod.Annotations = map[string]string{inPlaceImageTransitionAnnotation: `{"targetImages":{"main":"example.com/app:v2"}}`}
	c := legacyNewFakeClient(t, pod)

	stale := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stale); err != nil {
		t.Fatalf("get stale pod: %v", err)
	}
	concurrent := stale.DeepCopy()
	concurrent.Annotations["example.com/concurrent"] = "write"
	if err := c.Update(context.Background(), concurrent); err != nil {
		t.Fatalf("concurrent update: %v", err)
	}

	if err := removeInPlaceImageTransition(context.Background(), c, stale); !apierrors.IsConflict(err) {
		t.Fatalf("removeInPlaceImageTransition error = %v, want conflict", err)
	}
	got := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if _, present := got.Annotations[inPlaceImageTransitionAnnotation]; !present {
		t.Fatal("conflicted removal deleted the transition marker")
	}
	if got.Annotations["example.com/concurrent"] != "write" {
		t.Fatal("conflicted removal lost a concurrent annotation")
	}
}

func TestAnnotationsDiffReservesInPlaceImageTransition(t *testing.T) {
	podAnnotations := map[string]string{
		inPlaceImageTransitionAnnotation: "controller-state",
		"example.com/release":            "one",
	}
	previous := map[string]string{
		inPlaceImageTransitionAnnotation: "old-user-value",
		"example.com/release":            "one",
	}
	target := map[string]string{
		inPlaceImageTransitionAnnotation: "forged-user-value",
		"example.com/release":            "two",
	}
	diff := annotationsDiff(podAnnotations, previous, target)
	if _, found := diff[inPlaceImageTransitionAnnotation]; found {
		t.Fatalf("reserved annotation entered PodMeta diff: %#v", diff)
	}
	if diff["example.com/release"] != "two" {
		t.Fatalf("ordinary annotation diff was lost: %#v", diff)
	}
}

type stalePodListClient struct {
	client.Client
	pod *corev1.Pod
}

func (c *stalePodListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	pods, ok := list.(*corev1.PodList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}
	pods.Items = []corev1.Pod{*c.pod.DeepCopy()}
	return nil
}

func TestUpdateInPlaceUsesLivePodForOptimisticImagePatch(t *testing.T) {
	legacyResetExpectations(t)
	isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	pod := legacyRunningPodAtRevision(isvc, 0, 1, "llama:v1")
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "main", Image: "llama:v1"}}
	live := legacyNewFakeClient(t, isvc, ir, pod)
	legacySeedRunningRevision(t, live, isvc, workload.ComponentEngine, 0, legacyTargetSpecImage("llama:v1"))
	targetSpec := legacyTargetSpecImage("llama:v2")
	target := legacyEnsureTargetCR(t, live, isvc, targetSpec)

	stale := &corev1.Pod{}
	if err := live.Get(context.Background(), client.ObjectKeyFromObject(pod), stale); err != nil {
		t.Fatalf("get stale pod: %v", err)
	}
	concurrent := stale.DeepCopy()
	concurrent.Annotations = map[string]string{"example.com/concurrent": "write"}
	if err := live.Update(context.Background(), concurrent); err != nil {
		t.Fatalf("concurrent pod update: %v", err)
	}

	cached := &stalePodListClient{Client: live, pod: stale}
	deps := legacyTestDeps(cached)
	deps.APIReader = live
	input := legacyTestInput(isvc, cached, workload.ComponentEngine)
	markNotReady := false
	plan := legacyComponentPlan(workload.UpdateStrategyInPlaceIfPossible,
		&workload.InPlaceUpdateStrategy{MarkNotReadyDuringLifecycle: &markNotReady})

	for pass := 1; pass <= 2; pass++ {
		if _, err := Update(context.Background(), deps, input, plan, plan.Instances[0], target, targetSpec); err != nil {
			t.Fatalf("Update pass %d: %v", pass, err)
		}
	}
	got := &corev1.Pod{}
	if err := live.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
		t.Fatalf("get patched pod: %v", err)
	}
	if got.Spec.Containers[0].Image != "llama:v2" {
		t.Fatalf("image = %q, want llama:v2", got.Spec.Containers[0].Image)
	}
	if got.Annotations["example.com/concurrent"] != "write" {
		t.Fatal("concurrent annotation was lost")
	}
}
