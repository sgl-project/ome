package evictionplanner

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

const bytesPerGiB = float64(1 << 30)

func candidatesFor(node Node, input Input, identities map[types.UID]ModelIdentity) ([]Candidate, []ExcludedArtifact, error) {
	if node.Inventory == nil {
		return nil, nil, nil
	}
	if node.Inventory.NodeName != node.Name {
		return nil, nil, fmt.Errorf("node %q: inventory belongs to %q", node.Name, node.Inventory.NodeName)
	}
	var candidates []Candidate
	var excluded []ExcludedArtifact
	seenPaths := map[string]bool{}
	for _, artifact := range node.Inventory.Artifacts {
		cleanPath := path.Clean(artifact.Path)
		if !path.IsAbs(artifact.Path) || cleanPath == "/" || seenPaths[cleanPath] {
			return nil, nil, fmt.Errorf("node %q: invalid or duplicate artifact path %q", node.Name, artifact.Path)
		}
		seenPaths[cleanPath] = true
		candidate := Candidate{ID: GroupID{NodeName: node.Name, Path: cleanPath}}
		reason := ""
		switch {
		case len(artifact.Issues) > 0:
			reasons := make([]string, 0, len(artifact.Issues))
			for _, issue := range artifact.Issues {
				reasons = append(reasons, issue.Code+": "+issue.Message)
			}
			sort.Strings(reasons)
			reason = "inventory issues: " + strings.Join(reasons, "; ")
		case !artifact.EvictionEligible:
			reason = "inventory marks artifact ineligible"
		case artifact.EstimatedArtifactBytes == nil || *artifact.EstimatedArtifactBytes <= 0:
			reason = "unknown or nonpositive size"
		}
		if reason != "" {
			excluded = append(excluded, ExcludedArtifact{ID: candidate.ID, Reason: reason})
			continue
		}
		if len(artifact.Models) == 0 {
			return nil, nil, fmt.Errorf("artifact %v: no associated models", candidate.ID)
		}
		models := map[types.UID]ModelIdentity{}
		nodes := map[string]bool{node.Name: true}
		for _, ref := range artifact.Models {
			identity := ModelIdentity{Namespace: ref.Namespace, Name: ref.Name, UID: ref.UID}
			previous, exists := identities[ref.UID]
			if strings.TrimSpace(string(ref.UID)) == "" || strings.TrimSpace(ref.Name) == "" || strings.TrimSpace(ref.Namespace) == "" || (exists && previous != identity) {
				return nil, nil, fmt.Errorf("artifact %v: invalid or conflicting identity for UID %q", candidate.ID, ref.UID)
			}
			identities[ref.UID], models[ref.UID] = identity, identity
			for _, name := range ref.NodesReady {
				if name != "" {
					nodes[name] = true
				}
			}
		}
		candidate.Models = sortedModels(models)
		for name := range nodes {
			candidate.PotentialNodes = append(candidate.PotentialNodes, name)
		}
		sort.Strings(candidate.PotentialNodes)
		unknownHistory := false
		for _, model := range candidate.Models {
			decision, exists := input.EligibilityByUID[model.UID]
			if reason == "" && (!exists || !decision.Eligible) {
				detail := decision.Reason
				if !exists {
					detail = "eligibility missing"
				} else if detail == "" {
					detail = "not authorized"
				}
				reason = fmt.Sprintf("model %s: %s", model.UID, detail)
			}
			weight, idle := historyWeight(input.Now, decision.IdleSince, input.Policy)
			candidate.Weight = math.Max(candidate.Weight, weight)
			if idle == nil {
				unknownHistory = true
			} else if candidate.IdleSince == nil || idle.After(*candidate.IdleSince) {
				candidate.IdleSince = idle
			}
		}
		if reason != "" {
			excluded = append(excluded, ExcludedArtifact{ID: candidate.ID, Reason: reason})
			continue
		}
		if unknownHistory {
			candidate.IdleSince = nil
		}
		candidate.EstimatedBytes = *artifact.EstimatedArtifactBytes
		candidate.Cost = candidate.Weight * (float64(len(nodes)) + float64(candidate.EstimatedBytes)/bytesPerGiB/input.Policy.SizeScaleGiB)
		candidates = append(candidates, candidate)
	}
	// Canonical order also makes floating-point cost addition input-order stable.
	sort.Slice(candidates, func(i, j int) bool { return lessID(candidates[i].ID, candidates[j].ID) })
	sort.Slice(excluded, func(i, j int) bool { return lessID(excluded[i].ID, excluded[j].ID) })
	var totalBytes int64
	totalCost := 0.0
	for _, candidate := range candidates {
		if candidate.EstimatedBytes > math.MaxInt64-totalBytes {
			return nil, nil, fmt.Errorf("node %q: artifact sizes overflow int64", node.Name)
		}
		totalBytes += candidate.EstimatedBytes
		totalCost += candidate.Cost
		if math.IsInf(totalCost, 0) || math.IsNaN(totalCost) {
			return nil, nil, fmt.Errorf("node %q: scoring cost overflow", node.Name)
		}
	}
	return candidates, excluded, nil
}

