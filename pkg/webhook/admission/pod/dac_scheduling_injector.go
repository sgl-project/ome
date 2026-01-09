package pod

import (
	"context"
	"regexp"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	dacctrl "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/dac"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// Supported DAC profile patterns for AcceleratorClass mapping
	// Format: GPU_TYPE[-MEMORY]-x<count>
	// Examples: a10-x1, a100-40g-x2, H100-x4
	acSupportedDACProfilePattern = regexp.MustCompile(`^(a10|a100-40g|a100-80g|h100|h200)-x([1248])$`)
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
	isAcSupport := false
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
			isAcSupport = checkAcSupport(profile.Name)
		}
	}

	if isAcSupport {
		log.Info("Have AC support with this DAC, skip dac inject to pod", "DAC profile", dac.Spec.Profile)
		return nil
	}

	if dacSpec.Affinity != nil {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &v1.Affinity{}
		}
		pod.Spec.Affinity = dacSpec.Affinity
	}
	if dacSpec.Resources != nil {
		if pod.Spec.Resources == nil {
			pod.Spec.Resources = &v1.ResourceRequirements{}
		}
		pod.Spec.Resources = dacSpec.Resources
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

// During another inference-ac-mutator webhook, it will add ac in inference service with corresponding dac profile.
// ac will override affinity and other components.
// when ac supported , pod mutator webhook will skip dac spec inject to pod.
func checkAcSupport(profileName string) bool {
	matches := acSupportedDACProfilePattern.FindStringSubmatch(profileName)
	return matches != nil
}
