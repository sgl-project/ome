package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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
	Runtime        *unstructured.Unstructured `json:"runtime"`
	Score          int                        `json:"score"`
	CompatibleWith []string                   `json:"compatibleWith"`
	Reasons        []string                   `json:"reasons"`
	Warnings       []string                   `json:"warnings,omitempty"`
	Recommendation string                     `json:"recommendation"`
	Signals        RuntimeSignals             `json:"signals,omitempty"`
}

// RuntimeSignals contains the key decision signals behind a runtime recommendation.
type RuntimeSignals struct {
	MatchedFormat       string   `json:"matchedFormat,omitempty"`
	MatchedFramework    string   `json:"matchedFramework,omitempty"`
	MatchedArchitecture string   `json:"matchedArchitecture,omitempty"`
	ModelSizeRange      string   `json:"modelSizeRange,omitempty"`
	RuntimeFamily       string   `json:"runtimeFamily,omitempty"`
	Protocols           []string `json:"protocols,omitempty"`
	AutoSelectEnabled   bool     `json:"autoSelectEnabled"`
}

// CompatibilityCheck represents the result of a compatibility check
type CompatibilityCheck struct {
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons"`
	Warnings   []string `json:"warnings,omitempty"`
	Score      int      `json:"score"`
}

// ModelProfile contains the model attributes used to evaluate runtime compatibility.
type ModelProfile struct {
	Format       string
	Framework    string
	Architecture string
	ParameterSize string
}

type formatEvaluation struct {
	score          int
	compatibleWith []string
	reasons        []string
	warnings       []string
	signals        RuntimeSignals
}

// FindCompatibleRuntimes finds all runtimes compatible with a given model
func (s *RuntimeIntelligenceService) FindCompatibleRuntimes(ctx context.Context, profile ModelProfile) ([]RuntimeMatch, error) {
	// Get all cluster-scoped runtimes
	runtimes, err := s.k8sClient.ListClusterServingRuntimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtimes: %w", err)
	}

	var matches []RuntimeMatch
	for _, runtime := range runtimes.Items {
		match := s.evaluateRuntimeCompatibility(&runtime, profile)
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
func (s *RuntimeIntelligenceService) CheckCompatibility(ctx context.Context, runtimeName string, profile ModelProfile) (*CompatibilityCheck, error) {
	runtime, err := s.k8sClient.GetClusterServingRuntime(ctx, runtimeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	match := s.evaluateRuntimeCompatibility(runtime, profile)

	return &CompatibilityCheck{
		Compatible: match.Score > 0,
		Reasons:    match.Reasons,
		Warnings:   match.Warnings,
		Score:      match.Score,
	}, nil
}

// GetRecommendation gets the best runtime recommendation for a model
func (s *RuntimeIntelligenceService) GetRecommendation(ctx context.Context, profile ModelProfile) (*RuntimeMatch, error) {
	matches, err := s.FindCompatibleRuntimes(ctx, profile)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no compatible runtimes found for format=%s, framework=%s, architecture=%s, size=%s",
			profile.Format, profile.Framework, profile.Architecture, profile.ParameterSize)
	}

	// Return the highest scored match
	best := matches[0]
	best.Recommendation = buildRecommendation(best)
	return &best, nil
}

// evaluateRuntimeCompatibility evaluates how well a runtime matches a model
func (s *RuntimeIntelligenceService) evaluateRuntimeCompatibility(runtime *unstructured.Unstructured, profile ModelProfile) RuntimeMatch {
	match := RuntimeMatch{
		Runtime:        runtime,
		Score:          0,
		CompatibleWith: []string{},
		Reasons:        []string{},
		Warnings:       []string{},
		Signals:        RuntimeSignals{},
	}

	// Extract runtime spec
	spec, found, err := unstructured.NestedMap(runtime.Object, "spec")
	if !found || err != nil {
		match.Warnings = append(match.Warnings, "Runtime spec not found or invalid")
		match.Recommendation = "Runtime spec is incomplete"
		return match
	}

	// Check if runtime is disabled
	disabled, found, err := unstructured.NestedBool(spec, "disabled")
	if found && err == nil && disabled {
		match.Score = 0
		match.Warnings = append(match.Warnings, "Runtime is disabled")
		match.Recommendation = "Disabled runtimes cannot be selected"
		return match
	}

	match.Signals.Protocols = extractProtocolVersions(spec)
	match.Signals.RuntimeFamily = detectRuntimeFamily(spec)

	bestEval, evalFound := s.evaluateSupportedFormats(spec, profile)
	if evalFound {
		match.Score += bestEval.score
		match.CompatibleWith = append(match.CompatibleWith, bestEval.compatibleWith...)
		match.Reasons = append(match.Reasons, bestEval.reasons...)
		match.Warnings = append(match.Warnings, bestEval.warnings...)
		match.Signals = mergeSignals(match.Signals, bestEval.signals)
	} else {
		match.Warnings = append(match.Warnings, "No supported model formats specified")
	}

	if match.Score == 0 {
		match.Recommendation = buildRecommendation(match)
		return match
	}

	if profile.ParameterSize != "" {
		rangeLabel, sizeMatched := matchModelSize(spec, profile.ParameterSize)
		if rangeLabel != "" {
			match.Signals.ModelSizeRange = rangeLabel
			if sizeMatched {
				match.Score += 10
				match.Reasons = append(match.Reasons, fmt.Sprintf("Model size %s fits runtime range %s", profile.ParameterSize, rangeLabel))
			} else {
				match.Score = 0
				match.Warnings = append(match.Warnings, fmt.Sprintf("Model size %s is outside runtime range %s", profile.ParameterSize, rangeLabel))
				match.Recommendation = buildRecommendation(match)
				return match
			}
		}
	}

	// Check if runtime supports multi-model (bonus points)
	multiModel, found, err := unstructured.NestedBool(spec, "multiModel")
	if found && err == nil && multiModel {
		match.Score += 5
		match.Reasons = append(match.Reasons, "Supports multi-model serving")
	}

	if hasHTTPProtocol(match.Signals.Protocols) {
		match.Score += 5
		match.Reasons = append(match.Reasons, "Supports OpenAI or HTTP-based serving protocols")
	}

	if match.Signals.RuntimeFamily != "" {
		match.Reasons = append(match.Reasons, fmt.Sprintf("Uses %s serving stack", strings.ToUpper(match.Signals.RuntimeFamily)))
	}

	match.Recommendation = buildRecommendation(match)
	return match
}