func historyWeight(now time.Time, since *time.Time, policy ScoringPolicy) (float64, *time.Time) {
	if since == nil || since.IsZero() || since.After(now) {
		return policy.Weights[0], nil
	}
	idle := since.UTC() // Own this value; never expose a caller-owned timestamp.
	for i, window := range policy.IdleWindows {
		if now.Sub(idle) < window {
			return policy.Weights[i], &idle
		}
	}
	return policy.Weights[3], &idle
}

func lessID(a, b GroupID) bool {
	if a.NodeName != b.NodeName {
		return a.NodeName < b.NodeName
	}
	return a.Path < b.Path
}

func older(a, b Candidate) bool {
	if a.IdleSince == nil || b.IdleSince == nil {
		if (a.IdleSince == nil) != (b.IdleSince == nil) {
			return b.IdleSince == nil
		}
	} else if !a.IdleSince.Equal(*b.IdleSince) {
		return a.IdleSince.Before(*b.IdleSince)
	}
	return lessID(a.ID, b.ID)
}

func score(candidate Candidate, remaining int64) float64 {
	return candidate.Cost / (float64(min(candidate.EstimatedBytes, remaining)) / bytesPerGiB)
}

// selectLocal keeps the incoming UID union fixed. Complete alternatives, not
// individual scores, decide the winner. The pool is already in stable ID order.
func selectLocal(pool []Candidate, required int64) ([]Candidate, []Alternative) {
	if required <= 0 {
		return nil, nil
	}
	var total int64
	for _, candidate := range pool {
		total += candidate.EstimatedBytes
	}
	var alternatives []Alternative
	if total < required {
		alternatives = append(alternatives, makeAlternative("insufficient", pool, required))
	} else {
		for _, strategy := range []string{"greedy", "recency"} {
			remaining := required
			available := append([]Candidate(nil), pool...)
			var picks []Candidate
			for remaining > 0 {
				best := 0
				for i := 1; i < len(available); i++ {
					a, b := available[i], available[best]
					if strategy == "recency" {
						if older(a, b) {
							best = i
						}
					} else if score(a, remaining) < score(b, remaining) || (score(a, remaining) == score(b, remaining) && older(a, b)) {
						best = i
					}
				}
				picks = append(picks, available[best])
				remaining -= available[best].EstimatedBytes
				available = append(available[:best], available[best+1:]...)
			}
			alternatives = append(alternatives, makeAlternative(strategy, picks, required))
		}
		for _, candidate := range pool {
			if candidate.EstimatedBytes >= required {
				alternatives = append(alternatives, makeAlternative("single", []Candidate{candidate}, required))
			}
		}
	}
	best := 0
	for i := 1; i < len(alternatives); i++ {
		if betterAlternative(alternatives[i], alternatives[best]) {
			best = i
		}
	}
	alternatives[best].Selected = true
	byID := make(map[GroupID]Candidate, len(pool))
	for _, candidate := range pool {
		byID[candidate.ID] = candidate
	}
	var selected []Candidate
	for _, id := range alternatives[best].Groups {
		selected = append(selected, byID[id])
	}
	return selected, alternatives
}

func makeAlternative(strategy string, picks []Candidate, required int64) Alternative {
	result := Alternative{Strategy: strategy}
	for _, candidate := range picks {
		result.ReclaimedBytes += candidate.EstimatedBytes
	}
	removalOrder := append([]Candidate(nil), picks...)
	sort.Slice(removalOrder, func(i, j int) bool {
		if removalOrder[i].Cost != removalOrder[j].Cost {
			return removalOrder[i].Cost > removalOrder[j].Cost
		}
		return lessID(removalOrder[j].ID, removalOrder[i].ID)
	})
	removed := map[GroupID]bool{}
	for _, candidate := range removalOrder {
		if result.ReclaimedBytes-candidate.EstimatedBytes >= required {
			removed[candidate.ID] = true
			result.PrunedGroups = append(result.PrunedGroups, candidate.ID)
			result.ReclaimedBytes -= candidate.EstimatedBytes
		}
	}
	for _, candidate := range picks {
		if !removed[candidate.ID] {
			result.Groups = append(result.Groups, candidate.ID)
		}
	}
	sort.Slice(removalOrder, func(i, j int) bool { return lessID(removalOrder[i].ID, removalOrder[j].ID) })
	for _, candidate := range removalOrder {
		if !removed[candidate.ID] {
			result.Cost += candidate.Cost
		}
	}
	return result
}

func betterAlternative(a, b Alternative) bool {
	if a.Cost != b.Cost {
		return a.Cost < b.Cost
	}
	// Every compared alternative has the same required bytes, so comparing total
	// reclaimed bytes is equivalent to comparing excess bytes.
	if a.ReclaimedBytes != b.ReclaimedBytes {
		return a.ReclaimedBytes < b.ReclaimedBytes
	}
	if len(a.Groups) != len(b.Groups) {
		return len(a.Groups) < len(b.Groups)
	}
	for i := range a.Groups {
		if a.Groups[i] != b.Groups[i] {
			return lessID(a.Groups[i], b.Groups[i])
		}
	}
	return false
}
