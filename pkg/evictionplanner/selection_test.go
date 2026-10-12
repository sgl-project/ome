package evictionplanner

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/artifactinventory"
)

const gib int64 = 1 << 30

var evaluationTime = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func age(days float64) *time.Time {
	value := evaluationTime.Add(-time.Duration(days * float64(24*time.Hour)))
	return &value
}

func artifact(name string, sizeGiB int64, nodes int, uids ...string) artifactinventory.Artifact {
	size := sizeGiB * gib
	result := artifactinventory.Artifact{Path: "/models/" + name, EstimatedArtifactBytes: &size, EvictionEligible: true}
	for _, uid := range uids {
		model := artifactinventory.ModelRef{Namespace: "models", Name: uid, UID: types.UID(uid)}
		for i := 1; i < nodes; i++ {
			model.NodesReady = append(model.NodesReady, fmt.Sprintf("other-%d", i))
		}
		result.Models = append(result.Models, model)
	}
	return result
}

func fixture(requiredGiB int64, artifacts ...artifactinventory.Artifact) Input {
	// Usage is 90%; L leaves precisely the requested gap, and H is 85%.
	capacity := requiredGiB * 10 * gib
	input := Input{
		Now: evaluationTime,
		Nodes: []Node{{Name: "a", CapacityBytes: capacity, AvailableBytes: capacity / 10, HighWatermark: .85, LowWatermark: .8,
			Inventory: &artifactinventory.Inventory{NodeName: "a", Artifacts: artifacts}}},
		EligibilityByUID: map[types.UID]Eligibility{},
	}
	for _, a := range artifacts {
		for _, model := range a.Models {
			input.EligibilityByUID[model.UID] = Eligibility{Eligible: true, IdleSince: age(30)}
		}
	}
	return input
}

func build(t *testing.T, input Input) *Plan {
	t.Helper()
	plan, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func paths(groups []Candidate) []string {
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.ID.Path)
	}
	return result
}

func alternativePaths(alternative Alternative) []string {
	result := make([]string, 0, len(alternative.Groups))
	for _, id := range alternative.Groups {
		result = append(result, id.Path)
	}
	return result
}

func TestAgeWeightsAndBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		idle   *time.Time
		weight float64
	}{
		{"unknown", nil, 8},
		{"zero", &time.Time{}, 8},
		{"future", age(-1), 8},
		{"recent", age(.5), 8},
		{"one day", age(1), 4},
		{"before week", age(6.99), 4},
		{"one week", age(7), 2},
		{"before fortnight", age(13.99), 2},
		{"fortnight", age(14), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := fixture(100, artifact("x", 100, 2, "x"))
			input.EligibilityByUID["x"] = Eligibility{Eligible: true, IdleSince: test.idle}
			candidate := build(t, input).Nodes[0].Candidates[0]
			if candidate.Weight != test.weight || candidate.Cost != test.weight*(2+100.0/64) {
				t.Fatalf("weight/cost = %g/%g", candidate.Weight, candidate.Cost)
			}
			if test.idle == nil || test.idle.IsZero() || test.idle.After(evaluationTime) {
				if candidate.IdleSince != nil {
					t.Fatal("invalid history was treated as known")
				}
			}
		})
	}
}

func TestSharedGroupUsesNewestHistoryAndUnionOfNodes(t *testing.T) {
	a := artifact("shared", 64, 1, "old", "new", "old")
	a.Models[0].NodesReady = []string{"a", "b", "", "b"}
	a.Models[1].NodesReady = []string{"c", "b"}
	input := fixture(64, a)
	input.EligibilityByUID["new"] = Eligibility{Eligible: true, IdleSince: age(3)}
	c := build(t, input).Nodes[0].Candidates[0]
	if c.Weight != 4 || c.Cost != 16 || len(c.Models) != 2 || !c.IdleSince.Equal(*age(3)) || !reflect.DeepEqual(c.PotentialNodes, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected shared candidate: %+v", c)
	}
	input.EligibilityByUID["new"] = Eligibility{Eligible: true}
	c = build(t, input).Nodes[0].Candidates[0]
	if c.Weight != 8 || c.IdleSince != nil {
		t.Fatalf("unknown history: %+v", c)
	}
}

