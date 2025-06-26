// Package utils provides utility functions for the InferenceService controller
package utils

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	goerrors "github.com/pkg/errors"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
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

// sortSupportedRuntimeByPriority sorts runtimes by their priority for a specific model.
// The sorting considers:
// 1. Model size range compatibility
// 2. Explicit priority values
// 3. Creation timestamp and name as tiebreakers
func sortSupportedRuntimeByPriority(runtimes []v1beta1.SupportedRuntime, modelFormat v1beta1.ModelFormat, modelSize float64) {
	sort.Slice(runtimes, func(i, j int) bool {
		p1 := runtimes[i].Spec.GetPriority(modelFormat.Name)
		p2 := runtimes[j].Spec.GetPriority(modelFormat.Name)

		// First, prioritize by model size range
		r1HasSizeRange := runtimes[i].Spec.ModelSizeRange != nil
		r2HasSizeRange := runtimes[j].Spec.ModelSizeRange != nil

		// Check if both have size ranges and if one of them matches the model size better
		if r1HasSizeRange && r2HasSizeRange {
			r1FitsModel := modelSize >= parseModelSize(*runtimes[i].Spec.ModelSizeRange.Min) &&
				modelSize <= parseModelSize(*runtimes[i].Spec.ModelSizeRange.Max)
			r2FitsModel := modelSize >= parseModelSize(*runtimes[j].Spec.ModelSizeRange.Min) &&
				modelSize <= parseModelSize(*runtimes[j].Spec.ModelSizeRange.Max)

			if r1FitsModel && !r2FitsModel {
				return true
			} else if !r1FitsModel && r2FitsModel {
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

		// Finally, fallback to prioritizing by explicit priority values
		switch {
		case p1 == nil && p2 == nil: // if both runtimes do not specify the priority, the order is kept
			return false
		case p1 == nil && p2 != nil: // runtime with priority specified takes precedence
			return false
		case p1 != nil && p2 == nil:
			return true
		}
		return *p1 > *p2
	})
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
	// Check if runtime supports the model format
	modelLabel := getModelFormatLabel(baseModel)
	var supportedFormats []string

	// Check all supported formats, collecting them for error reporting
	formatSupported := false
	for _, format := range srSpec.SupportedModelFormats {
		label := generateLabel(
			format.ModelFormat,
			format.ModelArchitecture,
			format.Quantization,
			format.ModelFramework,
		)
		supportedFormats = append(supportedFormats, label)
		if label == modelLabel {
			formatSupported = true
			break
		}
	}

	if !formatSupported {
		return &RuntimeCompatibilityError{
			RuntimeName: runtimeName,
			ModelName:   "", // Will be filled by caller if available
			ModelFormat: baseModel.ModelFormat.Name,
			Reason:      fmt.Sprintf("model format '%s' not in supported formats %v", modelLabel, supportedFormats),
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

// formatToString converts a SupportedModelFormat to a human-readable string
func formatToString(format v1beta1.SupportedModelFormat) string {
	result := format.Name
	if format.ModelFormat != nil {
		result = format.ModelFormat.Name
		if format.ModelFormat.Version != nil {
			result += ":" + *format.ModelFormat.Version
		}
	}
	if format.ModelArchitecture != nil {
		result += "/" + *format.ModelArchitecture
	}
	if format.Quantization != nil {
		result += "/" + string(*format.Quantization)
	}
	return result
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

		srSpecs = append(srSpecs, v1beta1.SupportedRuntime{Name: rt.GetName(), Spec: rt.Spec})
	}

	// Sort namespace-scoped runtimes by priority
	if baseModel.ModelParameterSize != nil {
		sortSupportedRuntimeByPriority(srSpecs, baseModel.ModelFormat, parseModelSize(*baseModel.ModelParameterSize))
	} else {
		sortSupportedRuntimeByPriority(srSpecs, baseModel.ModelFormat, 0)
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

		clusterSrSpecs = append(clusterSrSpecs, v1beta1.SupportedRuntime{Name: crt.GetName(), Spec: crt.Spec})
	}

	// Sort cluster-scoped runtimes by priority
	if baseModel.ModelParameterSize != nil {
		sortSupportedRuntimeByPriority(clusterSrSpecs, baseModel.ModelFormat, parseModelSize(*baseModel.ModelParameterSize))
	} else {
		sortSupportedRuntimeByPriority(clusterSrSpecs, baseModel.ModelFormat, 0)
	}

	srSpecs = append(srSpecs, clusterSrSpecs...)
	return srSpecs, excludedRuntimes, nil
}
