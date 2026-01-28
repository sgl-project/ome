package acceleratorclassselector

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// candidateAccelerator represents an AcceleratorClass candidate for selection
type candidateAccelerator struct {
	Name   string
	Spec   *v1beta1.AcceleratorClassSpec
	Status *v1beta1.AcceleratorClassStatus
}

// scoredCandidate wraps a candidate with its calculated score
type scoredCandidate struct {
	candidateAccelerator
	Score  float64
	Reason string // Human-readable explanation for debugging
}

// candidatesFromNames fetches AcceleratorClass specs for given names
func candidatesFromNames(
	ctx context.Context,
	fetcher AcceleratorFetcher,
	names []string,
) ([]candidateAccelerator, error) {
	logger := log.FromContext(ctx)

	// Deduplicate names
	uniqueNames := make(map[string]struct{})
	for _, name := range names {
		uniqueNames[name] = struct{}{}
	}

	candidates := make([]candidateAccelerator, 0, len(uniqueNames))

	for name := range uniqueNames {
		spec, found, err := fetcher.GetAcceleratorClass(ctx, name)
		if err != nil {
			logger.Error(err, "Failed to fetch AcceleratorClass", "name", name)
			return nil, fmt.Errorf("failed to fetch AcceleratorClass %s: %w", name, err)
		}
		if !found {
			logger.V(1).Info("AcceleratorClass not found, skipping", "name", name)
			continue
		}
		if spec == nil {
			logger.Error(nil, "AcceleratorClass has nil spec", "name", name)
			continue
		}

		candidates = append(candidates, candidateAccelerator{
			Name: name,
			Spec: spec,
			// Status would need separate fetch if needed
		})
	}

	return candidates, nil
}

// filterCandidates applies constraints to filter out ineligible candidates
func filterCandidates(
	ctx context.Context,
	candidates []candidateAccelerator,
	constraints *v1beta1.AcceleratorConstraints,
	considerAvailability bool,
) []candidateAccelerator {
	logger := log.FromContext(ctx)
	filtered := make([]candidateAccelerator, 0, len(candidates))

	for _, candidate := range candidates {
		eligible, reason := meetsRequirements(candidate, constraints, considerAvailability)
		if eligible {
			filtered = append(filtered, candidate)
		} else {
			logger.V(1).Info("Filtered out candidate", "name", candidate.Name, "reason", reason)
		}
	}

	logger.Info("Filtered candidates", "total", len(candidates), "eligible", len(filtered))
	return filtered
}

