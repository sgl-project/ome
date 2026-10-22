package canary

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/omenative/canary/analysis"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/omenative/coordination"
	"sigs.k8s.io/ome/pkg/rollout"
)

// The analysis gate reads every member the push moved, each on its own
// canary revision, and decides for the unit: a breach on any member rolls
// the unit back, a pass needs every member, and a member not yet read, or
// no member to read, holds the step.

// memberScript answers each member's read from a script keyed by the
// request's Component and keeps every request it was handed. A Component
// the script does not name has no fresh sample.
type memberScript struct {
	outcomes map[string]analysis.Outcome
	at       time.Time
	requests []SampleRequest
}

func (s *memberScript) Get(req SampleRequest, _ time.Time) (analysis.Result, time.Time, bool) {
	s.requests = append(s.requests, req)
	outcome, ok := s.outcomes[req.Key.Component]
	if !ok {
		return analysis.Result{}, time.Time{}, false
	}
	return analysis.Result{Outcome: outcome, Metrics: []analysis.MetricResult{{Name: "err", Value: "0.5", Passed: outcome == analysis.Pass}}}, s.at, true
}

// reads lists the member reads the script saw, as component@revision, in order.
func (s *memberScript) reads() []string {
	out := make([]string, 0, len(s.requests))
	for _, req := range s.requests {
		out = append(out, req.Key.Component+"@"+req.Key.Revision)
	}
	return out
}

func wantReads(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("members read = %v, want %v", got, want)
	}
}

// wantMetricNames checks the names the unit's status records, in order.
func wantMetricNames(t *testing.T, cs *v1beta1.CanaryStatus, want ...string) {
	t.Helper()
	got := make([]string, 0, len(cs.MetricResults))
	for _, m := range cs.MetricResults {
		got = append(got, m.Name)
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("metricResults = %+v, want names %v", cs.MetricResults, want)
	}
}

// memberGate is a two-step ladder whose first step is gated on one metric
// with a failure limit of one; bake is that step's soak, zero for none.
func memberGate(bake time.Duration) []v1beta1.RolloutGroupStep {
	step := v1beta1.RolloutGroupStep{
		Capacity: intstr.FromString("50%"), Traffic: 50,
		Analysis: &v1beta1.RolloutAnalysis{
			Interval:     metav1.Duration{Duration: time.Minute},
			FailureLimit: 1,
			Metrics:      []v1beta1.AnalysisMetric{{Name: "err", Query: "q", Operator: v1beta1.ComparisonLTE, Threshold: "0.05"}},
		},
	}
	if bake > 0 {
		step.Pause = &v1beta1.RolloutPause{Duration: &metav1.Duration{Duration: bake}}
	}
	return []v1beta1.RolloutGroupStep{step, {Capacity: intstr.FromString("100%"), Traffic: 100}}
}

// memberUnit is an [engine, decoder] unit, engine primary, four instances
// per member, under a pinned run whose targets say which members the push
// moved: the decoder always moves from decold to decnew, the engine from
// engold to engnew only when engineBumped. A moved member has two ready
// pods on its stable revision and two on its target; a member left alone
// has four on its stable revision.
func memberUnit(t *testing.T, steps []v1beta1.RolloutGroupStep, engineBumped bool, sampler stepSampler) (*v1beta1.InferenceService, ReconcileInputs) {
	t.Helper()
	isvc := canaryISVC(steps, nil)
	isvc.Spec.Rollout.Groups[0].Components = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}
	pinActiveRun(isvc)
	engineTarget := "engold"
	enginePods := map[string]int32{"engold": 4}
	if engineBumped {
		engineTarget = "engnew"
		enginePods = map[string]int32{"engold": 2, "engnew": 2}
	}
	isvc.Status.Rollout.ActiveRun.TargetRevisions = []v1beta1.RolloutRunTarget{
		{Component: v1beta1.EngineComponent, Revision: engineTarget, StableRevision: "engold"},
		{Component: v1beta1.DecoderComponent, Revision: "decnew", StableRevision: "decold"},
	}
	decoderPods := map[string]int32{"decold": 2, "decnew": 2}
	in := baseInputs(isvc, enginePods)
	// The arming guard reads the run's targets for the group's members, so
	// the unit is handed its group as the dispatcher hands it.
	in.Group = rollout.CanaryGroup(isvc, rollout.Policies{})
	in.CanaryRevisionHash, in.StableRevisionHash = engineTarget, "engold"
	in.TargetID = "t1"
	in.Sampler = sampler
	in.BundledPrometheusAddress = "http://prometheus.example.com:9090"
	in.Secondaries = map[v1beta1.ComponentType]MemberRevisions{v1beta1.DecoderComponent: {
		CanaryRevisionHash: "decnew", StableRevisionHash: "decold", ReadyCanaryCapacity: 2, DesiredReplicas: 4,
	}}
	in.GroupReadyPerRevisionPods = map[v1beta1.ComponentType]map[string]int32{v1beta1.EngineComponent: enginePods, v1beta1.DecoderComponent: decoderPods}
	in.GroupTotalPerRevisionPods = map[v1beta1.ComponentType]map[string]int32{v1beta1.EngineComponent: enginePods, v1beta1.DecoderComponent: decoderPods}
	in.GroupStableRevisionHashes = map[v1beta1.ComponentType]string{v1beta1.EngineComponent: "engold", v1beta1.DecoderComponent: "decold"}
	return isvc, in
}

