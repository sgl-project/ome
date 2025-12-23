package utils

import (
	"bytes"
	"encoding/json"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// MergeRuntimeContainers Merge the predictor Container struct with the runtime Container struct, allowing users
func MergeRuntimeContainers(runtimeContainer *v1.Container, predictorContainer *v1.Container) (*v1.Container, error) {
	// Save runtime container name, as the name can be overridden as empty string during the Unmarshal below
	// since the Name field does not have the 'omitempty' struct tag.
	runtimeContainerName := runtimeContainer.Name

	// Use JSON Marshal/Unmarshal to merge Container structs using strategic merge patch
	runtimeContainerJson, err := json.Marshal(runtimeContainer)
	if err != nil {
		return nil, err
	}

	overrides, err := json.Marshal(predictorContainer)
	if err != nil {
		return nil, err
	}

	mergedContainer := v1.Container{}
	jsonResult, err := strategicpatch.StrategicMergePatch(runtimeContainerJson, overrides, mergedContainer)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &mergedContainer); err != nil {
		return nil, err
	}

	if mergedContainer.Name == "" {
		mergedContainer.Name = runtimeContainerName
	}

	// Strategic merge patch will replace args but more useful behaviour here is to concatenate
	mergedContainer.Args = append(append([]string{}, runtimeContainer.Args...), predictorContainer.Args...)

	return &mergedContainer, nil
}

// mergeSpec merges two Kubernetes-style specs using strategic merge patch.
// `runtimeInit` is the base (typically from a runtime default), and `override` comes from the user-defined resource.
// Fields from `override` will overwrite corresponding fields in `runtimeInit`.
func mergeSpec[T any](runtimeInit T, override T) (*T, error) {
	baseJSON, err := json.Marshal(runtimeInit)
	if err != nil {
		return nil, err
	}

	overrideJSON, err := json.Marshal(override)
	if err != nil {
		return nil, err
	}

	var merged T
	jsonResult, err := strategicpatch.StrategicMergePatch(baseJSON, overrideJSON, merged)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

// MergeRouterSpec merges a runtime-provided RouterSpec with a user-provided RouterSpec from InferenceService.
// The user-provided (isvcRouter) fields take precedence over the runtime defaults.
func MergeRouterSpec(isvcRouter, runtimeRouter *v1beta1.RouterSpec) (*v1beta1.RouterSpec, error) {
	switch {
	case isvcRouter == nil:
		// if router is not specified in isvc, return nil
		return nil, nil
	case runtimeRouter == nil:
		// if router is not specified in runtime, return a copy of isvcRouter
		return isvcRouter.DeepCopy(), nil
	}

	return mergeSpec(v1beta1.RouterSpec{
		ComponentExtensionSpec: runtimeRouter.ComponentExtensionSpec,
		PodSpec:                runtimeRouter.PodSpec,
		Runner:                 runtimeRouter.Runner,
		Config:                 runtimeRouter.Config,
	}, *isvcRouter)
}

// MergeEngineSpec merges a runtime-provided EngineSpec with a user-provided EngineSpec from InferenceService.
// The user-provided (isvcEngine) fields take precedence over the runtime defaults.
func MergeEngineSpec(runtimeEngine, isvcEngine *v1beta1.EngineSpec) (*v1beta1.EngineSpec, error) {
	switch {
	case isvcEngine == nil:
		// if engine is not specified in isvc, return nil
		return nil, nil
	case runtimeEngine == nil:
		// if engine is not specified in runtime, return a copy of isvcEngine
		return isvcEngine.DeepCopy(), nil
	}

	return mergeSpec(v1beta1.EngineSpec{
		ComponentExtensionSpec: runtimeEngine.ComponentExtensionSpec,
		PodSpec:                runtimeEngine.PodSpec,
		Runner:                 runtimeEngine.Runner,
		Leader:                 runtimeEngine.Leader,
		Worker:                 runtimeEngine.Worker,
	}, *isvcEngine)
}

// MergeDecoderSpec merges a runtime-provided DecoderSpec with a user-provided DecoderSpec from InferenceService.
// The user-provided (isvcDecoder) fields take precedence over the runtime defaults.
func MergeDecoderSpec(runtimeDecoder, isvcDecoder *v1beta1.DecoderSpec) (*v1beta1.DecoderSpec, error) {
	switch {
	case isvcDecoder == nil:
		// if decoder is not specified in isvc, return nil
		return nil, nil
	case runtimeDecoder == nil:
		// if decoder is not specified in runtime, return a copy of isvcDecoder
		return isvcDecoder.DeepCopy(), nil
	}

	return mergeSpec(v1beta1.DecoderSpec{
		ComponentExtensionSpec: runtimeDecoder.ComponentExtensionSpec,
		PodSpec:                runtimeDecoder.PodSpec,
		Runner:                 runtimeDecoder.Runner,
		Leader:                 runtimeDecoder.Leader,
		Worker:                 runtimeDecoder.Worker,
	}, *isvcDecoder)
}

// ConvertPodSpec converts v1beta1.PodSpec to v1.PodSpec
// This handles the conversion between the custom v1beta1.PodSpec type and the core v1.PodSpec type
func ConvertPodSpec(spec *v1beta1.PodSpec) (*v1.PodSpec, error) {
	if spec == nil {
		return nil, goerrors.New("cannot convert nil PodSpec")
	}

	// Use JSON marshaling to convert between the types
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, goerrors.Wrap(err, "failed to marshal v1beta1.PodSpec")
	}

	var podSpec v1.PodSpec
	if err := json.Unmarshal(data, &podSpec); err != nil {
		return nil, goerrors.Wrap(err, "failed to unmarshal to v1.PodSpec")
	}

	return &podSpec, nil
}