// meetsRequirements checks if a candidate satisfies all requirements from InferenceService
func meetsRequirements(
	candidate candidateAccelerator,
	constraints *v1beta1.AcceleratorConstraints,
	considerAvailability bool,
) (bool, string) {
	spec := candidate.Spec

	// If no constraints, all candidates are eligible
	if constraints == nil {
		return true, ""
	}

	// Check if excluded
	for _, excluded := range constraints.ExcludedClasses {
		if candidate.Name == excluded {
			return false, "explicitly excluded"
		}
	}

	// Check architecture family
	if len(constraints.ArchitectureFamilies) > 0 {
		candidateVendorFamily := fmt.Sprintf("%s-%s",
			strings.ToLower(spec.Vendor),
			strings.ToLower(spec.Family))
		candidateFamily := strings.ToLower(spec.Family)
		found := false
		for _, family := range constraints.ArchitectureFamilies {
			if strings.Contains(family, "-") {
				if strings.EqualFold(family, candidateVendorFamily) {
					found = true
					break
				}
			} else {
				if strings.EqualFold(family, candidateFamily) {
					found = true
					break
				}
			}
		}
		if !found {
			return false, fmt.Sprintf("architecture family %s not in allowed list", candidateFamily)
		}
	}

	// Check MinMemory
	if constraints.MinMemory != nil {
		if spec.Capabilities.MemoryGB == nil {
			return false, "missing memory specification for MinMemory check"
		}
		memGB := spec.Capabilities.MemoryGB.Value() / (1024 * 1024 * 1024)
		if memGB < *constraints.MinMemory {
			return false, fmt.Sprintf("memory %dGB < required %dGB", memGB, *constraints.MinMemory)
		}
	}

	// Check MaxMemory (cost control)
	if constraints.MaxMemory != nil {
		if spec.Capabilities.MemoryGB == nil {
			return false, "missing memory specification for MaxMemory check"
		}
		memGB := spec.Capabilities.MemoryGB.Value() / (1024 * 1024 * 1024)
		if memGB > *constraints.MaxMemory {
			return false, fmt.Sprintf("memory %dGB > max allowed %dGB", memGB, *constraints.MaxMemory)
		}
	}

	// NOTE: MinComputePerformanceTFLOPS is now handled as a scoring factor in calculateComputeCapabilityScore(), not as a hard filter

	// Check RequiredFeatures (ALL must be present - hard requirement)
	if len(constraints.RequiredFeatures) > 0 {
		candidateFeatures := make(map[string]struct{})
		for _, f := range spec.Capabilities.Features {
			candidateFeatures[strings.ToLower(f)] = struct{}{}
		}
		for _, required := range constraints.RequiredFeatures {
			if _, found := candidateFeatures[strings.ToLower(required)]; !found {
				return false, fmt.Sprintf("missing required feature: %s", required)
			}
		}
	}

	// Check MinArchitectureVersion (e.g., "8.0" for Ampere, "9.0" for Hopper)
	if constraints.MinArchitectureVersion != nil {
		if spec.Capabilities.ComputeCapability == "" {
			return false, "missing compute capability for architecture version check"
		}

		// Compare version strings lexicographically (works for NVIDIA format like "7.5", "8.0", "8.6", "9.0")
		candidateVersion := spec.Capabilities.ComputeCapability
		requiredVersion := *constraints.MinArchitectureVersion

		if candidateVersion < requiredVersion {
			return false, fmt.Sprintf("compute capability %s < required %s",
				candidateVersion, requiredVersion)
		}
	}

	// Check availability if enabled
	if considerAvailability {
		if candidate.Status != nil && candidate.Status.AvailableAccelerators <= 0 {
			return false, "no accelerators currently available"
		}
	}

	return true, ""
}

// calculateBestFitScore computes multi-criteria score for best fit
// Note: RequiredFeatures are NOT scored - they are hard filters applied before scoring
// Note: Precision matching is now integrated into compute scoring (via iterative precision fallback)
func calculateBestFitScore(
	candidate candidateAccelerator,
	constraints *v1beta1.AcceleratorConstraints,
) float64 {
	weights := struct {
		memory  float64
		compute float64
	}{0.70, 0.30}

	memScore := calculateMemoryFitScore(candidate, constraints)
	computeScore := calculateComputePerformanceTFLOPSScore(candidate, constraints)

	totalScore := weights.memory*memScore + weights.compute*computeScore

	return totalScore
}

// calculateMemoryFitScore penalizes over-provisioning
func calculateMemoryFitScore(
	candidate candidateAccelerator,
	constraints *v1beta1.AcceleratorConstraints,
) float64 {
	if constraints.MinMemory == nil {
		return 1.0 // No requirement, perfect score
	}

	// Should already be filtered, but safety check
	spec := candidate.Spec
	if spec.Capabilities.MemoryGB == nil {
		return 0.0
	}

	candidateMemGB := float64(spec.Capabilities.MemoryGB.Value()) / (1024 * 1024 * 1024)
	requiredMemGB := float64(*constraints.MinMemory)

	if candidateMemGB < requiredMemGB {
		return 0.0 // Should already be filtered, but safety check
	}

	if candidateMemGB == requiredMemGB {
		return 1.0 // Perfect match
	}

	// Penalize over-provisioning: score = 1 / ratio (NO CAP)
	// 2x over = 0.5, 10x over = 0.1 (allows full penalty)
	ratio := candidateMemGB / requiredMemGB
	score := 1.0 / ratio

	return score
}

// scoreFromTFLOPS calculates score based on TFLOPS ratio to requirement
// - tflops = 0: returns 0.0
// - requiredTFLOPS = 0: returns 1.0 (no requirement, any non-zero TFLOPS is perfect)
// - tflops >= requiredTFLOPS: returns 1.0 (meeting or exceeding requirement)
// - tflops < requiredTFLOPS: returns proportional score (e.g., 50/100 = 0.5)
func scoreFromTFLOPS(tflops int64, requiredTFLOPS int64) float64 {
	if tflops == 0 {
		return 0.0
	}
	if requiredTFLOPS == 0 {
		// No requirement specified, any non-zero TFLOPS is perfect
		return 1.0
	}
	ratio := float64(tflops) / float64(requiredTFLOPS)
	// Cap at 1.0 - meeting requirement is best score, exceeding doesn't give bonus
	if ratio >= 1.0 {
		return 1.0
	}
	// Proportional score for partial matches (e.g., 50 TFLOPS / 100 required = 0.5 score)
	return ratio
}