func TestSyntheticCompleteAlternatives(t *testing.T) {
	input := fixture(500,
		artifact("A", 320, 1, "A"), artifact("B", 200, 2, "B"), artifact("C", 600, 2, "C1", "C2", "C3"),
		artifact("D", 256, 5, "D"), artifact("E", 256, 5, "E"), artifact("F", 400, 1, "F"), artifact("G", 64, 1, "G"))
	for uid, days := range map[types.UID]float64{"A": 10, "B": 8, "C1": 18, "C2": 20, "C3": 25, "D": 40, "E": 35, "F": 3, "G": .5} {
		input.EligibilityByUID[uid] = Eligibility{Eligible: true, IdleSince: age(days)}
	}
	plan := build(t, input)
	if got := paths(plan.SelectedGroups); !reflect.DeepEqual(got, []string{"/models/C"}) {
		t.Fatalf("selected %v", got)
	}
	if len(plan.Nodes[0].Alternatives) != 3 {
		t.Fatalf("alternatives = %+v", plan.Nodes[0].Alternatives)
	}
	for _, candidate := range plan.Nodes[0].Candidates {
		want := map[string]float64{"A": 12, "B": 10.25, "C": 11.375, "D": 9, "E": 9, "F": 29, "G": 16}[candidate.ID.Path[len("/models/"):]]
		if candidate.Cost != want {
			t.Fatalf("%s cost %g, want %g", candidate.ID.Path, candidate.Cost, want)
		}
	}
	input.EligibilityByUID["C2"] = Eligibility{Eligible: true, IdleSince: age(3)}
	if got := paths(build(t, input).SelectedGroups); !reflect.DeepEqual(got, []string{"/models/D", "/models/E"}) {
		t.Fatalf("selected %v", got)
	}
}

func TestLargeVersusSmallArtifacts(t *testing.T) {
	for _, test := range []struct {
		name       string
		largeDays  float64
		largeNodes int
		smallNodes int
		selected   int
	}{
		{"older small groups", 10, 1, 1, 10},
		{"old large group", 30, 1, 1, 1},
		{"large group affects fewer nodes", 10, 10, 100, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			artifacts := []artifactinventory.Artifact{artifact("large", 1000, test.largeNodes, "large")}
			for i := 0; i < 10; i++ {
				name := string(rune('a' + i))
				artifacts = append(artifacts, artifact(name, 80, test.smallNodes, name))
			}
			input := fixture(800, artifacts...)
			input.EligibilityByUID["large"] = Eligibility{Eligible: true, IdleSince: age(test.largeDays)}
			if got := len(build(t, input).SelectedGroups); got != test.selected {
				t.Fatalf("selected %d, want %d", got, test.selected)
			}
		})
	}
}

func TestDynamicScoresRecomputedAndSurvivingOrderPreserved(t *testing.T) {
	// A wins initially. With only 40 GiB left, C then beats B.
	input := fixture(100, artifact("A", 60, 1, "A"), artifact("B", 100, 2, "B"), artifact("C", 40, 1, "C"))
	plan := build(t, input)
	greedy := plan.Nodes[0].Alternatives[0]
	if got := alternativePaths(greedy); !reflect.DeepEqual(got, []string{"/models/A", "/models/C"}) {
		t.Fatalf("greedy = %v", got)
	}
	if got := paths(plan.SelectedGroups); !reflect.DeepEqual(got, []string{"/models/B"}) {
		t.Fatalf("complete alternative winner = %v", got)
	}
}

func TestDynamicRescoringChangesWinningPlan(t *testing.T) {
	// Initial scores order A,C,B. After A, only 180 GiB remains and B is
	// cheaper than C. This beats the recency alternative C,A.
	input := fixture(500,
		artifact("A", 320, 1, "A"),
		artifact("B", 200, 1, "B"),
		artifact("C", 256, 1, "C"),
	)
	input.EligibilityByUID["A"] = Eligibility{Eligible: true, IdleSince: age(20)}
	input.EligibilityByUID["B"] = Eligibility{Eligible: true, IdleSince: age(15)}
	plan := build(t, input)
	if got := paths(plan.SelectedGroups); !reflect.DeepEqual(got, []string{"/models/A", "/models/B"}) {
		t.Fatalf("winning order = %v", got)
	}
	greedy, recency := plan.Nodes[0].Alternatives[0], plan.Nodes[0].Alternatives[1]
	if !greedy.Selected || greedy.Cost != 10.125 || recency.Cost != 11 {
		t.Fatalf("complete alternatives: greedy=%+v recency=%+v", greedy, recency)
	}
}

