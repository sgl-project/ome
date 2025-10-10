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
// Take the maximum value for each resource type to ensure sufficient allocation.
func MergeResource(container *v1.Container, acceleratorClass *v1beta1.AcceleratorClassSpec, runtime *v1beta1.ServingRuntimeSpec) {
	if container == nil {
		return
	}

	// Merge resource requests.
	if container.Resources.Requests == nil {
		container.Resources.Requests = v1.ResourceList{}
	}
	// Merge resource requests from runtime with the same container name.
	if runtime != nil && &runtime.ServingRuntimePodSpec != nil && runtime.ServingRuntimePodSpec.Containers != nil {
		for _, rtContainer := range runtime.ServingRuntimePodSpec.Containers {
			if rtContainer.Name == container.Name {
				for resourceName, quantity := range rtContainer.Resources.Requests {
					if existingQty, exists := container.Resources.Requests[resourceName]; !exists || quantity.Cmp(existingQty) > 0 {
						container.Resources.Requests[resourceName] = quantity.DeepCopy()
					}
				}
				break
			}
		}
	}
	// Merge resource requests from accelerator class when this resource is required in container.
	if acceleratorClass != nil && acceleratorClass.Resources != nil {
		for _, resource := range acceleratorClass.Resources {
			resourceName := v1.ResourceName(resource.Name)
			quantity := resource.Quantity
			if existingQty, exists := container.Resources.Requests[resourceName]; !exists || quantity.Cmp(existingQty) > 0 {
				container.Resources.Requests[resourceName] = quantity.DeepCopy()
			}
		}

	}

	// Merge resource limits
	if container.Resources.Limits == nil {
		container.Resources.Limits = v1.ResourceList{}
	}
	if runtime != nil && &runtime.ServingRuntimePodSpec != nil && runtime.ServingRuntimePodSpec.Containers != nil {
		for _, rtContainer := range runtime.ServingRuntimePodSpec.Containers {
			if rtContainer.Name == container.Name {
				for resourceName, quantity := range rtContainer.Resources.Limits {
					if existingQty, exists := container.Resources.Limits[resourceName]; !exists || quantity.Cmp(existingQty) > 0 {
						container.Resources.Limits[resourceName] = quantity.DeepCopy()
					}
				}
				break
			}
		}
	}
	if acceleratorClass != nil && acceleratorClass.Resources != nil {
		for _, resource := range acceleratorClass.Resources {
			resourceName := v1.ResourceName(resource.Name)
			quantity := resource.Quantity
			if existingQty, exists := container.Resources.Limits[resourceName]; !exists || quantity.Cmp(existingQty) > 0 {
				container.Resources.Limits[resourceName] = quantity.DeepCopy()
			}
		}

	}
}

// mergeMultilineArgs merges container args with override args by combining multi-line strings.
// It handles the case where args are multi-line strings (containing newlines or backslashes) and merges them
// into a single multi-line string instead of creating separate array elements.
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

		// Ensure base arg ends with backslash continuation if it doesn't already
		trimmedBase := strings.TrimRight(baseArg, " \t\n")
		if !strings.HasSuffix(trimmedBase, "\\") {
			// Add backslash continuation before appending overrides
			baseArg = trimmedBase + " \\"
		}

		// Collect all override args content
		var overrideContent strings.Builder
		for _, arg := range overrideArgs {
			trimmed := strings.TrimSpace(arg)
			if trimmed != "" {
				overrideContent.WriteString("\n")
				overrideContent.WriteString(trimmed)
			}
		}

		// Merge: append override content to base arg
		merged := baseArg + overrideContent.String()

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
