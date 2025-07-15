package utils

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	goerrors "github.com/pkg/errors"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	modelVer "github.com/sgl-project/ome/pkg/modelver"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// stringSet is a helper type that implements a set-like behavior for strings
// using a map with empty struct values for efficient membership testing
type stringSet map[string]struct{}

// add adds a string to the set
func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

// contains checks if a string exists in the set
func (ss stringSet) contains(s string) bool {
	_, found := ss[s]
	return found
}

// GetBaseModel retrieves a BaseModel or ClusterBaseModel by name.
// It first tries to find a namespace-scoped BaseModel, then falls back to a cluster-scoped ClusterBaseModel.
// Returns the model spec, metadata, and any error encountered.
func GetBaseModel(cl client.Client, name string, namespace string) (*v1beta1.BaseModelSpec, *metav1.ObjectMeta, error) {
	baseModel := &v1beta1.BaseModel{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, baseModel)
	if err == nil {
		return &baseModel.Spec, &baseModel.ObjectMeta, nil
	} else if !errors.IsNotFound(err) {
		return nil, nil, err
	}
	clusterBaseModel := &v1beta1.ClusterBaseModel{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterBaseModel)
	if err == nil {
		return &clusterBaseModel.Spec, &clusterBaseModel.ObjectMeta, nil
	} else if !errors.IsNotFound(err) {
		return nil, nil, err
	}
	return nil, nil, goerrors.New("No BaseModel or ClusterBaseModel with the name: " + name)
}

// generateLabel creates a standardized label string for model formats.
// The label includes:
// - Model format name and version
// - Model architecture
// - Quantization type
// - Model framework name and version
func generateLabel(mt *v1beta1.ModelFormat,
	modelArchitecture *string,
	quantization *v1beta1.ModelQuantization,
	modelFramework *v1beta1.ModelFrameworkSpec) string {

	label := "mt"
	if mt != nil {
		label += ":" + mt.Name
		if mt.Version != nil {
			label += ":" + *mt.Version
		}
	}
	if modelArchitecture != nil {
		label += ":" + *modelArchitecture
	}
	if quantization != nil {
		label += ":" + string(*quantization)
	}
	if modelFramework != nil {
		label += ":" + modelFramework.Name
		if modelFramework.Version != nil {
			label += ":" + *modelFramework.Version
		}
	}
	return label
}

// getModelFormatLabel generates a label for a base model spec
func getModelFormatLabel(modelSpec *v1beta1.BaseModelSpec) string {
	return generateLabel(
		&modelSpec.ModelFormat,
		modelSpec.ModelArchitecture,
		modelSpec.Quantization,
		modelSpec.ModelFramework,
	)
}

// sortServingRuntimeList sorts a list of ServingRuntimes by creation timestamp (desc) and name (asc)
func sortServingRuntimeList(runtimes *v1beta1.ServingRuntimeList) {
	sort.Slice(runtimes.Items, func(i, j int) bool {
		if runtimes.Items[i].CreationTimestamp.Before(&runtimes.Items[j].CreationTimestamp) {
			return false
		}
		if runtimes.Items[j].CreationTimestamp.Before(&runtimes.Items[i].CreationTimestamp) {
			return true
		}
		return runtimes.Items[i].Name < runtimes.Items[j].Name
	})
}

// sortClusterServingRuntimeList sorts a list of ClusterServingRuntimes by creation timestamp (desc) and name (asc)
func sortClusterServingRuntimeList(runtimes *v1beta1.ClusterServingRuntimeList) {
	sort.Slice(runtimes.Items, func(i, j int) bool {
		if runtimes.Items[i].CreationTimestamp.Before(&runtimes.Items[j].CreationTimestamp) {
			return false
		}
		if runtimes.Items[j].CreationTimestamp.Before(&runtimes.Items[i].CreationTimestamp) {
			return true
		}
		return runtimes.Items[i].Name < runtimes.Items[j].Name
	})
}