// A push that moved only the decoder is sampled on the decoder's own canary
// revision, never on the engine that stayed on its stable revision, and the
// decoder's breach rolls the whole unit back to stable.
func TestReconcile_AnalysisSamplesTheMovedSecondaryAlone(t *testing.T) {
	script := &memberScript{outcomes: map[string]analysis.Outcome{"decoder": analysis.Fail}, at: time.Unix(1000, 0)}
	isvc, in := memberUnit(t, memberGate(0), false, script)
	res := mustReconcile(t, isvc, in)
	if !res.RolledBack || phaseOf(isvc) != v1beta1.RolloutPhaseRollingBack {
		t.Fatalf("the breaching decoder must roll the unit back, got %+v phase %q", res, phaseOf(isvc))
	}
	wantReads(t, script.reads(), "decoder@decnew")
	req := script.requests[0]
	prefix := irprojector.RoleReplicaPrefix(isvc, v1beta1.DecoderComponent)
	if req.TemplateContext.Component != "decoder" ||
		req.TemplateContext.CanaryService != coordination.PerRevisionServiceName(prefix, v1beta1.DecoderComponent, "decnew") ||
		req.TemplateContext.StableService != coordination.PerRevisionServiceName(prefix, v1beta1.DecoderComponent, "decold") ||
		req.TemplateContext.CanaryRevision != "decnew" || req.TemplateContext.StableRevision != "decold" {
		t.Fatalf("the query must name the decoder's own revisions and Services, got %+v", req.TemplateContext)
	}
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	if cs.RolledBackRevisionHash == "" || cs.AnalysisFailedChecks != 1 {
		t.Fatalf("the unit must record the rejection and the failing check, got %+v", cs)
	}
	wantMetricNames(t, cs, "decoder/err")
	if cs.MetricResults[0].Passed {
		t.Fatalf("the decoder's breach must be recorded as the failing result, got %+v", cs.MetricResults)
	}
	expectTargets(t, isvc, v1beta1.DecoderComponent, map[string]trafficWant{"decold": {percent: 100}})
}

// On a push that moved both members a breach on the primary is the unit's
// breach even while the secondary passes, and every member's result is on
// record under its own name.
func TestReconcile_AnalysisBreachOnThePrimaryRollsThePushBack(t *testing.T) {
	script := &memberScript{outcomes: map[string]analysis.Outcome{"engine": analysis.Fail, "decoder": analysis.Pass}, at: time.Unix(1000, 0)}
	isvc, in := memberUnit(t, memberGate(0), true, script)
	res := mustReconcile(t, isvc, in)
	if !res.RolledBack || phaseOf(isvc) != v1beta1.RolloutPhaseRollingBack {
		t.Fatalf("the breaching engine must roll the unit back, got %+v phase %q", res, phaseOf(isvc))
	}
	wantReads(t, script.reads(), "engine@engnew", "decoder@decnew")
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	if cs.RolledBackRevisionHash != "engnew" {
		t.Fatalf("the rejected revision is the primary's canary revision, got %q", cs.RolledBackRevisionHash)
	}
	wantMetricNames(t, cs, "err", "decoder/err")
	if cs.MetricResults[0].Passed || !cs.MetricResults[1].Passed {
		t.Fatalf("the engine's breach and the decoder's pass must both be on record, got %+v", cs.MetricResults)
	}
	expectTargets(t, isvc, v1beta1.EngineComponent, map[string]trafficWant{"engold": {percent: 100}})
	expectTargets(t, isvc, v1beta1.DecoderComponent, map[string]trafficWant{"decold": {percent: 100}})
}

