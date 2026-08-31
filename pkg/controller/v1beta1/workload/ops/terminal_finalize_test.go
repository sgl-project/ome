package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func terminalStatusFixture(index int32) workload.InstanceStatus {
	surge := int32(9)
	started := metav1.NewTime(time.Date(2026, time.August, 15, 10, 0, 0, 987654321, time.UTC))
	return workload.InstanceStatus{
		Index:           index,
		Incarnation:     7,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "rev-old",
		TargetRevision:  "rev-new",
		ActiveOrdinal:   1,
		PodCount:        8,
		ReadyPodCount:   7,
		Operation: &workload.InstanceOperation{
			ID:              "gang-update-7",
			Type:            workload.InstanceOperationUpdate,
			Step:            updateStepSurge,
			RequestUUID:     "request-a",
			TargetRevision:  "rev-new",
			RetryCount:      2,
			Reason:          "rollout",
			FromNode:        "node-a",
			HintTargetNodes: []string{"node-b", "node-c"},
			SurgeIndex:      &surge,
			StartedAt:       started,
			LastProgressAt:  started,
			Deadline:        metav1.NewTime(started.Add(time.Hour)),
		},
	}
}

func TestTerminalInstanceIdentity_IgnoresWireTimeAndDerivedStatus(t *testing.T) {
	expected := terminalStatusFixture(3)
	current := cloneTerminalStatus(expected)
	current.Operation.StartedAt = metav1.NewTime(current.Operation.StartedAt.Truncate(time.Second))
	current.Operation.LastProgressAt = metav1.NewTime(current.Operation.LastProgressAt.Truncate(time.Second))
	current.Operation.Deadline = metav1.NewTime(current.Operation.Deadline.Truncate(time.Second))
	current.PodCount = 3
	current.ReadyPodCount = 2
	current.ServingPodCount = 1
	current.AvailablePodCount = 1
	current.ScheduledPodCount = 3
	current.NodesOccupied = []string{"node-b"}
	current.Conditions = []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse}}

	if !captureTerminalInstanceIdentity(&expected).matches(current) {
		t.Fatal("wire-normalized timestamps and derived observations changed terminal ownership")
	}
}

func TestTerminalInstanceIdentity_RejectsLifecycleDrift(t *testing.T) {
	expected := terminalStatusFixture(3)
	identity := captureTerminalInstanceIdentity(&expected)
	tests := []struct {
		name   string
		mutate func(*workload.InstanceStatus)
	}{
		{name: "index", mutate: func(s *workload.InstanceStatus) { s.Index++ }},
		{name: "incarnation", mutate: func(s *workload.InstanceStatus) { s.Incarnation++ }},
		{name: "phase", mutate: func(s *workload.InstanceStatus) { s.Phase = workload.InstancePhaseFailed }},
		{name: "running revision", mutate: func(s *workload.InstanceStatus) { s.RunningRevision = "other" }},
		{name: "target revision", mutate: func(s *workload.InstanceStatus) { s.TargetRevision = "other" }},
		{name: "active ordinal", mutate: func(s *workload.InstanceStatus) { s.ActiveOrdinal = 0 }},
		{name: "operation missing", mutate: func(s *workload.InstanceStatus) { s.Operation = nil }},
		{name: "operation ID", mutate: func(s *workload.InstanceStatus) { s.Operation.ID = "other" }},
		{name: "operation type", mutate: func(s *workload.InstanceStatus) { s.Operation.Type = workload.InstanceOperationMigrate }},
		{name: "operation step", mutate: func(s *workload.InstanceStatus) { s.Operation.Step = updateStepSurgeDrain }},
		{name: "request UUID", mutate: func(s *workload.InstanceStatus) { s.Operation.RequestUUID = "other" }},
		{name: "operation target", mutate: func(s *workload.InstanceStatus) { s.Operation.TargetRevision = "other" }},
		{name: "retry count", mutate: func(s *workload.InstanceStatus) { s.Operation.RetryCount++ }},
		{name: "operation reason", mutate: func(s *workload.InstanceStatus) { s.Operation.Reason = "other" }},
		{name: "source node", mutate: func(s *workload.InstanceStatus) { s.Operation.FromNode = "other" }},
		{name: "hint target nodes", mutate: func(s *workload.InstanceStatus) { s.Operation.HintTargetNodes[0] = "other" }},
		{name: "surge index", mutate: func(s *workload.InstanceStatus) { *s.Operation.SurgeIndex = *s.Operation.SurgeIndex + 1 }},
		{name: "surge index nil", mutate: func(s *workload.InstanceStatus) { s.Operation.SurgeIndex = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := cloneTerminalStatus(expected)
			test.mutate(&current)
			if identity.matches(current) {
				t.Fatal("lifecycle drift retained terminal ownership")
			}
		})
	}
}

