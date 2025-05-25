package pod

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

// +kubebuilder:webhook:path=/mutate-pods,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create,versions=v1,name=inferenceservice.ome-webhook-server.pod-mutator,reinvocationPolicy=IfNeeded
// +kubebuilder:webhook:path=/mutate-training-pods,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create,versions=v1,name=trainingjob.ome-webhook-server.pod-mutator,reinvocationPolicy=IfNeeded
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

	log.Info("mutating pod", "name", podName)

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

	log.Info("mutating pod completed", "pod", pod)

	patch, err := json.Marshal(pod)
	if err != nil {
		log.Error(err, "Failed to marshal pod", "name", podName)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("parsing pod completed", "name", podName)

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

	modelInitInjector := newModelInitInjector(configMap)

	fineTunedAdapterInjector := newFineTunedAdapterInjector(configMap, mutator.Client)

	servingSidecarInjector := newServingSidecarInjector(configMap)

	mutators := []func(pod *v1.Pod) error{
		agentInjector.InjectAgent,
		metricsAggregator.InjectMetricsAggregator,
		modelInitInjector.InjectModelInit,
		fineTunedAdapterInjector.InjectFineTunedAdapter,
		servingSidecarInjector.InjectServingSidecar,
	}

	for _, mutator := range mutators {
		if err := mutator(pod); err != nil {
			return err
		}
	}

	// Now sort InitContainers to ensure the order (Model Init must run before FineTuned Adapter)
	sort.SliceStable(pod.Spec.InitContainers, func(i, j int) bool {
		// Logic to ensure Model Init runs first, then FineTuned Adapter
		if pod.Spec.InitContainers[i].Name == constants.ModelInitContainerName && pod.Spec.InitContainers[j].Name == constants.FineTunedAdapterContainerName {
			return true // Model Init must come first
		}
		if pod.Spec.InitContainers[i].Name == constants.FineTunedAdapterContainerName && pod.Spec.InitContainers[j].Name == constants.ModelInitContainerName {
			return false // FineTuned Adapter must come second
		}
		return i < j // For all other containers, retain original order
	})

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