// sortSupportedRuntime sorts runtimes by their modelFormat and modelFramework weighted score (priority * weight)
// The sorting considers:
// 1. ModelFormat and ModelFramework weighted score (priority * weight)
// 2. Model size range compatibility
// Returns true if any runtime has a score > 0 (indicating support), false otherwise
func sortSupportedRuntime(runtimes []v1beta1.SupportedRuntime, baseModel *v1beta1.BaseModelSpec, modelSize float64) {
	sort.Slice(runtimes, func(i, j int) bool {
		// First, prioritize by modelFormat, modelFramework score
		// The score is calculated by the weight of modelFormat and modelFramework multiply priority
		r1Score := score(runtimes[i], baseModel)
		r2Score := score(runtimes[j], baseModel)
		if r1Score != r2Score {
			return r1Score > r2Score
		}

		// Second, prioritize by model size range
		r1HasSizeRange := runtimes[i].Spec.ModelSizeRange != nil
		r2HasSizeRange := runtimes[j].Spec.ModelSizeRange != nil

		// Check if both have size ranges and if one of them matches the model size better
		if r1HasSizeRange && r2HasSizeRange {
			r1MinDiff := math.Abs(parseModelSize(*runtimes[i].Spec.ModelSizeRange.Min) - modelSize)
			r1MaxDiff := math.Abs(parseModelSize(*runtimes[i].Spec.ModelSizeRange.Max) - modelSize)
			r2MinDiff := math.Abs(parseModelSize(*runtimes[j].Spec.ModelSizeRange.Min) - modelSize)
			r2MaxDiff := math.Abs(parseModelSize(*runtimes[j].Spec.ModelSizeRange.Max) - modelSize)

			if r1MinDiff+r1MaxDiff < r2MinDiff+r2MaxDiff {
				return true
			} else {
				return false
			}
		}

		// If only one has a size range, prioritize the one with the range
		if r1HasSizeRange && !r2HasSizeRange {
			return true
		}
		if !r1HasSizeRange && r2HasSizeRange {
			return false
		}
		return true
	})
}

// score returns a score for a runtime based on its modelFormat and modelFramework
// The score is calculated as follows:
// 1. For each supported model format in the runtime, check if it matches the baseModel's modelFormat and modelFramework.
// 2. If it matches, calculate the score by multiplying the weight of the model format and model framework by their priority.
// 3. Keep track of the maximum score found.
func score(runtime v1beta1.SupportedRuntime, baseModel *v1beta1.BaseModelSpec) int64 {
	var maxScore int64 = 0

	// 1. Go through all supported model formats in runtime
	for _, supportedFormat := range runtime.Spec.SupportedModelFormats {
		// 2. Get autoSelect flag, if it is false, continue to next supportedModelFormat
		if supportedFormat.AutoSelect != nil && !(*supportedFormat.AutoSelect) {
			continue
		}
		// 3. Get priority for it
		priority := int64(1) // Default priority
		if supportedFormat.Priority != nil {
			priority = int64(*supportedFormat.Priority)
		}

		// 3. Compare model format, if it doesn't match, continue to next supportedModelFormat
		modelFormatMatches := false
		if supportedFormat.ModelFormat != nil && &baseModel.ModelFormat.Name != nil {
			if supportedFormat.ModelFormat.Name != baseModel.ModelFormat.Name {
				continue
			}
			// Compare versions if both are specified
			if supportedFormat.ModelFormat.Version != nil && baseModel.ModelFormat.Version != nil {
				modelFormatMatches = compareModelFormat(supportedFormat.ModelFormat, &baseModel.ModelFormat, false)
				if !modelFormatMatches {
					continue
				}
			} else {
				modelFormatMatches = true
			}
		}

		// 4. Compare model framework, if it doesn't match, continue to next supportedModelFormat
		modelFrameworkMatches := false
		if supportedFormat.ModelFramework != nil && baseModel.ModelFramework != nil {
			if supportedFormat.ModelFramework.Name != baseModel.ModelFramework.Name {
				continue
			}
			// Compare versions if both are specified
			if supportedFormat.ModelFramework.Version != nil && baseModel.ModelFramework.Version != nil {
				modelFrameworkMatches = compareModelFramework(supportedFormat.ModelFramework, baseModel.ModelFramework, false)
				if !modelFrameworkMatches {
					continue
				}
			} else {
				modelFrameworkMatches = true
			}
		}

		// 5. If model format and model framework are all match, calculate score by their weight multiply priority and then sum it
		if (modelFormatMatches || (supportedFormat.ModelFormat == nil && &baseModel.ModelFormat == nil)) &&
			(modelFrameworkMatches || (supportedFormat.ModelFramework == nil && baseModel.ModelFramework == nil)) {

			// Calculate weighted score
			var currentScore int64 = 0
			if modelFormatMatches && supportedFormat.ModelFormat != nil {
				currentScore += supportedFormat.ModelFormat.Weight * priority
			}
			if modelFrameworkMatches && supportedFormat.ModelFramework != nil {
				currentScore += supportedFormat.ModelFramework.Weight * priority
			}

			// 6. Keep the max score
			if currentScore > maxScore {
				maxScore = currentScore
			}
		}
	}

	// 7. After all supportedModelFormat checked, return maxScore
	return maxScore
}

