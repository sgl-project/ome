package evictionplanner

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/artifactinventory"
)

func byteArtifact(name string, size int64, uids ...string) artifactinventory.Artifact {
	a := artifact(name, 1, 1, uids...)
	a.EstimatedArtifactBytes = &size
	return a
}

func byteNode(name string, used int64, artifacts ...artifactinventory.Artifact) Node {
	return Node{Name: name, CapacityBytes: 1000, AvailableBytes: 1000 - used, HighWatermark: .9, LowWatermark: .8,
		Inventory: &artifactinventory.Inventory{NodeName: name, Artifacts: artifacts}}
}

func batch(nodes ...Node) Input {
	input := Input{Now: evaluationTime, Nodes: nodes, EligibilityByUID: map[types.UID]Eligibility{}}
	for _, n := range nodes {
		if n.Inventory == nil {
			continue
		}
		for _, a := range n.Inventory.Artifacts {
			for _, m := range a.Models {
				input.EligibilityByUID[m.UID] = Eligibility{Eligible: true, IdleSince: age(30)}
			}
		}
	}
	return input
}

func nodeResult(t *testing.T, plan *Plan, name string) NodeResult {
	t.Helper()
	for _, node := range plan.Nodes {
		if node.Name == name {
			return node
		}
	}
	t.Fatalf("missing node %s", name)
	return NodeResult{}
}

func TestCreditUsesEachNodesLocalSize(t *testing.T) {
	input := batch(byteNode("a", 950, byteArtifact("x", 120, "x")), byteNode("b", 900, byteArtifact("x", 200, "x")))
	plan := build(t, input)
	if len(plan.SelectedGroups) != 1 || len(plan.Models) != 1 {
		t.Fatalf("selection = %+v", plan)
	}
	a, b := nodeResult(t, plan, "a"), nodeResult(t, plan, "b")
	if a.ProjectedReclaimedBytes != 120 || a.RemainingShortfallBytes != 30 || b.ProjectedReclaimedBytes != 200 || b.Status != "skipped" {
		t.Fatalf("local sizes not respected: a=%+v b=%+v", a, b)
	}
}

func TestPartialSharedReferencesHaveNoCredit(t *testing.T) {
	input := batch(byteNode("a", 950, byteArtifact("x", 150, "x")), byteNode("b", 900, byteArtifact("shared", 200, "x", "y")))
	input.EligibilityByUID["y"] = Eligibility{Eligible: false, Reason: "serving"}
	plan := build(t, input)
	b := nodeResult(t, plan, "b")
	if b.ProjectedReclaimedBytes != 0 || b.RemainingShortfallBytes != 100 || len(b.Candidates) != 0 || len(b.ExcludedArtifacts) != 1 || !strings.Contains(b.ExcludedArtifacts[0].Reason, "serving") {
		t.Fatalf("partial references or protected model credited: %+v", b)
	}
	if len(plan.Models) != 1 || plan.Models[0].UID != "x" {
		t.Fatalf("models = %+v", plan.Models)
	}
}

func TestLaterChoiceRemovesEarlierGroupAndPreservesSharedUID(t *testing.T) {
	input := batch(byteNode("a", 950, byteArtifact("x", 150, "x")), byteNode("b", 900, byteArtifact("shared", 200, "x", "y")))
	plan := build(t, input)
	if len(plan.SelectedGroups) != 1 || plan.SelectedGroups[0].ID.NodeName != "b" || len(plan.PrunedGroups) != 1 || len(plan.Models) != 2 {
		t.Fatalf("reverse prune = %+v", plan)
	}
	if nodeResult(t, plan, "a").ProjectedReclaimedBytes != 150 {
		t.Fatal("lost x while another group still references it")
	}
}

