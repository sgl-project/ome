package validation

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// ratioMemberShape is one Component of a maintainRatio group as a test
// declares it: its replica count (zero leaves both replica fields unset),
// its update strategy, and whether its Instances are leader+worker gangs.
type ratioMemberShape struct {
	replicas int
	strategy v1beta1.UpdateStrategyType
	gang     bool
}

func ratioExt(m ratioMemberShape) v1beta1.ComponentExtensionSpec {
	ext := v1beta1.ComponentExtensionSpec{
		Annotations: map[string]string{constants.DeploymentMode: string(constants.OMENative)},
	}
	if m.replicas > 0 {
		ext.MinReplicas = ptr.To(m.replicas)
		ext.MaxReplicas = m.replicas
	}
	if m.strategy != "" {
		ext.Lifecycle = &v1beta1.LifecycleSpec{UpdateStrategy: &v1beta1.UpdateStrategy{Type: m.strategy}}
	}
	return ext
}

// ratioSpec builds an OMENative spec with the given members and one group
// over all of them. A nil member leaves the Component undeclared; a nil
// tolerance omits maintainRatio.tolerance.
func ratioSpec(engine, decoder, router *ratioMemberShape, tolerance *int32) *v1beta1.InferenceServiceSpec {
	spec := &v1beta1.InferenceServiceSpec{}
	var members []v1beta1.ComponentType
	runner := &v1beta1.RunnerSpec{Container: corev1.Container{Image: "runtime-image:v1"}}
	if engine != nil {
		spec.Engine = &v1beta1.EngineSpec{ComponentExtensionSpec: ratioExt(*engine)}
		if engine.gang {
			spec.Engine.Leader = &v1beta1.LeaderSpec{Runner: runner.DeepCopy()}
			spec.Engine.Worker = &v1beta1.WorkerSpec{Size: ptr.To(1), Runner: runner.DeepCopy()}
		}
		members = append(members, v1beta1.EngineComponent)
	}
	if decoder != nil {
		spec.Decoder = &v1beta1.DecoderSpec{ComponentExtensionSpec: ratioExt(*decoder)}
		if decoder.gang {
			spec.Decoder.Leader = &v1beta1.LeaderSpec{Runner: runner.DeepCopy()}
			spec.Decoder.Worker = &v1beta1.WorkerSpec{Size: ptr.To(1), Runner: runner.DeepCopy()}
		}
		members = append(members, v1beta1.DecoderComponent)
	}
	if router != nil {
		spec.Router = &v1beta1.RouterSpec{ComponentExtensionSpec: ratioExt(*router)}
		members = append(members, v1beta1.RouterComponent)
	}
	spec.Rollout = &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{{
		Components:    members,
		RollingUpdate: &v1beta1.GroupRollingUpdate{},
		MaintainRatio: &v1beta1.MaintainRatio{Tolerance: tolerance},
	}}}
	return spec
}

