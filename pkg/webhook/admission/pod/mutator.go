package pod

import (
	"context"
	"encoding/json"
	"net/http"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
)

// +kubebuilder:webhook:path=/mutate-pods,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create,versions=v1,name=inferenceservice.ome-webhook-server.pod-mutator,reinvocationPolicy=IfNeeded
var log = logf.Log.WithName(constants.PodMutatorWebhookName)

// Mutator is a webhook that injects incoming pods
type Mutator struct {
	Client    client.Client
	Clientset kubernetes.Interface
	Decoder   admission.Decoder
}

// Handle decodes the incoming Pod and executes mutation logic.
func (mutator *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &v1.Pod{}

	podName := getPodName(pod)

	if err := mutator.Decoder.Decode(req, pod); err != nil {
		log.Error(err, "Failed to decode pod", "name", podName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !needMutate(pod) {
		return admission.ValidationResponse(true, "")
	}

	configMap, err := mutator.Clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(),
		constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Failed to find config map", "name", constants.InferenceServiceConfigMapName)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// For some reason pod namespace is always empty when coming to pod mutator, need to set from admission request
	pod.Namespace = req.AdmissionRequest.Namespace

	if err := mutator.mutate(pod, configMap); err != nil {
		log.Error(err, "Failed to mutate pod", "name", podName)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	patch, err := json.Marshal(pod)
	if err != nil {
		log.Error(err, "Failed to marshal pod", "name", podName)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.AdmissionRequest.Object.Raw, patch)
}

func (mutator *Mutator) mutate(pod *v1.Pod, configMap *v1.ConfigMap) error {

	loggerConfig, err := getLoggerConfigs(configMap)
	if err != nil {
		return err
	}

	agentConfig, err := getAgentConfigs(configMap)
	if err != nil {
		return err
	}

	agentInjector := &AgentInjector{
		agentConfig:  agentConfig,
		loggerConfig: loggerConfig,
	}

	metricsAggregator, err := newMetricsAggregator(configMap)
	if err != nil {
		return err
	}

	dedicatedAIClusterSchedulingInjector := NewDedicatedAIClusterSchedulingInjector(mutator.Client)

	modelInitInjector := newModelInitInjector(configMap)

	mergedFinetunedAdapterInjector := newMergedFinetunedAdapterInjector(configMap, mutator.Client, pod.Namespace)

	servingSidecarInjector := newServingSidecarInjector(configMap)

	trainingSidecarInjector := newTrainingSidecarInjector(configMap)

	mutators := []func(pod *v1.Pod) error{
		agentInjector.InjectAgent,
		metricsAggregator.InjectMetricsAggregator,
		dedicatedAIClusterSchedulingInjector.InjectAffinity,
		modelInitInjector.InjectModelInit,
		mergedFinetunedAdapterInjector.InjectMergedFinetunedAdapter,
		servingSidecarInjector.InjectServingSidecar,
		trainingSidecarInjector.InjectTrainingSidecar,
	}

	for _, mutator := range mutators {
		if err := mutator(pod); err != nil {
			return err
		}
	}

	return nil
}

func getPodName(pod *v1.Pod) string {
	var podName string
	_, ok := pod.Labels[constants.TrainingJobPodLabelKey]
	if ok {
		podName = pod.Labels[constants.TrainingJobPodLabelKey]
	} else {
		podName = pod.Labels[constants.InferenceServicePodLabelKey]
	}
	return podName
}

func needMutate(pod *v1.Pod) bool {
	// Skip webhook if pod not managed by ome
	_, inferencePodLabel := pod.Labels[constants.InferenceServicePodLabelKey]
	_, trainingPodLabel := pod.Labels[constants.TrainingJobPodLabelKey]
	return inferencePodLabel || trainingPodLabel
}