func TestSkippedNodeMustRemainStrictlyBelowHighWatermark(t *testing.T) {
	for _, ySize := range []int64{50, 51} {
		t.Run(strconv.FormatInt(ySize, 10), func(t *testing.T) {
			y := byteArtifact("y", 180, "y")
			y.Models[0].NodesReady = []string{"other"} // Makes x the cheaper initial choice on a.
			input := batch(
				byteNode("a", 980, byteArtifact("x", 180, "x"), y),
				byteNode("c", 950, byteArtifact("x", 51, "x"), byteArtifact("y", ySize, "y")),
				byteNode("b", 900, byteArtifact("y", 100, "y")),
			)
			plan := build(t, input)
			c := nodeResult(t, plan, "c")
			if c.Status != "skipped" || c.ProjectedUsedBytes >= 900 {
				t.Fatalf("skip goal broken: %+v", c)
			}
			want := 2
			if ySize == 51 {
				want = 1
			}
			if len(plan.SelectedGroups) != want {
				t.Fatalf("selected %d, want %d: %+v", len(plan.SelectedGroups), want, plan)
			}
		})
	}
}

func TestFrozenInsufficientGoalPreservesFinalBenefit(t *testing.T) {
	// a cannot reach L. A later group supplies x as well, allowing its earlier
	// group to be removed while preserving all 90 bytes of final benefit.
	input := batch(
		byteNode("a", 990, byteArtifact("x", 40, "x"), byteArtifact("y", 50, "y")),
		byteNode("b", 950, byteArtifact("shared", 150, "x", "z")),
	)
	plan := build(t, input)
	a := nodeResult(t, plan, "a")
	if a.ProjectedReclaimedBytes != 90 || a.RemainingShortfallBytes != 100 || len(plan.Models) != 3 {
		t.Fatalf("frozen insufficient benefit = %+v", plan)
	}
	if len(plan.SelectedGroups) != 2 {
		t.Fatalf("expected y and the later shared group: %+v", plan.SelectedGroups)
	}
}

func TestBelowHighWatermarkAndMissingInventory(t *testing.T) {
	below := byteNode("below", 899, byteArtifact("x", 100, "x"))
	exact := byteNode("exact", 900, byteArtifact("y", 100, "y"))
	missing := byteNode("missing", 990)
	missing.Inventory = nil
	missing.InventoryError = "mapping fetch failed"
	plan := build(t, batch(below, exact, missing))
	if len(plan.SelectedGroups) != 1 || plan.SelectedGroups[0].ID.NodeName != "exact" {
		t.Fatalf("selected %+v", plan.SelectedGroups)
	}
	if nodeResult(t, plan, "below").Status != "below_high_watermark" {
		t.Fatal("below H was planned")
	}
	result := nodeResult(t, plan, "missing")
	if result.Status != "unevaluated" || result.Reason != "mapping fetch failed" || result.ProjectedReclaimedBytes != 0 || result.RemainingShortfallBytes != 0 {
		t.Fatalf("missing inventory = %+v", result)
	}
}

func TestBelowHighWatermarkDoesNotInspectInventory(t *testing.T) {
	for _, test := range []struct {
		name      string
		inventory *artifactinventory.Inventory
	}{
		{"not collected", nil},
		{"irrelevant malformed mapping", &artifactinventory.Inventory{
			NodeName: "different-node", Artifacts: []artifactinventory.Artifact{{Path: "relative"}},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := byteNode("a", 899)
			node.Inventory = test.inventory
			plan := build(t, batch(node))
			if plan.Nodes[0].Status != "below_high_watermark" || len(plan.Models) != 0 {
				t.Fatalf("below-H node inspected or planned: %+v", plan)
			}
		})
	}
}

