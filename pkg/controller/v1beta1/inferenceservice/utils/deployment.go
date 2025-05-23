package utils

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
)

// DetermineEngineDeploymentMode determines the deployment mode for the engine based on its configuration
// and constraints from other components (e.g., decoder presence)
func DetermineEngineDeploymentMode(engine *v1beta1.EngineSpec) constants.DeploymentModeType {
	if engine == nil {
		return constants.RawDeployment
	}

	// Multi-node if leader and worker are defined
	if engine.Leader != nil || engine.Worker != nil {
		return constants.MultiNode
	}

	// Serverless if min replicas is 0
	if engine.MinReplicas != nil && *engine.MinReplicas == 0 {
		return constants.Serverless
	}

	// Default to raw deployment
	return constants.RawDeployment
}

// DetermineDeploymentModes determines the deployment modes for all components based on their specs
// and enforces compatibility constraints (e.g., decoder present → engine can't be serverless)
func DetermineDeploymentModes(engine *v1beta1.EngineSpec, decoder *v1beta1.DecoderSpec, router *v1beta1.RouterSpec, runtime *v1beta1.ServingRuntimeSpec) (engineMode, decoderMode, routerMode constants.DeploymentModeType, err error) {
	// Determine base modes for each component
	engineMode = determineComponentDeploymentMode(engine, runtime)
	decoderMode = constants.RawDeployment // Decoder only supports RawDeployment or MultiNode
	routerMode = constants.RawDeployment  // Default for router

	// Apply decoder constraints
	if decoder != nil {
		// Decoder present: engine cannot be serverless
		if engineMode == constants.Serverless {
			engineMode = constants.RawDeployment
		}

		// Determine decoder mode (only supports single node or multi node)
		if decoder.Leader != nil || decoder.Worker != nil {
			decoderMode = constants.MultiNode
		}
	}

	// Determine router mode if present
	if router != nil {
		routerMode = determineComponentDeploymentMode(router, runtime)
	}

	// Validate compatibility
	if err := validateDeploymentModeCompatibility(engineMode, decoderMode, routerMode, engine != nil, decoder != nil, router != nil); err != nil {
		return "", "", "", err
	}

	return engineMode, decoderMode, routerMode, nil
}

// determineComponentDeploymentMode determines deployment mode for a generic component
func determineComponentDeploymentMode(spec interface{}, runtime *v1beta1.ServingRuntimeSpec) constants.DeploymentModeType {
	switch s := spec.(type) {
	case *v1beta1.EngineSpec:
		// Delegate to the existing working function
		return DetermineEngineDeploymentMode(s)
	case *v1beta1.DecoderSpec:
		if s == nil {
			return constants.RawDeployment
		}
		// Multi-node if leader and worker are defined
		if s.Leader != nil || s.Worker != nil {
			return constants.MultiNode
		}
		// Decoder never supports serverless, so default to raw deployment
		return constants.RawDeployment
	case *v1beta1.RouterSpec:
		if s == nil {
			return constants.RawDeployment
		}
		// Router doesn't have Leader/Worker, check MinReplicas for serverless
		if s.MinReplicas != nil && *s.MinReplicas == 0 {
			return constants.Serverless
		}
		return constants.RawDeployment
	}

	// Default to raw deployment for unknown types
	return constants.RawDeployment
}

// validateDeploymentModeCompatibility validates that the deployment modes are compatible
func validateDeploymentModeCompatibility(engineMode, decoderMode, routerMode constants.DeploymentModeType, hasEngine, hasDecoder, hasRouter bool) error {
	// Rule 1: If decoder is present, engine cannot be serverless
	if hasDecoder && engineMode == constants.Serverless {
		return fmt.Errorf("engine cannot use serverless deployment when decoder is present")
	}

	// Rule 2: Decoder only supports RawDeployment or MultiNode
	if hasDecoder && decoderMode == constants.Serverless {
		return fmt.Errorf("decoder does not support serverless deployment")
	}

	// Rule 3: At least engine must be present
	if !hasEngine {
		return fmt.Errorf("engine component is required")
	}

	return nil
}

// DetermineEntrypointComponent determines which component should be the main entrypoint for the InferenceService.
// Priority: Router (if present) > Engine (always present)
// This function implements the automatic routing logic: router if present, else engine.
func DetermineEntrypointComponent(isvc *v1beta1.InferenceService) v1beta1.ComponentType {
	// Auto-determine: Router takes precedence if present
	if isvc.Spec.Router != nil {
		return v1beta1.RouterComponent
	}

	// Default to engine
	return v1beta1.EngineComponent
}