// ReplacePlaceholders Replace placeholders in runtime container by values from inferenceservice metadata
func ReplacePlaceholders(container *v1.Container, meta metav1.ObjectMeta) error {
	data, _ := json.Marshal(container)
	tmpl, err := template.New("container-tmpl").Parse(string(data))
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, meta)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), container)
}

// MergeNodeSelector merges node selectors from runtime, accelerator class, and inference service (isvc).
// Only add mergedNodeSelector to engine and decoder component.
func MergeNodeSelector(runtime *v1beta1.ServingRuntimeSpec, acceleratorClass *v1beta1.AcceleratorClassSpec, isvc *v1beta1.InferenceService, component v1beta1.ComponentType) map[string]string {
	// Start with runtime node selector
	mergedNodeSelector := map[string]string{}
	if runtime != nil && &runtime.ServingRuntimePodSpec != nil && runtime.ServingRuntimePodSpec.NodeSelector != nil {
		for k, v := range runtime.ServingRuntimePodSpec.NodeSelector {
			mergedNodeSelector[k] = v
		}
	}

	// Merge in accelerator class node selector, overriding any conflicts
	if acceleratorClass != nil && acceleratorClass.Discovery.NodeSelector != nil {
		for k, v := range acceleratorClass.Discovery.NodeSelector {
			mergedNodeSelector[k] = v
		}
	}

	// Finally merge in isvc node selector, overriding any conflicts
	switch component {
	case v1beta1.EngineComponent:
		if isvc.Spec.Engine != nil && &isvc.Spec.Engine.PodSpec != nil && isvc.Spec.Engine.PodSpec.NodeSelector != nil {
			for k, v := range isvc.Spec.Engine.PodSpec.NodeSelector {
				mergedNodeSelector[k] = v
			}
		}
		return mergedNodeSelector

	case v1beta1.DecoderComponent:
		if isvc.Spec.Decoder != nil && &isvc.Spec.Decoder.PodSpec != nil && isvc.Spec.Decoder.PodSpec.NodeSelector != nil {
			for k, v := range isvc.Spec.Decoder.PodSpec.NodeSelector {
				mergedNodeSelector[k] = v
			}
		}
		return mergedNodeSelector
	}

	return mergedNodeSelector
}