func TestValidateRatioDrainFeasibility(t *testing.T) {
	recreate := v1beta1.UpdateStrategyRecreatePod
	surge := v1beta1.UpdateStrategySurgeThenDrain
	inPlace := v1beta1.UpdateStrategyInPlaceIfPossible
	inPlaceOnly := v1beta1.UpdateStrategyInPlaceOnly

	tests := []struct {
		name               string
		spec               *v1beta1.InferenceServiceSpec
		defaultTolerance   *int32
		defaultMinReplicas *int
		// wantSubstrings are all expected in the rejection; empty accepts.
		wantSubstrings []string
	}{
		{
			name: "6:2 RecreatePod on both sides at tolerance 20: the batch step paces the group, admitted",
			spec: ratioSpec(&ratioMemberShape{6, recreate, false}, &ratioMemberShape{2, recreate, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "6:2 with a SurgeThenDrain engine and a RecreatePod decoder at tolerance 20: one decoder out is 100 points off, cap is 40",
			spec: ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, recreate, false}, nil, ptr.To[int32](20)),
			wantSubstrings: []string{"spec.rollout.groups[0]", "the decoder rolls drain-first (RecreatePod)",
				"with decoder 2 and engine 6", "one decoder pod out reads decoder 1 : engine 6", "100 points off",
				"at most 40 points (twice the tolerance of 20)", "widen maintainRatio.tolerance to at least 50",
				"SurgeThenDrain", "add decoder replicas", "roll every member of the group drain-first", ReasonRatioDrainInfeasible},
		},
		{
			name: "the same mixed group at tolerance 50: 100 points is exactly twice the tolerance, admitted",
			spec: ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, recreate, false}, nil, ptr.To[int32](50)),
		},
		{
			name:           "a RecreatePod decoder beside an engine with an unset strategy at tolerance 20: the unset member is not read as drain-first, so the ratio check keeps the decoder",
			spec:           ratioSpec(&ratioMemberShape{6, "", false}, &ratioMemberShape{2, recreate, false}, nil, ptr.To[int32](20)),
			wantSubstrings: []string{"the decoder rolls drain-first (RecreatePod)", "100 points off", ReasonRatioDrainInfeasible},
		},
		{
			name: "a single-replica RecreatePod decoder beside a SurgeThenDrain engine is rejected even at tolerance 100",
			spec: ratioSpec(&ratioMemberShape{4, surge, false}, &ratioMemberShape{1, recreate, false}, nil, ptr.To[int32](100)),
			wantSubstrings: []string{"the decoder rolls drain-first (RecreatePod)", "single replica", "nothing serving",
				"SurgeThenDrain", "add decoder replicas", "roll every member of the group drain-first", ReasonRatioDrainInfeasible},
		},
		{
			name: "4:1 RecreatePod on both sides at tolerance 20: the side of one goes in the first batch, the ratio check is not consulted, admitted",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{1, recreate, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "4:4 with a RecreatePod engine and a SurgeThenDrain decoder at tolerance 10: one out is 33 points off, cap is 20",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{4, surge, false}, nil, ptr.To[int32](10)),
			wantSubstrings: []string{"the engine rolls drain-first (RecreatePod)", "one engine pod out reads engine 3 : decoder 4",
				"33 points off", "at most 20 points (twice the tolerance of 10)", "at least 17", ReasonRatioDrainInfeasible},
		},
		{
			name: "the same mixed 4:4 at tolerance 20: 33 points is under the cap of 40, admitted",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{4, surge, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "4:4 RecreatePod on both sides at tolerance 10: the batch step paces the group, admitted",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{4, recreate, false}, nil, ptr.To[int32](10)),
		},
		{
			name: "6:2 SurgeThenDrain at tolerance 20: a surge adds capacity and is not a drain",
			spec: ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, surge, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "6:2 in-place single pods at tolerance 20: a same-pod patch keeps capacity",
			spec: ratioSpec(&ratioMemberShape{6, inPlace, false}, &ratioMemberShape{2, inPlace, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "6:2 in-place gangs on both sides at tolerance 20: every member rebuilds its gang, the batch step paces, admitted",
			spec: ratioSpec(&ratioMemberShape{6, inPlace, true}, &ratioMemberShape{2, inPlace, true}, nil, ptr.To[int32](20)),
		},
		{
			name: "6:2 with a SurgeThenDrain engine gang and an in-place decoder gang at tolerance 20: the gang's in-place update rebuilds the gang, read as a drain",
			spec: ratioSpec(&ratioMemberShape{6, surge, true}, &ratioMemberShape{2, inPlace, true}, nil, ptr.To[int32](20)),
			wantSubstrings: []string{"the decoder rolls drain-first (InPlaceIfPossible on multi-node gangs, which rebuilds the whole gang)",
				"one decoder gang out", ReasonRatioDrainInfeasible},
		},
		{
			name: "2:2 with an InPlaceOnly engine gang and a SurgeThenDrain decoder gang at tolerance 25: one gang out is 100 points off, cap is 50",
			spec: ratioSpec(&ratioMemberShape{2, inPlaceOnly, true}, &ratioMemberShape{2, surge, true}, nil, ptr.To[int32](25)),
			wantSubstrings: []string{"the engine rolls drain-first (InPlaceOnly on multi-node gangs, which rebuilds the whole gang)",
				"100 points off", ReasonRatioDrainInfeasible},
		},
		{
			name: "2:2 InPlaceOnly gangs on both sides at tolerance 25: the batch step paces the group, admitted",
			spec: ratioSpec(&ratioMemberShape{2, inPlaceOnly, true}, &ratioMemberShape{2, inPlaceOnly, true}, nil, ptr.To[int32](25)),
		},
		{
			name: "a RecreatePod engine gang beside an in-place decoder gang at tolerance 20: both rebuild whole, the batch step paces, admitted",
			spec: ratioSpec(&ratioMemberShape{3, recreate, true}, &ratioMemberShape{1, inPlace, true}, nil, ptr.To[int32](20)),
		},
		{
			name: "router+engine+decoder at tolerance 15: the engine passes against the decoder but fails against the router",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{2, surge, false}, &ratioMemberShape{5, surge, false}, ptr.To[int32](15)),
			wantSubstrings: []string{"the engine rolls drain-first (RecreatePod)", "with engine 4 and router 5",
				"one engine pod out reads engine 3 : router 5", "33 points off", "at most 30 points", ReasonRatioDrainInfeasible},
		},
		{
			name: "router+engine+decoder at tolerance 17: every pair of the engine's drain is within 34 points",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{2, surge, false}, &ratioMemberShape{5, surge, false}, ptr.To[int32](17)),
		},
		{
			name: "router+engine+decoder all RecreatePod at tolerance 15: three drain-first members, the batch step paces, admitted",
			spec: ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{2, recreate, false}, &ratioMemberShape{5, recreate, false}, ptr.To[int32](15)),
		},
		{
			name:           "router+engine+decoder with a surging router at tolerance 15: one surging member keeps the ratio check on the others",
			spec:           ratioSpec(&ratioMemberShape{4, recreate, false}, &ratioMemberShape{2, recreate, false}, &ratioMemberShape{5, surge, false}, ptr.To[int32](15)),
			wantSubstrings: []string{"the engine rolls drain-first (RecreatePod)", "with engine 4 and router 5", ReasonRatioDrainInfeasible},
		},
		{
			name:             "omitted tolerance with no operator default: no drift bound, nothing to check",
			spec:             ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, recreate, false}, nil, nil),
			defaultTolerance: nil,
		},
		{
			name:             "omitted tolerance resolves to the operator default of 20 and the mixed group is rejected",
			spec:             ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, recreate, false}, nil, nil),
			defaultTolerance: ptr.To[int32](20),
			wantSubstrings:   []string{"twice the tolerance of 20", ReasonRatioDrainInfeasible},
		},
		{
			name:             "an explicit tolerance of 50 wins over an operator default of 20",
			spec:             ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{2, recreate, false}, nil, ptr.To[int32](50)),
			defaultTolerance: ptr.To[int32](20),
		},
		{
			name: "an unset update strategy is not classified drain-first at admission",
			spec: ratioSpec(&ratioMemberShape{6, "", false}, &ratioMemberShape{2, "", false}, nil, ptr.To[int32](20)),
		},
		{
			name: "a RecreatePod decoder with no replica fields beside a SurgeThenDrain engine is sized at the API default of one and rejected",
			spec: ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{0, recreate, false}, nil, ptr.To[int32](20)),
			wantSubstrings: []string{"the decoder rolls drain-first (RecreatePod)", "single replica", "nothing serving",
				ReasonRatioDrainInfeasible},
		},
		{
			name: "a RecreatePod decoder with no replica fields beside a RecreatePod engine: sized at one, the batch step paces, admitted",
			spec: ratioSpec(&ratioMemberShape{6, recreate, false}, &ratioMemberShape{0, recreate, false}, nil, ptr.To[int32](20)),
		},
		{
			name: "a SurgeThenDrain decoder with no replica fields is admitted",
			spec: ratioSpec(&ratioMemberShape{6, recreate, false}, &ratioMemberShape{0, surge, false}, nil, ptr.To[int32](20)),
		},
		{
			name:               "no replica fields with a configured minReplicas default of 2: sized at 2, the mixed 6:2 at tolerance 20 rejected",
			spec:               ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{0, recreate, false}, nil, ptr.To[int32](20)),
			defaultMinReplicas: ptr.To(2),
			wantSubstrings:     []string{"with decoder 2 and engine 6", "100 points off", ReasonRatioDrainInfeasible},
		},
		{
			name:               "no replica fields with a configured minReplicas default of 2: sized at 2, the mixed 6:2 at tolerance 50 admitted",
			spec:               ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{0, recreate, false}, nil, ptr.To[int32](50)),
			defaultMinReplicas: ptr.To(2),
		},
		{
			name: "an explicit minReplicas of 0 falls to maxReplicas, the way an unset one does",
			spec: func() *v1beta1.InferenceServiceSpec {
				spec := ratioSpec(&ratioMemberShape{6, surge, false}, &ratioMemberShape{4, recreate, false}, nil, ptr.To[int32](20))
				spec.Decoder.MinReplicas = ptr.To(0)
				return spec
			}(),
		},
		{
			name: "the engine side of 6:2 is feasible: one engine out is 17 points off",
			spec: ratioSpec(&ratioMemberShape{6, recreate, false}, &ratioMemberShape{2, surge, false}, nil, ptr.To[int32](20)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRatioDrainFeasibility(tt.spec, RatioDrainDefaults{TolerancePercent: tt.defaultTolerance, MinReplicas: tt.defaultMinReplicas})
			if len(tt.wantSubstrings) == 0 {
				if err != nil {
					t.Fatalf("expected the spec to be admitted, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected a rejection naming %q, got nil", tt.wantSubstrings[0])
			}
			for _, want := range tt.wantSubstrings {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("rejection lacks %q:\n%s", want, err.Error())
				}
			}
		})
	}
}

