package irstatus

import (
	"flag"
	"path/filepath"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/testing/allocbudget"
)

var updateFleetAllocations = flag.Bool("update", false, "re-record testdata/fleet_allocations.json from this run")

// fleetRows is the benchmark's largest shape: 4,000 Instances on one
// InferenceReplica, at rest (every row equal) and mid-roll (the
// representative mix of phases, revisions and records).
const fleetRows = 4000

// fleetOp is one codec path at fleet size: the name the benchmark and the
// allocation golden share, the operation, and the stored bytes a decode reads.
type fleetOp struct {
	name  string
	op    func() error
	bytes int
}

func fleetBenchmarkCases() []struct {
	name string
	rows []v1beta1.OMENativeInstanceStatus
} {
	return []struct {
		name string
		rows []v1beta1.OMENativeInstanceStatus
	}{
		{name: "uniform/4000", rows: uniformRows(fleetRows)},
		{name: "representative/4000", rows: representativeRows(fleetRows)},
	}
}

// fleetEncodeOps is one status write's encode per fixture: the plain list
// alone, and the writer's selection, which builds both shapes and keeps the
// smaller one.
func fleetEncodeOps() []fleetOp {
	var ops []fleetOp
	for _, bc := range fleetBenchmarkCases() {
		logical := logicalStatus(bc.rows)
		ops = append(ops,
			fleetOp{name: bc.name + "/denseV1", op: encodeDenseV1Op(logical)},
			fleetOp{name: bc.name + "/selected", op: encodeSelectedOp(logical)})
	}
	return ops
}

// fleetDecodeOps is one read's decode per fixture and stored shape.
func fleetDecodeOps(tb testing.TB) []fleetOp {
	tb.Helper()
	var ops []fleetOp
	for _, bc := range fleetBenchmarkCases() {
		selection, err := SelectCandidate(logicalStatus(bc.rows), testLimit)
		if err != nil || selection.ColumnarV2 == nil {
			tb.Fatalf("SelectCandidate() = %+v, %v", selection, err)
		}
		dense := mustJSON(tb, selection.DenseV1.Status)
		columnar := mustJSON(tb, selection.ColumnarV2.Status)
		ops = append(ops,
			fleetOp{name: bc.name + "/denseV1", op: decodeStatusOp(dense), bytes: len(dense)},
			fleetOp{name: bc.name + "/columnarV2", op: decodeStatusOp(columnar), bytes: len(columnar)})
	}
	return ops
}

// BenchmarkFleetEncodeStatus is the cost of one status write's encode at
// fleet size.
func BenchmarkFleetEncodeStatus(b *testing.B) {
	for _, op := range fleetEncodeOps() {
		b.Run(op.name, func(b *testing.B) { benchmarkOp(b, op.op) })
	}
}

// BenchmarkFleetDecodeStatus is the cost of one read's decode at fleet size
// for each stored shape, with the stored bytes of that shape reported beside
// the time.
func BenchmarkFleetDecodeStatus(b *testing.B) {
	for _, op := range fleetDecodeOps(b) {
		b.Run(op.name, func(b *testing.B) {
			b.ReportMetric(float64(op.bytes), "status-bytes")
			benchmarkOp(b, op.op)
		})
	}
}

// TestFleetCodecAllocationsStayWithinBudget holds the allocations of one
// encode and one decode at 4,000 rows to the recorded golden, so a change in
// the codec's cost is made on purpose and carries its reason.
func TestFleetCodecAllocationsStayWithinBudget(t *testing.T) {
	golden := allocbudget.Open(t, filepath.Join("testdata", "fleet_allocations.json"), *updateFleetAllocations)
	defer golden.Close()
	for _, op := range fleetEncodeOps() {
		golden.Check("encode/"+op.name, mustRun(t, op.op))
	}
	for _, op := range fleetDecodeOps(t) {
		golden.Check("decode/"+op.name, mustRun(t, op.op))
	}
}

func mustRun(t *testing.T, op func() error) func() {
	return func() {
		if err := op(); err != nil {
			t.Fatal(err)
		}
	}
}
