package coordination

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// The batch step: a group of drain-first members rolls in batches of each
// member's own step, and nobody opens batch b+1 until everybody's batch b
// serves. Every table below uses a prefill/decode service
// with 6 prefill pods rolled 3 at a time beside 2 decode pods rolled 1 at a
// time, both under RecreatePod, the decode replacement Ready in seconds and
// the prefill replacement in minutes.

const batchServiceName, batchNamespace = "svc", "ns"

// batchRevision names a Component's revision the way the replica records
// it; "target" is the revision forwardTarget builds for the gate.
func batchRevision(comp v1beta1.ComponentType, which string) string {
	return batchServiceName + "-" + string(comp) + "-" + which
}

// batchMemberFixture is one member of the group as its InferenceReplica
// publishes it: the desired count, its own maxUnavailable, its strategy and
// shape, and its Instances by what the roll has done to them. Instances not
// listed are Ready on the old revision.
type batchMemberFixture struct {
	desired  int32
	step     string
	strategy v1beta1.UpdateStrategyType
	gang     bool
	promoted int32 // Ready on the target revision
	inFlight int32 // a recreate toward the target is open
	inPlace  int32 // an in-place patch toward the target is open
	surging  int32 // a surge toward the target is open, the source still serving
	failed   int32 // the attempt toward the target ended Failed
	parked   int32 // the attempt toward the target is parked
	creating int32 // created on the target by a scale-up, not serving yet
}

func batchReplica(comp v1beta1.ComponentType, m batchMemberFixture) *v1beta1.InferenceReplica {
	target, old := batchRevision(comp, "target"), batchRevision(comp, "old")
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Namespace: batchNamespace, Name: batchServiceName + "-" + string(comp)},
		Spec: v1beta1.InferenceReplicaSpec{
			Component: comp,
			Replicas:  ptr.To(m.desired),
			Lifecycle: &v1beta1.LifecycleSpec{UpdateStrategy: &v1beta1.UpdateStrategy{Type: m.strategy}},
		},
		Status: v1beta1.InferenceReplicaStatus{CurrentRevision: old, UpdateRevision: target, Replicas: m.desired},
	}
	if m.step != "" {
		ir.Spec.Lifecycle.UpdateStrategy.RollingUpdate = &v1beta1.RollingUpdate{MaxUnavailable: ptr.To(intstr.Parse(m.step))}
	}
	if m.gang {
		ir.Spec.Runners = []v1beta1.Runner{{Name: v1beta1.RunnerNameLeader, Size: 1}, {Name: v1beta1.RunnerNameWorker, Size: 2}}
	}
	attempt := func(step string) *v1beta1.InstanceOperation {
		return &v1beta1.InstanceOperation{ID: "attempt", Type: v1beta1.InstanceOperationUpdate, Step: step, TargetRevision: target}
	}
	var rows []v1beta1.OMENativeInstanceStatus
	add := func(n int32, row v1beta1.OMENativeInstanceStatus) {
		for i := int32(0); i < n; i++ {
			row.Index = int32(len(rows))
			if row.Operation != nil {
				op := *row.Operation
				row.Operation = &op
			}
			rows = append(rows, row)
		}
	}
	add(m.promoted, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceReady, RunningRevision: target, PodCount: 1, ServingPodCount: 1})
	add(m.inFlight, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceUpdating, RunningRevision: old, TargetRevision: target, Operation: attempt(workloadtypes.UpdateStepDrain)})
	add(m.inPlace, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceUpdating, RunningRevision: old, TargetRevision: target, PodCount: 1, Operation: attempt(workloadtypes.UpdateStepInPlace)})
	add(m.surging, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceUpdating, RunningRevision: old, TargetRevision: target, PodCount: 1, ServingPodCount: 1, Operation: attempt(workloadtypes.UpdateStepSurge)})
	add(m.failed, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceFailed, RunningRevision: old, TargetRevision: target, Operation: attempt(workloadtypes.UpdateStepDrain)})
	add(m.parked, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceReady, RunningRevision: old, TargetRevision: target, PodCount: 1, ServingPodCount: 1, Operation: attempt(workloadtypes.UpdateStepParked)})
	add(m.creating, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceCreating, TargetRevision: target, Operation: &v1beta1.InstanceOperation{ID: "create", Type: v1beta1.InstanceOperationCreate, TargetRevision: target}})
	untouched := m.desired - int32(len(rows))
	add(untouched, v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceReady, RunningRevision: old, PodCount: 1, ServingPodCount: 1})
	ir.Status.InstanceStatuses = rows
	ir.Status.UpdatedReplicas = m.promoted
	ir.Status.ServingReplicas = m.promoted + m.surging + m.parked + untouched
	ir.Status.ReadyReplicas = ir.Status.ServingReplicas
	return ir
}

