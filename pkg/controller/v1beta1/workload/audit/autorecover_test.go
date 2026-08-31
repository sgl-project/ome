package audit

import (
	"testing"
	"time"
)

func autoRecoverEntry(uuid, node string, completedOffset time.Duration) Entry {
	return Entry{
		RequestUUID:    uuid,
		Component:      "engine",
		SourceInstance: 0,
		Phase:          PhaseCompleted,
		Reason:         ReasonAutoRecover,
		Outcome:        OutcomeRelocateRecreate,
		FromNode:       node,
		StartedAt:      fixedNow.Add(completedOffset).Format(time.RFC3339),
		CompletedAt:    fixedNow.Add(completedOffset).Format(time.RFC3339),
	}
}

// TestRecentAutoRecoverFromNodes_DedupBeforeWindow pins the windowing
// order: duplicate FromNodes are collapsed BEFORE the last-limit slice,
// so a repeated node cannot shrink the effective exclusion memory below
// limit DISTINCT nodes. With entries [A, B, B] and limit 2, the older
// distinct node A must survive (slicing first would yield [B, B] → [B]).
func TestRecentAutoRecoverFromNodes_DedupBeforeWindow(t *testing.T) {
	ledger := &Ledger{Entries: []Entry{
		autoRecoverEntry("u1", "node-a", -3*time.Minute),
		autoRecoverEntry("u2", "node-b", -2*time.Minute),
		autoRecoverEntry("u3", "node-b", -time.Minute),
	}}

	got := RecentAutoRecoverFromNodes(ledger, "engine", 0, 2)
	if len(got) != 2 || got[0] != "node-a" || got[1] != "node-b" {
		t.Errorf("nodes: got %v want [node-a node-b] (dedup before window)", got)
	}

	// limit 1 keeps only the most recent distinct node.
	got = RecentAutoRecoverFromNodes(ledger, "engine", 0, 1)
	if len(got) != 1 || got[0] != "node-b" {
		t.Errorf("nodes (limit 1): got %v want [node-b]", got)
	}
}

func TestRecentAutoRecoverFromNodes_EmptyCases(t *testing.T) {
	if got := RecentAutoRecoverFromNodes(nil, "engine", 0, 3); got != nil {
		t.Errorf("nil ledger: got %v want nil", got)
	}
	ledger := &Ledger{Entries: []Entry{autoRecoverEntry("u1", "node-a", 0)}}
	if got := RecentAutoRecoverFromNodes(ledger, "engine", 0, 0); got != nil {
		t.Errorf("limit 0: got %v want nil", got)
	}
	if got := RecentAutoRecoverFromNodes(ledger, "decoder", 0, 3); got != nil {
		t.Errorf("other component: got %v want nil", got)
	}
}

// TestNewestAutoRecoverEntry pins the append-order recency contract the
// disposition replay guard relies on.
func TestNewestAutoRecoverEntry(t *testing.T) {
	if got := NewestAutoRecoverEntry(nil, "engine", 0); got != nil {
		t.Errorf("nil ledger: got %+v want nil", got)
	}
	ledger := &Ledger{Entries: []Entry{
		autoRecoverEntry("u1", "node-a", -2*time.Minute),
		autoRecoverEntry("u2", "node-b", -time.Minute),
		// Non-AutoRecover entry after the newest directive must not win.
		{RequestUUID: "u3", Component: "engine", SourceInstance: 0, Phase: PhaseStarted},
	}}
	got := NewestAutoRecoverEntry(ledger, "engine", 0)
	if got == nil || got.RequestUUID != "u2" || got.FromNode != "node-b" {
		t.Fatalf("newest: got %+v want u2/node-b", got)
	}
	if got := NewestAutoRecoverEntry(ledger, "engine", 1); got != nil {
		t.Errorf("other instance: got %+v want nil", got)
	}
}
