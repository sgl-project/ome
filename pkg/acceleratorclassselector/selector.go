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
//     d. Otherwise, return the first AcceleratorClass from runtime.AcceleratorRequirements
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
		if acName == "" {
			if acceleratorClass := s.getAcceleratorClassByPolicy(isvc, runtime, component); acceleratorClass != nil {
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

// TODO: Consider accelerator class selector by AcceleratorSelectionPolicy, currently only FirstAvailablePolicy is implemented
func (s *defaultSelector) getAcceleratorClassByPolicy(isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec, component v1beta1.ComponentType) *string {
	acceleratorPolicy := s.getAcceleratorPolicy(isvc, component)
	switch acceleratorPolicy {
	case v1beta1.BestFitPolicy:
		return nil
	case v1beta1.CheapestPolicy:
		return nil
	case v1beta1.MostCapablePolicy:
		return nil
	case v1beta1.FirstAvailablePolicy:
		// Return the first AcceleratorClass from runtime requirements as default for now
		if len(runtime.AcceleratorRequirements.AcceleratorClasses) > 0 {
			return &runtime.AcceleratorRequirements.AcceleratorClasses[0]
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
// defaulting to the selector's configured default policy if none is specified
func (s *defaultSelector) getAcceleratorPolicy(isvc *v1beta1.InferenceService, component v1beta1.ComponentType) v1beta1.AcceleratorSelectionPolicy {
	// Check component-specific AcceleratorOverride
	if policy := s.getComponentAcceleratorPolicy(isvc, component); policy != "" {
		return policy
	}

	return s.config.DefaultPolicy
}