func (s *RuntimeIntelligenceService) evaluateSupportedFormats(spec map[string]interface{}, profile ModelProfile) (formatEvaluation, bool) {
	supportedFormats, found, err := unstructured.NestedSlice(spec, "supportedModelFormats")
	if !found || err != nil || len(supportedFormats) == 0 {
		return formatEvaluation{}, false
	}

	var best formatEvaluation
	var bestFound bool
	for _, format := range supportedFormats {
		formatMap, ok := format.(map[string]interface{})
		if !ok {
			continue
		}

		eval := evaluateSupportedFormat(formatMap, profile)
		if !bestFound || eval.score > best.score {
			best = eval
			bestFound = true
		}
	}

	return best, bestFound
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

func evaluateSupportedFormat(format map[string]interface{}, profile ModelProfile) formatEvaluation {
	eval := formatEvaluation{
		compatibleWith: []string{},
		reasons:        []string{},
		warnings:       []string{},
		signals:        RuntimeSignals{},
	}

	modelFormat, _, _ := unstructured.NestedMap(format, "modelFormat")
	modelFramework, _, _ := unstructured.NestedMap(format, "modelFramework")

	formatName, _, _ := unstructured.NestedString(modelFormat, "name")
	formatVersion, _, _ := unstructured.NestedString(modelFormat, "version")
	frameworkName, _, _ := unstructured.NestedString(modelFramework, "name")
	frameworkVersion, _, _ := unstructured.NestedString(modelFramework, "version")
	architecture, _, _ := unstructured.NestedString(format, "modelArchitecture")
	autoSelect, autoSelectFound, _ := unstructured.NestedBool(format, "autoSelect")

	if !matchesFormat(profile.Format, formatName, formatVersion) {
		eval.warnings = append(eval.warnings, fmt.Sprintf("Supports %s, not %s", formatLabel(formatName, formatVersion), profile.Format))
		return eval
	}

	eval.score += 50
	eval.compatibleWith = append(eval.compatibleWith, formatLabel(formatName, formatVersion))
	eval.reasons = append(eval.reasons, fmt.Sprintf("Supports model format %s", formatLabel(formatName, formatVersion)))
	eval.signals.MatchedFormat = formatLabel(formatName, formatVersion)

	if profile.Framework != "" && frameworkName != "" {
		if strings.EqualFold(profile.Framework, frameworkName) {
			eval.score += 20
			eval.reasons = append(eval.reasons, fmt.Sprintf("Matches framework %s", formatLabel(frameworkName, frameworkVersion)))
			eval.signals.MatchedFramework = formatLabel(frameworkName, frameworkVersion)
		} else {
			eval.warnings = append(eval.warnings, fmt.Sprintf("Framework mismatch: runtime expects %s", formatLabel(frameworkName, frameworkVersion)))
			eval.score = 0
			return eval
		}
	}

	if profile.Architecture != "" && architecture != "" {
		if strings.EqualFold(profile.Architecture, architecture) {
			eval.score += 20
			eval.reasons = append(eval.reasons, fmt.Sprintf("Optimized for architecture %s", architecture))
			eval.signals.MatchedArchitecture = architecture
		} else {
			eval.warnings = append(eval.warnings, fmt.Sprintf("Architecture mismatch: runtime targets %s", architecture))
			eval.score = 0
			return eval
		}
	}

	if autoSelectFound {
		eval.signals.AutoSelectEnabled = autoSelect
		if autoSelect {
			eval.score += 10
			eval.reasons = append(eval.reasons, "Auto-select is enabled for this runtime format")
		} else {
			eval.warnings = append(eval.warnings, "Manual selection required because auto-select is disabled")
		}
	}

	return eval
}

func extractProtocolVersions(spec map[string]interface{}) []string {
	var protocols []string
	rawProtocols, found, err := unstructured.NestedSlice(spec, "protocolVersions")
	if !found || err != nil {
		return protocols
	}

	for _, protocol := range rawProtocols {
		switch value := protocol.(type) {
		case string:
			protocols = append(protocols, value)
		case map[string]interface{}:
			name, ok := value["name"].(string)
			if ok && name != "" {
				protocols = append(protocols, name)
			}
		}
	}

	return protocols
}

func detectRuntimeFamily(spec map[string]interface{}) string {
	image, _, _ := unstructured.NestedString(spec, "engineConfig", "runner", "image")
	imageLower := strings.ToLower(image)

	switch {
	case strings.Contains(imageLower, "sglang"):
		return "sglang"
	case strings.Contains(imageLower, "vllm"):
		return "vllm"
	case strings.Contains(imageLower, "triton"):
		return "triton"
	default:
		return ""
	}
}

func matchModelSize(spec map[string]interface{}, modelSize string) (string, bool) {
	modelSizeRange, found, err := unstructured.NestedMap(spec, "modelSizeRange")
	if !found || err != nil {
		return "", true
	}

	minValue, _, _ := unstructured.NestedString(modelSizeRange, "min")
	maxValue, _, _ := unstructured.NestedString(modelSizeRange, "max")
	if minValue == "" || maxValue == "" {
		return "", true
	}

	modelNumeric, ok := parseModelSize(modelSize)
	if !ok {
		return fmt.Sprintf("%s-%s", minValue, maxValue), true
	}
	minNumeric, minOK := parseModelSize(minValue)
	maxNumeric, maxOK := parseModelSize(maxValue)
	if !minOK || !maxOK {
		return fmt.Sprintf("%s-%s", minValue, maxValue), true
	}

	rangeLabel := fmt.Sprintf("%s-%s", minValue, maxValue)
	return rangeLabel, modelNumeric >= minNumeric && modelNumeric <= maxNumeric
}

func parseModelSize(size string) (float64, bool) {
	value := strings.TrimSpace(strings.ToUpper(size))
	if value == "" {
		return 0, false
	}

	multiplier := 1.0
	lastChar := value[len(value)-1]
	switch lastChar {
	case 'K':
		multiplier = 1e3
		value = value[:len(value)-1]
	case 'M':
		multiplier = 1e6
		value = value[:len(value)-1]
	case 'B':
		multiplier = 1e9
		value = value[:len(value)-1]
	case 'T':
		multiplier = 1e12
		value = value[:len(value)-1]
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	return parsed * multiplier, true
}

func formatLabel(name, version string) string {
	switch {
	case name == "" && version == "":
		return ""
	case version == "":
		return name
	case name == "":
		return version
	default:
		return fmt.Sprintf("%s %s", name, version)
	}
}

func hasHTTPProtocol(protocols []string) bool {
	for _, protocol := range protocols {
		protocolLower := strings.ToLower(protocol)
		if strings.Contains(protocolLower, "http") || strings.Contains(protocolLower, "rest") || strings.Contains(protocolLower, "openai") {
			return true
		}
	}
	return false
}

func mergeSignals(base RuntimeSignals, update RuntimeSignals) RuntimeSignals {
	if update.MatchedFormat != "" {
		base.MatchedFormat = update.MatchedFormat
	}
	if update.MatchedFramework != "" {
		base.MatchedFramework = update.MatchedFramework
	}
	if update.MatchedArchitecture != "" {
		base.MatchedArchitecture = update.MatchedArchitecture
	}
	if update.ModelSizeRange != "" {
		base.ModelSizeRange = update.ModelSizeRange
	}
	if update.RuntimeFamily != "" {
		base.RuntimeFamily = update.RuntimeFamily
	}
	if len(update.Protocols) > 0 {
		base.Protocols = update.Protocols
	}
	base.AutoSelectEnabled = update.AutoSelectEnabled
	return base
}

func buildRecommendation(match RuntimeMatch) string {
	if match.Score == 0 {
		if len(match.Warnings) > 0 {
			return match.Warnings[0]
		}
		return "Runtime does not satisfy the selected model requirements"
	}

	parts := []string{}
	if match.Signals.MatchedFormat != "" {
		parts = append(parts, fmt.Sprintf("Best fit for %s models", match.Signals.MatchedFormat))
	}
	if match.Signals.MatchedFramework != "" {
		parts = append(parts, fmt.Sprintf("using %s", match.Signals.MatchedFramework))
	}
	if match.Signals.MatchedArchitecture != "" {
		parts = append(parts, fmt.Sprintf("optimized for %s", match.Signals.MatchedArchitecture))
	}
	if match.Signals.ModelSizeRange != "" {
		parts = append(parts, fmt.Sprintf("within size range %s", match.Signals.ModelSizeRange))
	}
	if match.Signals.RuntimeFamily != "" {
		parts = append(parts, fmt.Sprintf("on %s", strings.ToUpper(match.Signals.RuntimeFamily)))
	}

	if len(parts) == 0 {
		return "Best match based on compatibility signals"
	}

	return strings.Join(parts, ", ")
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
