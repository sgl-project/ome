package kueuequeue

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

var cqLog = logf.Log.WithName("ClusterQueueReconciler")

type ClusterQueueReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	ClusterQueue *kueuev1beta1.ClusterQueue
}

func NewClusterQueueReconciler(client client.Client, scheme *runtime.Scheme, queueName string, resources *corev1.ResourceRequirements, count int) *ClusterQueueReconciler {
	clusterQueue := createClusterQueue(queueName, resources, count)
	return &ClusterQueueReconciler{
		client:       client,
		scheme:       scheme,
		ClusterQueue: clusterQueue,
	}
}

func createClusterQueue(queueName string, resources *corev1.ResourceRequirements, count int) *kueuev1beta1.ClusterQueue {
	// Require extra resources for rolling update
	cpuRequest := resources.Requests[corev1.ResourceCPU]
	utils.ResourceQuantityAfterMultiply(&cpuRequest, count+1)
	memoryRequest := resources.Requests[corev1.ResourceMemory]
	utils.ResourceQuantityAfterMultiply(&memoryRequest, count+1)
	gpuRequest := resources.Requests[corev1.ResourceName(constants.NvidiaGPUResourceType)]
	utils.ResourceQuantityAfterMultiply(&gpuRequest, count+1)

	return &kueuev1beta1.ClusterQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name: queueName,
		},
		Spec: kueuev1beta1.ClusterQueueSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					// This selects namespaces that can submit workloads to this queue.
					corev1.LabelMetadataName: queueName,
				},
			},
			ResourceGroups: []kueuev1beta1.ResourceGroup{
				{
					CoveredResources: []corev1.ResourceName{
						constants.NvidiaGPUResourceType,
						constants.CPUResourceType,
						constants.MemoryResourceType,
					},
					Flavors: []kueuev1beta1.FlavorQuotas{
						{
							Name: "default-flavor",
							Resources: []kueuev1beta1.ResourceQuota{
								{
									Name:         constants.NvidiaGPUResourceType,
									NominalQuota: gpuRequest,
								},
								{
									Name:         constants.CPUResourceType,
									NominalQuota: cpuRequest,
								},
								{
									Name:         constants.MemoryResourceType,
									NominalQuota: memoryRequest,
								},
							},
						},
					},
				},
			},
			QueueingStrategy:  constants.DefaultQueueingStrategy,
			FlavorFungibility: &constants.DefaultFlavorFungibility,
			Preemption:        &constants.DefaultPreemptionConfig,
			StopPolicy:        &constants.DefaultStopPolicy,
		},
	}
}

func (r *ClusterQueueReconciler) Reconcile() (*kueuev1beta1.ClusterQueue, error) {
	checkResult, existingClusterQueue, err := r.checkClusterQueueExist()
	if err != nil {
		return nil, err
	}
	cqLog.Info("DAC cluster queue reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.ClusterQueue)
	case constants.CheckResultUpdate:
		r.ClusterQueue.SetResourceVersion(existingClusterQueue.GetResourceVersion())
		opErr = r.client.Update(context.TODO(), r.ClusterQueue)
	default:
		return existingClusterQueue, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.ClusterQueue, nil
}

func (r *ClusterQueueReconciler) checkClusterQueueExist() (constants.CheckResultType, *kueuev1beta1.ClusterQueue, error) {
	existingClusterQueue := &kueuev1beta1.ClusterQueue{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.ClusterQueue.ObjectMeta.Name}, existingClusterQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	diff, err := kmp.SafeDiff(r.ClusterQueue.Spec, existingClusterQueue.Spec)
	if err != nil {
		return constants.CheckResultUnknown, nil, err
	}
	if diff != "" {
		cqLog.Info("ClusterQueue diff", "name", r.ClusterQueue.Name, "diff", diff)
		return constants.CheckResultUpdate, existingClusterQueue, nil
	}

	return constants.CheckResultExisted, existingClusterQueue, nil
}
