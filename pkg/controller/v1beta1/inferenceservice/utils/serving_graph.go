package utils

import (
	"fmt"

	"github.com/go-logr/logr"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// ServingGraph captures the serving components that the controller should reconcile.
// Predictor is only retained as a compatibility marker during rollout.
type ServingGraph struct {
	InferenceService         *v1beta1.InferenceService
	Model                    *v1beta1.ModelRef
	Runtime                  *v1beta1.ServingRuntimeRef
	Engine                   *v1beta1.EngineSpec
	Decoder                  *v1beta1.DecoderSpec
	Router                   *v1beta1.RouterSpec
	PredictorCompatibility   bool
	EntrypointComponent      v1beta1.ComponentType
	EntrypointDeploymentMode constants.DeploymentModeType
	EngineDeploymentMode     constants.DeploymentModeType
	DecoderDeploymentMode    constants.DeploymentModeType
	RouterDeploymentMode     constants.DeploymentModeType
}

// IsPredictorCompatibilityMode returns true when the deprecated predictor field is the
// only serving configuration and needs translation into engine/model/runtime fields.
func IsPredictorCompatibilityMode(isvc *v1beta1.InferenceService) bool {
	return isvc != nil && IsPredictorUsed(isvc) && isvc.Spec.Engine == nil && isvc.Spec.Model == nil
}

// ResolveServingGraph translates predictor compatibility input into the shared engine-first serving graph.
func ResolveServingGraph(isvc *v1beta1.InferenceService) (*ServingGraph, error) {
	if isvc == nil {
		return nil, fmt.Errorf("inference service is required")
	}

	resolvedISVC := isvc.DeepCopy()
	predictorCompatibility := IsPredictorCompatibilityMode(resolvedISVC)
	if predictorCompatibility {
		resolvedISVC.Spec.Model = translatePredictorModelRef(&resolvedISVC.Spec.Predictor)
		resolvedISVC.Spec.Runtime = translatePredictorRuntimeRef(&resolvedISVC.Spec.Predictor)
		resolvedISVC.Spec.Engine = translatePredictorEngineSpec(&resolvedISVC.Spec.Predictor)
	}

	return newServingGraph(
		resolvedISVC,
		resolvedISVC.Spec.Model,
		resolvedISVC.Spec.Runtime,
		resolvedISVC.Spec.Engine,
		resolvedISVC.Spec.Decoder,
		resolvedISVC.Spec.Router,
		predictorCompatibility,
	), nil
}

// ResolveServingGraphWithRuntime merges runtime defaults into the resolved serving graph
// and computes deployment-mode and entrypoint selection from the shared component view.
func ResolveServingGraphWithRuntime(graph *ServingGraph, runtime *v1beta1.ServingRuntimeSpec, log logr.Logger) (*ServingGraph, error) {
	if graph == nil || graph.InferenceService == nil {
		return nil, fmt.Errorf("serving graph is required")
	}

	resolvedISVC := graph.InferenceService.DeepCopy()
	engine, decoder, router, err := MergeRuntimeSpecs(resolvedISVC, runtime, log)
	if err != nil {
		return nil, err
	}

	resolvedGraph := newServingGraph(
		resolvedISVC,
		resolvedISVC.Spec.Model,
		resolvedISVC.Spec.Runtime,
		engine,
		decoder,
		router,
		graph.PredictorCompatibility,
	)

	if engine == nil && decoder == nil && router == nil {
		resolvedGraph.EntrypointDeploymentMode = constants.RawDeployment
		return resolvedGraph, nil
	}

	engineMode, decoderMode, routerMode, err := DetermineDeploymentModes(engine, decoder, router, runtime)
	if err != nil {
		return nil, err
	}

	resolvedGraph.EngineDeploymentMode = engineMode
	resolvedGraph.DecoderDeploymentMode = decoderMode
	resolvedGraph.RouterDeploymentMode = routerMode
	resolvedGraph.EntrypointDeploymentMode = resolvedGraph.DeploymentModeFor(resolvedGraph.EntrypointComponent)
	return resolvedGraph, nil
}

// DeploymentModeFor returns the deployment mode for the requested component.
func (g *ServingGraph) DeploymentModeFor(component v1beta1.ComponentType) constants.DeploymentModeType {
	if g == nil {
		return constants.RawDeployment
	}

	switch component {
	case v1beta1.RouterComponent:
		if g.Router != nil {
			return g.RouterDeploymentMode
		}
	case v1beta1.DecoderComponent:
		if g.Decoder != nil {
			return g.DecoderDeploymentMode
		}
	case v1beta1.PredictorComponent:
		fallthrough
	case v1beta1.EngineComponent:
		if g.Engine != nil {
			return g.EngineDeploymentMode
		}
	}

	return constants.RawDeployment
}

func newServingGraph(
	isvc *v1beta1.InferenceService,
	model *v1beta1.ModelRef,
	runtime *v1beta1.ServingRuntimeRef,
	engine *v1beta1.EngineSpec,
	decoder *v1beta1.DecoderSpec,
	router *v1beta1.RouterSpec,
	predictorCompatibility bool,
) *ServingGraph {
	graph := &ServingGraph{
		InferenceService:       isvc,
		Model:                  model,
		Runtime:                runtime,
		Engine:                 engine,
		Decoder:                decoder,
		Router:                 router,
		PredictorCompatibility: predictorCompatibility,
	}
	graph.EntrypointComponent = determineServingGraphEntrypoint(graph)
	return graph
}

func determineServingGraphEntrypoint(graph *ServingGraph) v1beta1.ComponentType {
	if graph != nil && graph.Router != nil {
		return v1beta1.RouterComponent
	}

	return v1beta1.EngineComponent
}