func TestLocalPruningPrefersCostThenDescendingID(t *testing.T) {
	// Both expensive groups are redundant, but after the first removal only
	// one can be dropped. Equal costs remove the larger ID first.
	picks := []Candidate{
		{ID: GroupID{Path: "/A"}, EstimatedBytes: 40, Cost: 1},
		{ID: GroupID{Path: "/B"}, EstimatedBytes: 60, Cost: 3},
		{ID: GroupID{Path: "/C"}, EstimatedBytes: 60, Cost: 3},
	}
	alternative := makeAlternative("test", picks, 100)
	if got := alternativePaths(alternative); !reflect.DeepEqual(got, []string{"/A", "/B"}) {
		t.Fatalf("surviving order = %v", got)
	}
	if len(alternative.PrunedGroups) != 1 || alternative.PrunedGroups[0].Path != "/C" || alternative.Cost != 4 {
		t.Fatalf("pruned alternative = %+v", alternative)
	}
}

func TestRecencyUnknownLastAndLocalRedundancyPruning(t *testing.T) {
	input := fixture(100, artifact("A", 60, 1, "A"), artifact("B", 100, 1, "B"), artifact("C", 100, 1, "C"))
	input.EligibilityByUID["A"] = Eligibility{Eligible: true, IdleSince: age(50)}
	input.EligibilityByUID["B"] = Eligibility{Eligible: true, IdleSince: age(40)}
	input.EligibilityByUID["C"] = Eligibility{Eligible: true}
	plan := build(t, input)
	recency := plan.Nodes[0].Alternatives[1]
	if got := alternativePaths(recency); !reflect.DeepEqual(got, []string{"/models/B"}) || len(recency.PrunedGroups) != 1 || recency.PrunedGroups[0].Path != "/models/A" {
		t.Fatalf("recency pruning = %+v", recency)
	}
	if got := paths(plan.SelectedGroups); !reflect.DeepEqual(got, []string{"/models/B"}) {
		t.Fatalf("selected %v", got)
	}
}

func TestLocalTieBreaks(t *testing.T) {
	input := fixture(100, artifact("z", 100, 1, "z"), artifact("a", 100, 1, "a"))
	if got := paths(build(t, input).SelectedGroups); !reflect.DeepEqual(got, []string{"/models/a"}) {
		t.Fatalf("stable ID tie = %v", got)
	}
	input.EligibilityByUID["z"] = Eligibility{Eligible: true, IdleSince: age(40)}
	greedy := build(t, input).Nodes[0].Alternatives[0]
	if got := alternativePaths(greedy); !reflect.DeepEqual(got, []string{"/models/z"}) {
		t.Fatalf("recency score tie = %v", got)
	}
	// Cost ties prefer less excess, then fewer groups, then ID sequence.
	id := func(s string) GroupID { return GroupID{NodeName: "a", Path: s} }
	base := Alternative{Cost: 4, ReclaimedBytes: 100, Groups: []GroupID{id("a"), id("b")}}
	for _, worse := range []Alternative{
		{Cost: 5, ReclaimedBytes: 100, Groups: []GroupID{id("a")}},
		{Cost: 4, ReclaimedBytes: 101, Groups: []GroupID{id("a")}},
		{Cost: 4, ReclaimedBytes: 100, Groups: []GroupID{id("a"), id("b"), id("c")}},
		{Cost: 4, ReclaimedBytes: 100, Groups: []GroupID{id("b"), id("a")}},
	} {
		if !betterAlternative(base, worse) || betterAlternative(worse, base) {
			t.Fatalf("tie order failed: %+v", worse)
		}
	}
}

func TestCustomVersionedPolicy(t *testing.T) {
	input := fixture(64, artifact("x", 64, 1, "x"))
	input.Policy = ScoringPolicy{Version: "test-v2", IdleWindows: [3]time.Duration{time.Hour, 2 * time.Hour, 3 * time.Hour}, Weights: [4]float64{16, 8, 4, 2}, SizeScaleGiB: 32}
	plan := build(t, input)
	if plan.PolicyVersion != "test-v2" || plan.Nodes[0].Candidates[0].Cost != 6 {
		t.Fatalf("policy = %+v", plan)
	}
	input.Policy.Weights[0] = math.Inf(1)
	if _, err := Build(input); err == nil {
		t.Fatal("accepted infinite policy")
	}
}
