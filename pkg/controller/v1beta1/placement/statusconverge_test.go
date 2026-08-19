package placement

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const testControlPlaneID = "cp-alpha"

// derivedISVC builds a remote derived InferenceService carrying the given origin
// and control-plane markers, for the resolver tests.
func derivedISVC(ns, name, originUID, controlPlane string, originViaAnnotation bool) *v1beta1.InferenceService {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns,
			Name:        name,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	if originUID != "" {
		if originViaAnnotation {
			isvc.Annotations[PlacementOriginUIDAnnotation] = originUID
		} else {
			isvc.Labels[PlacementOriginLabel] = originUID
		}
	}
	if controlPlane != "" {
		isvc.Labels[PlacementControlPlaneLabel] = controlPlane
	}
	return isvc
}

func TestLocalKeyForDerived(t *testing.T) {
	cases := []struct {
		name           string
		obj            *v1beta1.InferenceService
		controlPlaneID string
		wantOK         bool
		wantNS         string
		wantName       string
	}{
		{
			name:           "origin label, no control-plane filter",
			obj:            derivedISVC("team-a", "llama", "uid-1", "", false),
			controlPlaneID: "",
			wantOK:         true, wantNS: "team-a", wantName: "llama",
		},
		{
			name:           "origin via annotation only",
			obj:            derivedISVC("team-a", "llama", "uid-1", "", true),
			controlPlaneID: "",
			wantOK:         true, wantNS: "team-a", wantName: "llama",
		},
		{
			name:           "origin label + matching control plane",
			obj:            derivedISVC("team-b", "qwen", "uid-2", testControlPlaneID, false),
			controlPlaneID: testControlPlaneID,
			wantOK:         true, wantNS: "team-b", wantName: "qwen",
		},
		{
			name:           "no origin marker is rejected (user's same-named ISVC)",
			obj:            derivedISVC("team-a", "llama", "", "", false),
			controlPlaneID: "",
			wantOK:         false,
		},
		{
			name:           "control-plane mismatch is rejected (another CP's derived)",
			obj:            derivedISVC("team-b", "qwen", "uid-2", "cp-beta", false),
			controlPlaneID: testControlPlaneID,
			wantOK:         false,
		},
		{
			name:           "origin present but control-plane label missing under configured id is rejected",
			obj:            derivedISVC("team-b", "qwen", "uid-2", "", false),
			controlPlaneID: testControlPlaneID,
			wantOK:         false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := localKeyForDerived(tc.obj, tc.controlPlaneID)
			if ok != tc.wantOK {
				t.Fatalf("localKeyForDerived ok=%v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if key.Namespace != tc.wantNS || key.Name != tc.wantName {
				t.Errorf("localKeyForDerived key=%v, want %s/%s", key, tc.wantNS, tc.wantName)
			}
		})
	}
}

// TestLocalKeyForDerived_NilObject: a nil object must be rejected, never panic.
func TestLocalKeyForDerived_NilObject(t *testing.T) {
	if _, ok := localKeyForDerived(nil, ""); ok {
		t.Errorf("localKeyForDerived(nil) ok=true, want false")
	}
}

// TestOriginWatchSelector_Matches: the watch selector must match a derived this
// control plane owns and reject one it does not, so the remote apiserver streams
// only our objects.
func TestOriginWatchSelector_Matches(t *testing.T) {
	cases := []struct {
		name           string
		controlPlaneID string
		objLabels      map[string]string
		want           bool
	}{
		{
			name:           "no id: origin label present matches",
			controlPlaneID: "",
			objLabels:      map[string]string{PlacementOriginLabel: "uid-1"},
			want:           true,
		},
		{
			name:           "no id: missing origin label does not match",
			controlPlaneID: "",
			objLabels:      map[string]string{},
			want:           false,
		},
		{
			name:           "with id: origin + matching control plane matches",
			controlPlaneID: testControlPlaneID,
			objLabels:      map[string]string{PlacementOriginLabel: "uid-1", PlacementControlPlaneLabel: testControlPlaneID},
			want:           true,
		},
		{
			name:           "with id: origin but wrong control plane does not match",
			controlPlaneID: testControlPlaneID,
			objLabels:      map[string]string{PlacementOriginLabel: "uid-1", PlacementControlPlaneLabel: "cp-beta"},
			want:           false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := originWatchSelector(tc.controlPlaneID)
			if got := sel.Matches(labels.Set(tc.objLabels)); got != tc.want {
				t.Errorf("originWatchSelector(%q).Matches(%v) = %v, want %v", tc.controlPlaneID, tc.objLabels, got, tc.want)
			}
		})
	}
}

func TestFunnelConfigFor(t *testing.T) {
	cfg := FunnelConfigFor(testControlPlaneID)
	if cfg.NewList == nil || cfg.NewObject == nil || cfg.Resolve == nil || cfg.WatchSelector == nil {
		t.Fatalf("FunnelConfigFor returned an incomplete config: %+v", cfg)
	}
	if _, isList := cfg.NewList().(*v1beta1.InferenceServiceList); !isList {
		t.Errorf("NewList did not return an InferenceServiceList")
	}
	if _, isObj := cfg.NewObject().(*v1beta1.InferenceService); !isObj {
		t.Errorf("NewObject did not return an InferenceService")
	}
	// Resolve must honor the bound control-plane id.
	owned := derivedISVC("t", "m", "uid-9", testControlPlaneID, false)
	if key, ok := cfg.Resolve(owned); !ok || key.Name != "m" {
		t.Errorf("Resolve(owned) = %v,%v; want t/m,true", key, ok)
	}
	foreign := derivedISVC("t", "m", "uid-9", "cp-other", false)
	if _, ok := cfg.Resolve(foreign); ok {
		t.Errorf("Resolve(foreign derived) ok=true, want false")
	}
}