func TestValidateRatioDrainFeasibility_MaxReplicasFallback(t *testing.T) {
	spec := ratioSpec(&ratioMemberShape{6, v1beta1.UpdateStrategySurgeThenDrain, false},
		&ratioMemberShape{2, v1beta1.UpdateStrategyRecreatePod, false}, nil, ptr.To[int32](20))
	spec.Engine.MinReplicas = nil
	spec.Decoder.MinReplicas = nil
	err := ValidateRatioDrainFeasibility(spec, RatioDrainDefaults{})
	if err == nil || !strings.Contains(err.Error(), "with decoder 2 and engine 6") {
		t.Fatalf("maxReplicas should anchor a member whose minReplicas is unset, got %v", err)
	}
}

func TestValidateRatioDrainFeasibility_CanaryGroupSkipped(t *testing.T) {
	spec := ratioSpec(&ratioMemberShape{6, v1beta1.UpdateStrategySurgeThenDrain, false},
		&ratioMemberShape{2, v1beta1.UpdateStrategyRecreatePod, false}, nil, ptr.To[int32](20))
	spec.Rollout.Groups[0].RollingUpdate = nil
	spec.Rollout.Groups[0].Canary = &v1beta1.GroupCanary{}
	if err := ValidateRatioDrainFeasibility(spec, RatioDrainDefaults{}); err != nil {
		t.Fatalf("a canary group is the canary validator's to reject, got %v", err)
	}
}

