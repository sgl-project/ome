package acceleratorclassselector

import (
	"context"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// defaultSelector is the default implementation of the Selector interface.
type defaultSelector struct {
	config  *Config
	fetcher AcceleratorFetcher
}

// New creates a new Selector with default implementations.
func New(client client.Client) Selector {
	config := NewConfig(client)
	return NewWithConfig(config)
}

// NewWithConfig creates a new Selector with the provided configuration.
func NewWithConfig(config *Config) Selector {
	return &defaultSelector{
		config:  config,
		fetcher: NewDefaultAcceleratorFetcher(config.Client),
	}
}

// GetAcceleratorClass fetches a specific accelerator class by name.
func (s *defaultSelector) FetchAcceleratorClass(ctx context.Context, name string) (*v1beta1.AcceleratorClassSpec, bool, error) {
	return s.fetcher.GetAcceleratorClass(ctx, name)
}

// GetAcceleratorClass selects the best accelerator class for a given inference service, runtime, and component.
// This is a convenience function that implements the original simple selection logic.
// The selection follows this priority order:
//  1. If runtime doesn't contain AcceleratorRequirements, return nil
//  2. If runtime contains AcceleratorRequirements:
//     a. For engine component: check if engine has AcceleratorOverride with AcceleratorClass
//     b. For decoder component: check if decoder has AcceleratorOverride with AcceleratorClass
//     c. If component doesn't have AcceleratorOverride, check if InferenceService has AcceleratorSelector with AcceleratorClass
//     d. If InferenceService doesn't have AcceleratorClass in AccleratorSelecor, check if InferenceService has AcceleratorSelector with Policy
//     e. Otherwise, won't provide AcceleratorClass
func (s *defaultSelector) GetAcceleratorClass(ctx context.Context, isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec, component v1beta1.ComponentType) (*v1beta1.AcceleratorClassSpec, string, error) {
	logger := log.FromContext(ctx)
	logger.Info("Getting accelerator classes for inference service with runtime", "isvc", isvc.Name, "component", component, "runtime", runtime)
	acName := ""
	// 1. If runtime doesn't contain AcceleratorRequirements, return nil
	if runtime == nil || runtime.AcceleratorRequirements == nil || runtime.AcceleratorRequirements.AcceleratorClasses == nil {
		return nil, "", nil
	}

	if len(runtime.AcceleratorRequirements.AcceleratorClasses) > 0 {
		// get accelerator class by name if specified.
		if acceleratorClass := s.getAcceleratorClassByName(isvc, component); acceleratorClass != nil {
			acName = *acceleratorClass
		}
		// if accelerator class didn't have name specified, try to get accelerator class by policy
		// if policy is not specified, will skip getting acceleratorClass by policy
		if acName == "" {
			logger.Info("No acceleratorClass name found for component", "component", component, "inferenceService", isvc.Name)
			acceleratorPolicy := s.getAcceleratorPolicy(isvc, component)
			if acceleratorPolicy == "" {
				logger.Info("No acceleratorClass policy found for component", "component", component, "inferenceService", isvc.Name)
				return nil, "", nil
			}
			if acceleratorClass := s.getAcceleratorClassByPolicy(ctx, isvc, runtime, acceleratorPolicy); acceleratorClass != nil {
				acName = *acceleratorClass
			}
		}
	}

	// fetch and return the selected accelerator class spec
	if acName != "" {
		acSpec, _, err := s.fetcher.GetAcceleratorClass(ctx, acName)
		if err != nil {
			logger.Error(err, "Failed to fetch accelerator class", "name", acName)
			return nil, acName, err
		}
		return acSpec, acName, nil
	}
	return nil, "", nil
}

// getAcceleratorClassByName checks for component-specific AcceleratorOverride or InferenceService-level AcceleratorSelector
func (s *defaultSelector) getAcceleratorClassByName(isvc *v1beta1.InferenceService, component v1beta1.ComponentType) *string {
	// Use the component-specific AcceleratorOverride as the default.
	if acceleratorClass := s.getComponentAcceleratorOverride(isvc, component); acceleratorClass != nil {
		return acceleratorClass
	}

	// Check InferenceService-level AcceleratorSelector if specified
	if acceleratorClass := s.getInferenceServiceAcceleratorClass(isvc); acceleratorClass != nil {
		return acceleratorClass
	}

	return nil
}

// getAcceleratorClassByPolicy selects an accelerator class based on the specified policy
// It fetches candidates from runtime.AcceleratorClasses, filters by InferenceService constraints,
// and applies the policy-specific selection logic
func (s *defaultSelector) getAcceleratorClassByPolicy(ctx context.Context, isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec, acceleratorPolicy v1beta1.AcceleratorSelectionPolicy) *string {
	logger := log.FromContext(ctx)

	// Return nil if policy is empty
	if acceleratorPolicy == "" {
		return nil
	}

	// Fetch candidates from runtime.AcceleratorClasses (candidate pool)
	candidates, err := s.getCandidateAccelerators(ctx, runtime)
	if err != nil {
		logger.Error(err, "Failed to fetch candidate accelerators")
		return nil
	}

	if len(candidates) == 0 {
		logger.Info("No accelerator candidates available")
		return nil
	}

	// Get constraints from InferenceService
	var constraints *v1beta1.AcceleratorConstraints
	if isvc.Spec.AcceleratorSelector != nil {
		constraints = isvc.Spec.AcceleratorSelector.Constraints
	}

	// Filter candidates by InferenceService constraints
	validCandidates := filterCandidates(ctx, candidates, constraints, s.config.ConsiderAvailability)

	if len(validCandidates) == 0 {
		logger.Info("No candidates passed filtering", "policy", acceleratorPolicy)
		return nil
	}

	logger.Info("Candidates after filtering", "count", len(validCandidates), "policy", acceleratorPolicy)

	// Select by policy
	switch acceleratorPolicy {
	case v1beta1.BestFitPolicy:
		return s.selectBestFit(ctx, validCandidates, constraints)

	case v1beta1.CheapestPolicy:
		return s.selectCheapest(ctx, validCandidates)

	case v1beta1.MostCapablePolicy:
		// Get preferred precisions for capability scoring
		preferredPrecisions := []string{}
		if constraints != nil && len(constraints.PreferredPrecisions) > 0 {
			preferredPrecisions = constraints.PreferredPrecisions
		}
		return s.selectMostCapable(ctx, validCandidates, preferredPrecisions)

	case v1beta1.FirstAvailablePolicy:
		// Return the first AcceleratorClass from runtime requirements
		if len(runtime.AcceleratorRequirements.AcceleratorClasses) > 0 {
			firstAvailable := runtime.AcceleratorRequirements.AcceleratorClasses[0]
			logger.Info("FirstAvailable selected", "name", firstAvailable)
			return &firstAvailable
		}
	}

	return nil
}

// getComponentAcceleratorOverride checks if the specified component has an AcceleratorOverride with AcceleratorClass
func (s *defaultSelector) getComponentAcceleratorOverride(isvc *v1beta1.InferenceService, component v1beta1.ComponentType) *string {
	if isvc == nil {
		return nil
	}

	switch component {
	case v1beta1.EngineComponent:
		if isvc.Spec.Engine != nil &&
			isvc.Spec.Engine.AcceleratorOverride != nil &&
			isvc.Spec.Engine.AcceleratorOverride.AcceleratorClass != nil {
			return isvc.Spec.Engine.AcceleratorOverride.AcceleratorClass
		}
	case v1beta1.DecoderComponent:
		if isvc.Spec.Decoder != nil &&
			isvc.Spec.Decoder.AcceleratorOverride != nil &&
			isvc.Spec.Decoder.AcceleratorOverride.AcceleratorClass != nil {
			return isvc.Spec.Decoder.AcceleratorOverride.AcceleratorClass
		}
	}

	return nil
}

// getInferenceServiceAcceleratorClass checks if the InferenceService has an AcceleratorSelector with AcceleratorClass
func (s *defaultSelector) getInferenceServiceAcceleratorClass(isvc *v1beta1.InferenceService) *string {
	if isvc == nil ||
		isvc.Spec.AcceleratorSelector == nil ||
		isvc.Spec.AcceleratorSelector.AcceleratorClass == nil {
		return nil
	}

	return isvc.Spec.AcceleratorSelector.AcceleratorClass
}

// getComponentAcceleratorPolicy checks if the specified component has an AcceleratorOverride with Policy
func (s *defaultSelector) getComponentAcceleratorPolicy(isvc *v1beta1.InferenceService, component v1beta1.ComponentType) v1beta1.AcceleratorSelectionPolicy {
	if isvc == nil {
		return ""
	}

	// Check component-specific AcceleratorOverride by first
	switch component {
	case v1beta1.EngineComponent:
		if isvc.Spec.Engine != nil &&
			isvc.Spec.Engine.AcceleratorOverride != nil &&
			isvc.Spec.Engine.AcceleratorOverride.Policy != "" {
			return isvc.Spec.Engine.AcceleratorOverride.Policy
		}
	case v1beta1.DecoderComponent:
		if isvc.Spec.Decoder != nil &&
			isvc.Spec.Decoder.AcceleratorOverride != nil &&
			isvc.Spec.Decoder.AcceleratorOverride.Policy != "" {
			return isvc.Spec.Decoder.AcceleratorOverride.Policy
		}
	}

	// if component-specific AcceleratorOverride not found, check InferenceService-level AcceleratorSelector
	if isvc.Spec.AcceleratorSelector != nil &&
		isvc.Spec.AcceleratorSelector.Policy != "" {
		return isvc.Spec.AcceleratorSelector.Policy
	}

	return ""
}

// getAcceleratorPolicy determines the effective AcceleratorSelectionPolicy for the given component and InferenceService
// If no policy is specified, an empty string is returned
func (s *defaultSelector) getAcceleratorPolicy(isvc *v1beta1.InferenceService, component v1beta1.ComponentType) v1beta1.AcceleratorSelectionPolicy {
	// Check component-specific AcceleratorOverride
	return s.getComponentAcceleratorPolicy(isvc, component)
}

// getCandidateAccelerators fetches AcceleratorClass candidates from runtime.AcceleratorClasses
func (s *defaultSelector) getCandidateAccelerators(ctx context.Context, runtime *v1beta1.ServingRuntimeSpec) ([]candidateAccelerator, error) {
	logger := log.FromContext(ctx)

	if runtime == nil || runtime.AcceleratorRequirements == nil || len(runtime.AcceleratorRequirements.AcceleratorClasses) == 0 {
		logger.V(1).Info("No accelerator classes in runtime requirements")
		return nil, nil
	}

	// Fetch all candidates from the runtime's accelerator class list
	candidates, err := candidatesFromNames(ctx, s.fetcher, runtime.AcceleratorRequirements.AcceleratorClasses)
	if err != nil {
		return nil, err
	}

	logger.Info("Fetched candidate accelerators", "count", len(candidates))
	return candidates, nil
}

// selectBestFit selects the accelerator with the best fit score (50% memory, 50% compute performance)
func (s *defaultSelector) selectBestFit(ctx context.Context, validCandidates []candidateAccelerator, constraints *v1beta1.AcceleratorConstraints) *string {
	logger := log.FromContext(ctx)

	if len(validCandidates) == 0 {
		logger.Info("No valid candidates for BestFit selection")
		return nil
	}

	// If only one candidate, return it immediately
	if len(validCandidates) == 1 {
		logger.Info("Single candidate after filtering", "name", validCandidates[0].Name)
		return &validCandidates[0].Name
	}

	// Score all candidates
	scored := make([]scoredCandidate, 0, len(validCandidates))
	for _, candidate := range validCandidates {
		score := calculateBestFitScore(candidate, constraints)
		reason := "BestFit scoring"
		scored = append(scored, scoredCandidate{
			candidateAccelerator: candidate,
			Score:                score,
			Reason:               reason,
		})
		logger.V(1).Info("Scored candidate", "name", candidate.Name, "score", score)
	}

	// Sort by score descending
	sortScoredCandidates(scored, false)

	// Return highest scoring candidate
	selected := scored[0].Name
	logger.Info("BestFit selected", "name", selected, "score", scored[0].Score)
	return &selected
}

// selectCheapest selects the lowest cost accelerator that meets requirements
func (s *defaultSelector) selectCheapest(ctx context.Context, validCandidates []candidateAccelerator) *string {
	logger := log.FromContext(ctx)

	if len(validCandidates) == 0 {
		logger.Info("No valid candidates for Cheapest selection")
		return nil
	}

	// If only one candidate, return it immediately
	if len(validCandidates) == 1 {
		logger.Info("Single candidate after filtering", "name", validCandidates[0].Name)
		return &validCandidates[0].Name
	}

	// Extract costs and filter candidates with cost data
	type candidateWithCost struct {
		candidate candidateAccelerator
		cost      *v1beta1.AcceleratorCost
		costValue string
	}

	candidatesWithCost := make([]candidateWithCost, 0, len(validCandidates))
	for _, candidate := range validCandidates {
		cost, costType, err := getCandidateCost(candidate)
		if err != nil {
			logger.V(1).Info("Skipping candidate without cost data", "name", candidate.Name, "error", err.Error())
			continue
		}
		candidatesWithCost = append(candidatesWithCost, candidateWithCost{
			candidate: candidate,
			cost:      candidate.Spec.Cost,
			costValue: costType,
		})
		logger.V(1).Info("Candidate cost", "name", candidate.Name, "costType", costType, "cost", cost.String())
	}

	if len(candidatesWithCost) == 0 {
		logger.Error(nil, "No cost data available for any candidate")
		return nil
	}

	// Sort by cost ascending
	cheapest := candidatesWithCost[0]
	for i := 1; i < len(candidatesWithCost); i++ {
		candidate := candidatesWithCost[i]
		currentCost, _, _ := getCandidateCost(cheapest.candidate)
		candidateCost, _, _ := getCandidateCost(candidate.candidate)
		if compareCosts(candidateCost, currentCost) < 0 {
			cheapest = candidate
		}
	}

	logger.Info("Cheapest selected", "name", cheapest.candidate.Name, "costType", cheapest.costValue)
	return &cheapest.candidate.Name
}

// selectMostCapable selects the most powerful accelerator based on performance metrics
func (s *defaultSelector) selectMostCapable(ctx context.Context, validCandidates []candidateAccelerator, preferredPrecisions []string) *string {
	logger := log.FromContext(ctx)

	if len(validCandidates) == 0 {
		logger.Info("No valid candidates for MostCapable selection")
		return nil
	}

	// If only one candidate, return it immediately
	if len(validCandidates) == 1 {
		logger.Info("Single candidate after filtering", "name", validCandidates[0].Name)
		return &validCandidates[0].Name
	}

	// Score all candidates by capability
	scored := make([]scoredCandidate, 0, len(validCandidates))
	for _, candidate := range validCandidates {
		score := calculateCapabilityScore(candidate, preferredPrecisions)
		reason := "MostCapable scoring"
		scored = append(scored, scoredCandidate{
			candidateAccelerator: candidate,
			Score:                score,
			Reason:               reason,
		})
		logger.V(1).Info("Scored candidate", "name", candidate.Name, "score", score)
	}

	// Sort by score descending
	sortScoredCandidates(scored, false)

	// Return highest scoring candidate
	selected := scored[0].Name
	logger.Info("MostCapable selected", "name", selected, "score", scored[0].Score)
	return &selected
}