// parseModelSize converts a model size string (e.g., "7B", "13B", "70B") to a float64 value.
// It handles different size suffixes (T, B, M) and converts them to their base unit.
func parseModelSize(sizeStr string) float64 {
	var multiplier float64 = 1

	switch {
	case strings.HasSuffix(sizeStr, "T"):
		multiplier = 1_000_000_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "T")
	case strings.HasSuffix(sizeStr, "B"):
		multiplier = 1_000_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	case strings.HasSuffix(sizeStr, "M"):
		multiplier = 1_000_000
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	}

	size, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0 // Handle the error or return a default value
	}

	return size * multiplier
}

// NewArchitecture: Functions for the new engine/decoder architecture
// These functions work directly with BaseModelSpec instead of ModelSpec

// RuntimeCompatibilityError represents an error when a runtime doesn't support a model
type RuntimeCompatibilityError struct {
	RuntimeName   string
	ModelName     string
	ModelFormat   string
	Reason        string
	DetailedError error
}

func (e *RuntimeCompatibilityError) Error() string {
	if e.DetailedError != nil {
		return fmt.Sprintf("runtime %s does not support model %s: %s (%v)",
			e.RuntimeName, e.ModelName, e.Reason, e.DetailedError)
	}
	return fmt.Sprintf("runtime %s does not support model %s: %s",
		e.RuntimeName, e.ModelName, e.Reason)
}

// RuntimeSupportsModel checks if a runtime can support a specific model in the new architecture.
// It returns nil if the runtime supports the model, or a RuntimeCompatibilityError if not.
func RuntimeSupportsModel(baseModel *v1beta1.BaseModelSpec, srSpec *v1beta1.ServingRuntimeSpec, runtimeName string) error {
	// Check all supported formats, collecting them for error reporting
	formatSupported := false
	for _, format := range srSpec.SupportedModelFormats {
		if compareSupportedModelFormats(baseModel, format) {
			formatSupported = true
			break
		}
	}

	if !formatSupported {
		return &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   "", // Will be filled by caller if available
			ModelFormat: baseModel.ModelFormat.Name,
			Reason:      fmt.Sprintf("model format '%s' not in supported formats", getModelFormatLabel(baseModel)),
		}
	}

	// Check if model size is within runtime's supported range
	if baseModel.ModelParameterSize != nil && srSpec.ModelSizeRange != nil {
		modelSize := parseModelSize(*baseModel.ModelParameterSize)
		minSize := parseModelSize(*srSpec.ModelSizeRange.Min)
		maxSize := parseModelSize(*srSpec.ModelSizeRange.Max)

		if modelSize < minSize || modelSize > maxSize {
			return &RuntimeCompatibilityError{
				RuntimeName: runtimeName,
				ModelName:   "", // Will be filled by caller if available
				ModelFormat: baseModel.ModelFormat.Name,
				Reason: fmt.Sprintf("model size %s is outside supported range [%s, %s]",
					*baseModel.ModelParameterSize, *srSpec.ModelSizeRange.Min, *srSpec.ModelSizeRange.Max),
			}
		}
	}

	return nil
}