// TestResolveConvergeConfig_Defaults: unset timing options fall back to the
// package defaults; supplied options win.
func TestResolveConvergeConfig_Defaults(t *testing.T) {
	def := resolveConvergeConfig()
	if def.batchPeriod != DefaultStatusBatchPeriod {
		t.Errorf("default batchPeriod = %v, want %v", def.batchPeriod, DefaultStatusBatchPeriod)
	}
	if def.safetyRequeue != DefaultStatusSafetyRequeue {
		t.Errorf("default safetyRequeue = %v, want %v", def.safetyRequeue, DefaultStatusSafetyRequeue)
	}
	if def.statusEvents != nil {
		t.Errorf("default statusEvents = non-nil, want nil")
	}

	ch := make(chan event.GenericEvent)
	got := resolveConvergeConfig(
		WithStatusEvents(ch),
		WithStatusBatchPeriod(7*time.Millisecond),
		WithStatusSafetyRequeue(42*time.Second),
	)
	if got.batchPeriod != 7*time.Millisecond {
		t.Errorf("batchPeriod = %v, want 7ms", got.batchPeriod)
	}
	if got.safetyRequeue != 42*time.Second {
		t.Errorf("safetyRequeue = %v, want 42s", got.safetyRequeue)
	}
	if got.statusEvents == nil {
		t.Errorf("statusEvents = nil, want the wired channel")
	}
}

// TestSafetyRequeue_FallbackChain: with no convergeConfig, the safety requeue
// honors the seeded Requeue struct field, then the package default.
func TestSafetyRequeue_FallbackChain(t *testing.T) {
	// No config, no struct field: package default.
	r := &Reconciler{}
	if got := r.safetyRequeue(); got != DefaultStatusSafetyRequeue {
		t.Errorf("safetyRequeue() with nothing set = %v, want %v", got, DefaultStatusSafetyRequeue)
	}
	// No config, struct field set: struct field wins (legacy API compatibility).
	r2 := &Reconciler{Requeue: 17 * time.Second}
	if got := r2.safetyRequeue(); got != 17*time.Second {
		t.Errorf("safetyRequeue() with Requeue set = %v, want 17s", got)
	}
	// Resolved config wins over the struct field.
	cfg := resolveConvergeConfig(WithStatusSafetyRequeue(99 * time.Second))
	r3 := &Reconciler{Requeue: 17 * time.Second, converge: &cfg}
	if got := r3.safetyRequeue(); got != 99*time.Second {
		t.Errorf("safetyRequeue() with config = %v, want 99s", got)
	}
}

// recordingQueue captures Add / AddAfter calls. Only those two methods are
// exercised by statusEventHandler, so the embedded (nil) interface is never
// dereferenced — same idiom as the stub clients elsewhere in this tree.
type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[ctrl.Request]
	added      []ctrl.Request
	addedAfter map[ctrl.Request]time.Duration
}

func newRecordingQueue() *recordingQueue {
	return &recordingQueue{addedAfter: map[ctrl.Request]time.Duration{}}
}

func (q *recordingQueue) Add(item ctrl.Request) {
	q.added = append(q.added, item)
}

func (q *recordingQueue) AddAfter(item ctrl.Request, d time.Duration) {
	q.addedAfter[item] = d
}

// TestStatusEventHandler_BatchesWithAddAfter: a funnel event enqueues the local
// key via AddAfter with the configured batch period (debounce), NOT an immediate
// Add, so a burst of derived-status updates folds into one reconcile.
func TestStatusEventHandler_BatchesWithAddAfter(t *testing.T) {
	r := &Reconciler{}
	const batch = 250 * time.Millisecond
	h := r.statusEventHandler(batch)

	q := newRecordingQueue()
	ev := event.GenericEvent{Object: &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "llama"},
	}}
	h.Generic(context.Background(), ev, q)

	if len(q.added) != 0 {
		t.Errorf("expected no immediate Add when batching, got %v", q.added)
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "llama"}}
	got, ok := q.addedAfter[req]
	if !ok {
		t.Fatalf("expected AddAfter for %v, got %v", req, q.addedAfter)
	}
	if got != batch {
		t.Errorf("AddAfter delay = %v, want %v", got, batch)
	}
}

// TestStatusEventHandler_ImmediateWhenNoBatch: a zero batch period enqueues
// immediately via Add (no debounce).
func TestStatusEventHandler_ImmediateWhenNoBatch(t *testing.T) {
	r := &Reconciler{}
	h := r.statusEventHandler(0)

	q := newRecordingQueue()
	ev := event.GenericEvent{Object: &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "llama"},
	}}
	h.Generic(context.Background(), ev, q)

	if len(q.addedAfter) != 0 {
		t.Errorf("expected no AddAfter when batch=0, got %v", q.addedAfter)
	}
	if len(q.added) != 1 || q.added[0].Namespace != "team-a" || q.added[0].Name != "llama" {
		t.Errorf("expected immediate Add of team-a/llama, got %v", q.added)
	}
}

// TestStatusEventHandler_NilObject: a malformed event with no object must be
// dropped, never panic or enqueue an empty key.
func TestStatusEventHandler_NilObject(t *testing.T) {
	r := &Reconciler{}
	h := r.statusEventHandler(time.Second)
	q := newRecordingQueue()
	h.Generic(context.Background(), event.GenericEvent{}, q)
	if len(q.added) != 0 || len(q.addedAfter) != 0 {
		t.Errorf("nil-object event must not enqueue; added=%v addedAfter=%v", q.added, q.addedAfter)
	}
}
