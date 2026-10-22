package types

// HeldInstances is the pass-scoped record of the Instances the roll's
// partition holds on the revision they run: the lowest-indexed Instances
// still off the roll target that no update is moving, as many as the
// partition names. Reading the set costs one lookup per planned Instance,
// and the restart selection asks about it once per row that needs a
// repair while a roll is open, so the first reader of a pass records the
// set here and every later reader takes it as recorded. All methods are
// nil-safe: a nil record is never filled, so a verb run outside a pass
// reads the rows on every ask.
type HeldInstances struct {
	read   bool
	target string
	held   map[int32]bool
}

// Recorded is the set recorded for target, and whether one is: a record
// filled for another target answers for nothing.
func (h *HeldInstances) Recorded(target string) (map[int32]bool, bool) {
	if h == nil || !h.read || h.target != target {
		return nil, false
	}
	return h.held, true
}

// Record keeps held as the pass's set for target. The first record of a
// pass stands; a later one is dropped.
func (h *HeldInstances) Record(target string, held map[int32]bool) {
	if h == nil || h.read {
		return
	}
	h.read, h.target, h.held = true, target, held
}
