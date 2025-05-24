package utils

import (
	"context"
	"strconv"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	generalutils "github.com/sgl-project/sgl-ome/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDedicatedAIClusterConfigMap(client client.Client) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: constants.DedicatedAIClusterConfigMapName, Namespace: constants.OMENamespace}, configMap)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func ResourceQuantityAfterMultiply(res *resource.Quantity, count int) {
	res.Mul(int64(count))
}

func GetInt32Pointer(input int) *int32 {
	int32Input := int32(input)
	return &int32Input
}

func GetPointerOfIntOrString(val int) *intstr.IntOrString {
	return &intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(val),
	}
}

func IsVolcanoQueuePresent(client client.Client, clientConfig *rest.Config, queueName string) (bool, error) {
	isVolcanoQueueCrdPresent, err := generalutils.IsCrdAvailable(clientConfig, schedulingv1beta1.SchemeGroupVersion.String(), constants.VolcanoQueueKind)
	if err != nil {
		return false, err
	}
	if !isVolcanoQueueCrdPresent {
		return false, nil
	}
	volcanoQueue := &schedulingv1beta1.Queue{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: queueName}, volcanoQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func IsCapacityReserved(dac *v1beta1.DedicatedAICluster) (bool, error) {
	label, exists := dac.ObjectMeta.Labels[constants.DACCapacityReservedLabelKey]
	if !exists {
		return false, nil
	}

	isCapacityReserved, err := strconv.ParseBool(label)
	if err != nil {
		return false, err
	}

	return isCapacityReserved, nil
}
