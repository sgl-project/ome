package kueuequeue

import (
	"context"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

var lqLog = logf.Log.WithName("LocalQueueReconciler")

type LocalQueueReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	LocalQueue *kueuev1beta1.LocalQueue
}

func NewLocalQueueReconciler(client client.Client, scheme *runtime.Scheme, queueName string) *LocalQueueReconciler {
	localQueue := createLocalQueue(queueName)
	return &LocalQueueReconciler{
		client:     client,
		scheme:     scheme,
		LocalQueue: localQueue,
	}
}

func createLocalQueue(queueName string) *kueuev1beta1.LocalQueue {
	return &kueuev1beta1.LocalQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      queueName,
			Namespace: queueName,
		},
		Spec: kueuev1beta1.LocalQueueSpec{
			ClusterQueue: kueuev1beta1.ClusterQueueReference(queueName),
			StopPolicy:   &constants.DefaultStopPolicy,
		},
	}
}

func (r *LocalQueueReconciler) Reconcile() (*kueuev1beta1.LocalQueue, error) {
	checkResult, existingLocalQueue, err := r.checkLocalQueueExist()
	if err != nil {
		return nil, err
	}
	lqLog.Info("DAC local queue reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.LocalQueue)
	case constants.CheckResultUpdate:
		r.LocalQueue.SetResourceVersion(existingLocalQueue.GetResourceVersion())
		opErr = r.client.Update(context.TODO(), r.LocalQueue)
	default:
		return existingLocalQueue, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.LocalQueue, nil
}

func (r *LocalQueueReconciler) checkLocalQueueExist() (constants.CheckResultType, *kueuev1beta1.LocalQueue, error) {
	existingLocalQueue := &kueuev1beta1.LocalQueue{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.LocalQueue.ObjectMeta.Name, Namespace: r.LocalQueue.Namespace}, existingLocalQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	diff, err := kmp.SafeDiff(r.LocalQueue.Spec, existingLocalQueue.Spec)
	if err != nil {
		return constants.CheckResultUnknown, nil, err
	}
	if diff != "" {
		lqLog.Info("LocalQueue diff", "name", r.LocalQueue.Name, "diff", diff)
		return constants.CheckResultUpdate, existingLocalQueue, nil
	}

	return constants.CheckResultExisted, existingLocalQueue, nil
}
