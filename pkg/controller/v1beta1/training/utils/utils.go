package utils

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

func GetFineTunedModelName(trainingJobName string) string {
	return trainingJobName[len(constants.TrainingJobNamePrefix):]
}

// GetTrainingRuntime Get a TrainingRuntime by name.
// First, TrainingRuntime in the given namespace will be checked.
// If a resource of the specified name is not found, then ClusterTrainingRuntime will be checked.
func GetTrainingRuntime(cl client.Client, name string, namespace string) (*v1beta1.TrainingRuntimeSpec, error) {
	runtime := &v1beta1.TrainingRuntime{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, runtime)
	if err == nil {
		return &runtime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}

	clusterRuntime := &v1beta1.ClusterTrainingRuntime{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterRuntime)
	if err == nil {
		return &clusterRuntime.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No TrainingRuntime or ClusterTrainingRuntime with the name: " + name)
}

func CheckFailedPodFailure(tjob *v1beta1.TrainingJob, pod v1.Pod, logger logr.Logger) (error, constants.TrainingFailedReason) {
	err, failedReason := checkFailedPodStatus(tjob, pod, logger)
	if err == nil {
		return checkPodContainerStatus(tjob, pod, logger)
	}
	return err, failedReason
}

// Check pod status with container statuses
func checkPodContainerStatus(tjob *v1beta1.TrainingJob, pod v1.Pod, logger logr.Logger) (error, constants.TrainingFailedReason) {
	if pod.Status.InitContainerStatuses != nil {
		for _, initContainerStatus := range pod.Status.InitContainerStatuses {
			initContainerName := initContainerStatus.Name
			if initContainerStatus.State.Waiting != nil {
				if isWaitingWithFailedReason(initContainerStatus.State.Waiting.Reason) {
					logger.Info("Init container failed during waiting", "tjob", tjob.Name, "podName", pod.Name, "containerName", initContainerName, "reason", initContainerStatus.State.Waiting.Reason, "message", initContainerStatus.State.Waiting.Message)
					return fmt.Errorf("init container %s failed to start: %s", initContainerName, initContainerStatus.State.Waiting.Reason), constants.TrainingContainerStartingFailed
				}
			}
			if initContainerStatus.State.Terminated != nil && initContainerStatus.State.Terminated.ExitCode != 0 {
				logger.Info("Init container terminated with non zero exit code", "tjob", tjob.Name, "podName", pod.Name, "containerName", initContainerName, "reason", initContainerStatus.State.Terminated.Reason, "message", initContainerStatus.State.Terminated.Message)
				return fmt.Errorf("init container %s terminated with exit code %d: %s", initContainerName, initContainerStatus.State.Terminated.ExitCode, initContainerStatus.State.Terminated.Reason), constants.K8SJobFailed
			}
		}
	}

	if pod.Status.ContainerStatuses != nil {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			containerName := containerStatus.Name
			if containerStatus.State.Waiting != nil {
				if isWaitingWithFailedReason(containerStatus.State.Waiting.Reason) {
					logger.Info("Container failed during waiting", "tjob", tjob.Name, "podName", pod.Name, "containerName", containerName, "reason", containerStatus.State.Waiting.Reason, "message", containerStatus.State.Waiting.Message)
					return fmt.Errorf("container %s failed to start: %s", containerName, containerStatus.State.Waiting.Reason), constants.TrainingContainerStartingFailed
				}
			}

			// Skip when main training container exited with non-zero code, let sidecar and k8s job handles it
			if containerName != constants.TrainingJobContainerName && containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
				logger.Info("Container terminated with non zero exit code", "tjob", tjob.Name, "podName", pod.Name, "containerName", containerName, "reason", containerStatus.State.Terminated.Reason, "message", containerStatus.State.Terminated.Message)
				if isDataIssue(containerStatus.State.Terminated.Message) {
					return fmt.Errorf(containerStatus.State.Terminated.Message), constants.BadTrainingData
				}
				return fmt.Errorf("container %s terminated with exit code %d: %s", containerName, containerStatus.State.Terminated.ExitCode, containerStatus.State.Terminated.Message), constants.K8SJobFailed
			}
		}
	}
	return nil, ""
}

