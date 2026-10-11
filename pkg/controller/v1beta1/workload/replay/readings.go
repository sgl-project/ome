package replay

import (
	"fmt"
	"sort"
	"time"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading/compare"
)

// ReadingFinding is one place where a reader the passes use and the one
// reading of the same pods (package reading) answer differently.
type ReadingFinding struct {
	// Index is the Instance whose pods were read.
	Index int32
	// Reader names the reader compared, as package compare names it.
	Reader string
	// Class is the stable key a tally over recorded scenarios groups by:
	// the question asked, the reader's side and what each side answered,
	// with no number or name in it. Every question compare asks is about
	// the Serving level or a verdict built on it.
	Class string
	// Detail is the one-line account with the index, the names and the
	// numbers.
	Detail string
}

// CompareReadings asks the one reading and the passes' readers the same
// questions about every Instance's pods in the view and lists where they
// disagree, in Instance order. The reading gets the pass's clock, the
// Component's two windows and the routed rotation the view carries. The
// replay runs one Component with no pairing protocol, so the Pairable
// level reads as Serving. The rows are not handed to the comparison: the
// driver publishes PodCount onto them and no serving count, so a reader
// of the persisted serving count would be measured against the driver,
// not against the pods.
func CompareReadings(view TickView) []ReadingFinding {
	ctx := reading.Context{
		Now:                view.Now,
		AvailabilityWindow: time.Duration(view.Spec.MinReadySeconds) * time.Second,
		UnreadyGrace:       view.StuckPodGrace,
		Rotation:           rotationOf(view.Routed),
	}
	indexes := make([]int32, 0, len(view.Pods))
	for index := range view.Pods {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
	var findings []ReadingFinding
	for _, index := range indexes {
		for _, d := range compare.Compare(view.Pods[index], view.Spec.PodsPerInstance, nil, ctx) {
			findings = append(findings, ReadingFinding{
				Index:  index,
				Reader: d.Reader,
				Class:  d.Class,
				Detail: fmt.Sprintf("index %d: %s", index, d.Detail),
			})
		}
	}
	return findings
}

// rotationOf is the reading's rotation set: every pod some routed Service
// publishes for new traffic.
func rotationOf(routed map[string]string) reading.Rotation {
	out := make(reading.Rotation, len(routed))
	for name, service := range routed {
		if service != "" {
			out[name] = struct{}{}
		}
	}
	return out
}