func TestTransitionTerminalOperationStep_GuardedAndWireStable(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 123456789, time.UTC)
	expected := terminalStatusFixture(3)
	persisted := cloneTerminalStatus(expected)
	persisted.Operation.StartedAt = metav1.NewTime(persisted.Operation.StartedAt.Truncate(time.Second))
	persisted.Operation.LastProgressAt = metav1.NewTime(persisted.Operation.LastProgressAt.Truncate(time.Second))
	store := &terminalMutationStore{ownerUID: "owner-a", statuses: map[int32]workload.InstanceStatus{3: persisted}}
	input := workload.ReconcileInput{
		OwnerObject:                          &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
		Clock:                                clocktesting.NewFakeClock(now),
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}

	claimed, err := transitionTerminalOperationStep(context.Background(), input, &expected, updateStepSurgeDrain, true)
	if err != nil || !claimed {
		t.Fatalf("transition: claimed=%v err=%v", claimed, err)
	}
	if store.writes != 1 || store.statuses[3].Operation.Step != updateStepSurgeDrain {
		t.Fatalf("persisted transition: writes=%d status=%+v", store.writes, store.statuses[3])
	}
	if !store.statuses[3].Operation.LastProgressAt.Time.Equal(now) {
		t.Fatalf("LastProgressAt=%v want %v", store.statuses[3].Operation.LastProgressAt, now)
	}

	stale := terminalStatusFixture(3)
	stale.Operation.ID = "stale-attempt"
	claimed, err = transitionTerminalOperationStep(context.Background(), input, &stale, updateStepSurgeDrainSettle, true)
	if err != nil || claimed || store.writes != 1 {
		t.Fatalf("stale transition: claimed=%v err=%v writes=%d", claimed, err, store.writes)
	}
}