// Check failed pod status when there are no container statuses
func checkFailedPodStatus(tjob *v1beta1.TrainingJob, pod v1.Pod, logger logr.Logger) (error, constants.TrainingFailedReason) {
	if pod.Status.Phase == v1.PodFailed && (pod.Status.Message != "" || pod.Status.Reason != "") {
		logger.Info("Training job pod failed", "tjob", tjob.Name, "podName", pod.Name, "reason", pod.Status.Reason, "message", pod.Status.Message)

		failedReason := constants.K8SJobFailed
		if pod.Status.Reason == "UnexpectedAdmissionError" {
			failedReason = constants.K8SJobUnexpectedAdmissionError
		}

		return fmt.Errorf("training job pod failed: %s", pod.Status.Message), failedReason
	}
	return nil, ""
}

func isWaitingWithFailedReason(reason string) bool {
	return reason == "ImagePullBackOff" || reason == "ErrImagePull" || reason == "CrashLoopBackOff" ||
		reason == "CreateContainerConfigError" || reason == "CreateContainerError" || reason == "ImageInspectError" ||
		reason == "InvalidImageName"
}

func isDataIssue(terminationMessage string) bool {
	// Todo: check other framework bad data issue
	return strings.Contains(terminationMessage, constants.PeftTrainingBadDataErrorMessagePrefix)
}

func GetPodsControlledByJob(cli client.Client, jobName string, namespace string) (*v1.PodList, error) {
	labelRequirementMap := map[string][]string{
		constants.TrainingJobNameLabelKey: {jobName},
	}
	selector := BuildLabelSelector(labelRequirementMap)
	pods, err := SearchPodsByLabelSelector(cli, selector, namespace)
	if err != nil {
		return nil, err
	}
	return pods, nil
}

func BuildLabelSelector(labelRequirementMap map[string][]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelRequirementMap {
		labelSelector, _ := labels.NewRequirement(key, selection.Equals, value)
		selector = selector.Add(*labelSelector)
	}
	return selector
}

func SearchPodsByLabelSelector(cli client.Client, labelSelector labels.Selector, namespace string) (*v1.PodList, error) {
	listOpt := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}

	var podList v1.PodList
	err := cli.List(context.TODO(), &podList, &listOpt)
	if err == nil {
		if len(podList.Items) > 0 {
			return &podList, nil
		}
		return nil, fmt.Errorf("no pods found with given label selector %+v under namespace %s", labelSelector, namespace)
	}
	return nil, err
}

func CheckActivePodFailureIfAny(tjob *v1beta1.TrainingJob, pod v1.Pod, logger logr.Logger) (error, constants.TrainingFailedReason) {
	err, failedReason := checkPodScheduleFailure(tjob, pod, logger)
	if err == nil {
		return checkPodContainerStatus(tjob, pod, logger)
	}
	return err, failedReason
}

// Check the scheduling failure of the pod
func checkPodScheduleFailure(tjob *v1beta1.TrainingJob, pod v1.Pod, logger logr.Logger) (error, constants.TrainingFailedReason) {
	for _, podCondition := range pod.Status.Conditions {
		// pod failed to scheduled
		if podCondition.Type == v1.PodScheduled && podCondition.Status == v1.ConditionFalse {
			podCreationTime := pod.GetCreationTimestamp().Time
			if time.Since(podCreationTime) > constants.TrainingMaxSchedulingTimeoutDuration {
				logger.Info("Pod scheduling failed", "tjob", tjob.Name, "podName", pod.Name, "message", podCondition.Message, "timeSinceCreation", time.Since(podCreationTime), "maxScheduleTimeoutDuration", constants.TrainingMaxSchedulingTimeoutDuration)
				return fmt.Errorf("pod scheduling failed: %s", podCondition.Message), constants.K8SJobSchedulingFailed
			}
			logger.Info("Pod scheduling failed but did not timeout yet", "tjob", tjob.Name, "podName", pod.Name, "message", podCondition.Message, "timeSinceCreation", time.Since(podCreationTime), "maxScheduleTimeoutDuration", constants.TrainingMaxSchedulingTimeoutDuration)
		}
	}
	return nil, ""
}

