package runtimerevision

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const gcKey = "ome.io/gc-eligible-since"

func rev(name, runtimeOf string, ageSec int, gcSince *time.Time) appsv1.ControllerRevision {
	created := time.Now().Add(time.Duration(-ageSec) * time.Second)
	out := appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.Time{Time: created},
			Labels:            map[string]string{"ome.io/runtime-of": runtimeOf},
			Annotations:       map[string]string{},
		},
	}
	if gcSince != nil {
		out.Annotations[gcKey] = gcSince.Format(time.RFC3339)
	}
	return out
}

func decisionsByName(ds []Decision) map[string]Decision {
	m := make(map[string]Decision, len(ds))
	for _, d := range ds {
		m[d.Revision.Name] = d
	}
	return m
}

func TestPlan_KeepAllWhenUnderRetention(t *testing.T) {
	now := time.Now()
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 100, nil),
		rev("r2", "rtA", 50, nil),
		rev("r3", "rtA", 10, nil),
	}
	got := decisionsByName(Plan(revs, nil, now, 10, time.Hour, gcKey))
	for _, name := range []string{"r1", "r2", "r3"} {
		if got[name].Action != Keep {
			t.Errorf("%s: want Keep, got %v", name, got[name].Action)
		}
	}
}

func TestPlan_MarksOldestBeyondRetention(t *testing.T) {
	now := time.Now()
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 300, nil),
		rev("r2", "rtA", 200, nil),
		rev("r3", "rtA", 100, nil),
		rev("r4", "rtA", 50, nil),
	}
	got := decisionsByName(Plan(revs, nil, now, 2, time.Hour, gcKey))
	if got["r4"].Action != Keep || got["r3"].Action != Keep {
		t.Fatal("expected newest two retained")
	}
	if got["r2"].Action != Mark || got["r1"].Action != Mark {
		t.Fatalf("expected oldest two marked, got r1=%v r2=%v", got["r1"].Action, got["r2"].Action)
	}
}

func TestPlan_ReferencedAlwaysKept(t *testing.T) {
	now := time.Now()
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 300, nil), // oldest — would be marked, but referenced
		rev("r2", "rtA", 200, nil),
		rev("r3", "rtA", 100, nil),
		rev("r4", "rtA", 50, nil),
	}
	got := decisionsByName(Plan(revs, map[string]bool{"r1": true}, now, 2, time.Hour, gcKey))
	if got["r1"].Action != Keep {
		t.Fatalf("referenced r1 must be kept, got %v", got["r1"].Action)
	}
	// Still mark r2 (next-oldest, unreferenced).
	if got["r2"].Action != Mark {
		t.Fatalf("r2 should still be marked, got %v", got["r2"].Action)
	}
}

func TestPlan_WaitsDuringGrace(t *testing.T) {
	now := time.Now()
	marked := now.Add(-30 * time.Minute) // marked 30m ago, grace is 1h
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 300, &marked),
		rev("r2", "rtA", 50, nil),
	}
	got := decisionsByName(Plan(revs, nil, now, 1, time.Hour, gcKey))
	if got["r1"].Action != Wait {
		t.Fatalf("r1 within grace should Wait, got %v", got["r1"].Action)
	}
}

func TestPlan_DeletesAfterGrace(t *testing.T) {
	now := time.Now()
	marked := now.Add(-2 * time.Hour) // grace is 1h → expired
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 300, &marked),
		rev("r2", "rtA", 50, nil),
	}
	got := decisionsByName(Plan(revs, nil, now, 1, time.Hour, gcKey))
	if got["r1"].Action != Delete {
		t.Fatalf("r1 past grace should Delete, got %v", got["r1"].Action)
	}
}

func TestPlan_ClearsAnnotationWhenBackInRetainSet(t *testing.T) {
	now := time.Now()
	marked := now.Add(-30 * time.Minute)
	// Only 1 revision; retention is 5; was previously marked (somehow),
	// must now Keep and the controller will clear the annotation.
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 100, &marked),
	}
	got := decisionsByName(Plan(revs, nil, now, 5, time.Hour, gcKey))
	if got["r1"].Action != Keep {
		t.Fatalf("r1 in retain set should Keep, got %v", got["r1"].Action)
	}
	if !got["r1"].HadAnnotation {
		t.Fatal("HadAnnotation should signal the controller to clear")
	}
}

func TestPlan_PerRuntimeIndependentRetention(t *testing.T) {
	// rtA has 3 revisions, rtB has 3. Retention is 2.
	// Each runtime should mark its own oldest, not share the budget.
	now := time.Now()
	revs := []appsv1.ControllerRevision{
		rev("a1", "rtA", 300, nil), rev("a2", "rtA", 200, nil), rev("a3", "rtA", 100, nil),
		rev("b1", "rtB", 300, nil), rev("b2", "rtB", 200, nil), rev("b3", "rtB", 100, nil),
	}
	got := decisionsByName(Plan(revs, nil, now, 2, time.Hour, gcKey))
	// Newest two per runtime are kept; oldest is marked.
	for _, kept := range []string{"a2", "a3", "b2", "b3"} {
		if got[kept].Action != Keep {
			t.Errorf("%s: want Keep, got %v", kept, got[kept].Action)
		}
	}
	for _, marked := range []string{"a1", "b1"} {
		if got[marked].Action != Mark {
			t.Errorf("%s: want Mark, got %v", marked, got[marked].Action)
		}
	}
}

func TestPlan_CorruptAnnotationReMarks(t *testing.T) {
	now := time.Now()
	revs := []appsv1.ControllerRevision{
		rev("r1", "rtA", 300, nil),
		rev("r2", "rtA", 50, nil),
	}
	revs[0].Annotations[gcKey] = "not-a-timestamp"
	got := decisionsByName(Plan(revs, nil, now, 1, time.Hour, gcKey))
	if got["r1"].Action != Mark {
		t.Fatalf("corrupt annotation should re-Mark, got %v", got["r1"].Action)
	}
}