// A push that moved both members promotes only on a pass from both: inside
// the bake window the step holds with both results on record, and once the
// window has elapsed the next passing sample of both advances the step.
func TestReconcile_AnalysisPromotesOnEveryMemberPassing(t *testing.T) {
	now := time.Unix(1000, 0)
	script := &memberScript{outcomes: map[string]analysis.Outcome{"engine": analysis.Pass, "decoder": analysis.Pass}, at: now}
	isvc, in := memberUnit(t, memberGate(10*time.Minute), true, script)
	in.Now = now
	if res := mustReconcile(t, isvc, in); res.Stepped || phaseOf(isvc) != v1beta1.RolloutPhasePaused {
		t.Fatalf("a pass inside the bake window holds the step, got %+v phase %q", res, phaseOf(isvc))
	}
	wantReads(t, script.reads(), "engine@engnew", "decoder@decnew")
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	wantMetricNames(t, cs, "err", "decoder/err")
	if !cs.MetricResults[0].Passed || !cs.MetricResults[1].Passed || cs.LastEvaluationTime == nil || !cs.LastEvaluationTime.Time.Equal(now) {
		t.Fatalf("both members' passing results must be on record, dated by the sample, got %+v", cs)
	}

	in.Now = now.Add(11 * time.Minute)
	script.at = in.Now
	if res := mustReconcile(t, isvc, in); !res.Stepped || cs.CurrentStep != 1 {
		t.Fatalf("a pass from both members once the window elapsed advances the step, got %+v step %d", res, cs.CurrentStep)
	}
	wantReads(t, script.reads(), "engine@engnew", "decoder@decnew", "engine@engnew", "decoder@decnew")
}

// A member whose sample has not landed holds the step whatever the other
// members read, with nothing consumed; every member's query is still
// kicked on the same pass.
func TestReconcile_AnalysisHoldsUntilEveryMemberIsRead(t *testing.T) {
	script := &memberScript{outcomes: map[string]analysis.Outcome{"engine": analysis.Pass}, at: time.Unix(1000, 0)}
	isvc, in := memberUnit(t, memberGate(0), true, script)
	if res := mustReconcile(t, isvc, in); res.Stepped || res.RolledBack || phaseOf(isvc) != v1beta1.RolloutPhasePaused {
		t.Fatalf("an unread member holds the step, got %+v phase %q", res, phaseOf(isvc))
	}
	wantReads(t, script.reads(), "engine@engnew", "decoder@decnew")
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	if cs.LastEvaluationTime != nil || cs.MetricResults != nil || cs.AnalysisFailedChecks != 0 || cs.CurrentStep != 0 {
		t.Fatalf("nothing is consumed until every member is read, got %+v", cs)
	}
}

// With no member on record as moved there is no canary pod a query could
// read: the gate reads nothing and holds rather than promote on no sample.
func TestEvaluateAnalysisStep_NoMovedMemberHoldsWithoutSampling(t *testing.T) {
	now := time.Unix(1000, 0)
	steps := memberGate(0)
	isvc := canaryISVC(steps, nil)
	isvc.Spec.Rollout.Groups[0].Components = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}
	pinActiveRun(isvc)
	isvc.Status.Rollout.ActiveRun.TargetRevisions = []v1beta1.RolloutRunTarget{
		{Component: v1beta1.EngineComponent, Revision: "engold", StableRevision: "engold"},
		{Component: v1beta1.DecoderComponent, Revision: "decold", StableRevision: "decold"},
	}
	cs := &v1beta1.CanaryStatus{TargetID: "t1", CanaryRevisionHash: "engold", StableRevisionHash: "engold", StepEnteredTime: &metav1.Time{Time: now}}
	isvc.Status.Canary = cs
	script := &memberScript{outcomes: map[string]analysis.Outcome{"engine": analysis.Pass, "decoder": analysis.Pass}, at: now}
	in := ReconcileInputs{
		ISVC: isvc, Component: v1beta1.EngineComponent, CanaryRevisionHash: "engold", StableRevisionHash: "engold", Now: now,
		Sampler: script, BundledPrometheusAddress: "http://prometheus.example.com:9090",
		Secondaries: map[v1beta1.ComponentType]MemberRevisions{v1beta1.DecoderComponent: {CanaryRevisionHash: "decold", StableRevisionHash: "decold"}},
	}
	if got := evaluateAnalysisStep(context.Background(), in, steps[0].Analysis, cs, steps[0]); got != decHold {
		t.Fatalf("no moved member = %v, want hold", got)
	}
	if len(script.requests) != 0 || cs.LastEvaluationTime != nil || cs.MetricResults != nil {
		t.Fatalf("nothing is read and nothing is stamped, got reads %v status %+v", script.reads(), cs)
	}
}