// batchGroupFixture is the group the members roll in: one rollingUpdate
// group over the listed Components (engine and decoder when empty) with the
// given group maxUnavailable, under maintainRatio at tolerance when set.
type batchGroupFixture struct {
	maxUnavailable string
	tolerance      *int32
	components     []v1beta1.ComponentType
}

func batchService(g batchGroupFixture, members map[v1beta1.ComponentType]batchMemberFixture) *v1beta1.InferenceService {
	components := g.components
	if len(components) == 0 {
		components = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}
	}
	group := v1beta1.RolloutGroup{
		Components:    components,
		RollingUpdate: &v1beta1.GroupRollingUpdate{MaxUnavailable: ptr.To(intstr.Parse(g.maxUnavailable))},
	}
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: batchServiceName, Namespace: batchNamespace},
		Spec:       v1beta1.InferenceServiceSpec{Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{group}}},
	}
	if g.tolerance != nil {
		isvc.Spec.Rollout.Groups[0].MaintainRatio = &v1beta1.MaintainRatio{Tolerance: g.tolerance}
		original := map[v1beta1.ComponentType]int32{}
		for comp, m := range members {
			original[comp] = m.desired
		}
		isvc.Status.RolloutCoordination = &v1beta1.RolloutCoordinationStatus{Groups: []v1beta1.RolloutCoordinationGroupStatus{{
			Name:          "0",
			Components:    components,
			Policy:        v1beta1.CoordinationPolicyRollingUpdate,
			ObservedRatio: &v1beta1.RolloutCoordinationRatio{Original: original},
		}}}
	}
	return pinActiveRun(isvc)
}

func batchClient(t *testing.T, members map[v1beta1.ComponentType]batchMemberFixture) client.Client {
	t.Helper()
	var objs []client.Object
	for comp, m := range members {
		objs = append(objs, batchReplica(comp, m))
	}
	return fake.NewClientBuilder().WithScheme(pairingScheme(t)).WithObjects(objs...).Build()
}

// batchGate asks the production decision site about one more drain of comp
// on the given mechanism, with inFlight starts already taken this wake-up.
func batchGate(t *testing.T, isvc *v1beta1.InferenceService, c client.Client, comp v1beta1.ComponentType, mechanism workloadtypes.UpdateStrategyType, inFlight int32) (bool, v1beta1.RolloutHoldGate, string) {
	t.Helper()
	return EvaluateUpdateGate(context.Background(), c, isvc, comp, "", forwardTarget(isvc, comp), nil, GroupDefaults{}, mechanism, 0, inFlight)
}

// batchPacings are the two pacings the batch step applies under alike.
var batchPacings = []struct {
	name      string
	tolerance *int32
}{
	{name: "PerComponent"},
	{name: "maintainRatio-50", tolerance: ptr.To(int32(50))},
}

type batchCase struct {
	name       string
	group      string
	engine     batchMemberFixture
	decoder    batchMemberFixture
	comp       v1beta1.ComponentType
	inFlight   int32
	wantHold   bool
	wantReason string
}

