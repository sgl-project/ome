package ops

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/audit"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestMigrate_DrainRejectsAuthoritativePairDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1beta1.InferenceReplica, int32, int32)
	}{
		{
			name: "source operation replaced",
			mutate: func(ir *v1beta1.InferenceReplica, sourceIdx, _ int32) {
				status := migrationStatusByIndex(t, ir, sourceIdx)
				status.Phase = v1beta1.OMENativeInstanceUpdating
				status.Operation.Type = v1beta1.InstanceOperationUpdate
				status.Operation.RequestUUID = "replacement-source"
			},
		},
		{
			name: "surge slot repurposed",
			mutate: func(ir *v1beta1.InferenceReplica, _, surgeIdx int32) {
				status := migrationStatusByIndex(t, ir, surgeIdx)
				status.Phase = v1beta1.OMENativeInstanceReady
				status.RunningRevision = "unrelated-revision"
				status.TargetRevision = ""
				status.Operation = nil
			},
		},
		{
			name: "surge slot removed",
			mutate: func(ir *v1beta1.InferenceReplica, _, surgeIdx int32) {
				removeMigrationStatusByIndex(ir, surgeIdx)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, uuid, surgeIdx := migrationGangReadyToDrain(t)
			finalizations := 0
			f.finalizeInstanceResources = func(context.Context, int32) (bool, error) {
				finalizations++
				return true, nil
			}
			input := f.input(t)
			mutateMigrationIRStatus(t, f, func(ir *v1beta1.InferenceReplica) {
				test.mutate(ir, 0, surgeIdx)
			})
			wantEffects := snapshotMigrationEffects(t, f)

			done, accepted, err := Migrate(context.Background(), f.deps(), input, f.plan, 0, uuid, migrationRequest(t, f, uuid))
			if err != nil || done || !accepted {
				t.Fatalf("guarded drain: done=%v accepted=%v err=%v", done, accepted, err)
			}
			assertMigrationEffectsUnchanged(t, f, wantEffects)
			if finalizations != 0 {
				t.Fatalf("pair drift triggered %d resource finalizations", finalizations)
			}
		})
	}
}

