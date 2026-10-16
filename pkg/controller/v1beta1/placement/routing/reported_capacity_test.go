package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	kptr "k8s.io/utils/ptr"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func reportedCapacityPolicy(t *testing.T) ResolvedCapacityPolicy {
	t.Helper()
	cfg := singleSampleConfig()
	cfg.Mode = v1beta1.RoutingCapacityModeReported
	return testCapacityPolicy(t, cfg)
}

func TestReportedCapacityReplacesReplicaCalculation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []float64
		want   []int32
	}{
		{"equal bottleneck", []float64{30, 30}, []int32{1, 1}},
		{"fractional capacities", []float64{0.25, 0.75}, []int32{1, 3}},
		{"zero capacity", []float64{0, 30}, []int32{0, 1}},
		{"all zero", []float64{0, 0}, []int32{0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isvc := splitISVC([]string{"a", "b"}, []int32{60, 80}, []int32{0, 80})
			isvc.Spec.Routing = &v1beta1.RoutingSpec{CapacityFactors: map[string]resource.Quantity{
				"a": resource.MustParse("100"), "b": resource.MustParse("0.01"),
			}}
			poller := newTestPoller(t)
			policy := reportedCapacityPolicy(t)
			for i := range tc.values {
				target := targetForCandidate(isvc, &isvc.Status.Placement.Candidates[i], "")
				require.True(t, poller.record(target, policy, &tc.values[i], ""))
			}
			spec, status, reason := buildSpec(isvc, ResolvedProbePolicy{}, policy, nil, poller)
			require.Len(t, spec.Entries, 2)
			for i, entry := range spec.Entries {
				assert.Equal(t, tc.want[i], entry.Weight)
				assert.Equal(t, v1beta1.RoutingCapacityModeReported, spec.CapacityMode)
				require.NotNil(t, entry.Capacity.ReportedWeight)
				assert.InDelta(t, tc.values[i], entry.Capacity.ReportedWeight.AsApproximateFloat64(), 1e-9)
				assert.Nil(t, entry.Capacity.Reported)
				assert.Nil(t, entry.Capacity.Factor, "the report already weights the hardware")
				assert.Equal(t, v1beta1.CapacitySourceEndpoint, entry.Capacity.Source)
			}
			if tc.want[0] == 0 && tc.want[1] == 0 {
				assert.Equal(t, metav1.ConditionFalse, status)
				assert.Equal(t, v1beta1.TrafficMapReasonNoRoutableCapacity, reason)
			} else {
				assert.Equal(t, metav1.ConditionTrue, status)
			}
		})
	}
}

func TestReportedCapacityFailureKeepsOnlyFreshEvidence(t *testing.T) {
	poller := newTestPoller(t)
	policy := reportedCapacityPolicy(t)
	target := testCapacityTarget("a", "https://a.example")
	clk := poller.Clock.(*clocktesting.FakeClock)

	value, reason := poller.observation(target, policy)
	assert.Nil(t, value)
	assert.NotEmpty(t, reason)
	require.True(t, poller.record(target, policy, kptr.To(30.5), ""))
	clk.Step(policy.Capacity.MaxAge - time.Second)
	require.True(t, poller.record(target, policy, nil, "request timed out"))
	value, reason = poller.observation(target, policy)
	require.NotNil(t, value)
	assert.Equal(t, 30.5, *value)
	assert.Equal(t, "request timed out", reason)

	clk.Step(time.Second)
	value, reason = poller.observation(target, policy)
	assert.Nil(t, value, "failed polls cannot renew the last report")
	assert.Contains(t, reason, "stale")
	require.True(t, poller.record(target, policy, kptr.To(0.0), ""))
	value, reason = poller.observation(target, policy)
	require.NotNil(t, value)
	assert.Zero(t, *value, "zero is an accepted observation")
	assert.Empty(t, reason)
}

