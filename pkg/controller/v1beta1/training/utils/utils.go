package utils

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
