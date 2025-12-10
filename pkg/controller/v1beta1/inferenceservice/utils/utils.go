package utils

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func LoadingMergedFineTunedWeight(fineTunedWeights []*v1beta1.FineTunedWeight) (bool, error) {
	mergedFineTunedWeights, err := IsMergedFineTunedWeight(fineTunedWeights[0])
	if err != nil {
		return false, err
	}
	return len(fineTunedWeights) == 1 && mergedFineTunedWeights, nil
}

func IsMergedFineTunedWeight(fineTunedWeight *v1beta1.FineTunedWeight) (bool, error) {
	if fineTunedWeight != nil {
		var configMap map[string]interface{}
		if err := json.Unmarshal(fineTunedWeight.Spec.Configuration.Raw, &configMap); err != nil {
			return false, err
		}
		if mergedWeights, exists := configMap[constants.FineTunedWeightMergedWeightsConfigKey]; exists && mergedWeights == true {
			return true, nil
		}
	}
	return false, nil
}

func GetScaledObjectName(isvcName string) string {
	const (
		prefix     = "scaledobject-"
		maxNameLen = 50
	)
	if len(isvcName) > maxNameLen {
		isvcName = isvcName[len(isvcName)-maxNameLen:]
	}
	return fmt.Sprintf("%s%s", prefix, isvcName)
}

// GetValueFromRawExtension extracts a value by key from a JSON-encoded runtime.RawExtension.
// It returns nil if the key does not exist or the data is not a map.
func GetValueFromRawExtension(raw runtime.RawExtension, key string) (interface{}, error) {
	if len(raw.Raw) == 0 {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw.Raw, &data); err != nil {
		return nil, err
	}

	val, ok := data[key]
	if !ok {
		return nil, nil // or optionally return an error if key must exist
	}

	return val, nil
}

// GetTargetServicePort returns the port of the target service (router or engine).
// For raw deployment mode, it uses RouterServiceName/EngineServiceName.
// For serverless mode, it uses DefaultRouterServiceName/PredictorServiceName.
// Returns the port from the service, or constants.CommonISVCPort as default if service lookup fails.
func GetTargetServicePort(ctx context.Context, c client.Client, isvc *v1beta1.InferenceService, serverless bool) (int32, error) {
	var serviceName string
	if serverless {
		// Serverless mode uses different service naming
		if isvc.Spec.Router != nil {
			serviceName = constants.DefaultRouterServiceName(isvc.Name)
		} else {
			serviceName = constants.PredictorServiceName(isvc.Name)
		}
	} else {
		// Raw deployment mode
		if isvc.Spec.Router != nil {
			serviceName = constants.RouterServiceName(isvc.Name)
		} else {
			serviceName = constants.EngineServiceName(isvc.Name)
		}
	}

	service := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: isvc.Namespace}, service); err != nil {
		return 0, err
	}

	port := int32(constants.CommonISVCPort) // default port
	if len(service.Spec.Ports) > 0 {
		port = service.Spec.Ports[0].Port
	}

	return port, nil
}
