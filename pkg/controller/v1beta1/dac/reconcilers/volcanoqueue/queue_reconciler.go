package volcanoqueue

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmp"
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

func NewQueueReconciler(client client.Client, scheme *runtime.Scheme, queueName string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity) (*QueueReconciler, error) {
	queue := createQueue(queueName, resources, affinity)
	return &QueueReconciler{
		client: client,
		scheme: scheme,
		Queue:  queue,
	}, nil
}

func createQueue(queueName string, resources *corev1.ResourceRequirements, affinity *corev1.Affinity) *schedulingv1beta1.Queue {
	reclaimable := false
	weight := 1
	values := extractValuesFromNodeAffinity(affinity.NodeAffinity)

	return &schedulingv1beta1.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: queueName,
		},
		Spec: schedulingv1beta1.QueueSpec{
			Reclaimable: &reclaimable,
			Weight:      int32(weight),
			Capability: corev1.ResourceList{
				"cpu":            resources.Requests[corev1.ResourceCPU],
				"memory":         resources.Requests[corev1.ResourceMemory],
				"nvidia.com/gpu": resources.Requests[corev1.ResourceName("nvidia.com/gpu")],
			},
			Guarantee: schedulingv1beta1.Guarantee{
				Resource: corev1.ResourceList{
					"cpu":            resources.Requests[corev1.ResourceCPU],
					"memory":         resources.Requests[corev1.ResourceMemory],
					"nvidia.com/gpu": resources.Requests[corev1.ResourceName("nvidia.com/gpu")],
				},
			},
			Affinity: &schedulingv1beta1.Affinity{
				NodeGroupAffinity: &schedulingv1beta1.NodeGroupAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: values,
				},
			},
		},
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
	if err := r.client.Update(context.TODO(), r.Queue, client.DryRunAll); err != nil {
		log.Error(err, "Failed to perform dry-run update of queue", "Queue", r.Queue.Name)
		return constants.CheckResultUnknown, nil, err
	}
	if diff, err := kmp.SafeDiff(r.Queue.Spec, existingQueue.Spec); err != nil {
		return constants.CheckResultUnknown, nil, err
	} else if diff != "" {
		log.Info("Queue Updated", "Diff", diff)
		return constants.CheckResultUpdate, existingQueue, nil
	}
	return constants.CheckResultExisted, existingQueue, nil
}

func (r *QueueReconciler) Reconcile() (*schedulingv1beta1.Queue, error) {
	existingQueue := &schedulingv1beta1.Queue{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.Queue.Name, Namespace: r.Queue.Namespace}, existingQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), r.Queue)
			if err != nil {
				return nil, err
			}
			return r.Queue, nil
		}
		return nil, err
	}
	return existingQueue, nil
}