func TestFinalizeAndRemoveInstance_StrongOrderingAndGuards(t *testing.T) {
	const namespace, owner = "prod", "model"
	index := int32(3)
	expected := terminalStatusFixture(index)
	tests := []struct {
		name              string
		prepare           func(*terminalMutationStore, *workload.InstanceStatus)
		afterFinalize     func(*terminalMutationStore)
		finalizeErr       error
		finalizePending   bool
		wantComplete      bool
		wantErr           bool
		wantStatusPresent bool
		wantFinalizes     int
		wantWrites        int
		wantForgotten     bool
	}{
		{
			name:              "exact owner finalizes before committed removal",
			wantComplete:      true,
			wantStatusPresent: false,
			wantFinalizes:     1,
			wantWrites:        1,
			wantForgotten:     true,
		},
		{
			name:              "resource finalization failure retains status",
			finalizeErr:       errors.New("podgroup delete failed"),
			wantErr:           true,
			wantStatusPresent: true,
			wantFinalizes:     1,
		},
		{
			name:              "resource deletion in progress retains status",
			finalizePending:   true,
			wantStatusPresent: true,
			wantFinalizes:     1,
		},
		{
			name: "lifecycle drift during finalization aborts removal",
			afterFinalize: func(store *terminalMutationStore) {
				status := store.statuses[index]
				status.Operation.ID = "new-owner"
				store.statuses[index] = status
			},
			wantStatusPresent: true,
			wantFinalizes:     1,
		},
		{
			name: "owner replacement during finalization aborts removal",
			afterFinalize: func(store *terminalMutationStore) {
				store.ownerUID = "owner-b"
			},
			wantStatusPresent: true,
			wantFinalizes:     1,
		},
		{
			name: "concurrent status removal is idempotent",
			afterFinalize: func(store *terminalMutationStore) {
				delete(store.statuses, index)
			},
			wantComplete:      true,
			wantStatusPresent: false,
			wantFinalizes:     1,
			wantForgotten:     true,
		},
		{
			name: "changed operation aborts guarded removal",
			prepare: func(store *terminalMutationStore, _ *workload.InstanceStatus) {
				status := store.statuses[index]
				status.Operation.ID = "new-owner"
				store.statuses[index] = status
			},
			wantStatusPresent: true,
		},
		{
			name: "status write failure retains status and expectations",
			prepare: func(store *terminalMutationStore, _ *workload.InstanceStatus) {
				store.applyErr = errors.New("status write failed")
			},
			wantErr:           true,
			wantStatusPresent: true,
			wantFinalizes:     1,
		},
		{
			name: "owner recreation aborts guarded removal",
			prepare: func(store *terminalMutationStore, _ *workload.InstanceStatus) {
				store.ownerUID = "owner-b"
			},
			wantStatusPresent: true,
		},
		{
			name: "owner disappearance stops lifecycle tail",
			prepare: func(store *terminalMutationStore, _ *workload.InstanceStatus) {
				store.readErr = workload.ErrStatusOwnerGone
			},
			wantStatusPresent: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := []string{}
			store := &terminalMutationStore{
				ownerUID: "owner-a",
				statuses: map[int32]workload.InstanceStatus{index: cloneTerminalStatus(expected)},
				log:      &log,
			}
			expectations := workload.NewExpectations()
			expectations.ExpectDeletes(namespace, owner, workload.ComponentEngine, index, 1)
			finalizes := 0
			input := workload.ReconcileInput{
				OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
				Key:         workload.Key{Namespace: namespace, OwnerName: owner, Component: workload.ComponentEngine},
				FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
					finalizes++
					log = append(log, "finalize")
					if test.afterFinalize != nil {
						test.afterFinalize(store)
					}
					return test.finalizeErr == nil && !test.finalizePending, test.finalizeErr
				},
				ApplyInstanceMutationsWithRetryBlock: store.apply,
			}
			if test.prepare != nil {
				test.prepare(store, &expected)
			}

			complete, err := finalizeAndRemoveInstance(context.Background(), workload.Deps{Expectations: expectations}, input, index, &expected)
			if (err != nil) != test.wantErr || complete != test.wantComplete {
				t.Fatalf("result: complete=%v err=%v", complete, err)
			}
			_, present := store.statuses[index]
			if present != test.wantStatusPresent || finalizes != test.wantFinalizes || store.writes != test.wantWrites {
				t.Fatalf("state: present=%v finalizes=%d writes=%d log=%v", present, finalizes, store.writes, log)
			}
			forgotten := expectations.Satisfied(namespace, owner, workload.ComponentEngine, index)
			if forgotten != test.wantForgotten {
				t.Fatalf("expectations forgotten=%v want %v", forgotten, test.wantForgotten)
			}
			if finalizes > 0 && (len(log) < 2 || log[0] != "status" || log[1] != "finalize") {
				t.Fatalf("effect order=%v want status preflight before finalizer", log)
			}
			if store.applyCall > 1 && (len(log) < 3 || log[2] != "status") {
				t.Fatalf("effect order=%v want status removal after finalizer", log)
			}
		})
	}
}

func TestFinalizeAndRemoveInstance_RetriesAfterStatusFailure(t *testing.T) {
	expected := terminalStatusFixture(3)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{3: cloneTerminalStatus(expected)},
		applyErr: errors.New("transient status failure"),
	}
	finalizes := 0
	input := workload.ReconcileInput{
		OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
		Key:         workload.Key{Namespace: "prod", OwnerName: "model", Component: workload.ComponentEngine},
		FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
			finalizes++
			return true, nil
		},
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}

	complete, err := finalizeAndRemoveInstance(context.Background(), workload.Deps{}, input, 3, &expected)
	if err == nil || complete {
		t.Fatalf("first attempt: complete=%v err=%v", complete, err)
	}
	store.applyErr = nil
	complete, err = finalizeAndRemoveInstance(context.Background(), workload.Deps{}, input, 3, &expected)
	if err != nil || !complete {
		t.Fatalf("retry: complete=%v err=%v", complete, err)
	}
	if finalizes != 2 || store.writes != 1 {
		t.Fatalf("retry state: finalizes=%d writes=%d", finalizes, store.writes)
	}
}

func TestFinalizeAndRemoveInstance_ConfirmedAbsenceIsIdempotent(t *testing.T) {
	const namespace, owner = "prod", "model"
	index := int32(3)
	expected := terminalStatusFixture(index)
	store := &terminalMutationStore{ownerUID: "owner-a", statuses: map[int32]workload.InstanceStatus{}}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes(namespace, owner, workload.ComponentEngine, index, 1)
	finalizes := 0
	input := workload.ReconcileInput{
		OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
		Key:         workload.Key{Namespace: namespace, OwnerName: owner, Component: workload.ComponentEngine},
		FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
			finalizes++
			return true, nil
		},
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}

	complete, err := finalizeAndRemoveInstance(context.Background(), workload.Deps{Expectations: expectations}, input, index, &expected)
	if err != nil || !complete {
		t.Fatalf("confirmed absence: complete=%v err=%v", complete, err)
	}
	if finalizes != 0 || store.writes != 0 || store.applyCall != 1 {
		t.Fatalf("idempotent absence: finalizes=%d writes=%d applyCalls=%d", finalizes, store.writes, store.applyCall)
	}
	if !expectations.Satisfied(namespace, owner, workload.ComponentEngine, index) {
		t.Fatal("authoritatively absent status did not forget expectations")
	}
}

