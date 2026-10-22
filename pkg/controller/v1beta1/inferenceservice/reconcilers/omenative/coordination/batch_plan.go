package coordination

// The ratio rule of a prefill/decode group plans the two sides' batches
// together from their desired counts, their steps and the group's
// tolerance. A side's share is the number of its Instances started toward
// the new version over its desired count, and the skew of a picture is the
// gap between the two shares. Before each batch both sides' full steps are
// tried; when the picture after them would sit past the tolerance the
// batch shrinks to the largest one within it, and when none fits the one
// closest to the line is taken, never an empty one, so a tight tolerance
// slows the roll and never stops it. With 12 prefill beside 8 decode, both
// stepping 10 percent, at a tolerance of 10 percent the decode side steps
// 1 per batch and the prefill side 2, 2, 1, 2, 1, 2, 1, 1.

// pairedBatchPlan returns the cumulative count each side has started after
// every batch, prefill then decode, planned from zero. A side with nothing
// to roll or a step of zero plans nothing: the sides then keep their own
// steps.
func pairedBatchPlan(prefill, decode, prefillStep, decodeStep, tolerancePercent int32) (prefillMarks, decodeMarks []int32) {
	if prefill <= 0 || decode <= 0 || prefillStep <= 0 || decodeStep <= 0 {
		return nil, nil
	}
	var p, d int32
	for p < prefill || d < decode {
		bp, bd := fitBatch(prefill, decode, p, d, min32(prefillStep, prefill-p), min32(decodeStep, decode-d), tolerancePercent)
		p += bp
		d += bd
		prefillMarks = append(prefillMarks, p)
		decodeMarks = append(decodeMarks, d)
	}
	return prefillMarks, decodeMarks
}

// batchChoice is one candidate batch: how many of each side it starts,
// whether the picture after it sits within the tolerance, and that
// picture's skew.
type batchChoice struct {
	p, d int32
	fits bool
	skew int64
}

// beats orders candidates: one within the tolerance before one outside it;
// within it the larger batch, then the smaller skew; outside it the smaller
// skew, then the larger batch; the decode side moving more breaks the last
// tie.
func (c batchChoice) beats(o batchChoice) bool {
	switch {
	case c.fits != o.fits:
		return c.fits
	case c.fits && c.p+c.d != o.p+o.d:
		return c.p+c.d > o.p+o.d
	case c.skew != o.skew:
		return c.skew < o.skew
	case c.p+c.d != o.p+o.d:
		return c.p+c.d > o.p+o.d
	}
	return c.d > o.d
}

// fitBatch chooses the next batch from the picture p:d with the full steps
// fp and fd on offer: every batch up to the full one, an empty one
// excepted, ranked by batchChoice.beats.
func fitBatch(prefill, decode, p, d, fp, fd, tolerancePercent int32) (int32, int32) {
	limit := int64(tolerancePercent) * int64(prefill) * int64(decode)
	var best batchChoice
	found := false
	for bp := fp; bp >= 0; bp-- {
		for bd := fd; bd >= 0; bd-- {
			if bp+bd == 0 {
				continue
			}
			c := batchChoice{p: bp, d: bd, skew: scaledSkew(prefill, decode, p+bp, d+bd)}
			c.fits = c.skew <= limit
			if !found || c.beats(best) {
				best, found = c, true
			}
		}
	}
	return best.p, best.d
}

// scaledSkew is the gap between the two shares, pNew over prefill and dNew
// over decode, scaled by 100 times both desired counts so the comparison
// with a whole-percent tolerance stays in integers.
func scaledSkew(prefill, decode, pNew, dNew int32) int64 {
	skew := (int64(pNew)*int64(decode) - int64(dNew)*int64(prefill)) * 100
	if skew < 0 {
		skew = -skew
	}
	return skew
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
