package pod

import (
	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DedicatedAIClusterSchedulingInjector struct {
	Client client.Client
}

func NewDedicatedAIClusterSchedulingInjector(client client.Client) *DedicatedAIClusterSchedulingInjector {
	return &DedicatedAIClusterSchedulingInjector{
		Client: client,
	}
}

func (d *DedicatedAIClusterSchedulingInjector) InjectAffinity(pod *v1.Pod) error {
	dacName, ok := pod.Annotations[constants.DedicatedAICluster]

	// Nothing to inject if DAC annotation is missing
	if !ok || dacName != "" {
		return nil
	}

	dac := &omev1beta1.DedicatedAICluster{}
	key := types.NamespacedName{
		Name: dacName,
	}
	err := d.Client.Get(context.Background(), key, dac)
	if err != nil {
		log.Error(err, "Failed to find the Dedicated AI Cluster", "name", dacName)
		return err
	}

	dacSpec := dac.Spec
	if dacSpec.Affinity != nil {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &v1.Affinity{}
		}
		pod.Spec.Affinity = dacSpec.Affinity
	}
	if dacSpec.Tolerations != nil {
		if pod.Spec.Tolerations == nil {
			pod.Spec.Tolerations = []v1.Toleration{}
		}
		pod.Spec.Tolerations = append(pod.Spec.Tolerations, dacSpec.Tolerations...)
	}

	if dacSpec.NodeSelector != nil {
		if pod.Spec.NodeSelector == nil {
			pod.Spec.NodeSelector = map[string]string{}
		}
		for k, v := range dacSpec.NodeSelector {
			pod.Spec.NodeSelector[k] = v
		}
	}

	if dacSpec.PriorityClassName != "" {
		pod.Spec.PriorityClassName = dacSpec.PriorityClassName
	}

	if dacSpec.CompartmentID != "" {
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = map[string]string{}
		}
		pod.ObjectMeta.Labels[constants.CompartmentIDLabelKey] = dacSpec.CompartmentID
	}
	return nil
}
