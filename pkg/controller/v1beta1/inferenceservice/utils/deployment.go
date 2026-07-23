package utils

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// DetermineEngineDeploymentMode determines the deployment mode for the engine based on its configuration
func DetermineEngineDeploymentMode(engine *v1beta1.EngineSpec) constants.DeploymentModeType {
	if engine == nil {
		return constants.RawDeployment
	}

	// Check for deployment mode annotation (e.g., MultiNodeRayVLLM)
	if mode, found := GetDeploymentModeFromAnnotations(engine.Annotations); found {
		return mode
	}

	// Multi-node if leader and worker are defined
	if engine.Leader != nil || engine.Worker != nil {
		return constants.MultiNode
	}

	// Default to raw deployment
	return constants.RawDeployment
}

// DetermineDeploymentModes determines the deployment modes for all components based on their specs
func DetermineDeploymentModes(engine *v1beta1.EngineSpec, decoder *v1beta1.DecoderSpec, router *v1beta1.RouterSpec, runtime *v1beta1.ServingRuntimeSpec) (engineMode, decoderMode, routerMode constants.DeploymentModeType, err error) {
	// Determine base modes for each component
	engineMode = determineComponentDeploymentMode(engine, runtime)
	decoderMode = constants.RawDeployment // Decoder only supports RawDeployment or MultiNode
	routerMode = constants.RawDeployment  // Default for router

	// Determine decoder mode (only supports single node or multi node)
	if decoder != nil && (decoder.Leader != nil || decoder.Worker != nil) {
		decoderMode = constants.MultiNode
	}

	// Determine router mode if present
	if router != nil {
		routerMode = determineComponentDeploymentMode(router, runtime)
	}

	// At least the engine must be present
	if engine == nil {
		return "", "", "", fmt.Errorf("engine component is required")
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
		return constants.RawDeployment
	case *v1beta1.RouterSpec:
		if s == nil {
			return constants.RawDeployment
		}
		// Router doesn't have Leader/Worker, so it is always a raw deployment
		return constants.RawDeployment
	}

	// Default to raw deployment for unknown types
	return constants.RawDeployment
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