// MergeResource merges resource requests and limits from runtime, accelerator class, and container spec.
// It only merges resources from the runtime and accelerator class when the user has not explicitly specified resources in the InferenceService spec.
// The acceleratorClass takes precedence and overrides the runtime resource, if it existed.
func MergeResource(container *v1.Container, acceleratorClass *v1beta1.AcceleratorClassSpec, runtime *v1beta1.ServingRuntimeSpec) {
	if container == nil {
		return
	}

	// Merge resource requests.
	if container.Resources.Requests == nil {
		container.Resources.Requests = v1.ResourceList{}
	}
	// Merge resource requests from runtime with the same container name if it does not already exist.
	if runtime != nil && &runtime.ServingRuntimePodSpec != nil && runtime.ServingRuntimePodSpec.Containers != nil {
		for _, rtContainer := range runtime.ServingRuntimePodSpec.Containers {
			if rtContainer.Name == container.Name {
				for resourceName, quantity := range rtContainer.Resources.Requests {
					if _, exists := container.Resources.Requests[resourceName]; !exists {
						container.Resources.Requests[resourceName] = quantity.DeepCopy()
					}
				}
				break
			}
		}
	}
	// Merge resource requests from accelerator class.
	// AcceleratorClass takes precedence and overrides runtime resources.
	if acceleratorClass != nil && acceleratorClass.Resources != nil {
		for _, resource := range acceleratorClass.Resources {
			resourceName := v1.ResourceName(resource.Name)
			quantity := resource.Quantity
			container.Resources.Requests[resourceName] = quantity.DeepCopy()
		}
	}

	// Merge resource limits
	if container.Resources.Limits == nil {
		container.Resources.Limits = v1.ResourceList{}
	}
	// Merge resource limits from runtime with the same container name if it does not already exist.
	if runtime != nil && &runtime.ServingRuntimePodSpec != nil && runtime.ServingRuntimePodSpec.Containers != nil {
		for _, rtContainer := range runtime.ServingRuntimePodSpec.Containers {
			if rtContainer.Name == container.Name {
				for resourceName, quantity := range rtContainer.Resources.Limits {
					if _, exists := container.Resources.Limits[resourceName]; !exists {
						container.Resources.Limits[resourceName] = quantity.DeepCopy()
					}
				}
				break
			}
		}
	}
	// AcceleratorClass takes precedence and overrides runtime resource limits.
	if acceleratorClass != nil && acceleratorClass.Resources != nil {
		for _, resource := range acceleratorClass.Resources {
			resourceName := v1.ResourceName(resource.Name)
			quantity := resource.Quantity
			container.Resources.Limits[resourceName] = quantity.DeepCopy()
		}
	}
}

// extractArgKey extracts the key from an argument string for deduplication and override matching.
// For flags (starting with -), it extracts just the flag name.
// For non-flags (like "python3 -m server"), it returns the entire string as the key.
// This allows proper deduplication of both flags and command lines.
// Examples:
//   - "--tp-size=4" -> "--tp-size"
//   - "--tp-size 4" -> "--tp-size"
//   - "--enable-metrics" -> "--enable-metrics"
//   - "python3 -m server" -> "python3 -m server" (entire string is the key)
func extractArgKey(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}

	// If it doesn't start with -, treat the entire string as the key
	// This handles commands like "python3 -m server"
	if !strings.HasPrefix(arg, "-") {
		return arg
	}

	// For flags, extract just the flag name
	// Handle --key=value format
	if idx := strings.Index(arg, "="); idx != -1 {
		return arg[:idx]
	}

	// Handle --key value format (space-separated)
	// Extract just the key part before any space
	parts := strings.Fields(arg)
	if len(parts) > 0 {
		return parts[0]
	}

	return arg
}

