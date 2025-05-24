package pod

import (
	"context"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	dacctrl "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
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
	if !ok || dacName == "" {
		log.Info("No DAC annotation, skip the DAC scheduling injection", "podName", pod.Name)
		return nil
	}

	dac := &omev1beta1.DedicatedAICluster{}
	err := d.Client.Get(context.Background(), types.NamespacedName{Name: dacName}, dac)
	if err != nil {
		log.Error(err, "Failed to find the Dedicated AI Cluster", "name", dacName)
		return err
	}

	// Get DAC spec by merging with the profile if specified
	dacSpec := dac.Spec.DeepCopy()
	// If a profile is specified, fetch the corresponding DedicatedAIClusterProfile
	if dac.Spec.Profile != "" {
		profile := &omev1beta1.DedicatedAIClusterProfile{}
		if err := d.Client.Get(context.Background(), types.NamespacedName{Name: dac.Spec.Profile}, profile); err != nil {
			if apierr.IsNotFound(err) {
				log.Error(err, "Non-blocking error: DAC profile not found in DAC scheduling injector", "DAC profile name", dac.Spec.Profile)
			}
			log.Error(err, "Non-blocking error: failed to get DAC profile in DAC scheduling injector", "DAC profile name", dac.Spec.Profile)
		} else {
			dacSpec = dacctrl.MergeSpecs(&profile.Spec, dacSpec)
		}
	}

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