func TestFinalizeAndRemoveInstance_AbsentIdentityGuardsResidualFinalization(t *testing.T) {
	const namespace, owner = "prod", "model"
	index := int32(3)
	tests := []struct {
		name          string
		prepare       func(*terminalMutationStore)
		finalizeErr   error
		wantComplete  bool
		wantErr       bool
		wantFinalizes int
		wantApply     int
		wantForgotten bool
	}{
		{
			name:          "authoritative absence finalizes residual resources",
			wantComplete:  true,
			wantFinalizes: 1,
			wantApply:     2,
			wantForgotten: true,
		},
		{
			name: "live status blocks finalization",
			prepare: func(store *terminalMutationStore) {
				store.statuses[index] = terminalStatusFixture(index)
			},
			wantApply: 1,
		},
		{
			name: "owner recreation blocks finalization",
			prepare: func(store *terminalMutationStore) {
				store.ownerUID = "owner-b"
			},
			wantApply: 1,
		},
		{
			name:          "finalization failure remains retryable",
			finalizeErr:   errors.New("podgroup delete failed"),
			wantErr:       true,
			wantFinalizes: 1,
			wantApply:     1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := []string{}
			store := &terminalMutationStore{
				ownerUID: "owner-a",
				statuses: map[int32]workload.InstanceStatus{},
				log:      &log,
			}
			if test.prepare != nil {
				test.prepare(store)
			}
			expectations := workload.NewExpectations()
			expectations.ExpectDeletes(namespace, owner, workload.ComponentEngine, index, 1)
			finalizes := 0
			input := workload.ReconcileInput{
				OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
				Key:         workload.Key{Namespace: namespace, OwnerName: owner, Component: workload.ComponentEngine},
				FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
					finalizes++
					log = append(log, "finalize")
					return test.finalizeErr == nil, test.finalizeErr
				},
				ApplyInstanceMutationsWithRetryBlock: store.apply,
			}

			complete, err := finalizeAndRemoveInstance(
				context.Background(),
				workload.Deps{Expectations: expectations},
				input,
				index,
				nil,
			)
			if complete != test.wantComplete || (err != nil) != test.wantErr {
				t.Fatalf("result: complete=%v err=%v", complete, err)
			}
			if finalizes != test.wantFinalizes || store.applyCall != test.wantApply || store.writes != 0 {
				t.Fatalf("effects: finalizes=%d applyCalls=%d writes=%d log=%v", finalizes, store.applyCall, store.writes, log)
			}
			forgotten := expectations.Satisfied(namespace, owner, workload.ComponentEngine, index)
			if forgotten != test.wantForgotten {
				t.Fatalf("expectations forgotten=%v want %v", forgotten, test.wantForgotten)
			}
			if finalizes > 0 && (len(log) < 2 || log[0] != "status" || log[1] != "finalize") {
				t.Fatalf("effect order=%v want authoritative status read before finalization", log)
			}
		})
	}
}