func TestValidateRatioDrainFeasibility_NilAndNoGroups(t *testing.T) {
	defaults := RatioDrainDefaults{TolerancePercent: ptr.To[int32](20)}
	if err := ValidateRatioDrainFeasibility(nil, defaults); err != nil {
		t.Fatalf("nil spec: %v", err)
	}
	if err := ValidateRatioDrainFeasibility(&v1beta1.InferenceServiceSpec{}, defaults); err != nil {
		t.Fatalf("no groups: %v", err)
	}
}

// A member the group names but the spec does not declare has no count to
// size, so it is left out and a single remaining member has nothing to pair
// against (the orphan rule rejects the shape before this one runs).
func TestValidateRatioDrainFeasibility_UndeclaredMemberLeftOut(t *testing.T) {
	spec := ratioSpec(&ratioMemberShape{6, v1beta1.UpdateStrategySurgeThenDrain, false},
		&ratioMemberShape{1, v1beta1.UpdateStrategyRecreatePod, false}, nil, ptr.To[int32](20))
	spec.Decoder = nil
	if err := ValidateRatioDrainFeasibility(spec, RatioDrainDefaults{}); err != nil {
		t.Fatalf("an undeclared member leaves a single member, got %v", err)
	}
}

// The update path runs the same rule on the new spec: a decoder scaled from
// four to one under RecreatePod beside a surging engine is refused, where the
// old shape was admitted.
func TestValidateRatioDrainFeasibility_ScaleDownToOneRejected(t *testing.T) {
	old := ratioSpec(&ratioMemberShape{4, v1beta1.UpdateStrategySurgeThenDrain, false},
		&ratioMemberShape{4, v1beta1.UpdateStrategyRecreatePod, false}, nil, ptr.To[int32](20))
	if err := ValidateRatioDrainFeasibility(old, RatioDrainDefaults{}); err != nil {
		t.Fatalf("4:4 at tolerance 20 should be admitted, got %v", err)
	}
	updated := old.DeepCopy()
	updated.Decoder.MinReplicas = ptr.To(1)
	updated.Decoder.MaxReplicas = 1
	err := ValidateRatioDrainFeasibility(updated, RatioDrainDefaults{})
	if err == nil || !strings.Contains(err.Error(), "single replica") || !strings.Contains(err.Error(), ReasonRatioDrainInfeasible) {
		t.Fatalf("scaling the decoder to one under RecreatePod should be refused, got %v", err)
	}
}

