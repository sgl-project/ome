package utils

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// Returns the port from the service, or constants.CommonISVCPort as default if service lookup fails.
func GetTargetServicePort(ctx context.Context, c client.Client, isvc *v1beta1.InferenceService) (int32, error) {
	var serviceName string
	if isvc.Spec.Router != nil {
		serviceName = constants.RouterServiceName(isvc.Name)
	} else {
		serviceName = constants.EngineServiceName(isvc.Name)
	}

	// if serviceName reached 63 character, the service name will be truncated during service creation. update name otherwise the service can't found
	serviceName = constants.TruncateNameWithMaxLength(serviceName, 63)

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

// AddPreferredNodeAffinityForModel adds a preferred node affinity term to the pod spec
// for scheduling pods on nodes where the base model is ready.
// This is used by both InferenceService and BenchmarkJob controllers to ensure pods
// are scheduled on nodes with the model available.
//
// Parameters:
//   - podSpec: The pod spec to update (must not be nil)
//   - baseModelMeta: The metadata of the base model (ClusterBaseModel or BaseModel)
//
// The function:
//   - Determines the label key based on whether it's a ClusterBaseModel (empty namespace) or BaseModel
//   - Adds a preferred node affinity with weight 100 to prefer nodes with "Ready" model status
//   - Avoids adding duplicate affinity terms if one already exists for the same model
func AddPreferredNodeAffinityForModel(podSpec *corev1.PodSpec, baseModelMeta *metav1.ObjectMeta) {
	if podSpec == nil || baseModelMeta == nil {
		return
	}

	// Determine if this is a ClusterBaseModel or BaseModel based on namespace
	var labelKey string
	isClusterScoped := baseModelMeta.Namespace == ""

	if isClusterScoped {
		// ClusterBaseModel
		labelKey = constants.GetClusterBaseModelLabel(baseModelMeta.Name)
	} else {
		// BaseModel (namespace-scoped)
		labelKey = constants.GetBaseModelLabel(baseModelMeta.Namespace, baseModelMeta.Name)
	}

	// Initialize affinity structures if nil
	if podSpec.Affinity == nil {
		podSpec.Affinity = &corev1.Affinity{}
	}
	if podSpec.Affinity.NodeAffinity == nil {
		podSpec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}

	// Check if this model affinity term already exists to avoid duplicates
	affinityExists := false
	for _, term := range podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		for _, expr := range term.Preference.MatchExpressions {
			if expr.Key == labelKey {
				affinityExists = true
				break
			}
		}
		if affinityExists {
			break
		}
	}

	if !affinityExists {
		// Use max weight (100) to strongly prefer nodes with ready models
		preferredTerm := corev1.PreferredSchedulingTerm{
			Weight: 100,
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      labelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"Ready"},
					},
				},
			},
		}
		podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			podSpec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			preferredTerm,
		)
	}
}