// calculateComputePerformanceTFLOPSScore scores compute performance with iterative precision fallback
// NOTE: This now uses MinComputePerformanceTFLOPS as a scoring factor (not a hard filter)
// Iterative precision logic:
// - If no PreferredPrecisions: use max TFLOPS across all precisions
// - If PreferredPrecisions exist: iterate through them
//   - First precision with TFLOPS > 0: baseScore * 1.0
//   - Second precision: baseScore * 0.5 (penalty *= 0.5)
//   - Third precision: baseScore * 0.25 (penalty *= 0.5)
//
// - Fallback to fp16 if not in list
// - Return 0.0 if no precision has any TFLOPS data
func calculateComputePerformanceTFLOPSScore(
	candidate candidateAccelerator,
	constraints *v1beta1.AcceleratorConstraints,
) float64 {
	spec := candidate.Spec
	if spec.Capabilities.Performance == nil {
		return 0.0
	}

	perf := spec.Capabilities.Performance

	// If no MinComputePerformanceTFLOPS requirement, return 1.0
	requiredTFLOPS := int64(0)
	if constraints.MinComputePerformanceTFLOPS != nil {
		requiredTFLOPS = *constraints.MinComputePerformanceTFLOPS
	}

	// Case 1: No preferred precisions specified - use max TFLOPS across all precisions
	if len(constraints.PreferredPrecisions) == 0 {
		tflops := getMaxTFLOPS(perf)
		return scoreFromTFLOPS(tflops, requiredTFLOPS)
	}

	// Case 2: Try each preferred precision in order with penalty degradation
	penalty := 1.0
	for _, precision := range constraints.PreferredPrecisions {
		tflops := getTFLOPSForPrecision(perf, strings.ToLower(precision))
		if tflops > 0 {
			baseScore := scoreFromTFLOPS(tflops, requiredTFLOPS)
			// Apply precision penalty (1.0 for first, 0.5 for second, 0.25 for third, etc.)
			return baseScore * penalty
		}
		// Degrade penalty for next precision
		penalty *= 0.5
	}

	// Case 3: Fallback to fp16 if not already checked
	fp16InList := false
	for _, p := range constraints.PreferredPrecisions {
		if strings.ToLower(p) == "fp16" {
			fp16InList = true
			break
		}
	}

	if !fp16InList {
		tflops := getTFLOPSForPrecision(perf, "fp16")
		if tflops > 0 {
			baseScore := scoreFromTFLOPS(tflops, requiredTFLOPS)
			return baseScore * penalty // Heavily penalized fallback
		}
	}

	// Case 4: No precision has any TFLOPS data
	return 0.0
}

// getCandidateCost extracts cost with priority: spot > hourly > token > tier
func getCandidateCost(candidate candidateAccelerator) (*resource.Quantity, string, error) {
	spec := candidate.Spec
	if spec.Cost == nil {
		return nil, "", fmt.Errorf("no cost information")
	}

	// Priority 1: Spot pricing (if available)
	if spec.Cost.SpotPerHour != nil {
		return spec.Cost.SpotPerHour, "spot-hourly", nil
	}

	// Priority 2: On-demand hourly
	if spec.Cost.PerHour != nil {
		return spec.Cost.PerHour, "hourly", nil
	}

	// Priority 3: Token-based (less preferred, harder to compare)
	if spec.Cost.PerMillionTokens != nil {
		return spec.Cost.PerMillionTokens, "per-million-tokens", nil
	}

	// Fallback: Try to use cost tier as string
	if spec.Cost.Tier != "" {
		// Map tier to numeric value: low=1, medium=2, high=3
		tierValue := int64(2) // default medium
		switch strings.ToLower(spec.Cost.Tier) {
		case "low":
			tierValue = 1
		case "medium":
			tierValue = 2
		case "high":
			tierValue = 3
		}
		qty := resource.NewQuantity(tierValue, resource.DecimalSI)
		return qty, "tier", nil
	}

	return nil, "", fmt.Errorf("no cost data available")
}

