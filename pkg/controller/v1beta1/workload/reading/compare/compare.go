// Package compare sets the readers the passes use beside the reading of
// the same pods and lists where they disagree, so a test can count the
// disagreements over recorded replay scenarios. It only measures: no pass
// calls it, and it keeps the reading package free of the other readers.
package compare

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/omenative/coordination"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/evidence"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/status"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// Disagreement records one place where a reader the passes use and the
// reading answer differently for the same pods.
type Disagreement struct {
	// Reader names the reader compared, such as "evidence.PodSetServesNothing".
	Reader string
	// Class is a short, stable key for grouping, such as
	// "servesNothing/evidence=true/reading=false".
	Class string
	// Detail says what each side saw, for a person reading the report.
	Detail string
}

// Reader names, as the Reader field of a Disagreement carries them.
const (
	ReaderCountServingPods     = "status.CountServingPods"
	ReaderPodSetFullyServing   = "query.PodSetFullyServing"
	ReaderPodSetServesNothing  = "evidence.PodSetServesNothing"
	ReaderPodSetOutOfRotation  = "evidence.PodSetOutOfRotation"
	ReaderUnavailableOnParked  = "evidence.PodSetUnavailableOnParkedMember"
	ReaderRecreateOfDarkRow    = "types.RecreateOfDarkRow"
	ReaderPersistedServingPods = "InstanceStatus.ServingPodCount"
	ReaderPodReadyAndServing   = "coordination.PodReadyAndServing"
)

// Compare reads the pods (reading.Read) and evaluates the readers the
// passes use beside that reading, listing where they disagree. row is the
// Instance's persisted status, one pass old by the time the pods are
// read; nil skips the row readers. The per-pod comparison sets
// coordination's serving predicate beside podreadiness.ReadyAndServing,
// the one the Serving level is built on.
func Compare(pods []*corev1.Pod, desired int32, row *types.InstanceStatus, ctx reading.Context) []Disagreement {
	r := reading.Read(pods, desired, ctx)
	var out []Disagreement

	if got := status.CountServingPods(pods); got != r.Counts.Serving {
		out = append(out, Disagreement{
			Reader: ReaderCountServingPods,
			Class:  "servingCount/status" + relation(got, r.Counts.Serving) + "reading",
			Detail: fmt.Sprintf("status counts %d serving pods, the reading %d", got, r.Counts.Serving),
		})
	}
	out = appendVerdict(out, ReaderPodSetFullyServing, "fullyServing", "query",
		query.PodSetFullyServing(pods, desired), r.FullyServing())
	out = appendVerdict(out, ReaderPodSetServesNothing, "servesNothing", "evidence",
		evidence.PodSetServesNothing(pods, ctx.Now, ctx.UnreadyGrace), r.ServesNothing())
	out = appendVerdict(out, ReaderPodSetOutOfRotation, "outOfRotation", "evidence",
		evidence.PodSetOutOfRotation(pods), r.OutOfRotation())
	out = appendVerdict(out, ReaderUnavailableOnParked, "unavailableOnParkedMember", "evidence",
		evidence.PodSetUnavailableOnParkedMember(pods, desired, ctx.Now, ctx.UnreadyGrace), r.UnavailableOnParkedMember())

	if row != nil {
		// RecreateOfDarkRow reads the persisted counter only on a Failed row
		// that is not an update continuation, so the comparison is made
		// there alone, under a non-surge strategy so the counter decides.
		if types.StateOf(row) == types.StateFailed && !types.UpdateContinuation(row) {
			dark := types.RecreateOfDarkRow(row, types.UpdateStrategyRecreatePod)
			if dark != r.ServesNothing() {
				out = append(out, Disagreement{
					Reader: ReaderRecreateOfDarkRow,
					Class:  fmt.Sprintf("recreateOfDarkRow/row=%t/reading=%t", dark, r.ServesNothing()),
					Detail: fmt.Sprintf("the Failed row persisted %d serving pods one pass ago; this pass the pods read servesNothing=%t",
						row.ServingPodCount, r.ServesNothing()),
				})
			}
		}
		if row.ServingPodCount != r.Counts.Serving {
			out = append(out, Disagreement{
				Reader: ReaderPersistedServingPods,
				Class:  "persistedServingCount/row" + relation(row.ServingPodCount, r.Counts.Serving) + "reading",
				Detail: fmt.Sprintf("the row persisted %d serving pods one pass ago, the reading counts %d", row.ServingPodCount, r.Counts.Serving),
			})
		}
	}

	for _, pod := range pods {
		if pod == nil {
			continue
		}
		byCoordination := coordination.PodReadyAndServing(pod)
		byPodreadiness := podreadiness.ReadyAndServing(pod)
		if byCoordination == byPodreadiness {
			continue
		}
		out = append(out, Disagreement{
			Reader: ReaderPodReadyAndServing,
			Class:  fmt.Sprintf("podReadyAndServing/coordination=%t/podreadiness=%t", byCoordination, byPodreadiness),
			Detail: fmt.Sprintf("pod %s: coordination reads serving=%t, podreadiness reads %t, the reading's Serving level is %t",
				pod.Name, byCoordination, byPodreadiness, reading.ReadPod(pod, ctx).Serving),
		})
	}
	return out
}

// appendVerdict appends a disagreement when a boolean reader and the
// matching verdict differ.
func appendVerdict(out []Disagreement, reader, verdict, side string, theirs, ours bool) []Disagreement {
	if theirs == ours {
		return out
	}
	return append(out, Disagreement{
		Reader: reader,
		Class:  fmt.Sprintf("%s/%s=%t/reading=%t", verdict, side, theirs, ours),
		Detail: fmt.Sprintf("%s reads %t, the reading %t", reader, theirs, ours),
	})
}

// relation renders which side of a count comparison is larger.
func relation(theirs, ours int32) string {
	if theirs > ours {
		return ">"
	}
	return "<"
}
