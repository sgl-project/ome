package fakeclient_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	schedulingv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func configMap(name, value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Data:       map[string]string{"k": value},
	}
}

func pod(name, node string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec:       corev1.PodSpec{NodeName: node},
	}
}

func TestNew_Objects_AreReadable(t *testing.T) {
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm))
	got := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(cm), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data["k"] != "v" {
		t.Errorf("data = %v, want k=v", got.Data)
	}
}

func TestNew_StatusSubresource_SplitsUpdateFromStatusUpdate(t *testing.T) {
	p := pod("a", "")
	c := fakeclient.New(t, fakeclient.WithObjects(p), fakeclient.WithStatusSubresource(&corev1.Pod{}))
	ctx := context.Background()

	p.Status.Phase = corev1.PodRunning
	if err := c.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	got := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(p), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != "" {
		t.Errorf("Update wrote the status: phase = %q", got.Status.Phase)
	}

	got.Status.Phase = corev1.PodRunning
	if err := c.Status().Update(ctx, got); err != nil {
		t.Fatalf("status update: %v", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(p), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status.Phase != corev1.PodRunning {
		t.Errorf("Status().Update did not write the status: phase = %q", got.Status.Phase)
	}
}

func TestNew_Index_AnswersAFieldSelector(t *testing.T) {
	const field = "spec.nodeName"
	c := fakeclient.New(t,
		fakeclient.WithObjects(pod("a", "node-1"), pod("b", "node-2")),
		fakeclient.WithIndex(&corev1.Pod{}, field, func(o client.Object) []string {
			return []string{o.(*corev1.Pod).Spec.NodeName}
		}),
	)
	list := &corev1.PodList{}
	if err := c.List(context.Background(), list, client.MatchingFields{field: "node-1"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "a" {
		t.Errorf("list = %d items, want only pod a", len(list.Items))
	}
}

func TestNew_SchemeHandedIn_IsBuiltOnAndExtended(t *testing.T) {
	s := fakeclient.Scheme(t)
	pg := &schedulingv1alpha1.PodGroup{ObjectMeta: metav1.ObjectMeta{Name: "g", Namespace: "ns"}}
	c := fakeclient.New(t,
		fakeclient.WithScheme(s),
		fakeclient.WithTypes(schedulingv1alpha1.AddToScheme),
		fakeclient.WithObjects(pg),
	)
	if !knows(s, pg) {
		t.Error("WithTypes did not add the group to the scheme handed in")
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pg), &schedulingv1alpha1.PodGroup{}); err != nil {
		t.Errorf("get through the extended scheme: %v", err)
	}
}

func TestNew_RawInterceptor_RunsInsideTheCounter(t *testing.T) {
	hook := errors.New("hook answered")
	gets := fakeclient.CountingVerb(fakeclient.Get)
	c := fakeclient.New(t,
		fakeclient.WithInterceptors(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return hook
			},
		}),
		gets,
	)
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "a"}, &corev1.ConfigMap{})
	if !errors.Is(err, hook) {
		t.Fatalf("get = %v, want the hook's error", err)
	}
	if gets.Count() != 1 {
		t.Errorf("gets = %d, want the hook's call counted", gets.Count())
	}
}

func TestNew_NoBehaviour_IsNotFoundOnAnEmptyStore(t *testing.T) {
	c := fakeclient.New(t)
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "a"}, &corev1.ConfigMap{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("get = %v, want NotFound", err)
	}
}

// fatalRecorder stands in for the test so that a fatal inside New can be
// read instead of ending this test.
type fatalRecorder struct {
	testing.TB
	msg string
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func fatalOf(t *testing.T, build func(tb testing.TB)) string {
	t.Helper()
	rec := &fatalRecorder{TB: t}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		build(rec)
	}()
	wg.Wait()
	return rec.msg
}

func TestNew_UnknownVerb_FailsTheTest(t *testing.T) {
	msg := fatalOf(t, func(tb testing.TB) {
		fakeclient.New(tb, fakeclient.CountingVerb("Watch"))
	})
	if msg == "" {
		t.Fatal("a counter on a verb no hook serves was accepted")
	}
}

func TestNew_NegativeCallNumber_FailsTheTest(t *testing.T) {
	msg := fatalOf(t, func(tb testing.TB) {
		fakeclient.New(tb, fakeclient.FailingVerb(fakeclient.Get, -1, errors.New("never")))
	})
	if msg == "" {
		t.Fatal("a refusing rule on call -1 was accepted")
	}
}

func TestWrap_View_ReadsAndWritesTheBaseStore(t *testing.T) {
	cm := configMap("a", "v")
	base := fakeclient.New(t, fakeclient.WithObjects(cm))
	refused := errors.New("cache down")
	gets := fakeclient.CountingVerb(fakeclient.Get)
	view := fakeclient.Wrap(t, base, gets,
		fakeclient.FailingVerb(fakeclient.Get, fakeclient.EveryCall, refused).OnKind(&corev1.Pod{}))
	ctx := context.Background()

	got := &corev1.ConfigMap{}
	if err := view.Get(ctx, client.ObjectKeyFromObject(cm), got); err != nil || got.Data["k"] != "v" {
		t.Fatalf("get through the view = %v, %v; want the base's object", got.Data, err)
	}
	podKey := client.ObjectKey{Namespace: "ns", Name: "p"}
	if err := view.Get(ctx, podKey, &corev1.Pod{}); !errors.Is(err, refused) {
		t.Errorf("pod get through the view = %v, want the refusal", err)
	}
	if err := view.Create(ctx, pod("p", "")); err != nil {
		t.Fatalf("create through the view: %v", err)
	}
	if err := base.Get(ctx, podKey, &corev1.Pod{}); err != nil {
		t.Errorf("the base does not see a write made through the view: %v", err)
	}
	if gets.Count() != 2 {
		t.Errorf("the view counted %d gets, want 2 (the refused one included)", gets.Count())
	}
}

func TestWrap_TwoViews_CountTheirOwnCalls(t *testing.T) {
	base := fakeclient.New(t)
	cacheGets := fakeclient.CountingVerb(fakeclient.Get)
	cache := fakeclient.Wrap(t, base, cacheGets)
	liveGets := fakeclient.CountingVerb(fakeclient.Get)
	live := fakeclient.Wrap(t, base, liveGets)
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "ns", Name: "a"}

	_ = cache.Get(ctx, key, &corev1.ConfigMap{})
	_ = live.Get(ctx, key, &corev1.ConfigMap{})
	_ = live.Get(ctx, key, &corev1.ConfigMap{})
	_ = base.Get(ctx, key, &corev1.ConfigMap{})
	if cacheGets.Count() != 1 || liveGets.Count() != 2 {
		t.Errorf("cache saw %d gets and live %d, want 1 and 2; the base's own call counts nowhere", cacheGets.Count(), liveGets.Count())
	}
}

func TestWrap_StoreOption_FailsTheTest(t *testing.T) {
	base := fakeclient.New(t)
	msg := fatalOf(t, func(tb testing.TB) {
		fakeclient.Wrap(tb, base, fakeclient.CountingVerb(fakeclient.Get), fakeclient.WithObjects(configMap("a", "v")))
	})
	if !strings.Contains(msg, "WithObjects") {
		t.Fatalf("an option that shapes the store was accepted by Wrap: %q", msg)
	}
}