func TestIntegerWatermarkBoundaries(t *testing.T) {
	for _, used := range []int64{750, 751} {
		node := byteNode("a", 0, byteArtifact("x", 51, "x"))
		node.CapacityBytes, node.AvailableBytes, node.HighWatermark, node.LowWatermark = 1001, 1001-used, .75, .70
		plan := build(t, batch(node))
		if used == 750 && len(plan.SelectedGroups) != 0 {
			t.Fatal("rounded down H")
		}
		if used == 751 && (len(plan.SelectedGroups) != 1 || plan.Nodes[0].RequiredBytes != 51 || plan.Nodes[0].ProjectedUsedBytes != 700) {
			t.Fatalf("H/L rounding = %+v", plan)
		}
	}
	// Values near int64 maximum cannot safely be multiplied by a float.
	node := Node{Name: "a", CapacityBytes: math.MaxInt64, AvailableBytes: math.MaxInt64 / 10, HighWatermark: .9, LowWatermark: .8,
		Inventory: &artifactinventory.Inventory{NodeName: "a"}}
	plan := build(t, batch(node))
	if plan.Nodes[0].RequiredBytes != 922337203685477582 {
		t.Fatalf("large exact threshold = %+v", plan.Nodes[0])
	}
}

func TestExcludedArtifactsAndInsufficientStableOrder(t *testing.T) {
	issues := byteArtifact("issue", 100, "issue")
	issues.Issues = []artifactinventory.Issue{{Code: "mapping", Message: "uncertain reference"}}
	notEligible := byteArtifact("not-eligible", 100, "not-eligible")
	notEligible.EvictionEligible = false
	unknown := byteArtifact("unknown", 100, "unknown")
	unknown.EstimatedArtifactBytes = nil
	zero := byteArtifact("zero", 0, "zero")
	negative := byteArtifact("negative", -1, "negative")
	input := batch(byteNode("a", 950, byteArtifact("z", 30, "z"), byteArtifact("a", 20, "a"), issues, notEligible, unknown, zero, negative, byteArtifact("missing-policy", 100, "missing-policy")))
	delete(input.EligibilityByUID, "missing-policy")
	plan := build(t, input)
	if got := paths(plan.SelectedGroups); !reflect.DeepEqual(got, []string{"/models/a", "/models/z"}) {
		t.Fatalf("insufficient order = %v", got)
	}
	if plan.Nodes[0].RemainingShortfallBytes != 100 || len(plan.Nodes[0].ExcludedArtifacts) != 6 {
		t.Fatalf("exclusions = %+v", plan.Nodes[0])
	}
	empty := build(t, batch(byteNode("a", 950)))
	if len(empty.Models) != 0 || empty.Nodes[0].RemainingShortfallBytes != 150 {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestKnownInventoryIssuesExcludeMissingModelReferences(t *testing.T) {
	// The inventory builder can resolve a shared path even when every related
	// CR lookup failed. Its issue is useful evidence, not a malformed candidate.
	missingModel := artifactinventory.Artifact{
		Path:   "/models/missing",
		Issues: []artifactinventory.Issue{{Code: "modelNotFound", Message: "Ready CR no longer exists"}},
	}
	input := batch(byteNode("a", 950, byteArtifact("valid", 150, "valid"), missingModel))
	plan := build(t, input)
	if len(plan.Models) != 1 || plan.Models[0].UID != "valid" || len(plan.Nodes[0].ExcludedArtifacts) != 1 {
		t.Fatalf("known mapping issue affected valid candidate: %+v", plan)
	}
}

func TestGlobalProtectionAppliesToEveryNode(t *testing.T) {
	input := batch(byteNode("a", 950, byteArtifact("x", 150, "x")), byteNode("b", 900, byteArtifact("x", 200, "x")))
	input.EligibilityByUID["x"] = Eligibility{Eligible: false, Reason: "active claim"}
	plan := build(t, input)
	if len(plan.Models) != 0 || len(plan.SelectedGroups) != 0 {
		t.Fatalf("protected UID selected: %+v", plan)
	}
	for _, n := range plan.Nodes {
		if n.ProjectedReclaimedBytes != 0 {
			t.Fatal("protected artifact credited")
		}
	}
}

func TestInvalidStructuralInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Input)
	}{
		{"missing clock", func(i *Input) { i.Now = time.Time{} }},
		{"duplicate node", func(i *Input) { i.Nodes = append(i.Nodes, i.Nodes[0]) }},
		{"empty node", func(i *Input) { i.Nodes[0].Name = "" }},
		{"bad capacity", func(i *Input) { i.Nodes[0].CapacityBytes = 0 }},
		{"bad availability", func(i *Input) { i.Nodes[0].AvailableBytes = -1 }},
		{"availability above capacity", func(i *Input) { i.Nodes[0].AvailableBytes = 1001 }},
		{"bad high watermark", func(i *Input) { i.Nodes[0].HighWatermark = math.NaN() }},
		{"low above high", func(i *Input) { i.Nodes[0].LowWatermark = .95 }},
		{"wrong inventory node", func(i *Input) { i.Nodes[0].Inventory.NodeName = "b" }},
		{"relative path", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Path = "relative" }},
		{"empty path", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Path = "" }},
		{"root path", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Path = "/" }},
		{"duplicate normalized path", func(i *Input) {
			a := i.Nodes[0].Inventory.Artifacts[0]
			a.Path = "/models/other/../x/"
			i.Nodes[0].Inventory.Artifacts = append(i.Nodes[0].Inventory.Artifacts, a)
		}},
		{"empty model UID", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Models[0].UID = "" }},
		{"empty model name", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Models[0].Name = "" }},
		{"empty model namespace", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Models[0].Namespace = "" }},
		{"no models", func(i *Input) { i.Nodes[0].Inventory.Artifacts[0].Models = nil }},
		{"conflicting identity", func(i *Input) {
			a := byteArtifact("y", 100, "x")
			a.Models[0].Name = "another"
			i.Nodes[0].Inventory.Artifacts = append(i.Nodes[0].Inventory.Artifacts, a)
		}},
		{"size sum overflow", func(i *Input) {
			i.Nodes[0].Inventory.Artifacts = []artifactinventory.Artifact{byteArtifact("x", math.MaxInt64, "x"), byteArtifact("y", 1, "y")}
			i.EligibilityByUID["y"] = Eligibility{Eligible: true}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := batch(byteNode("a", 950, byteArtifact("x", 100, "x")))
			test.change(&input)
			if _, err := Build(input); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestDeterminismAndNoInputMutation(t *testing.T) {
	input := batch(
		byteNode("b", 950, byteArtifact("shared", 150, "x", "y")),
		byteNode("a", 950, byteArtifact("x", 120, "x"), byteArtifact("y", 40, "y")),
	)
	before, _ := json.Marshal(input)
	want := build(t, input)
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("Build mutated input")
	}
	rng := rand.New(rand.NewSource(27538))
	for round := 0; round < 20; round++ {
		rng.Shuffle(len(input.Nodes), func(i, j int) { input.Nodes[i], input.Nodes[j] = input.Nodes[j], input.Nodes[i] })
		for _, n := range input.Nodes {
			rng.Shuffle(len(n.Inventory.Artifacts), func(i, j int) {
				n.Inventory.Artifacts[i], n.Inventory.Artifacts[j] = n.Inventory.Artifacts[j], n.Inventory.Artifacts[i]
			})
			for _, a := range n.Inventory.Artifacts {
				rng.Shuffle(len(a.Models), func(i, j int) { a.Models[i], a.Models[j] = a.Models[j], a.Models[i] })
			}
		}
		if got := build(t, input); !reflect.DeepEqual(got, want) {
			t.Fatalf("input permutation changed plan: %+v versus %+v", got, want)
		}
	}
	// Output history is copied: the caller's snapshot remains unchanged.
	if len(want.SelectedGroups) > 0 && want.SelectedGroups[0].IdleSince != nil {
		*want.SelectedGroups[0].IdleSince = evaluationTime
		if !input.EligibilityByUID["x"].IdleSince.Equal(*age(30)) {
			t.Fatal("output aliases caller's history")
		}
	}
}