func compareSupportedModelFormats(baseModel *v1beta1.BaseModelSpec, supportedFormat v1beta1.SupportedModelFormat) bool {
	// 1. Compare model artitecture name
	if baseModel.ModelArchitecture != nil && supportedFormat.ModelArchitecture != nil {
		if *baseModel.ModelArchitecture != *supportedFormat.ModelArchitecture {
			return false
		}
	} else if (baseModel.ModelArchitecture == nil) != (supportedFormat.ModelArchitecture == nil) {
		// If only one of them is nil, they don't match
		return false
	}

	// 2. Compare model quantization
	if baseModel.Quantization != nil && supportedFormat.Quantization != nil {
		// ModelQuantization is a string type, so we can compare directly
		if *baseModel.Quantization != *supportedFormat.Quantization {
			return false
		}
	} else if (baseModel.Quantization == nil) != (supportedFormat.Quantization == nil) {
		// If only one of them is nil, they don't match
		return false
	}
	// 3. Compare ModelFormat versions
	modelFormatMatches := true
	// If version is specified in supportedFormat, compare with baseModel
	if supportedFormat.ModelFormat != nil && &baseModel.ModelFormat != nil {
		// Compare format names (must be equal)
		if supportedFormat.ModelFormat.Name != baseModel.ModelFormat.Name {
			return false
		}

		if supportedFormat.ModelFormat.Version != nil && baseModel.ModelFormat.Version != nil {
			modelFormatMatches = compareModelFormat(supportedFormat.ModelFormat, &baseModel.ModelFormat, modelFormatMatches)
			// If ModelFormat versions don't match, the formats are incompatible
			if !modelFormatMatches {
				return modelFormatMatches
			}
		} else {
			return false
		}
	} else if (supportedFormat.ModelFormat != nil) != (&baseModel.ModelFormat != nil) {
		// If only one of them is nil, they don't match
		return false
	}

	// 4. Compare ModelFramework (if exists)
	modelFrameworkMatches := true
	if supportedFormat.ModelFramework != nil && baseModel.ModelFramework != nil {
		// Compare framework names (must be equal)
		if supportedFormat.ModelFramework.Name != baseModel.ModelFramework.Name {
			return false
		}
		// Compare framework versions if both are specified
		if supportedFormat.ModelFramework.Version != nil && baseModel.ModelFramework.Version != nil {
			modelFrameworkMatches = compareModelFramework(supportedFormat.ModelFramework, baseModel.ModelFramework, modelFrameworkMatches)
			// If ModelFramework versions don't match, the formats are incompatible
			if !modelFrameworkMatches {
				return modelFrameworkMatches
			}
		} else {
			return false
		}
	} else if (supportedFormat.ModelFramework != nil) != (baseModel.ModelFramework != nil) {
		// If only one of them is nil, they don't match
		return false
	}

	// 5. If we got this far, the formats are compatible
	return true
}

// compareModelFormat compares two model formats based on their versions and operators.
func compareModelFormat(supportedModelFormat *v1beta1.ModelFormat, basemodelModeFormat *v1beta1.ModelFormat, modelFormatMatches bool) bool {
	hasUnofficialFormatVersion := false
	// Parse versions
	baseModelFormatVersion, err := modelVer.Parse(*basemodelModeFormat.Version)
	if err != nil {
		fmt.Println("Error parsing basModel modelFormat version:", err)
		return false
	}

	supportedFormatVersion, err := modelVer.Parse(*supportedModelFormat.Version)
	if err != nil {
		fmt.Println("Error parsing supportedFormat modelFormat version:", err)
		return false
	}

	// Check if versions have unofficial parts (requirement #1)
	hasUnofficialFormatVersion = modelVer.ContainsUnofficialVersion(baseModelFormatVersion) ||
		modelVer.ContainsUnofficialVersion(supportedFormatVersion)

	// Get operator from modelFormat in supportedFormat
	operator := getRuntimeSelectorOperator(supportedModelFormat.Operator)

	// Compare versions based on operator and whether unofficial versions exist (requirements #1, #2, #3)
	if hasUnofficialFormatVersion || operator == "Equal" {
		modelFormatMatches = modelVer.Equal(supportedFormatVersion, baseModelFormatVersion)
	} else if operator == "GreaterThan" {
		modelFormatMatches = modelVer.GreaterThan(supportedFormatVersion, baseModelFormatVersion)
	} else if operator == "GreaterThanOrEqual" {
		modelFormatMatches = modelVer.GreaterThanOrEqual(supportedFormatVersion, baseModelFormatVersion)
	} else {
		// Default to Equal for unknown operators
		modelFormatMatches = modelVer.Equal(supportedFormatVersion, baseModelFormatVersion)
	}
	return modelFormatMatches
}

