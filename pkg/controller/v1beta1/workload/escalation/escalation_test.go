package escalation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TestEvidenceFor_Deadline: the pass's evidence reports DeadlinePassed
// for a transient-phase instance whose Operation.Deadline is in the past,
// and not otherwise. Evidence only — no writes. (No stuck pod: no pods
// are observed, so StuckPod stays nil.)
func TestEvidenceFor_Deadline(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	insts := []types.InstanceStatus{
		{ // deadline in the past → DeadlinePassed
			Index: 0, Phase: types.InstancePhaseUpdating,
			Operation: &types.InstanceOperation{Deadline: metav1.NewTime(now.Add(-time.Minute))},
		},
		{ // deadline in the future → not passed
			Index: 1, Phase: types.InstancePhaseUpdating,
			Operation: &types.InstanceOperation{Deadline: metav1.NewTime(now.Add(time.Minute))},
		},
	}

	observed := types.WorkloadObservedState{InstanceStatuses: insts}
	if ev := evidenceFor(&observed, nil, 0, now, 30*time.Second, ""); !ev.DeadlinePassed || ev.StuckPod != nil {
		t.Errorf("instance 0: got DeadlinePassed=%v StuckPod=%v, want true/nil", ev.DeadlinePassed, ev.StuckPod)
	}
	if ev := evidenceFor(&observed, nil, 1, now, 30*time.Second, ""); ev.DeadlinePassed {
		t.Errorf("instance 1: future deadline must not be passed")
	}
}

// TestRunRowSteps_StopsAtTheFirstStepThatEndsTheRow holds the runner every
// row of the pass walks: a step that goes on runs the next, the first that
// ends the walk or fails is the last one run, and a failing step's error
// comes back as it is.
func TestRunRowSteps_StopsAtTheFirstStepThatEndsTheRow(t *testing.T) {
	var ran []string
	step := func(name string, next bool, err error) rowStep {
		return func(context.Context) (bool, error) {
			ran = append(ran, name)
			return next, err
		}
	}
	run := func(steps ...rowStep) (done bool, order string, err error) {
		ran = nil
		done, err = runRowSteps(context.Background(), steps)
		return done, strings.Join(ran, ","), err
	}

	done, order, err := run(step("first", true, nil), step("second", true, nil))
	if err != nil || !done || order != "first,second" {
		t.Fatalf("every step went on: done=%v err=%v ran=%q, want done=true err=nil ran=first,second", done, err, order)
	}
	done, order, err = run(step("first", true, nil), step("ends", false, nil), step("never", true, nil))
	if err != nil || done || order != "first,ends" {
		t.Fatalf("a step ended the walk: done=%v err=%v ran=%q, want done=false err=nil ran=first,ends", done, err, order)
	}
	writeFailed := errors.New("write the status")
	done, order, err = run(step("fails", true, writeFailed), step("never", true, nil))
	if !errors.Is(err, writeFailed) || done || order != "fails" {
		t.Fatalf("a step failed: done=%v err=%v ran=%q, want done=false the step's error ran=fails", done, err, order)
	}
}
