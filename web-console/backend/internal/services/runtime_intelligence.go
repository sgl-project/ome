package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RuntimeIntelligenceService provides intelligent runtime matching and recommendations
type RuntimeIntelligenceService struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewRuntimeIntelligenceService creates a new runtime intelligence service
func NewRuntimeIntelligenceService(k8sClient *k8s.Client, logger *zap.Logger) *RuntimeIntelligenceService {
	return &RuntimeIntelligenceService{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// RuntimeMatch represents a runtime that matches a model with a compatibility score
type RuntimeMatch struct {
	Runtime         *unstructured.Unstructured `json:"runtime"`
	Score           int                        `json:"score"`
	CompatibleWith  []string                   `json:"compatibleWith"`
	Reasons         []string                   `json:"reasons"`
	Warnings        []string                   `json:"warnings,omitempty"`
	Recommendation  string                     `json:"recommendation"`
}

// CompatibilityCheck represents the result of a compatibility check
type CompatibilityCheck struct {
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons"`
	Warnings   []string `json:"warnings,omitempty"`
	Score      int      `json:"score"`
}

// FindCompatibleRuntimes finds all runtimes compatible with a given model
func (s *RuntimeIntelligenceService) FindCompatibleRuntimes(ctx context.Context, modelFormat string, modelFramework string) ([]RuntimeMatch, error) {
	// Get all cluster-scoped runtimes
	runtimes, err := s.k8sClient.ListClusterServingRuntimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtimes: %w", err)
	}

	var matches []RuntimeMatch
	for _, runtime := range runtimes.Items {
		match := s.evaluateRuntimeCompatibility(&runtime, modelFormat, modelFramework)
		if match.Score > 0 {
			matches = append(matches, match)
		}
	}

	// Sort by score (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches, nil
}

// CheckCompatibility checks if a specific runtime is compatible with a model
func (s *RuntimeIntelligenceService) CheckCompatibility(ctx context.Context, runtimeName string, modelFormat string, modelFramework string) (*CompatibilityCheck, error) {
	runtime, err := s.k8sClient.GetClusterServingRuntime(ctx, runtimeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	match := s.evaluateRuntimeCompatibility(runtime, modelFormat, modelFramework)

	return &CompatibilityCheck{
		Compatible: match.Score > 0,
		Reasons:    match.Reasons,
		Warnings:   match.Warnings,
		Score:      match.Score,
	}, nil
}

// GetRecommendation gets the best runtime recommendation for a model
func (s *RuntimeIntelligenceService) GetRecommendation(ctx context.Context, modelFormat string, modelFramework string) (*RuntimeMatch, error) {
	matches, err := s.FindCompatibleRuntimes(ctx, modelFormat, modelFramework)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no compatible runtimes found for format=%s, framework=%s", modelFormat, modelFramework)
	}

	// Return the highest scored match
	best := matches[0]
	best.Recommendation = "Best match based on compatibility score"
	return &best, nil
}

// evaluateRuntimeCompatibility evaluates how well a runtime matches a model
func (s *RuntimeIntelligenceService) evaluateRuntimeCompatibility(runtime *unstructured.Unstructured, modelFormat string, modelFramework string) RuntimeMatch {
	match := RuntimeMatch{
		Runtime:        runtime,
		Score:          0,
		CompatibleWith: []string{},
		Reasons:        []string{},
		Warnings:       []string{},
	}

	// Extract runtime spec
	spec, found, err := unstructured.NestedMap(runtime.Object, "spec")
	if !found || err != nil {
		match.Warnings = append(match.Warnings, "Runtime spec not found or invalid")
		return match
	}

	// Check supported model formats
	supportedFormats, found, err := unstructured.NestedSlice(spec, "supportedModelFormats")
	if !found || err != nil {
		match.Warnings = append(match.Warnings, "No supported model formats specified")
	} else {
		formatMatched := false
		for _, format := range supportedFormats {
			formatMap, ok := format.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := formatMap["name"].(string)
			version, _ := formatMap["version"].(string)

			// Check if format matches
			if matchesFormat(modelFormat, name, version) {
				formatMatched = true
				match.Score += 50 // High score for format match
				match.CompatibleWith = append(match.CompatibleWith, fmt.Sprintf("%s:%s", name, version))
				match.Reasons = append(match.Reasons, fmt.Sprintf("Supports model format %s", name))
				break
			}
		}

		if !formatMatched {
			match.Warnings = append(match.Warnings, fmt.Sprintf("Model format %s may not be supported", modelFormat))
		}
	}

	// Check if runtime supports multi-model (bonus points)
	multiModel, found, err := unstructured.NestedBool(spec, "multiModel")
	if found && err == nil && multiModel {
		match.Score += 10
		match.Reasons = append(match.Reasons, "Supports multi-model serving")
	}

	// Check if runtime is disabled
	disabled, found, err := unstructured.NestedBool(spec, "disabled")
	if found && err == nil && disabled {
		match.Score = 0
		match.Warnings = append(match.Warnings, "Runtime is disabled")
		return match
	}

	// Check protocol versions (bonus for HTTP/REST support)
	protocolVersions, found, err := unstructured.NestedSlice(spec, "protocolVersions")
	if found && err == nil {
		for _, pv := range protocolVersions {
			pvMap, ok := pv.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := pvMap["name"].(string)
			if strings.Contains(strings.ToLower(name), "http") || strings.Contains(strings.ToLower(name), "rest") {
				match.Score += 5
				match.Reasons = append(match.Reasons, "Supports HTTP/REST protocol")
				break
			}
		}
	}

	// Framework compatibility (if specified)
	if modelFramework != "" {
		// Check built-in adapters or container configurations for framework hints
		containers, found, err := unstructured.NestedSlice(spec, "containers")
		if found && err == nil {
			for _, container := range containers {
				containerMap, ok := container.(map[string]interface{})
				if !ok {
					continue
				}

				image, _ := containerMap["image"].(string)
				if matchesFramework(modelFramework, image) {
					match.Score += 20
					match.Reasons = append(match.Reasons, fmt.Sprintf("Container image supports %s framework", modelFramework))
					break
				}
			}
		}
	}

	return match
}

// matchesFormat checks if a model format matches a runtime's supported format
func matchesFormat(modelFormat, runtimeFormat, runtimeVersion string) bool {
	modelFormatLower := strings.ToLower(modelFormat)
	runtimeFormatLower := strings.ToLower(runtimeFormat)

	// Exact match
	if modelFormatLower == runtimeFormatLower {
		return true
	}

	// Handle common format aliases
	formatAliases := map[string][]string{
		"pytorch":     {"torch", "pt", "pth"},
		"tensorflow":  {"tf", "savedmodel"},
		"onnx":        {"onnx"},
		"safetensors": {"safetensor", "st"},
	}

	for canonical, aliases := range formatAliases {
		if modelFormatLower == canonical || contains(aliases, modelFormatLower) {
			if runtimeFormatLower == canonical || contains(aliases, runtimeFormatLower) {
				return true
			}
		}
	}

	return false
}

// matchesFramework checks if a framework is supported by a container image
func matchesFramework(framework, containerImage string) bool {
	frameworkLower := strings.ToLower(framework)
	imageLower := strings.ToLower(containerImage)

	// Check if framework name appears in image
	return strings.Contains(imageLower, frameworkLower)
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// ValidateRuntimeConfiguration validates a runtime configuration before creation
func (s *RuntimeIntelligenceService) ValidateRuntimeConfiguration(ctx context.Context, runtime *unstructured.Unstructured) ([]string, []string, error) {
	var errors []string
	var warnings []string

	// Extract spec
	spec, found, err := unstructured.NestedMap(runtime.Object, "spec")
	if !found || err != nil {
		errors = append(errors, "Runtime spec is required")
		return errors, warnings, nil
	}

	// Validate supported model formats
	supportedFormats, found, err := unstructured.NestedSlice(spec, "supportedModelFormats")
	if !found || err != nil {
		warnings = append(warnings, "No supported model formats specified")
	} else if len(supportedFormats) == 0 {
		warnings = append(warnings, "Supported model formats list is empty")
	}

	// Validate containers
	containers, found, err := unstructured.NestedSlice(spec, "containers")
	if !found || err != nil {
		errors = append(errors, "At least one container is required")
	} else if len(containers) == 0 {
		errors = append(errors, "Containers list cannot be empty")
	} else {
		// Validate each container
		for i, container := range containers {
			containerMap, ok := container.(map[string]interface{})
			if !ok {
				errors = append(errors, fmt.Sprintf("Container %d is invalid", i))
				continue
			}

			// Check required fields
			if _, found := containerMap["name"]; !found {
				errors = append(errors, fmt.Sprintf("Container %d is missing 'name' field", i))
			}
			if _, found := containerMap["image"]; !found {
				errors = append(errors, fmt.Sprintf("Container %d is missing 'image' field", i))
			}
		}
	}

	// Validate protocol versions
	protocolVersions, found, err := unstructured.NestedSlice(spec, "protocolVersions")
	if !found || err != nil {
		warnings = append(warnings, "No protocol versions specified")
	} else if len(protocolVersions) == 0 {
		warnings = append(warnings, "Protocol versions list is empty")
	}

	return errors, warnings, nil
}