// compareModelFramework compares two modelFrameworks based on their versions and operators.
func compareModelFramework(supportedModelFramework *v1beta1.ModelFrameworkSpec, baseModelFramework *v1beta1.ModelFrameworkSpec, modelFrameworkMatches bool) bool {
	hasUnofficialFrameworkVersion := false
	// Parse framework versions
	baseFrameworkVersion, err := modelVer.Parse(*baseModelFramework.Version)
	if err != nil {
		fmt.Println("Error parsing baseModel modelFramework version:", err)
		return false
	}

	supportedFrameworkVersion, err := modelVer.Parse(*supportedModelFramework.Version)
	if err != nil {
		fmt.Println("Error parsing supportedFormat modelFramework version:", err)
		return false
	}

	// Check if versions have unofficial parts (requirement #1)
	hasUnofficialFrameworkVersion = modelVer.ContainsUnofficialVersion(baseFrameworkVersion) ||
		modelVer.ContainsUnofficialVersion(supportedFrameworkVersion)

	// Get operator from modelFramework in supportedFormat
	operator := getRuntimeSelectorOperator(supportedModelFramework.Operator)

	// If there are unofficial versions or operator is Equal, use Equal comparison (requirements #1, #2)
	if hasUnofficialFrameworkVersion || operator == "Equal" {
		modelFrameworkMatches = modelVer.Equal(supportedFrameworkVersion, baseFrameworkVersion)
	} else if operator == "GreaterThan" {
		modelFrameworkMatches = modelVer.GreaterThan(supportedFrameworkVersion, baseFrameworkVersion)
	} else if operator == "GreaterThanOrEqual" {
		modelFrameworkMatches = modelVer.GreaterThanOrEqual(supportedFrameworkVersion, baseFrameworkVersion)
	} else {
		// Default to Equal for unknown operators
		modelFrameworkMatches = modelVer.Equal(supportedFrameworkVersion, baseFrameworkVersion)
	}
	return modelFrameworkMatches
}

// getRuntimeSelectorOperator return a string representation of the RuntimeSelectorOperator.
// If the operator is nil, it defaults to "Equal".
func getRuntimeSelectorOperator(operator *v1beta1.RuntimeSelectorOperator) string {
	if operator == nil {
		return string(v1beta1.RuntimeSelectorOpEqual)
	}
	return string(*operator)
}