// MergeRuntimeContainers Merge the trainer Container struct with the runtime Container struct, allowing users
// to override runtime container settings from the trainer spec.
func MergeRuntimeContainers(runtimeContainer *v1.Container, trainerContainer *v1.Container) (*v1.Container, error) {
	// Save runtime container name, as the name can be overridden as empty string during the Unmarshal below
	// since the Name field does not have the 'omitempty' struct tag.
	runtimeContainerName := runtimeContainer.Name

	// Use JSON Marshal/Unmarshal to merge Container structs using strategic merge patch
	runtimeContainerJson, err := json.Marshal(runtimeContainer)
	if err != nil {
		return nil, err
	}

	overrides, err := json.Marshal(trainerContainer)
	if err != nil {
		return nil, err
	}

	mergedContainer := v1.Container{}
	jsonResult, err := strategicpatch.StrategicMergePatch(runtimeContainerJson, overrides, mergedContainer)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &mergedContainer); err != nil {
		return nil, err
	}

	if mergedContainer.Name == "" {
		mergedContainer.Name = runtimeContainerName
	}
	// Strategic merge patch will replace args but more useful behaviour here is to concatenate
	mergedContainer.Args = append(append([]string{}, runtimeContainer.Args...), trainerContainer.Args...)

	return &mergedContainer, nil
}

// MergePodSpec Merge the trainer PodSpec struct with the runtime PodSpec struct, allowing users
// to override runtime PodSpec settings from the trainer spec.
func MergePodSpec(runtimePodSpec *v1.PodSpec, trainerPodSpec *v1.PodSpec) (*v1.PodSpec, error) {
	runtimePodSpecJson, err := json.Marshal(v1.PodSpec{
		NodeSelector:     runtimePodSpec.NodeSelector,
		Affinity:         runtimePodSpec.Affinity,
		Tolerations:      runtimePodSpec.Tolerations,
		Volumes:          runtimePodSpec.Volumes,
		ImagePullSecrets: runtimePodSpec.ImagePullSecrets,
		SchedulerName:    runtimePodSpec.SchedulerName,
		HostNetwork:      runtimePodSpec.HostNetwork,
		HostIPC:          runtimePodSpec.HostIPC,
	})
	if err != nil {
		return nil, err
	}

	// Use JSON Marshal/Unmarshal to merge PodSpec structs.
	overrides, err := json.Marshal(trainerPodSpec)
	if err != nil {
		return nil, err
	}

	mergedPodSpec := v1.PodSpec{}
	jsonResult, err := strategicpatch.StrategicMergePatch(runtimePodSpecJson, overrides, mergedPodSpec)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonResult, &mergedPodSpec); err != nil {
		return nil, err
	}

	return &mergedPodSpec, nil
}

func GetContainerIndex(containerName string, containers []v1.Container) int {
	for i := range containers {
		if containers[i].Name == containerName {
			return i
		}
	}
	return -1
}

// Get a DedicatedAICluster by a reference.
func GetDedicatedAIClusterResource(cl client.Client, dedicatedAIClusterRef *v1.ObjectReference) (*v1beta1.DedicatedAICluster, error) {
	if dedicatedAIClusterRef == nil {
		return nil, nil
	}

	dedicatedAiCluster := &v1beta1.DedicatedAICluster{}
	err := cl.Get(context.TODO(), types.NamespacedName{
		Name: dedicatedAIClusterRef.Name,
	}, dedicatedAiCluster)
	if err != nil {
		return nil, err
	}

	if dedicatedAiCluster.Status.DacLifecycleState != v1beta1.ACTIVE {
		return nil, fmt.Errorf("dedicatedAiCluster %s is not in a Active life cycle state", dedicatedAIClusterRef.Name)
	}

	return dedicatedAiCluster, nil
}

func ExtractPureObjectName(objectPath string) string {
	if !strings.Contains(objectPath, "/") {
		return objectPath
	}

	values := strings.Split(objectPath, "/")
	return values[len(values)-1]
}
