package ops

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// The expiry pass consumes exactly the records the dispatcher's
// decision layer selects on: non-terminal Manual records with a set,
// elapsed Deadline. Everything else — Auto records (born terminal by
// contract, but the trigger filter must hold even for a malformed
// non-terminal one), zero Deadlines, unelapsed Deadlines, and records
// already terminal — must never be treated as expiry work.
func TestHasExpiredMigrationCandidate_FiltersTriggerPhaseAndDeadline(t *testing.T) {
	now := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	past := metav1.NewTime(now.Add(-time.Minute))
	future := metav1.NewTime(now.Add(time.Minute))

	record := func(trigger workload.MigrationTrigger, phase workload.MigrationPhase, deadline metav1.Time) workload.MigrationRecord {
		return workload.MigrationRecord{
			RequestUUID: "expire-filter-uuid",
			Trigger:     trigger,
			Phase:       phase,
			Deadline:    deadline,
		}
	}

	tests := []struct {
		name    string
		records []workload.MigrationRecord
		want    bool
	}{
		{name: "no records", records: nil, want: false},
		{
			name:    "manual non-terminal past deadline",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseSurgePending, past)},
			want:    true,
		},
		{
			name:    "manual accepted past deadline",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseAccepted, past)},
			want:    true,
		},
		{
			name:    "manual deadline not yet elapsed",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseSurgePending, future)},
			want:    false,
		},
		{
			name:    "manual deadline exactly now",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseSurgePending, metav1.NewTime(now))},
			want:    false,
		},
		{
			name:    "manual zero deadline never expires",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseSurgePending, metav1.Time{})},
			want:    false,
		},
		{
			name:    "manual terminal completed",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseCompleted, past)},
			want:    false,
		},
		{
			name:    "manual terminal failed",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerManual, workload.MigrationPhaseFailed, past)},
			want:    false,
		},
		{
			name:    "auto record past deadline",
			records: []workload.MigrationRecord{record(workload.MigrationTriggerAuto, workload.MigrationPhaseAccepted, past)},
			want:    false,
		},
		{
			name: "one candidate among non-candidates",
			records: []workload.MigrationRecord{
				record(workload.MigrationTriggerAuto, workload.MigrationPhaseRelocated, past),
				record(workload.MigrationTriggerManual, workload.MigrationPhaseCompleted, past),
				record(workload.MigrationTriggerManual, workload.MigrationPhaseDraining, past),
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HasExpiredMigrationCandidate(test.records, now); got != test.want {
				t.Fatalf("HasExpiredMigrationCandidate = %v, want %v", got, test.want)
			}
		})
	}
}