func TestMigrate_InitialPairStampRejectsAuthoritativeDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1beta1.InferenceReplica, int32, int32)
	}{
		{
			name: "source operation replaced",
			mutate: func(ir *v1beta1.InferenceReplica, sourceIdx, _ int32) {
				status := migrationStatusByIndex(t, ir, sourceIdx)
				status.Phase = v1beta1.OMENativeInstanceUpdating
				status.Operation = &v1beta1.InstanceOperation{
					Type:        v1beta1.InstanceOperationUpdate,
					RequestUUID: "replacement-source",
				}
			},
		},
		{
			name: "surge slot occupied",
			mutate: func(ir *v1beta1.InferenceReplica, _, surgeIdx int32) {
				ir.Status.InstanceStatuses = append(ir.Status.InstanceStatuses, v1beta1.OMENativeInstanceStatus{
					Index:           surgeIdx,
					Incarnation:     1,
					Phase:           v1beta1.OMENativeInstanceReady,
					RunningRevision: "unrelated-revision",
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newSinglePodMigFixture(t)
			finalizations := 0
			f.finalizeInstanceResources = func(context.Context, int32) (bool, error) {
				finalizations++
				return true, nil
			}
			const uuid = "migration-initial-pair-fence"
			f.records = []workload.MigrationRecord{mkMigRecord(uuid, 0, "node-a")}
			input := f.input(t)
			baseMutateMigration := input.MutateMigration
			var wantEffects string
			input.MutateMigration = func(ctx context.Context, requestUUID string, mutate func(*workload.MigrationRecord) bool) error {
				if err := baseMutateMigration(ctx, requestUUID, mutate); err != nil {
					return err
				}
				record := f.record(t, uuid)
				if record.SurgeInstance == nil {
					t.Fatal("allocation did not persist a surge index")
				}
				mutateMigrationIRStatus(t, f, func(ir *v1beta1.InferenceReplica) {
					test.mutate(ir, 0, *record.SurgeInstance)
				})
				wantEffects = snapshotMigrationEffects(t, f)
				return nil
			}

			done, accepted, err := Migrate(context.Background(), f.deps(), input, f.plan, 0, uuid, migrationRequest(t, f, uuid))
			if err != nil || done || !accepted {
				t.Fatalf("guarded initial stamp: done=%v accepted=%v err=%v", done, accepted, err)
			}
			if wantEffects == "" {
				t.Fatal("test did not capture post-allocation effects")
			}
			assertMigrationEffectsUnchanged(t, f, wantEffects)
			if finalizations != 0 {
				t.Fatalf("initial pair drift triggered %d resource finalizations", finalizations)
			}
		})
	}
}

func TestMigrate_PromotionRejectsPairDriftAfterDrainClaim(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1beta1.InferenceReplica, int32, int32)
	}{
		{
			name: "source operation replaced",
			mutate: func(ir *v1beta1.InferenceReplica, sourceIdx, _ int32) {
				status := migrationStatusByIndex(t, ir, sourceIdx)
				status.Operation.Type = v1beta1.InstanceOperationUpdate
				status.Operation.RequestUUID = "replacement-source"
			},
		},
		{
			name: "surge slot repurposed",
			mutate: func(ir *v1beta1.InferenceReplica, _, surgeIdx int32) {
				status := migrationStatusByIndex(t, ir, surgeIdx)
				status.Phase = v1beta1.OMENativeInstanceReady
				status.RunningRevision = "unrelated-revision"
				status.TargetRevision = ""
				status.Operation = nil
			},
		},
		{
			name: "surge slot removed",
			mutate: func(ir *v1beta1.InferenceReplica, _, surgeIdx int32) {
				removeMigrationStatusByIndex(ir, surgeIdx)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, uuid, surgeIdx := migrationGangReadyToPromote(t)
			finalizations := 0
			f.finalizeInstanceResources = func(context.Context, int32) (bool, error) {
				finalizations++
				return true, nil
			}
			input := f.input(t)
			baseApply := input.ApplyInstanceMutationsWithRetryBlock
			applyCalls := 0
			var wantEffects string
			input.ApplyInstanceMutationsWithRetryBlock = func(
				ctx context.Context,
				mutations []workload.InstanceMutation,
				targetRevision string,
				mutateRetryBlock func(*workload.RetryBlock) workload.RetryBlockDisposition,
			) error {
				applyCalls++
				err := baseApply(ctx, mutations, targetRevision, mutateRetryBlock)
				if err == nil && applyCalls == 1 {
					mutateMigrationIRStatus(t, f, func(ir *v1beta1.InferenceReplica) {
						test.mutate(ir, 0, surgeIdx)
					})
					wantEffects = snapshotMigrationEffects(t, f)
				}
				return err
			}

			done, accepted, err := Migrate(context.Background(), f.deps(), input, f.plan, 0, uuid, migrationRequest(t, f, uuid))
			if err != nil || done || !accepted {
				t.Fatalf("guarded promotion: done=%v accepted=%v err=%v", done, accepted, err)
			}
			if applyCalls < 2 {
				t.Fatalf("promotion did not recheck the pair after drain claim: apply calls=%d", applyCalls)
			}
			if wantEffects == "" {
				t.Fatal("test did not inject pair drift")
			}
			assertMigrationEffectsUnchanged(t, f, wantEffects)
			if finalizations != 0 {
				t.Fatalf("promotion pair drift triggered %d resource finalizations", finalizations)
			}
		})
	}
}

func TestMigrate_CompletionRecoveryRejectsAuthoritativeTargetDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1beta1.InferenceReplica, int32)
	}{
		{
			name: "promoted target removed",
			mutate: func(ir *v1beta1.InferenceReplica, surgeIdx int32) {
				removeMigrationStatusByIndex(ir, surgeIdx)
			},
		},
		{
			name: "promoted target repurposed",
			mutate: func(ir *v1beta1.InferenceReplica, surgeIdx int32) {
				status := migrationStatusByIndex(t, ir, surgeIdx)
				status.RunningRevision = "unrelated-revision"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newSinglePodMigFixture(t)
			clk := f.withFakeClock()
			const uuid = "migration-completion-pair-fence"
			seedMigrationCompletionTailWindow(t, f, true, nil)
			surgeIdx := int32(1)
			record := mkMigRecordWithDeadline(uuid, 0, "node-a", clk.Now().Add(30*time.Minute))
			record.SurgeInstance = &surgeIdx
			record.Phase = workload.MigrationPhaseDraining
			f.records = []workload.MigrationRecord{record}
			finalizations := 0
			f.finalizeInstanceResources = func(context.Context, int32) (bool, error) {
				finalizations++
				return true, nil
			}
			input := f.input(t)
			mutateMigrationIRStatus(t, f, func(ir *v1beta1.InferenceReplica) {
				test.mutate(ir, surgeIdx)
			})
			wantEffects := snapshotMigrationEffects(t, f)

			done, accepted, err := Migrate(context.Background(), f.deps(), input, f.plan, 0, uuid, migrationRequest(t, f, uuid))
			if err != nil || done || !accepted {
				t.Fatalf("guarded completion recovery: done=%v accepted=%v err=%v", done, accepted, err)
			}
			assertMigrationEffectsUnchanged(t, f, wantEffects)
			if finalizations != 0 {
				t.Fatalf("completion pair drift triggered %d resource finalizations", finalizations)
			}
		})
	}
}