// The same scale-down under RecreatePod on both sides is admitted: the batch
// step paces the group and takes the side of one in its first batch, so the
// ratio check that would refuse the drain is never consulted.
func TestValidateRatioDrainFeasibility_ScaleDownToOneUnderBatchStepAdmitted(t *testing.T) {
	spec := ratioSpec(&ratioMemberShape{4, v1beta1.UpdateStrategyRecreatePod, false},
		&ratioMemberShape{1, v1beta1.UpdateStrategyRecreatePod, false}, nil, ptr.To[int32](20))
	if err := ValidateRatioDrainFeasibility(spec, RatioDrainDefaults{}); err != nil {
		t.Fatalf("4:1 RecreatePod on both sides should be admitted, got %v", err)
	}
}

// batchStepGoverns reads the engine's rule off the declared members: every
// member drain-first, two or more of them.
func TestBatchStepGoverns(t *testing.T) {
	drain := func(c v1beta1.ComponentType) ratioMember {
		return ratioMember{component: c, replicas: 2, drainFirst: "RecreatePod"}
	}
	surge := func(c v1beta1.ComponentType) ratioMember { return ratioMember{component: c, replicas: 2} }
	if !batchStepGoverns([]ratioMember{drain(v1beta1.EngineComponent), drain(v1beta1.DecoderComponent)}) {
		t.Fatal("two drain-first members are paced by the batch step")
	}
	if batchStepGoverns([]ratioMember{drain(v1beta1.EngineComponent), surge(v1beta1.DecoderComponent)}) {
		t.Fatal("a surging member keeps the ratio check in charge")
	}
	if batchStepGoverns([]ratioMember{drain(v1beta1.EngineComponent)}) {
		t.Fatal("a single member has no peer to batch with")
	}
	if batchStepGoverns([]ratioMember{drain(v1beta1.EngineComponent), drain(v1beta1.DecoderComponent), surge(v1beta1.RouterComponent)}) {
		t.Fatal("a surging router keeps the ratio check in charge of the whole group")
	}
}
