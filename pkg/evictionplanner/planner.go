package evictionplanner

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

type nodeState struct {
	node      Node
	result    NodeResult
	highStart int64
	active    bool
	goal      int64
}

// Build validates the snapshot, compares local choices, then removes redundant
// groups while preserving every pressure node's frozen final goal. It neither
// mutates input nor infers protection or local bytes from other-node records.
func Build(input Input) (*Plan, error) {
	if input.Now.IsZero() {
		return nil, fmt.Errorf("evaluation time is required")
	}
	if input.Policy == (ScoringPolicy{}) {
		input.Policy = DefaultPolicy()
	}
	if err := validatePolicy(input.Policy); err != nil {
		return nil, err
	}
	plan := &Plan{EvaluatedAt: input.Now.UTC(), PolicyVersion: input.Policy.Version}
	states, identities, err := prepareNodes(input)
	if err != nil {
		return nil, err
	}
	selectedUIDs := map[types.UID]bool{}
	// Local winners append in node order, expanding the round's shared UID union.
	for i := range states {
		state := &states[i]
		if !state.active {
			continue
		}
		credited := benefit(state.result.Candidates, selectedUIDs)
		if state.result.InitialUsedBytes-credited < state.highStart {
			state.result.Status, state.result.Reason = "skipped", "selected CRs already project usage below H"
			continue
		}
		state.result.Status = "planned"
		remaining := state.result.RequiredBytes - credited
		var pool []Candidate
		for j := range state.result.Candidates {
			candidate := &state.result.Candidates[j]
			if !covered(*candidate, selectedUIDs) {
				candidate.InitialScore = score(*candidate, remaining)
				pool = append(pool, *candidate)
			}
		}
		choice, alternatives := selectLocal(pool, remaining)
		state.result.Alternatives = alternatives
		plan.SelectedGroups = append(plan.SelectedGroups, choice...)
		selectedUIDs = uidUnion(plan.SelectedGroups)
	}
	// Freeze goals once, including any benefit supplied by later local choices.
	for i := range states {
		state := &states[i]
		if !state.active {
			continue
		}
		if state.result.Status == "skipped" {
			state.goal = state.result.InitialUsedBytes - state.highStart + 1
		} else {
			state.goal = min(state.result.RequiredBytes, benefit(state.result.Candidates, selectedUIDs))
		}
	}
	// Each insertion is tried once in reverse order. Rebuild the UID union:
	// deleting a group's UIDs directly would lose references from other groups.
	for i := len(plan.SelectedGroups) - 1; i >= 0; i-- {
		trial := append([]Candidate(nil), plan.SelectedGroups[:i]...)
		trial = append(trial, plan.SelectedGroups[i+1:]...)
		union := uidUnion(trial)
		preservesGoals := true
		for _, state := range states {
			if state.active && benefit(state.result.Candidates, union) < state.goal {
				preservesGoals = false
				break
			}
		}
		if preservesGoals {
			plan.PrunedGroups = append(plan.PrunedGroups, plan.SelectedGroups[i].ID)
			plan.SelectedGroups = trial
		}
	}
	// Recompute identities and every node projection from the pruned plan.
	selectedUIDs = uidUnion(plan.SelectedGroups)
	finalModels := make(map[types.UID]ModelIdentity, len(selectedUIDs))
	for uid := range selectedUIDs {
		finalModels[uid] = identities[uid]
	}
	plan.Models = sortedModels(finalModels)
	for _, state := range states {
		if state.active {
			state.result.ProjectedReclaimedBytes = benefit(state.result.Candidates, selectedUIDs)
			state.result.ProjectedUsedBytes = max(0, state.result.InitialUsedBytes-state.result.ProjectedReclaimedBytes)
			state.result.RemainingShortfallBytes = max(0, state.result.RequiredBytes-state.result.ProjectedReclaimedBytes)
		}
		plan.Nodes = append(plan.Nodes, state.result)
	}
	return plan, nil
}

