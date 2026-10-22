package types

import "testing"

func TestHeldInstances_RecordsOnceAndAnswersForItsTargetOnly(t *testing.T) {
	var none *HeldInstances
	if _, ok := none.Recorded("v2"); ok {
		t.Fatal("a nil record must read as not recorded")
	}
	none.Record("v2", map[int32]bool{1: true})

	h := &HeldInstances{}
	if _, ok := h.Recorded("v2"); ok {
		t.Fatal("an empty record must read as not recorded")
	}
	h.Record("v2", map[int32]bool{0: true, 1: true})
	h.Record("v2", map[int32]bool{7: true})
	got, ok := h.Recorded("v2")
	if !ok || len(got) != 2 || !got[0] || !got[1] {
		t.Fatalf("the first record must stand: got %v, recorded=%v", got, ok)
	}
	if _, ok := h.Recorded("v3"); ok {
		t.Fatal("a record for another target must read as not recorded")
	}
}
