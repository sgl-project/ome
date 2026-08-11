package utils

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// DetermineEngineDeploymentMode determines the deployment mode for the engine based on its configuration.
//
// Resolution order:
//  1. Per-Component annotation (ome.io/deploymentMode) — highest priority,
//     preserved as the operator escape hatch for mixed-mode experiments.
//  2. specMode — the typed top-level spec.deploymentMode field. When set,
//     propagates to every Component without mutating per-Component
//     annotations (kept clean for `kubectl get -o yaml`).
//  3. Leader/Worker shape inference → MultiNode.
//  4. Default → RawDeployment.
func DetermineEngineDeploymentMode(engine *v1beta1.EngineSpec, specMode *constants.DeploymentModeType) constants.DeploymentModeType {
	if engine == nil {
		return constants.RawDeployment
	}

	// Check for deployment mode annotation
	if mode, found := GetDeploymentModeFromAnnotations(engine.Annotations); found {
		return mode
	}

	if mode, found := deploymentModeFromSpecField(specMode); found {
		return mode
	}

	// Multi-node if leader and worker are defined
	if engine.Leader != nil || engine.Worker != nil {
		return constants.MultiNode
	}

	// Default to raw deployment
	return constants.RawDeployment
}

// DetermineDeploymentModes determines the deployment modes for all components based on their specs.
// See DetermineEngineDeploymentMode for the precedence chain that also governs the Decoder.
func DetermineDeploymentModes(engine *v1beta1.EngineSpec, decoder *v1beta1.DecoderSpec, router *v1beta1.RouterSpec, runtime *v1beta1.ServingRuntimeSpec, specMode *constants.DeploymentModeType) (engineMode, decoderMode, routerMode constants.DeploymentModeType, err error) {
	engineMode = determineComponentDeploymentMode(engine, runtime, specMode)
	decoderMode = determineComponentDeploymentMode(decoder, runtime, specMode)
	routerMode = determineComponentDeploymentMode(router, runtime, specMode)

	// At least the engine must be present
	if engine == nil {
		return "", "", "", fmt.Errorf("engine component is required")
	}

	return engineMode, decoderMode, routerMode, nil
}

// determineComponentDeploymentMode determines deployment mode for a generic component.
// Per-Component annotation > spec.deploymentMode (specMode) > Leader/Worker shape
// (MultiNode) > RawDeployment default. The Router has no Leader/Worker shape and its
// reconciler only supports RawDeployment, so it always resolves to RawDeployment.
func determineComponentDeploymentMode(spec interface{}, runtime *v1beta1.ServingRuntimeSpec, specMode *constants.DeploymentModeType) constants.DeploymentModeType {
	switch s := spec.(type) {
	case *v1beta1.EngineSpec:
		// Delegate to the existing working function
		return DetermineEngineDeploymentMode(s, specMode)
	case *v1beta1.DecoderSpec:
		if s == nil {
			return constants.RawDeployment
		}
		if mode, found := GetDeploymentModeFromAnnotations(s.Annotations); found {
			return mode
		}
		if mode, found := deploymentModeFromSpecField(specMode); found {
			return mode
		}
		// Multi-node if leader and worker are defined
		if s.Leader != nil || s.Worker != nil {
			return constants.MultiNode
		}
		return constants.RawDeployment
	case *v1beta1.RouterSpec:
		return constants.RawDeployment
	}

	// Default to raw deployment for unknown types
	return constants.RawDeployment
}

// deploymentModeFromSpecField reads the typed top-level spec.deploymentMode
// value; unset or invalid values are ignored (the webhook validates the
// field, so invalid only occurs on admission-bypassed writes).
func deploymentModeFromSpecField(specMode *constants.DeploymentModeType) (constants.DeploymentModeType, bool) {
	if specMode == nil {
		return "", false
	}
	if !specMode.IsValid() {
		return "", false
	}
	return *specMode, true
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