// TestEvaluateUpdateGate_BatchStepPacesDrainFirstMembers holds the batch step
// to the worked examples: 6 prefill at 3 beside 2 decode at 1, 8 at 2 beside
// 4 at 1, and 7 at 3 with a last batch of one. A row reads "with these
// Instances promoted, moving or ended, may comp open one more drain?", and
// a hold is reported under the Ratio gate with the batch reason.
func TestEvaluateUpdateGate_BatchStepPacesDrainFirstMembers(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	recreate := v1beta1.UpdateStrategyRecreatePod
	six := func(m batchMemberFixture) batchMemberFixture {
		m.desired, m.step, m.strategy = 6, "3", recreate
		return m
	}
	two := func(m batchMemberFixture) batchMemberFixture {
		m.desired, m.step, m.strategy = 2, "1", recreate
		return m
	}
	eight := func(m batchMemberFixture) batchMemberFixture {
		m.desired, m.step, m.strategy = 8, "2", recreate
		return m
	}
	four := func(m batchMemberFixture) batchMemberFixture {
		m.desired, m.step, m.strategy = 4, "1", recreate
		return m
	}
	seven := func(m batchMemberFixture) batchMemberFixture {
		m.desired, m.step, m.strategy = 7, "3", recreate
		return m
	}
	cases := []batchCase{
		{
			name: "6:2 at 3/1 decode fast: the decoder's second drain waits for the first prefill batch", group: "3",
			engine: six(batchMemberFixture{inFlight: 3}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (0 of 3 serving)",
		},
		{
			name: "6:2 at 3/1 decode fast: one new prefill serving is not the batch", group: "3",
			engine: six(batchMemberFixture{promoted: 1, inFlight: 2}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (1 of 3 serving)",
		},
		{
			name: "6:2 at 3/1 prefill fast: the engine's fourth drain waits for the first decode", group: "3",
			engine: six(batchMemberFixture{promoted: 3}), decoder: two(batchMemberFixture{inFlight: 1}), comp: eng,
			wantHold: true, wantReason: "engine batch 2 waits for decoder batch 1 to serve (0 of 1 serving)",
		},
		{
			name: "6:2 at 3/1 at the step boundary: the engine's fourth drain is admitted", group: "3",
			engine: six(batchMemberFixture{promoted: 3}), decoder: two(batchMemberFixture{promoted: 1}), comp: eng,
		},
		{
			name: "6:2 at 3/1 at the step boundary: the decoder's second drain is admitted", group: "3",
			engine: six(batchMemberFixture{promoted: 3}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "6:2 at 3/1 first batch: the third prefill drain of one wake-up is admitted", group: "3",
			engine: six(batchMemberFixture{}), decoder: two(batchMemberFixture{}), comp: eng, inFlight: 2,
		},
		{
			name: "6:2 at 3/1 first batch: the decoder's first drain is admitted beside three prefill drains", group: "3",
			engine: six(batchMemberFixture{inFlight: 3}), decoder: two(batchMemberFixture{}), comp: dec,
		},
		{
			name: "6:2 at 3/1 first batch: a fourth prefill drain opens batch 2 before batch 1 serves", group: "3",
			engine: six(batchMemberFixture{inFlight: 3}), decoder: two(batchMemberFixture{inFlight: 1}), comp: eng,
			wantHold: true, wantReason: "engine batch 2 waits for decoder batch 1 to serve (0 of 1 serving)",
		},
		{
			name: "8:4 at 2/1 decode fast: the decoder's second drain waits for two prefill", group: "2",
			engine: eight(batchMemberFixture{inFlight: 2}), decoder: four(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (0 of 2 serving)",
		},
		{
			name: "8:4 at 2/1 prefill fast: the engine's third drain waits for the first decode", group: "2",
			engine: eight(batchMemberFixture{promoted: 2}), decoder: four(batchMemberFixture{inFlight: 1}), comp: eng,
			wantHold: true, wantReason: "engine batch 2 waits for decoder batch 1 to serve (0 of 1 serving)",
		},
		{
			name: "8:4 at 2/1 at the step boundary: the engine's third drain is admitted", group: "2",
			engine: eight(batchMemberFixture{promoted: 2}), decoder: four(batchMemberFixture{promoted: 1}), comp: eng,
		},
		{
			name: "8:4 at 2/1 at the step boundary: the decoder's second drain is admitted", group: "2",
			engine: eight(batchMemberFixture{promoted: 2}), decoder: four(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "8:4 at 2/1 third decode batch waits for the second prefill batch", group: "2",
			engine: eight(batchMemberFixture{promoted: 3, inFlight: 1}), decoder: four(batchMemberFixture{promoted: 2}), comp: dec,
			wantHold: true, wantReason: "decoder batch 3 waits for engine batch 2 to serve (3 of 4 serving)",
		},
		{
			name: "7 prefill at 3: the last batch of one is admitted once the decoder is done", group: "3",
			engine: seven(batchMemberFixture{promoted: 6}), decoder: two(batchMemberFixture{promoted: 2}), comp: eng,
		},
		{
			name: "7 prefill at 3: the last batch of one waits for the decoder's second batch", group: "3",
			engine: seven(batchMemberFixture{promoted: 6}), decoder: two(batchMemberFixture{promoted: 1, inFlight: 1}), comp: eng,
			wantHold: true, wantReason: "engine batch 3 waits for decoder batch 2 to serve (1 of 2 serving)",
		},
		{
			name: "7 prefill at 3: the decoder's second drain waits for the first batch of three", group: "3",
			engine: seven(batchMemberFixture{promoted: 2, inFlight: 1}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (2 of 3 serving)",
		},
		{
			name: "a prefill whose attempt ended Failed counts as settled: the decoder goes on", group: "3",
			engine: six(batchMemberFixture{promoted: 2, failed: 1}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "a prefill batch half served and half failed still waits for its last Instance", group: "3",
			engine: six(batchMemberFixture{promoted: 1, failed: 1, inFlight: 1}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (1 of 3 serving, 1 failed)",
		},
		{
			name: "a parked prefill attempt counts as settled: the decoder goes on", group: "3",
			engine: six(batchMemberFixture{promoted: 2, parked: 1}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "a finished decoder no longer paces the engine", group: "3",
			engine: six(batchMemberFixture{promoted: 4, inFlight: 1}), decoder: two(batchMemberFixture{promoted: 2}), comp: eng,
		},
		{
			name: "a finished engine no longer paces the decoder", group: "3",
			engine: six(batchMemberFixture{promoted: 6}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "a scale-up from 2 to 6 prefill mid-roll raises the first prefill batch to three", group: "3",
			engine: six(batchMemberFixture{promoted: 2, creating: 4}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
			wantHold: true, wantReason: "decoder batch 2 waits for engine batch 1 to serve (2 of 3 serving)",
		},
		{
			name: "before the scale-up two prefill were the whole first batch", group: "3",
			engine: batchMemberFixture{desired: 2, step: "3", strategy: recreate, promoted: 2}, decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
		{
			name: "a scaled-up prefill that serves completes the first batch", group: "3",
			engine: six(batchMemberFixture{promoted: 3, creating: 3}), decoder: two(batchMemberFixture{promoted: 1}), comp: dec,
		},
	}
	for _, pacing := range batchPacings {
		for _, tc := range cases {
			t.Run(pacing.name+"/"+tc.name, func(t *testing.T) {
				members := map[v1beta1.ComponentType]batchMemberFixture{eng: tc.engine, dec: tc.decoder}
				isvc := batchService(batchGroupFixture{maxUnavailable: tc.group, tolerance: pacing.tolerance}, members)
				allowed, gate, reason := batchGate(t, isvc, batchClient(t, members), tc.comp, workloadtypes.UpdateStrategyRecreatePod, tc.inFlight)
				if tc.wantHold == allowed {
					t.Fatalf("allowed=%v, want hold=%v (gate=%q reason=%q)", allowed, tc.wantHold, gate, reason)
				}
				if !tc.wantHold {
					return
				}
				if gate != v1beta1.RolloutHoldGateRatio {
					t.Errorf("gate=%q, want %q (reason=%q)", gate, v1beta1.RolloutHoldGateRatio, reason)
				}
				if reason != tc.wantReason {
					t.Errorf("reason=%q, want %q", reason, tc.wantReason)
				}
			})
		}
	}
}

// TestEvaluateUpdateGate_BatchStepWaivesTheRatioBand pins that the batch step
// is the lockstep of a drain-first group under maintainRatio: the serving
// band and the fraction lockstep are not consulted for its members. At
// tolerance 20 the band refuses the decoder's first drain for good (one of
// two decode out beside six prefill reads six to one), and the fraction
// lockstep holds an engine two of six ahead of a decoder at zero; the batch
// step admits both, since neither opens a batch past the peer's.
func TestEvaluateUpdateGate_BatchStepWaivesTheRatioBand(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	recreate := v1beta1.UpdateStrategyRecreatePod
	cases := []struct {
		name     string
		engine   batchMemberFixture
		decoder  batchMemberFixture
		comp     v1beta1.ComponentType
		inFlight int32
	}{
		{
			name:   "the decoder's first drain is admitted beside six serving prefill",
			engine: batchMemberFixture{desired: 6, step: "3", strategy: recreate}, decoder: batchMemberFixture{desired: 2, step: "1", strategy: recreate}, comp: dec,
		},
		{
			name:   "the decoder's first drain is admitted beside three prefill drains",
			engine: batchMemberFixture{desired: 6, step: "3", strategy: recreate, inFlight: 3}, decoder: batchMemberFixture{desired: 2, step: "1", strategy: recreate}, comp: dec,
		},
		{
			name:   "the engine's third drain is admitted with two of six promoted and the decoder at zero",
			engine: batchMemberFixture{desired: 6, step: "3", strategy: recreate, promoted: 2}, decoder: batchMemberFixture{desired: 2, step: "1", strategy: recreate, inFlight: 1}, comp: eng,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			members := map[v1beta1.ComponentType]batchMemberFixture{eng: tc.engine, dec: tc.decoder}
			isvc := batchService(batchGroupFixture{maxUnavailable: "3", tolerance: ptr.To(int32(20))}, members)
			allowed, gate, reason := batchGate(t, isvc, batchClient(t, members), tc.comp, workloadtypes.UpdateStrategyRecreatePod, tc.inFlight)
			if !allowed {
				t.Fatalf("held by gate=%q reason=%q; the batch step governs this group and the band must not run", gate, reason)
			}
		})
	}
}

// TestEvaluateUpdateGate_BatchStepGovernsOnlyDrainFirstGroups pins the
// boundary of the rule. The group is governed when every member rolls
// drain-first: RecreatePod, or an in-place strategy on a multi-pod Instance,
// whose update rebuilds the gang and reaches the gate as a RecreatePod
// start. One SurgeThenDrain member, an unset strategy (the SurgeThenDrain
// default) or a single-pod in-place member leaves the other gates in charge,
// which admit a start the batch step would hold.
func TestEvaluateUpdateGate_BatchStepGovernsOnlyDrainFirstGroups(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	cases := []struct {
		name      string
		engine    batchMemberFixture
		decoder   batchMemberFixture
		comp      v1beta1.ComponentType
		mechanism workloadtypes.UpdateStrategyType
		wantHold  bool
	}{
		{
			name:    "both RecreatePod: the engine's fourth drain waits for the first decode",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod, inFlight: 1},
			comp:    eng, mechanism: workloadtypes.UpdateStrategyRecreatePod, wantHold: true,
		},
		{
			name:    "a SurgeThenDrain decoder: the engine's fourth drain is admitted by the other gates",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", strategy: v1beta1.UpdateStrategySurgeThenDrain, surging: 1},
			comp:    eng, mechanism: workloadtypes.UpdateStrategyRecreatePod,
		},
		{
			name:    "a decoder with no strategy set: the engine's fourth drain is admitted by the other gates",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", surging: 1},
			comp:    eng, mechanism: workloadtypes.UpdateStrategyRecreatePod,
		},
		{
			name:    "an in-place engine gang: the decoder's second drain waits for the first gang batch",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyInPlaceIfPossible, gang: true, inFlight: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 1},
			comp:    dec, mechanism: workloadtypes.UpdateStrategyRecreatePod, wantHold: true,
		},
		{
			name:    "an in-place engine gang starts as a RecreatePod and its fourth start waits for the first decode",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyInPlaceOnly, gang: true, promoted: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod, inFlight: 1},
			comp:    eng, mechanism: workloadtypes.UpdateStrategyRecreatePod, wantHold: true,
		},
		{
			name:    "a single-pod in-place engine keeps its pods: the decoder's second drain is admitted by the other gates",
			engine:  batchMemberFixture{desired: 6, step: "3", strategy: v1beta1.UpdateStrategyInPlaceIfPossible, inPlace: 3},
			decoder: batchMemberFixture{desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 1},
			comp:    dec, mechanism: workloadtypes.UpdateStrategyRecreatePod,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			members := map[v1beta1.ComponentType]batchMemberFixture{eng: tc.engine, dec: tc.decoder}
			isvc := batchService(batchGroupFixture{maxUnavailable: "3"}, members)
			allowed, gate, reason := batchGate(t, isvc, batchClient(t, members), tc.comp, tc.mechanism, 0)
			if tc.wantHold == allowed {
				t.Fatalf("allowed=%v, want hold=%v (gate=%q reason=%q)", allowed, tc.wantHold, gate, reason)
			}
			if tc.wantHold && (gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "batch")) {
				t.Errorf("the hold must be the Ratio gate's batch reason, got gate=%q reason=%q", gate, reason)
			}
		})
	}
}

// TestEvaluateUpdateGate_BatchStepIgnoresASingleMemberGroup pins that a
// member alone in its group has nobody to keep step with: its fourth drain
// of six at a step of three is admitted while the group budget of six has
// room, where a peer would have made it batch 2.
func TestEvaluateUpdateGate_BatchStepIgnoresASingleMemberGroup(t *testing.T) {
	eng := v1beta1.EngineComponent
	members := map[v1beta1.ComponentType]batchMemberFixture{
		eng: {desired: 6, step: "3", strategy: v1beta1.UpdateStrategyRecreatePod, inFlight: 3},
	}
	isvc := batchService(batchGroupFixture{maxUnavailable: "6", components: []v1beta1.ComponentType{eng}}, members)
	allowed, gate, reason := batchGate(t, isvc, batchClient(t, members), eng, workloadtypes.UpdateStrategyRecreatePod, 0)
	if !allowed {
		t.Fatalf("a single-member group is not paced in batches; held by gate=%q reason=%q", gate, reason)
	}
}

// TestEvaluateUpdateGate_BatchStepFailsClosedOnAReadError pins that a
// replica the batch step cannot read denies the start with a retryable
// Ratio hold, like the other gates: admitting a drain on unreadable peer
// state is how a batch runs ahead unseen.
func TestEvaluateUpdateGate_BatchStepFailsClosedOnAReadError(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	members := map[v1beta1.ComponentType]batchMemberFixture{
		eng: {desired: 6, step: "3", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 3},
		dec: {desired: 2, step: "1", strategy: v1beta1.UpdateStrategyRecreatePod, promoted: 1},
	}
	isvc := batchService(batchGroupFixture{maxUnavailable: "3"}, members)
	var objs []client.Object
	for comp, m := range members {
		objs = append(objs, batchReplica(comp, m))
	}
	failing := fake.NewClientBuilder().WithScheme(pairingScheme(t)).WithObjects(objs...).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, isIR := obj.(*v1beta1.InferenceReplica); isIR && key.Name == batchServiceName+"-"+string(dec) {
				return errors.New("apiserver unavailable")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}).Build()
	allowed, gate, reason := batchGate(t, isvc, failing, eng, workloadtypes.UpdateStrategyRecreatePod, 0)
	if allowed || gate != v1beta1.RolloutHoldGateRatio || !strings.Contains(reason, "failing closed") {
		t.Fatalf("an unreadable peer must fail the batch step closed under the Ratio gate: allowed=%v gate=%q reason=%q", allowed, gate, reason)
	}
}