func TestReportedCapacityHTTPContract(t *testing.T) {
	policy := reportedCapacityPolicy(t)
	for _, tc := range []struct {
		name string
		body map[string]any
		want *float64
	}{
		{"fraction", map[string]any{"weight": 0.375, "observedAt": capacityTestNow}, kptr.To(0.375)},
		{"zero", map[string]any{"weight": 0, "observedAt": capacityTestNow}, kptr.To(0.0)},
		{"replicas are not weights", map[string]any{"servable": 30, "observedAt": capacityTestNow}, nil},
		{"negative", map[string]any{"weight": -1, "observedAt": capacityTestNow}, nil},
		{"missing timestamp", map[string]any{"weight": 30}, nil},
		{"stale", map[string]any{"weight": 30, "observedAt": capacityTestNow.Add(-policy.Capacity.MaxAge)}, nil},
		{"future", map[string]any{"weight": 30, "observedAt": capacityTestNow.Add(time.Second)}, nil},
		{"too large", map[string]any{"weight": 3e9, "observedAt": capacityTestNow}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(tc.body))
			}))
			defer srv.Close()
			poller := newTestPoller(t)
			value, reason := poller.pollOne(context.Background(), testCapacityTarget("a", srv.URL), policy.Capacity)
			assert.Equal(t, tc.want, value)
			assert.Equal(t, tc.want == nil, reason != "")
		})
	}
}

func TestReportedCapacityQuorumSurvivesOnlyFreshSamples(t *testing.T) {
	poller := newTestPoller(t)
	config := singleSampleConfig()
	config.Mode = v1beta1.RoutingCapacityModeReported
	config.Samples, config.Quorum = 3, 2
	policy := testCapacityPolicy(t, config)
	target := testCapacityTarget("a", "https://a.example")
	clk := poller.Clock.(*clocktesting.FakeClock)
	start := clk.Now()
	poller.record(target, policy, kptr.To(0.75), "")
	assert.Nil(t, poller.Reported(target, policy), "one sample cannot supply weight")
	clk.Step(time.Second)
	poller.record(target, policy, kptr.To(0.75), "")
	clk.Step(time.Second)
	poller.record(target, policy, kptr.To(0.25), "")
	require.Equal(t, kptr.To(0.75), poller.Reported(target, policy), "one low reading cannot lower the weight")
	poller.record(target, policy, nil, "request timed out")
	clk.SetTime(start.Add(config.MaxAge))
	value, reason := poller.observation(target, policy)
	assert.Equal(t, kptr.To(0.25), value, "two fresh samples still form a quorum, and the smaller one applies")
	assert.Equal(t, "request timed out", reason, "expiry is not a successful poll")
	clk.Step(time.Second)
	assert.Nil(t, poller.Reported(target, policy), "one remaining sample cannot maintain weight")

	poller.record(target, policy, kptr.To(0.0), "")
	assert.Equal(t, kptr.To(0.0), poller.Reported(target, policy), "an accepted zero withdraws capacity")
}