// compareCosts returns -1 if cost1 < cost2, 0 if equal, 1 if cost1 > cost2
func compareCosts(cost1, cost2 *resource.Quantity) int {
	return cost1.Cmp(*cost2)
}

// calculateCapabilityScore computes performance score based on precision
func calculateCapabilityScore(
	candidate candidateAccelerator,
	preferredPrecisions []string,
) float64 {
	spec := candidate.Spec

	// Determine primary precision
	primaryPrecision := "fp16" // default
	if len(preferredPrecisions) > 0 {
		primaryPrecision = strings.ToLower(preferredPrecisions[0])
	}

	// Get TFLOPS for primary precision
	tflops := float64(getTFLOPSForPrecision(spec.Capabilities.Performance, primaryPrecision))

	// If primary precision not available, try fallbacks
	if tflops == 0.0 && len(preferredPrecisions) > 1 {
		for i := 1; i < len(preferredPrecisions); i++ {
			fallbackTFLOPS := getTFLOPSForPrecision(spec.Capabilities.Performance, strings.ToLower(preferredPrecisions[i]))
			if fallbackTFLOPS > 0 {
				tflops = float64(fallbackTFLOPS)
				break
			}
		}
	}

	memGB := 0.0
	if spec.Capabilities.MemoryGB != nil {
		memGB = float64(spec.Capabilities.MemoryGB.Value()) / (1024 * 1024 * 1024)
	}

	bandwidthGBps := 0.0
	if spec.Capabilities.MemoryBandwidthGBps != nil {
		bandwidthGBps = float64(spec.Capabilities.MemoryBandwidthGBps.Value())
	}

	// Normalize (assuming max values for rough normalization)
	// This should be tuned based on actual accelerator ranges
	normTFLOPS := tflops / 2000.0           // Normalize to ~2000 TFLOPS max
	normMemGB := memGB / 100.0              // Normalize to ~100GB max
	normBandwidth := bandwidthGBps / 3000.0 // Normalize to ~3000 GB/s max

	// Composite score: 50% memory, 30% bandwidth, 20% performance
	score := 0.5*normMemGB + 0.3*normBandwidth + 0.2*normTFLOPS

	return score
}

// getTFLOPSForPrecision extracts TFLOPS value for given precision
func getTFLOPSForPrecision(
	perf *v1beta1.AcceleratorPerformance,
	precision string,
) int64 {
	if perf == nil {
		return 0
	}

	switch strings.ToLower(precision) {
	case "fp32":
		if perf.Fp32Tflops != nil {
			return *perf.Fp32Tflops
		}
	case "fp16":
		if perf.Fp16Tflops != nil {
			return *perf.Fp16Tflops
		}
	case "int8", "fp8":
		if perf.Int8Tops != nil {
			return *perf.Int8Tops
		}
	case "int4":
		if perf.Int4Tops != nil {
			return *perf.Int4Tops
		}
	}

	return 0
}

// getMaxTFLOPS returns the maximum TFLOPS across all precisions
// Used when no precision preference is specified
func getMaxTFLOPS(perf *v1beta1.AcceleratorPerformance) int64 {
	if perf == nil {
		return 0
	}

	maxTFLOPS := int64(0)

	if perf.Fp32Tflops != nil && *perf.Fp32Tflops > maxTFLOPS {
		maxTFLOPS = *perf.Fp32Tflops
	}
	if perf.Fp16Tflops != nil && *perf.Fp16Tflops > maxTFLOPS {
		maxTFLOPS = *perf.Fp16Tflops
	}
	if perf.Int8Tops != nil && *perf.Int8Tops > maxTFLOPS {
		maxTFLOPS = *perf.Int8Tops
	}
	if perf.Int4Tops != nil && *perf.Int4Tops > maxTFLOPS {
		maxTFLOPS = *perf.Int4Tops
	}

	return maxTFLOPS
}

// sortScoredCandidates sorts candidates by score
// ascending=true sorts lowest to highest (for cost minimization)
// ascending=false sorts highest to lowest (for capability/fit maximization)
func sortScoredCandidates(scored []scoredCandidate, ascending bool) {
	sort.Slice(scored, func(i, j int) bool {
		if ascending {
			return scored[i].Score < scored[j].Score
		}
		return scored[i].Score > scored[j].Score
	})
}
