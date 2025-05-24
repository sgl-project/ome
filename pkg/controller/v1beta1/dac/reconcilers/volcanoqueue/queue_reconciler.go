package volcanoqueue

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

var log = logf.Log.WithName("QueueReconciler")

type QueueReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	Queue  *schedulingv1beta1.Queue
}

func NewQueueReconciler(client client.Client, scheme *runtime.Scheme, queueName string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) (*QueueReconciler, error) {
	queue := createQueue(queueName, resources, affinity, count)
	return &QueueReconciler{
		client: client,
		scheme: scheme,
		Queue:  queue,
	}, nil
}

func createQueue(queueName string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity, count int) *schedulingv1beta1.Queue {
	reclaimable := false

	if count > 0 {
		values := extractValuesFromNodeAffinity(affinity.NodeAffinity)

		// Volcano need as least one pod buffer on CPU and Memory to start scheduling
		cpuRequest := resources.Requests[corev1.ResourceCPU]
		utils.ResourceQuantityAfterMultiply(&cpuRequest, count+1)
		memoryRequest := resources.Requests[corev1.ResourceMemory]
		utils.ResourceQuantityAfterMultiply(&memoryRequest, count+1)
		gpuRequest := resources.Requests[corev1.ResourceName("nvidia.com/gpu")]
		utils.ResourceQuantityAfterMultiply(&gpuRequest, count)

		return &schedulingv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: queueName,
			},
			Spec: schedulingv1beta1.QueueSpec{
				Reclaimable: &reclaimable,
				Weight:      1,
				Capability: corev1.ResourceList{
					"cpu":            cpuRequest,
					"memory":         memoryRequest,
					"nvidia.com/gpu": gpuRequest,
				},
				Affinity: &schedulingv1beta1.Affinity{
					NodeGroupAffinity: &schedulingv1beta1.NodeGroupAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: values,
					},
				},
			},
		}
	} else {
		return &schedulingv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: queueName,
			},
			Spec: schedulingv1beta1.QueueSpec{
				Reclaimable: &reclaimable,
				Weight:      0,
			},
		}
	}
}

func extractValuesFromNodeAffinity(nodeAffinity *corev1.NodeAffinity) []string {
	var values []string
	if nodeAffinity == nil {
		return values
	}

	for _, term := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			values = append(values, expr.Values...)
		}
	}

	return values
}

func (r *QueueReconciler) checkQueueExist() (constants.CheckResultType, *schedulingv1beta1.Queue, error) {
	existingQueue := &schedulingv1beta1.Queue{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.Queue.Name, Namespace: r.Queue.Namespace}, existingQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	// existed, check equivalent
	if semanticQueueEquals(r.Queue, existingQueue) {
		return constants.CheckResultExisted, existingQueue, nil
	}

	return constants.CheckResultUpdate, existingQueue, nil
}

func semanticQueueEquals(desired, existing *schedulingv1beta1.Queue) bool {
	return equality.Semantic.DeepEqual(desired.Spec.Weight, existing.Spec.Weight) &&
		equality.Semantic.DeepEqual(desired.Spec.Reclaimable, existing.Spec.Reclaimable) &&
		equality.Semantic.DeepEqual(desired.Spec.Capability, existing.Spec.Capability) &&
		equality.Semantic.DeepEqual(desired.Spec.Affinity, existing.Spec.Affinity)
}

func (r *QueueReconciler) Reconcile() (*schedulingv1beta1.Queue, error) {
	checkResult, queue, err := r.checkQueueExist()

	if err != nil {
		return nil, err
	}
	log.Info("queue reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.Queue)
	case constants.CheckResultUpdate:
		r.Queue.SetResourceVersion(queue.GetResourceVersion())
		opErr = r.client.Update(context.TODO(), r.Queue)
	default:
		return queue, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.Queue, nil
}