// mergeMultilineArgs merges container args with override args by combining multi-line strings.
// It handles the case where args are multi-line strings (containing newlines or backslashes) and merges them
// into a single multi-line string instead of creating separate array elements.
// Smart override: if an override has the same key but different value, it replaces the existing value.
func MergeMultilineArgs(containerArgs []string, overrideArgs []string) []string {
	if len(overrideArgs) == 0 {
		return containerArgs
	}
	if len(containerArgs) == 0 {
		return overrideArgs
	}

	// Check if the first arg in containerArgs is a multi-line string (contains newlines or backslashes)
	// Note: The "|" YAML literal block scalar indicator is stripped by K8s during parsing
	if len(containerArgs) > 0 && (strings.Contains(containerArgs[0], "\n") || strings.Contains(containerArgs[0], "\\")) {
		// Parse the multi-line string from containerArgs
		baseArg := containerArgs[0]

		// Parse existing args into a map: key -> full arg string
		// This enables smart override: if override has same key but different value, replace it
		existingArgs := make(map[string]string) // key -> full arg (e.g., "--tp-size" -> "--tp-size=4")
		argKeys := make(map[string]int)         // key -> line index for replacement
		lines := strings.Split(baseArg, "\n")

		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Remove trailing backslash for parsing
			trimmed = strings.TrimRight(trimmed, "\\")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				key := extractArgKey(trimmed)
				if key != "" {
					existingArgs[key] = trimmed
					argKeys[key] = i
				}
			}
		}

		// Process override args: replace existing keys or add new ones
		overridesToAdd := make([]string, 0)
		keysToReplace := make(map[string]string) // key -> new value

		for _, arg := range overrideArgs {
			// Split multi-line override args into individual lines
			overrideLines := strings.Split(arg, "\n")

			for _, line := range overrideLines {
				trimmed := strings.TrimSpace(line)
				// Remove trailing backslash
				trimmed = strings.TrimRight(trimmed, "\\")
				trimmed = strings.TrimSpace(trimmed)

				if trimmed == "" {
					continue
				}

				key := extractArgKey(trimmed)
				if key == "" {
					// If we can't extract a key, add it as-is
					overridesToAdd = append(overridesToAdd, trimmed)
					continue
				}

				if existingValue, exists := existingArgs[key]; exists {
					// Key exists - check if value is different
					if existingValue != trimmed {
						// Different value - mark for replacement
						keysToReplace[key] = trimmed
					}
					// Same value - do nothing (keep existing)
				} else {
					// New key - add it
					overridesToAdd = append(overridesToAdd, trimmed)
				}
			}
		}

		// Rebuild the args with replacements (only if there are replacements to make)
		var rebuiltArgs strings.Builder
		if len(keysToReplace) > 0 {
			for i, line := range lines {
				// Preserve leading whitespace
				leadingWhitespace := ""
				trimmed := strings.TrimLeft(line, " \t")
				if len(line) > len(trimmed) {
					leadingWhitespace = line[:len(line)-len(trimmed)]
				}

				// Remove trailing backslash for key extraction
				trimmedNoBackslash := strings.TrimRight(trimmed, " \t\\")
				trimmedNoBackslash = strings.TrimSpace(trimmedNoBackslash)

				if trimmedNoBackslash != "" {
					key := extractArgKey(trimmedNoBackslash)
					if newValue, shouldReplace := keysToReplace[key]; shouldReplace {
						// Replace this line with the new value (preserve leading whitespace of original)
						if i > 0 {
							rebuiltArgs.WriteString("\n")
						}
						rebuiltArgs.WriteString(leadingWhitespace)
						rebuiltArgs.WriteString(newValue)
						if i < len(lines)-1 || len(overridesToAdd) > 0 {
							rebuiltArgs.WriteString(" \\")
						}
						delete(keysToReplace, key) // Mark as processed
					} else {
						// Keep the original line as-is
						if i > 0 {
							rebuiltArgs.WriteString("\n")
						}
						rebuiltArgs.WriteString(line)
					}
				} else if i == 0 {
					// First line is empty, keep it
					rebuiltArgs.WriteString(line)
				}
			}
		}

		// Add new args that weren't replacements
		var overrideContent strings.Builder
		for _, arg := range overridesToAdd {
			overrideContent.WriteString("\n")
			overrideContent.WriteString(arg)
		}

		// Merge: use rebuilt args if there were replacements, otherwise use baseArg
		merged := baseArg
		if rebuiltArgs.Len() > 0 {
			// We rebuilt the args due to replacements
			merged = rebuiltArgs.String()
		}
		if overrideContent.Len() > 0 {
			// Add new args that weren't duplicates or replacements
			trimmedMerged := strings.TrimRight(merged, " \t\n")
			if !strings.HasSuffix(trimmedMerged, "\\") {
				merged = trimmedMerged + " \\"
			}
			merged = merged + overrideContent.String()
		}

		// Return merged arg followed by remaining args
		result := []string{merged}
		if len(containerArgs) > 1 {
			result = append(result, containerArgs[1:]...)
		}
		return result
	}

	// If not multi-line format, just append
	return append(append([]string{}, containerArgs...), overrideArgs...)
}

// OverrideIntParam overrides a specific integer parameter in a multiline command string.
// If the key exists (e.g., "--tp-size=4"), it replaces the value with the new one.
// Returns the updated args and a boolean indicating whether the key was found and replaced.
func OverrideIntParam(containerArgs []string, key string, value int64) ([]string, bool) {
	if len(containerArgs) == 0 {
		return containerArgs, false
	}

	arg := containerArgs[0]
	// Build regex pattern to match the key with its value
	// Matches: --tp-size=4 or --tp-size 4
	// Escapes special regex characters in the key
	escapedKey := regexp.QuoteMeta(key)
	pattern := regexp.MustCompile(escapedKey + `(?:=|\s+)\d+`)

	// Check if the key exists in the string
	if !pattern.MatchString(arg) {
		return containerArgs, false
	}

	// Replace the existing value
	replacement := key + "=" + strconv.Itoa(int(value))
	arg = pattern.ReplaceAllString(arg, replacement)
	// Update the containerArgs with the modified value
	containerArgs[0] = arg

	return containerArgs, true
}