// The controller-side dispatch over a prefill/decode unit whose push moved
// only the decoder, under a ladder gated on a metric the decoder breaches
// from its first sample: the gate reads the decoder on its own canary
// revision, never the engine, rolls the unit back and points the decoder's
// replica at its stable revision.
func TestDispatch_DecoderOnlyPushIsSampledOnTheDecoder(t *testing.T) {
	ns, name := "default", "pdanalysis"
	n2 := 2
	isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
	isvc.Spec.Engine = &v1beta1.EngineSpec{ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: &n2}}
	isvc.Spec.Decoder = &v1beta1.DecoderSpec{ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: &n2}}
	isvc.Spec.Rollout = &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
		Canary:     &v1beta1.GroupCanary{Steps: memberGate(0)},
	}}}
	pinActiveRun(isvc)
	isvc.Status.Rollout.ActiveRun.TargetRevisions = []v1beta1.RolloutRunTarget{
		{Component: v1beta1.EngineComponent, Revision: "engold", StableRevision: "engold"},
		{Component: v1beta1.DecoderComponent, Revision: "decnew", StableRevision: "decold"},
	}
	engineIR := ir(ns, name, v1beta1.EngineComponent, "engold")
	engineIR.Status.CurrentRevision = name + "-engine-engold"
	decoderIR := ir(ns, name, v1beta1.DecoderComponent, "decnew")
	decoderIR.Status.CurrentRevision = name + "-decoder-decold"
	stableDecoder := pairedRevision(ns, name, "decoder", "decold", 1, pdStableProtocol)
	objs := []runtime.Object{isvc, engineIR, decoderIR,
		canaryPod(ns, name, "engine", "engold", name+"-engine-0"),
		canaryPod(ns, name, "engine", "engold", name+"-engine-1"),
		canaryPod(ns, name, "decoder", "decold", name+"-decoder-0"),
		canaryPod(ns, name, "decoder", "decnew", name+"-decoder-1"),
		pairedRevision(ns, name, "engine", "engold", 1, pdStableProtocol),
		stableDecoder,
		pairedRevision(ns, name, "decoder", "decnew", 2, pdCanaryProtocol),
	}
	c := fake.NewClientBuilder().WithScheme(canaryScheme(t)).WithRuntimeObjects(objs...).Build()

	// The metrics source breaches for the decoder's canary revision and
	// passes for anything else, and keeps what it was asked.
	var mu sync.Mutex
	var asked []string
	events := make(chan event.GenericEvent, 4)
	sampler := NewSampler(func(_ context.Context, req SampleRequest) analysis.Result {
		mu.Lock()
		asked = append(asked, req.Key.Component+"@"+req.Key.Revision)
		mu.Unlock()
		if req.Key.Component == string(v1beta1.DecoderComponent) {
			return analysis.Result{Outcome: analysis.Fail, Metrics: []analysis.MetricResult{{Name: "err", Value: "0.5"}}}
		}
		return analysis.Result{Outcome: analysis.Pass, Metrics: []analysis.MetricResult{{Name: "err", Value: "0", Passed: true}}}
	}, events, 4, time.Minute)
	deps := DispatchDeps{
		Client: c, Reader: c, ISVC: isvc, ComponentRunnerPorts: canaryRunnerPorts(),
		Group: rollout.CanaryGroup(isvc, rollout.Policies{}), Sampler: sampler,
	}
	ctx := context.Background()

	// The first pass serves the split and kicks the decoder's query.
	syncInstanceRows(t, c, isvc, v1beta1.EngineComponent, v1beta1.DecoderComponent)
	if _, err := Dispatch(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if got := isvc.Status.Components[v1beta1.EngineComponent].RolloutPhase; got != v1beta1.RolloutPhasePaused {
		t.Fatalf("the gate holds while the decoder's sample is in flight, got %q", got)
	}
	<-events // the decoder's sample lands
	// The next pass consumes it: the breaching decoder rolls the unit back.
	if _, err := Dispatch(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if got := isvc.Status.Components[v1beta1.EngineComponent].RolloutPhase; got != v1beta1.RolloutPhaseRollingBack {
		t.Fatalf("the breaching decoder rolls the unit back, got %q", got)
	}
	mu.Lock()
	wantReads(t, asked, "decoder@decnew")
	mu.Unlock()
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	wantMetricNames(t, cs, "decoder/err")
	if cs.MetricResults[0].Passed {
		t.Fatalf("the decoder's breach is the recorded result, got %+v", cs.MetricResults)
	}
	got := &v1beta1.InferenceReplica{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-decoder"}, got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Pacing == nil || got.Spec.Pacing.RollbackToRevision == nil || *got.Spec.Pacing.RollbackToRevision != stableDecoder.Name {
		t.Fatalf("the decoder's replica must be pointed at its stable revision %q, got %+v", stableDecoder.Name, got.Spec.Pacing)
	}
}
