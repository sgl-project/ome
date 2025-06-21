package utils

import (
	"context"
	"fmt"
	"sort"
	"strings"

	goerrors "github.com/pkg/errors"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetServingRuntime Get a ServingRuntime by name. First, ServingRuntimes in the given namespace will be checked.
// If a resource of the specified name is not found, then ClusterServingRuntimes will be checked.
func GetServingRuntime(cl client.Client, name string, namespace string) (*v1beta1.ServingRuntimeSpec, error) {
	runtime := &v1beta1.ServingRuntime{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, runtime)
	if err == nil {
		return &runtime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}

	clusterRuntime := &v1beta1.ClusterServingRuntime{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterRuntime)
	if err == nil {
		return &clusterRuntime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No ServingRuntimes or ClusterServingRuntimes with the name: " + name)
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

// GetRuntimeForNewArchitecture retrieves the runtime for the new architecture
// It either uses the specified runtime or auto-selects based on the model
func GetRuntimeForNewArchitecture(cl client.Client, isvc *v1beta1.InferenceService, baseModel *v1beta1.BaseModelSpec) (*v1beta1.ServingRuntimeSpec, string, error) {
	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		// Use specified runtime
		rt, err := GetServingRuntime(cl, isvc.Spec.Runtime.Name, isvc.Namespace)
		if err != nil {
			return nil, "", err
		}

		if rt.IsDisabled() {
			return nil, "", fmt.Errorf("specified runtime %s is disabled", isvc.Spec.Runtime.Name)
		}

		// Verify the runtime supports the model
		if err := RuntimeSupportsModelNewArchitecture(baseModel, rt, isvc.Spec.Runtime.Name); err != nil {
			// Fill in model name in error if available
			if compatErr, ok := err.(*RuntimeCompatibilityError); ok {
				compatErr.ModelName = isvc.Spec.Model.Name
			}
			return nil, "", err
		}

		return rt, isvc.Spec.Runtime.Name, nil
	}

	// Auto-select runtime based on model
	runtimes, excludedRuntimes, err := GetSupportingRuntimesNewArchitecture(baseModel, cl, isvc.Namespace)
	if err != nil {
		return nil, "", err
	}

	if len(runtimes) == 0 {
		// Generate a detailed error message including why runtimes were excluded
		var excludedReasons []string
		for name, reason := range excludedRuntimes {
			excludedReasons = append(excludedReasons, fmt.Sprintf("%s: %v", name, reason))
		}

		errMsg := fmt.Sprintf("no runtime found to support model %s with format %s",
			isvc.Spec.Model.Name, baseModel.ModelFormat.Name)
		if len(excludedReasons) > 0 {
			sort.Strings(excludedReasons)
			errMsg += ". Excluded runtimes: " + strings.Join(excludedReasons, "; ")
		}
		return nil, "", goerrors.New(errMsg)
	}

	// Use the first supporting runtime (highest priority)
	selectedRuntime := &runtimes[0]
	return &selectedRuntime.Spec, selectedRuntime.Name, nil
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