// prepareNodes checks capacity first: mappings are only needed for pressure
// nodes. This permits callers to omit collection below H entirely.
func prepareNodes(input Input) ([]nodeState, map[types.UID]ModelIdentity, error) {
	identities := map[types.UID]ModelIdentity{}
	seenNodes := map[string]bool{}
	states := make([]nodeState, 0, len(input.Nodes))
	for _, node := range input.Nodes {
		if strings.TrimSpace(node.Name) == "" || seenNodes[node.Name] || node.CapacityBytes <= 0 || node.AvailableBytes < 0 || node.AvailableBytes > node.CapacityBytes {
			return nil, nil, fmt.Errorf("invalid or duplicate node snapshot %q", node.Name)
		}
		if math.IsNaN(node.HighWatermark) || math.IsNaN(node.LowWatermark) || node.HighWatermark > 1 || node.HighWatermark <= 0 || node.LowWatermark < 0 || node.LowWatermark >= node.HighWatermark {
			return nil, nil, fmt.Errorf("node %q: require 0 <= L < H <= 1", node.Name)
		}
		seenNodes[node.Name] = true
		used := node.CapacityBytes - node.AvailableBytes
		state := nodeState{node: node, highStart: watermarkBytes(node.CapacityBytes, node.HighWatermark, true),
			result: NodeResult{Name: node.Name, InitialUsedBytes: used, ProjectedUsedBytes: used}}
		switch {
		case used < state.highStart:
			state.result.Status, state.result.Reason = "below_high_watermark", "initial usage is below H"
		case node.Inventory == nil:
			state.result.Status, state.result.Reason = "unevaluated", node.InventoryError
			if state.result.Reason == "" {
				state.result.Reason = "inventory unavailable"
			}
		default:
			candidates, excluded, err := candidatesFor(node, input, identities)
			if err != nil {
				return nil, nil, err
			}
			state.active = true
			state.result.RequiredBytes = used - watermarkBytes(node.CapacityBytes, node.LowWatermark, false)
			state.result.Candidates, state.result.ExcludedArtifacts = candidates, excluded
		}
		states = append(states, state)
	}
	// Compare utilization ratios exactly, without overflow or floating rounding.
	sort.Slice(states, func(i, j int) bool {
		a := new(big.Rat).SetFrac64(states[i].result.InitialUsedBytes, states[i].node.CapacityBytes)
		b := new(big.Rat).SetFrac64(states[j].result.InitialUsedBytes, states[j].node.CapacityBytes)
		if comparison := a.Cmp(b); comparison != 0 {
			return comparison > 0
		}
		return states[i].node.Name < states[j].node.Name
	})
	return states, identities, nil
}

func validatePolicy(policy ScoringPolicy) error {
	if strings.TrimSpace(policy.Version) == "" || policy.SizeScaleGiB <= 0 || math.IsNaN(policy.SizeScaleGiB) || math.IsInf(policy.SizeScaleGiB, 0) {
		return fmt.Errorf("policy requires a version and finite positive size scale")
	}
	for i, window := range policy.IdleWindows {
		if window <= 0 || (i > 0 && window <= policy.IdleWindows[i-1]) {
			return fmt.Errorf("policy idle windows must be positive and increasing")
		}
	}
	for i, weight := range policy.Weights {
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) || (i > 0 && weight > policy.Weights[i-1]) {
			return fmt.Errorf("policy weights must be finite, positive and nonincreasing")
		}
	}
	return nil
}

// Decimal fractions match caller-facing H/L values (e.g. 0.9). H starts at
// ceil(capacity*H); L allows floor(capacity*L). big.Rat avoids int64 overflow
// and avoids treating decimal 0.9 as its slightly different binary float.
func watermarkBytes(capacity int64, fraction float64, roundUp bool) int64 {
	ratio, _ := new(big.Rat).SetString(strconv.FormatFloat(fraction, 'f', -1, 64))
	ratio.Mul(ratio, new(big.Rat).SetInt64(capacity))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(ratio.Num(), ratio.Denom(), remainder)
	if roundUp && remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.Int64()
}

func covered(candidate Candidate, union map[types.UID]bool) bool {
	for _, model := range candidate.Models {
		if !union[model.UID] {
			return false
		}
	}
	return true
}

func benefit(candidates []Candidate, union map[types.UID]bool) int64 {
	var bytes int64
	for _, candidate := range candidates {
		if covered(candidate, union) {
			bytes += candidate.EstimatedBytes
		}
	}
	return bytes
}

func uidUnion(groups []Candidate) map[types.UID]bool {
	union := map[types.UID]bool{}
	for _, group := range groups {
		for _, model := range group.Models {
			union[model.UID] = true
		}
	}
	return union
}

func sortedModels(models map[types.UID]ModelIdentity) []ModelIdentity {
	result := make([]ModelIdentity, 0, len(models))
	for _, model := range models {
		result = append(result, model)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.UID < b.UID
	})
	return result
}