// GetSupportingRuntimes returns a list of ServingRuntimeSpecs that can support the given model.
// It considers both namespace-scoped and cluster-scoped runtimes, and sorts them by priority.
// It also returns detailed reasons why each runtime was excluded, which can be used for debugging.
func GetSupportingRuntimes(baseModel *v1beta1.BaseModelSpec, cl client.Client, namespace string) ([]v1beta1.SupportedRuntime, map[string]error, error) {
	excludedRuntimes := make(map[string]error)

	// List all namespace-scoped runtimes
	runtimes := &v1beta1.ServingRuntimeList{}
	if err := cl.List(context.TODO(), runtimes, client.InNamespace(namespace)); err != nil {
		return nil, nil, err
	}
	// Sort namespace-scoped runtimes by created timestamp desc and name asc
	sortServingRuntimeList(runtimes)

	// List all cluster-scoped runtimes
	clusterRuntimes := &v1beta1.ClusterServingRuntimeList{}
	if err := cl.List(context.TODO(), clusterRuntimes); err != nil {
		return nil, nil, err
	}
	// Sort cluster-scoped runtimes by created timestamp desc and name asc
	sortClusterServingRuntimeList(clusterRuntimes)

	var srSpecs []v1beta1.SupportedRuntime
	var clusterSrSpecs []v1beta1.SupportedRuntime

	// Process namespace-scoped runtimes
	for i := range runtimes.Items {
		rt := &runtimes.Items[i]

		if rt.Spec.IsDisabled() {
			excludedRuntimes[rt.GetName()] = fmt.Errorf("runtime is disabled")
			continue
		}

		if err := RuntimeSupportsModel(baseModel, &rt.Spec, rt.GetName()); err != nil {
			excludedRuntimes[rt.GetName()] = err
			continue
		}

		// Check if runtime has auto-select enabled for at least one supported format
		hasAutoSelect := false
		for _, format := range rt.Spec.SupportedModelFormats {
			if format.AutoSelect != nil && *format.AutoSelect {
				hasAutoSelect = true
				break
			}
		}

		if !hasAutoSelect {
			excludedRuntimes[rt.GetName()] = fmt.Errorf("runtime does not have auto-select enabled")
			continue
		}

		// Filter out runtimes that don't support the model's format/framework,
		// Score calculation considers modelFormat and modelFramework compatibility with priority weights
		// A score <= 0 indicates no format/framework match or this supportedFormat autoselect is false, so runtime cannot serve this model
		if score(v1beta1.SupportedRuntime{Name: rt.GetName(), Spec: rt.Spec}, baseModel) <= 0 {
			continue
		}

		srSpecs = append(srSpecs, v1beta1.SupportedRuntime{Name: rt.GetName(), Spec: rt.Spec})
	}
	// Sort namespace-scoped runtimes by priority
	if baseModel.ModelParameterSize != nil {
		sortSupportedRuntime(srSpecs, baseModel, parseModelSize(*baseModel.ModelParameterSize))
	} else {
		sortSupportedRuntime(srSpecs, baseModel, 0)
	}

	// Process cluster-scoped runtimes
	for i := range clusterRuntimes.Items {
		crt := &clusterRuntimes.Items[i]

		if crt.Spec.IsDisabled() {
			excludedRuntimes[crt.GetName()] = fmt.Errorf("runtime is disabled")
			continue
		}

		if err := RuntimeSupportsModel(baseModel, &crt.Spec, crt.GetName()); err != nil {
			excludedRuntimes[crt.GetName()] = err
			continue
		}

		// Check if runtime has auto-select enabled for at least one supported format
		hasAutoSelect := false
		for _, format := range crt.Spec.SupportedModelFormats {
			if format.AutoSelect != nil && *format.AutoSelect {
				hasAutoSelect = true
				break
			}
		}

		if !hasAutoSelect {
			excludedRuntimes[crt.GetName()] = fmt.Errorf("runtime does not have auto-select enabled")
			continue
		}

		// Filter out runtimes that don't support the model's format/framework,
		// Score calculation considers modelFormat and modelFramework compatibility with priority weights
		// A score <= 0 indicates no format/framework match or this supportedFormat autoselect is false, so runtime cannot serve this model
		if score(v1beta1.SupportedRuntime{Name: crt.GetName(), Spec: crt.Spec}, baseModel) <= 0 {
			continue
		}

		clusterSrSpecs = append(clusterSrSpecs, v1beta1.SupportedRuntime{Name: crt.GetName(), Spec: crt.Spec})
	}
	// Sort cluster-scoped runtimes by priority
	if baseModel.ModelParameterSize != nil {
		sortSupportedRuntime(clusterSrSpecs, baseModel, parseModelSize(*baseModel.ModelParameterSize))
	} else {
		sortSupportedRuntime(clusterSrSpecs, baseModel, 0)
	}

	srSpecs = append(srSpecs, clusterSrSpecs...)

	return srSpecs, excludedRuntimes, nil
}