func TestFinalizeAndRemoveInstance_LegacyRemovalWithoutPerInstanceResources(t *testing.T) {
	const namespace, owner = "prod", "model"
	index := int32(3)
	expected := terminalStatusFixture(index)
	tests := []struct {
		name          string
		removed       bool
		removeErr     error
		configure     bool
		wantComplete  bool
		wantErr       bool
		wantForgotten bool
		wantCalls     int
	}{
		{
			name:          "committed removal completes",
			removed:       true,
			configure:     true,
			wantComplete:  true,
			wantForgotten: true,
			wantCalls:     1,
		},
		{
			name:          "already absent completes",
			configure:     true,
			wantComplete:  true,
			wantForgotten: true,
			wantCalls:     1,
		},
		{
			name:      "removal failure retries",
			removeErr: errors.New("status update failed"),
			configure: true,
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:    "missing removal adapter fails",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expectations := workload.NewExpectations()
			expectations.ExpectDeletes(namespace, owner, workload.ComponentEngine, index, 1)
			calls := 0
			input := workload.ReconcileInput{
				Key: workload.Key{Namespace: namespace, OwnerName: owner, Component: workload.ComponentEngine},
			}
			if test.configure {
				input.RemoveInstance = func(context.Context, int32) (bool, error) {
					calls++
					return test.removed, test.removeErr
				}
			}

			complete, err := finalizeAndRemoveInstance(
				context.Background(),
				workload.Deps{Expectations: expectations},
				input,
				index,
				&expected,
			)
			if complete != test.wantComplete || (err != nil) != test.wantErr {
				t.Fatalf("result: complete=%v err=%v", complete, err)
			}
			if calls != test.wantCalls {
				t.Fatalf("RemoveInstance calls=%d want %d", calls, test.wantCalls)
			}
			forgotten := expectations.Satisfied(namespace, owner, workload.ComponentEngine, index)
			if forgotten != test.wantForgotten {
				t.Fatalf("expectations forgotten=%v want %v", forgotten, test.wantForgotten)
			}
		})
	}
}

func TestTerminalLifecycle_FailsClosedWithoutStrongOwnerIdentity(t *testing.T) {
	expected := terminalStatusFixture(3)
	tests := []struct {
		name  string
		input workload.ReconcileInput
	}{
		{
			name: "missing owner",
			input: workload.ReconcileInput{
				ApplyInstanceMutationsWithRetryBlock: (&terminalMutationStore{ownerUID: "owner-a", statuses: map[int32]workload.InstanceStatus{3: expected}}).apply,
			},
		},
		{
			name: "empty owner UID",
			input: workload.ReconcileInput{
				OwnerObject:                          &corev1.ConfigMap{},
				ApplyInstanceMutationsWithRetryBlock: (&terminalMutationStore{statuses: map[int32]workload.InstanceStatus{3: expected}}).apply,
			},
		},
		{
			name: "missing strong adapter",
			input: workload.ReconcileInput{
				OwnerObject:               &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
				ApplyInstanceMutations:    func(context.Context, []workload.InstanceMutation) error { return nil },
				FinalizeInstanceResources: func(context.Context, int32) (bool, error) { return true, nil },
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := transitionTerminalOperationStep(context.Background(), test.input, &expected, updateStepSurgeDrain, true); err == nil {
				t.Fatal("terminal marker accepted an unverifiable owner or adapter")
			}
			if test.input.FinalizeInstanceResources == nil {
				test.input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
			}
			if _, err := finalizeAndRemoveInstance(context.Background(), workload.Deps{}, test.input, 3, &expected); err == nil {
				t.Fatal("terminal finalization accepted an unverifiable owner or adapter")
			}
		})
	}
}

