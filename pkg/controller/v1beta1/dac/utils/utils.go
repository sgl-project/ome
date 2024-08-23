package utils

import (
	"context"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDedicatedAIClausterConfigMap(client client.Client, ) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: constants.DedicatedAIClusterConfigMapName, Namespace: constants.OMENamespace}, configMap)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}