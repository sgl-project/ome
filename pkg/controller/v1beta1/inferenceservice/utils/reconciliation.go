package utils

import (
	"context"
	"fmt"

	goerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

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

// GetFineTunedWeight Get the fine-tuned weight from the given fine-tuned weight name.
func GetFineTunedWeight(cl client.Client, name string) (*v1beta1.FineTunedWeight, error) {
	fineTunedWeight := &v1beta1.FineTunedWeight{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name}, fineTunedWeight)
	if err == nil {
		return fineTunedWeight, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No FineTunedWeight with the name: " + name)
}

// ReconcileBaseModel retrieves and validates the base model for an InferenceService
func ReconcileBaseModel(cl client.Client, isvc *v1beta1.InferenceService) (*v1beta1.BaseModelSpec, *metav1.ObjectMeta, error) {
	if isvc.Spec.Model == nil || isvc.Spec.Model.Name == "" {
		return nil, nil, goerrors.New("model reference is required")
	}

	baseModel, baseModelMeta, err := GetBaseModel(cl, isvc.Spec.Model.Name, isvc.Namespace)
	if err != nil {
		return nil, nil, err
	}

	if baseModel.Disabled != nil && *baseModel.Disabled {
		return nil, nil, fmt.Errorf("specified base model %s is disabled", isvc.Spec.Model.Name)
	}

	return baseModel, baseModelMeta, nil
}

// MergeRuntimeSpecs merges the runtime and isvc specs to get final engine, decoder, and router specs
func MergeRuntimeSpecs(isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec) (*v1beta1.EngineSpec, *v1beta1.DecoderSpec, *v1beta1.RouterSpec, error) {
	var runtimeEngine *v1beta1.EngineSpec
	var runtimeDecoder *v1beta1.DecoderSpec
	var runtimeRouter *v1beta1.RouterSpec

	// Extract runtime specs if available
	if runtime != nil {
		runtimeEngine = runtime.EngineConfig
		runtimeDecoder = runtime.DecoderConfig
		runtimeRouter = runtime.RouterConfig
	}

	// Merge engine specs
	mergedEngine, err := MergeEngineSpec(runtimeEngine, isvc.Spec.Engine)
	if err != nil {
		return nil, nil, nil, goerrors.Wrap(err, "failed to merge engine specs")
	}

	// Merge decoder specs
	mergedDecoder, err := MergeDecoderSpec(runtimeDecoder, isvc.Spec.Decoder)
	if err != nil {
		return nil, nil, nil, goerrors.Wrap(err, "failed to merge decoder specs")
	}

	// Merge router specs
	mergedRouter, err := MergeRouterSpec(isvc.Spec.Router, runtimeRouter)
	if err != nil {
		return nil, nil, nil, goerrors.Wrap(err, "failed to merge router specs")
	}

	return mergedEngine, mergedDecoder, mergedRouter, nil
}