func TestRestoreGangSurgeTargetMarker_StrongOwnershipGuard(t *testing.T) {
	source := terminalStatusFixture(3)
	surgeIndex := *source.Operation.SurgeIndex
	targetRevision := source.Operation.TargetRevision
	tests := []struct {
		name           string
		prepare        func(*terminalMutationStore)
		wantResolution gangSurgeTargetMarkerResolution
		wantWrites     int
	}{
		{
			name:           "exact source restores marker",
			wantResolution: gangSurgeTargetMarkerRestored,
			wantWrites:     1,
		},
		{
			name: "wire-normalized source restores marker",
			prepare: func(store *terminalMutationStore) {
				current := store.statuses[source.Index]
				current.Operation.StartedAt = metav1.NewTime(current.Operation.StartedAt.Truncate(time.Second))
				current.Operation.LastProgressAt = metav1.NewTime(current.Operation.LastProgressAt.Truncate(time.Second))
				store.statuses[source.Index] = current
			},
			wantResolution: gangSurgeTargetMarkerRestored,
			wantWrites:     1,
		},
		{
			name: "existing matching marker is confirmed",
			prepare: func(store *terminalMutationStore) {
				store.statuses[surgeIndex] = workload.InstanceStatus{
					Index:          surgeIndex,
					Incarnation:    1,
					Phase:          workload.InstancePhaseCreating,
					TargetRevision: targetRevision,
					Operation: &workload.InstanceOperation{
						ID:             "existing-target",
						Type:           workload.InstanceOperationUpdate,
						Step:           workload.UpdateStepGangSurgeTarget,
						TargetRevision: targetRevision,
					},
				}
			},
			wantResolution: gangSurgeTargetMarkerActive,
		},
		{
			name: "existing cleanup marker is reported",
			prepare: func(store *terminalMutationStore) {
				marker := workload.InstanceStatus{
					Index:          surgeIndex,
					Incarnation:    1,
					Phase:          workload.InstancePhaseCreating,
					TargetRevision: targetRevision,
					Operation: &workload.InstanceOperation{
						ID:             "existing-target",
						Type:           workload.InstanceOperationUpdate,
						Step:           workload.UpdateStepGangSurgeTargetCleanup,
						TargetRevision: targetRevision,
					},
				}
				store.statuses[surgeIndex] = marker
			},
			wantResolution: gangSurgeTargetMarkerCleanup,
		},
		{
			name: "source ownership drift rejects marker",
			prepare: func(store *terminalMutationStore) {
				current := store.statuses[source.Index]
				current.Operation.ID = "replacement-source"
				store.statuses[source.Index] = current
			},
		},
		{
			name: "owner recreation rejects marker",
			prepare: func(store *terminalMutationStore) {
				store.ownerUID = "owner-b"
			},
		},
		{
			name: "occupied target rejects marker",
			prepare: func(store *terminalMutationStore) {
				store.statuses[surgeIndex] = workload.InstanceStatus{
					Index: surgeIndex, Incarnation: 4, Phase: workload.InstancePhaseReady,
					RunningRevision: "unrelated",
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &terminalMutationStore{
				ownerUID: "owner-a",
				statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
			}
			if test.prepare != nil {
				test.prepare(store)
			}
			input := workload.ReconcileInput{
				OwnerObject:                          &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
				FinalizeInstanceResources:            func(context.Context, int32) (bool, error) { return true, nil },
				ApplyInstanceMutationsWithRetryBlock: store.apply,
			}

			resolution, err := restoreInstanceStatusGangSurgeTarget(
				context.Background(), input, &source, surgeIndex, targetRevision, time.Minute,
			)
			if err != nil || resolution != test.wantResolution {
				t.Fatalf("result: resolution=%v err=%v", resolution, err)
			}
			if store.writes != test.wantWrites {
				t.Fatalf("status writes=%d want %d", store.writes, test.wantWrites)
			}
			if test.wantResolution == gangSurgeTargetMarkerRestored || test.wantResolution == gangSurgeTargetMarkerActive {
				marker := store.statuses[surgeIndex]
				if !gangSurgeTargetMatches(&marker, targetRevision) {
					t.Fatalf("target marker=%+v", marker)
				}
			}
		})
	}
}

func TestTerminalRemovalOwnershipPredicates(t *testing.T) {
	surge := int32(9)
	migration := terminalStatusFixture(3)
	migration.Phase = workload.InstancePhaseMigrating
	migration.TargetRevision = ""
	migration.Operation.Type = workload.InstanceOperationMigrate
	migration.Operation.RequestUUID = "migration-a"
	if !migrationSourceOwnsRemoval(&migration, "migration-a", surge) ||
		migrationSourceOwnsRemoval(&migration, "migration-b", surge) ||
		migrationSourceOwnsRemoval(&migration, "migration-a", surge+1) {
		t.Fatal("migration source ownership did not bind request and surge identities")
	}
	wrongMigrationPhase := migration
	wrongMigrationPhase.Phase = workload.InstancePhaseReady
	if migrationSourceOwnsRemoval(&wrongMigrationPhase, "migration-a", surge) {
		t.Fatal("migration source ownership accepted a non-migrating lifecycle")
	}

	gangSource := terminalStatusFixture(3)
	gangSource.Operation.Step = updateStepSurgeDrain
	if !gangSurgeSourceOwnsRemoval(&gangSource, surge) || gangSurgeSourceOwnsRemoval(&gangSource, surge+1) {
		t.Fatal("gang source ownership did not bind the replacement index")
	}

	gangTarget := terminalStatusFixture(surge)
	gangTarget.Operation.Step = workload.UpdateStepGangSurgeTarget
	if !gangSurgeTargetOwnsRemoval(&gangTarget, false) || gangSurgeTargetOwnsRemoval(&gangTarget, true) {
		t.Fatal("gang target ownership accepted the wrong cleanup phase")
	}
	gangTarget.Operation.Step = workload.UpdateStepGangSurgeTargetCleanup
	if !gangSurgeTargetOwnsRemoval(&gangTarget, true) || gangSurgeTargetOwnsRemoval(&gangTarget, false) {
		t.Fatal("gang target cleanup ownership accepted the wrong deployment mode")
	}
}