func TestReportedCapacityEstimateIsLowerMedian(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   v1beta1.RoutingCapacityMode
		window []float64
		want   float64
		ok     bool
	}{
		{"below quorum", v1beta1.RoutingCapacityModeReported, []float64{4}, 0, false},
		{"one low reading cannot lower it", v1beta1.RoutingCapacityModeReported, []float64{4, 4, 1}, 4, true},
		{"one high reading cannot raise it", v1beta1.RoutingCapacityModeReported, []float64{2, 2, 9}, 2, true},
		{"an even window takes the smaller reading", v1beta1.RoutingCapacityModeReported, []float64{4, 2}, 2, true},
		{"a ceiling takes the quorum-th smallest", v1beta1.RoutingCapacityModeCeiling, []float64{4, 2}, 4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := singleSampleConfig()
			config.Mode = tc.mode
			config.Samples, config.Quorum = 3, 2
			st := &capacityState{}
			var got float64
			var ok bool
			for _, v := range tc.window {
				got, ok = st.observe(v, capacityTestNow.Add(time.Hour), config)
			}
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestReportedCapacityFallsBackPerHome(t *testing.T) {
	// a reports more than its replicas; b and c have no report, and c has no
	// ready replicas either.
	isvc := splitISVC([]string{"a", "b", "c"}, []int32{60, 80, 40}, []int32{60, 50, 0})
	isvc.Spec.Routing = &v1beta1.RoutingSpec{CapacityFactors: map[string]resource.Quantity{
		"a": resource.MustParse("3"), "b": resource.MustParse("2"),
	}}
	poller := newTestPoller(t)
	policy := reportedCapacityPolicy(t)
	target := targetForCandidate(isvc, &isvc.Status.Placement.Candidates[0], "")
	require.True(t, poller.record(target, policy, kptr.To(90.0), ""))

	spec, status, reason := buildSpec(isvc, ResolvedProbePolicy{}, policy, nil, poller)
	require.Len(t, spec.Entries, 3)
	// a weighs its report, neither capped by its replicas nor scaled by its
	// factor; b weighs min(80, 50) * 2 = 100; c weighs zero.
	assert.Equal(t, int32(9), spec.Entries[0].Weight)
	assert.Equal(t, int32(10), spec.Entries[1].Weight)
	assert.Zero(t, spec.Entries[2].Weight)
	assert.Equal(t, metav1.ConditionTrue, status)
	assert.Equal(t, v1beta1.TrafficMapReasonRoutable, reason)

	reported := spec.Entries[0].Capacity
	assert.Equal(t, v1beta1.CapacitySourceEndpoint, reported.Source)
	require.NotNil(t, reported.ReportedWeight)
	assert.Nil(t, reported.Factor)
	assert.Empty(t, reported.FallbackReason)
	assert.True(t, spec.Entries[0].Healthy)
	for i, healthy := range []bool{true, false} {
		fallback := spec.Entries[i+1].Capacity
		assert.Equal(t, v1beta1.CapacitySourceControlPlane, fallback.Source)
		assert.Nil(t, fallback.ReportedWeight)
		assert.NotEmpty(t, fallback.FallbackReason)
		assert.Equal(t, healthy, spec.Entries[i+1].Healthy)
	}
	require.NotNil(t, spec.Entries[1].Capacity.Factor)
	assert.Equal(t, "2", spec.Entries[1].Capacity.Factor.String())
	assert.Equal(t, 2, capacityFallbackCount(spec))
}

func TestReportedCapacityWithoutReportsOrReadyReplicasIsUnready(t *testing.T) {
	isvc := splitISVC([]string{"a", "b"}, []int32{60, 80}, []int32{0, 0})
	spec, status, reason := buildSpec(isvc, ResolvedProbePolicy{}, reportedCapacityPolicy(t), nil, newTestPoller(t))
	require.Len(t, spec.Entries, 2)
	assert.Equal(t, metav1.ConditionFalse, status)
	assert.Equal(t, v1beta1.TrafficMapReasonAllHomesUnready, reason)
}

func TestReportedCapacityStillHonorsGates(t *testing.T) {
	isvc := splitISVC([]string{"a", "b"}, []int32{60, 60}, []int32{60, 60})
	poller := newTestPoller(t)
	policy := reportedCapacityPolicy(t)
	for i := range isvc.Status.Placement.Candidates {
		target := targetForCandidate(isvc, &isvc.Status.Placement.Candidates[i], "")
		require.True(t, poller.record(target, policy, kptr.To(30.0), ""))
	}
	prober, probePolicy := failingProbeState(t, isvc, AllFailedPolicyDrain, "a")
	spec, _, _ := buildSpec(isvc, probePolicy, policy, prober, poller)
	assert.Zero(t, spec.Entries[0].Weight)
	assert.Positive(t, spec.Entries[1].Weight)

	isvc.Status.Placement.Plan = &v1beta1.PlacementPlanStatus{Mode: v1beta1.PlacementModeSplit}
	isvc.Status.Placement.Candidates[1].Allocation = &v1beta1.CandidateAllocationStatus{CurrentReplicas: 60, DrainRequested: true}
	spec, _, _ = buildSpec(isvc, probePolicy, policy, prober, poller)
	assert.Zero(t, spec.Entries[0].Weight)
	assert.Zero(t, spec.Entries[1].Weight)
}

func TestReportedCapacityUnavailableSourceCannotRequestPublicationReplay(t *testing.T) {
	for _, missingPlacement := range []bool{false, true} {
		isvc := splitISVC([]string{"a"}, []int32{60}, []int32{60})
		wantReason := v1beta1.TrafficMapReasonNoAddressableHome
		if missingPlacement {
			isvc.Status.Placement = nil
			wantReason = v1beta1.TrafficMapReasonNotPlaced
		} else {
			isvc.Status.Placement.Candidates[0].Endpoint = nil
		}
		spec, status, reason := buildSpec(isvc, ResolvedProbePolicy{}, reportedCapacityPolicy(t), nil, nil)
		assert.Empty(t, spec.Entries)
		assert.Equal(t, v1beta1.RoutingCapacityModeReported, spec.CapacityMode)
		assert.Equal(t, metav1.ConditionFalse, status)
		assert.Equal(t, wantReason, reason)
	}
}

func TestReportedCapacityAdmissionUsesPhaseRatherThanReplicaCount(t *testing.T) {
	isvc := splitISVC([]string{"a"}, []int32{0}, []int32{0})
	poller := newTestPoller(t)
	policy := reportedCapacityPolicy(t)
	candidate := &isvc.Status.Placement.Candidates[0]
	target := targetForCandidate(isvc, candidate, "")
	require.True(t, poller.record(target, policy, kptr.To(30.0), ""))
	spec, _, _ := buildSpec(isvc, ResolvedProbePolicy{}, policy, nil, poller)
	require.Len(t, spec.Entries, 1)
	assert.Positive(t, spec.Entries[0].Weight, "replica counts do not override the router's capacity")

	candidate.Phase = v1beta1.CandidatePhaseAdmitting
	spec, _, _ = buildSpec(isvc, ResolvedProbePolicy{}, policy, nil, poller)
	assert.Empty(t, spec.Entries, "only admitted candidates can receive traffic")

	candidate.Phase = v1beta1.CandidatePhaseAdmitted
	isvc.Status.Placement.Plan = &v1beta1.PlacementPlanStatus{Mode: v1beta1.PlacementModeSplit}
	candidate.Allocation = &v1beta1.CandidateAllocationStatus{}
	spec, _, _ = buildSpec(isvc, ResolvedProbePolicy{}, policy, nil, poller)
	assert.Empty(t, spec.Entries, "retired allocations cannot reuse an old report")
}

func TestReportedCapacityPreserveTrafficCannotInventCapacity(t *testing.T) {
	homes := []Home{
		{Mode: v1beta1.RoutingCapacityModeReported, ReportedWeight: kptr.To(resource.MustParse("0.75")), Reachable: kptr.To(false)},
		{Mode: v1beta1.RoutingCapacityModeReported, Allocated: 60, Ready: 0, Reachable: kptr.To(false)},
	}
	assert.Equal(t, []int32{1, 0}, weights(homes, AllFailedPolicyPreserveTraffic))
	assert.Equal(t, []int32{0, 0}, weights(homes, AllFailedPolicyDrain))
}

func TestReportedCapacityModeChangesObservationIdentity(t *testing.T) {
	poller := newTestPoller(t)
	policy := reportedCapacityPolicy(t)
	target := testCapacityTarget("a", "https://a.example")
	require.True(t, poller.record(target, policy, kptr.To(30.5), ""))
	ceiling := policy.Capacity
	ceiling.Mode = v1beta1.RoutingCapacityModeCeiling
	value, _ := poller.observation(target, testCapacityPolicy(t, ceiling))
	assert.Nil(t, value)
	target.OwnerUID = "replacement"
	value, _ = poller.observation(target, policy)
	assert.Nil(t, value)
}

func TestReportedCapacityConfigResolution(t *testing.T) {
	cfg := controllerTestConfig()
	cfg.Capacity = reportedCapacityPolicy(t).Capacity
	resolved, err := ResolveCapacityPolicy(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, v1beta1.RoutingCapacityModeReported, resolved.Capacity.Mode)
	inline := &v1beta1.InferenceServiceSpec{Routing: &v1beta1.RoutingSpec{
		Capacity: controllerCapacitySpec(singleSampleConfig()),
	}}
	resolved, err = ResolveCapacityPolicy(cfg, inline)
	require.NoError(t, err)
	assert.Empty(t, resolved.Capacity.Mode, "a complete inline block does not inherit the global mode")
	inline.Routing.Capacity = &v1beta1.RoutingCapacitySpec{Disabled: true}
	resolved, err = ResolveCapacityPolicy(cfg, inline)
	require.NoError(t, err)
	assert.False(t, resolved.IsEnabled())
	cfg.Capacity.Path = ""
	_, err = ResolveCapacityPolicy(cfg, nil)
	assert.ErrorContains(t, err, "requires capacity polling")
}

func TestReconcileReportedWeightsFromHTTP(t *testing.T) {
	var unavailable [2]atomic.Bool
	var zeroFirst atomic.Bool
	clk := clocktesting.NewFakeClock(capacityTestNow)
	isvc := splitISVC([]string{"a", "b"}, []int32{60, 80}, []int32{0, 80})
	for i := range isvc.Status.Placement.Candidates {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if unavailable[i].Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			weight := 30.5
			if i == 1 {
				weight = 15.25
			} else if zeroFirst.Load() {
				weight = 0
			}
			_ = json.NewEncoder(w).Encode(CapacityReport{Weight: &weight, ObservedAt: clk.Now()})
		}))
		t.Cleanup(server.Close)
		endpoint, err := apis.ParseURL(server.URL)
		require.NoError(t, err)
		isvc.Status.Placement.Candidates[i].Endpoint = endpoint
	}
	config := controllerTestCapacityConfig()
	config.Mode = v1beta1.RoutingCapacityModeReported
	isvc.Spec.Routing = &v1beta1.RoutingSpec{Capacity: controllerCapacitySpec(config)}
	r, client := newReconciler(t, controllerTestConfig(), isvc)
	r.Capacity = newControllerCapacityPoller(t, NewObserverClient(), clk)
	setUnavailable := func(v bool) {
		for i := range unavailable {
			unavailable[i].Store(v)
		}
	}
	routableReason := func(tm *v1beta1.TrafficMap) string {
		for _, condition := range tm.Status.Conditions {
			if condition.Type == v1beta1.TrafficMapRoutable {
				return condition.Reason
			}
		}
		return ""
	}

	setUnavailable(true)
	reconcile(t, r)
	tm, exists := getTrafficMap(t, client)
	require.True(t, exists)
	require.Len(t, tm.Spec.Entries, 2)
	assert.Zero(t, tm.Spec.Entries[0].Weight, "no report and no ready replicas")
	assert.Positive(t, tm.Spec.Entries[1].Weight, "a home without a report keeps its replica count")
	assert.Nil(t, tm.Spec.Entries[1].Capacity.ReportedWeight)
	assert.Equal(t, v1beta1.CapacitySourceControlPlane, tm.Spec.Entries[1].Capacity.Source)

	setUnavailable(false)
	clk.Step(config.Period)
	reconcile(t, r)
	tm, exists = getTrafficMap(t, client)
	require.True(t, exists)
	require.Len(t, tm.Spec.Entries, 2)
	assert.Equal(t, int32(2), tm.Spec.Entries[0].Weight)
	assert.Equal(t, int32(1), tm.Spec.Entries[1].Weight)
	assert.True(t, tm.Spec.Entries[0].Healthy, "a router report supplies serving health despite lagging replica readiness")

	zeroFirst.Store(true)
	clk.Step(config.Period)
	reconcile(t, r)
	tm, _ = getTrafficMap(t, client)
	assert.Zero(t, tm.Spec.Entries[0].Weight)
	assert.Positive(t, tm.Spec.Entries[1].Weight)

	setUnavailable(true)
	clk.Step(config.Period)
	reconcile(t, r)
	tm, _ = getTrafficMap(t, client)
	assert.Zero(t, tm.Spec.Entries[0].Weight)
	assert.Positive(t, tm.Spec.Entries[1].Weight)
	assert.NotNil(t, tm.Spec.Entries[1].Capacity.ReportedWeight, "a fresh report survives a failed poll")
	assert.NotEmpty(t, tm.Spec.Entries[1].Capacity.FallbackReason)

	clk.Step(config.MaxAge)
	reconcile(t, r)
	tm, _ = getTrafficMap(t, client)
	for _, entry := range tm.Spec.Entries {
		assert.Nil(t, entry.Capacity.ReportedWeight)
	}
	assert.Zero(t, tm.Spec.Entries[0].Weight)
	assert.Positive(t, tm.Spec.Entries[1].Weight, "expired reports fall back to the replica count")
	assert.Equal(t, v1beta1.TrafficMapReasonRoutable, routableReason(tm))

	// a reports again with no ready replicas; b still cannot report.
	zeroFirst.Store(false)
	unavailable[0].Store(false)
	clk.Step(config.Period)
	reconcile(t, r)
	tm, _ = getTrafficMap(t, client)
	// 30.5 reported against 80 replicas.
	assert.Equal(t, int32(61), tm.Spec.Entries[0].Weight)
	assert.Equal(t, int32(160), tm.Spec.Entries[1].Weight)
	assert.Equal(t, v1beta1.CapacitySourceEndpoint, tm.Spec.Entries[0].Capacity.Source)
	assert.Equal(t, v1beta1.CapacitySourceControlPlane, tm.Spec.Entries[1].Capacity.Source)
	assert.Equal(t, v1beta1.TrafficMapReasonRoutable, routableReason(tm))
}