func migrationGangReadyToDrain(t *testing.T) (*migFixture, string, int32) {
	t.Helper()
	f := newGangMigFixture(t)
	f.finalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
	const uuid = "migration-pair-fence"
	f.records = []workload.MigrationRecord{mkMigRecord(uuid, 0, "node-b")}

	f.pass(t, uuid)
	f.pass(t, uuid)
	f.react(t)
	f.pass(t, uuid)
	f.react(t)

	record := f.record(t, uuid)
	if record.SurgeInstance == nil || record.Phase != workload.MigrationPhaseSurgePending {
		t.Fatalf("migration did not reach the pre-drain gate: %+v", *record)
	}
	return f, uuid, *record.SurgeInstance
}

func migrationGangReadyToPromote(t *testing.T) (*migFixture, string, int32) {
	t.Helper()
	f, uuid, surgeIdx := migrationGangReadyToDrain(t)
	f.pass(t, uuid)
	f.react(t)
	f.pass(t, uuid)
	f.react(t)

	for _, pod := range f.listPods(t) {
		if pod.Labels[query.LabelInstanceIdx] == "0" {
			t.Fatalf("source pod %s remained before promotion tail", pod.Name)
		}
	}
	if record := f.record(t, uuid); record.Phase != workload.MigrationPhaseDraining {
		t.Fatalf("migration did not reach the promotion tail: %+v", *record)
	}
	return f, uuid, surgeIdx
}

func migrationRequest(t *testing.T, f *migFixture, uuid string) *audit.MigrationRequest {
	t.Helper()
	record := f.record(t, uuid)
	return &audit.MigrationRequest{
		SchemaVersion:   audit.SchemaV1,
		Component:       string(f.component),
		Instance:        record.SourceInstance,
		FromNode:        record.FromNode,
		HintTargetNodes: append([]string(nil), record.HintTargetNodes...),
		Reason:          record.Reason,
	}
}

func mutateMigrationIRStatus(t *testing.T, f *migFixture, mutate func(*v1beta1.InferenceReplica)) {
	t.Helper()
	ir := f.getIR(t)
	mutate(ir)
	if err := f.c.Status().Update(context.Background(), ir); err != nil {
		t.Fatalf("update migration status fixture: %v", err)
	}
}

func migrationStatusByIndex(t *testing.T, ir *v1beta1.InferenceReplica, index int32) *v1beta1.OMENativeInstanceStatus {
	t.Helper()
	for i := range ir.Status.InstanceStatuses {
		if ir.Status.InstanceStatuses[i].Index == index {
			return &ir.Status.InstanceStatuses[i]
		}
	}
	t.Fatalf("instance status %d missing", index)
	return nil
}

func removeMigrationStatusByIndex(ir *v1beta1.InferenceReplica, index int32) {
	for i := range ir.Status.InstanceStatuses {
		if ir.Status.InstanceStatuses[i].Index != index {
			continue
		}
		ir.Status.InstanceStatuses = append(ir.Status.InstanceStatuses[:i], ir.Status.InstanceStatuses[i+1:]...)
		return
	}
}

func snapshotMigrationEffects(t *testing.T, f *migFixture) string {
	t.Helper()
	ir := f.getIR(t)
	pods := f.listPods(t)
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})
	ledger, err := audit.LoadLedgerForOwner(context.Background(), f.c, f.isvc)
	if err != nil {
		t.Fatalf("load migration effects ledger: %v", err)
	}
	snapshot := struct {
		Status  v1beta1.InferenceReplicaStatus
		Pods    []*corev1.Pod
		Records []workload.MigrationRecord
		Ledger  *audit.Ledger
	}{
		Status:  ir.Status,
		Pods:    pods,
		Records: f.records,
		Ledger:  ledger,
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal migration effects: %v", err)
	}
	return string(data)
}

func assertMigrationEffectsUnchanged(t *testing.T, f *migFixture, want string) {
	t.Helper()
	if got := snapshotMigrationEffects(t, f); got != want {
		t.Fatal("migration pair drift changed status, pods, readiness, record, or ledger")
	}
}
