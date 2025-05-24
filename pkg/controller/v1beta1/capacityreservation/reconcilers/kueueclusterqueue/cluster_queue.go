package kueueclusterqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/constants"
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

var log = logf.Log.WithName("ClusterQueueReconciler")

type ClusterQueueReconciler struct {
	client                      client.Client
	scheme                      *runtime.Scheme
	ClusterQueue                *kueuev1beta1.ClusterQueue
	CreationFailedTimeThreshold time.Duration
}

type ClusterQueueConfig struct {
	CreationFailedTimeThresholdSecond int `json:"creationFailedTimeThresholdSecond"`
}

func NewClusterQueueReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	clusterQueueName string,
	resourceGroups []kueuev1beta1.ResourceGroup,
	cohort string,
	preemptionRule *kueuev1beta1.ClusterQueuePreemption,
) (*ClusterQueueReconciler, error) {
	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: constants.CapacityReservationConfigMapName, Namespace: constants.OMENamespace}, configMap)
	if err != nil {
		return nil, err
	}
	clusterQueueConfig := &ClusterQueueConfig{}
	if cqConfig, ok := configMap.Data["clusterQueue"]; ok {
		err = json.Unmarshal([]byte(cqConfig), &clusterQueueConfig)
		if err != nil {
			panic(fmt.Errorf("unable to unmarshall %v json string due to %v ", "clusterQueue", err))
		}
	} else {
		panic(fmt.Errorf("missing the %v json config in the capacityreservation-config ConfigMap", "clusterQueue"))
	}

	if cohort == "" {
		cohort = constants.DedicatedServingCohort
	}
	if preemptionRule == nil {
		preemptionRule = &constants.DefaultPreemptionConfig
	}
	defaultPreemptionConfig := &constants.DefaultPreemptionConfig
	if preemptionRule.BorrowWithinCohort == nil {
		preemptionRule.BorrowWithinCohort = defaultPreemptionConfig.BorrowWithinCohort
	}
	if preemptionRule.ReclaimWithinCohort == "" {
		preemptionRule.ReclaimWithinCohort = defaultPreemptionConfig.ReclaimWithinCohort
	}
	if preemptionRule.WithinClusterQueue == "" {
		preemptionRule.WithinClusterQueue = defaultPreemptionConfig.WithinClusterQueue
	}

	clusterQueue := &kueuev1beta1.ClusterQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterQueueName,
		},
		Spec: kueuev1beta1.ClusterQueueSpec{
			ResourceGroups:    resourceGroups,
			Cohort:            cohort,
			Preemption:        preemptionRule,
			QueueingStrategy:  constants.DefaultQueueingStrategy,
			StopPolicy:        &constants.DefaultStopPolicy,
			FlavorFungibility: &constants.DefaultFlavorFungibility,
		},
	}
	return &ClusterQueueReconciler{
		client:                      client,
		scheme:                      scheme,
		ClusterQueue:                clusterQueue,
		CreationFailedTimeThreshold: time.Duration(clusterQueueConfig.CreationFailedTimeThresholdSecond) * time.Second,
	}, nil
}

func (r *ClusterQueueReconciler) checkExist() (constants.CheckResultType, *kueuev1beta1.ClusterQueue, error) {
	existingClusterQueue := &kueuev1beta1.ClusterQueue{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.ClusterQueue.Name}, existingClusterQueue)
	if err != nil {
		if errors.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	// existed, check equivalent
	r.ClusterQueue.SetResourceVersion(existingClusterQueue.GetResourceVersion())

	// kmp.SafeDiff(x, y) output format json { -x +y }
	diff, err := kmp.SafeDiff(existingClusterQueue.Spec, r.ClusterQueue.Spec)
	if err != nil {
		return constants.CheckResultUnknown, nil, err
	}
	if diff != "" {
		log.Info("ClusterQueue diff", "name", r.ClusterQueue.Name, "diff", diff)
		return constants.CheckResultUpdate, existingClusterQueue, nil
	}
	return constants.CheckResultExisted, existingClusterQueue, nil
}

func (r *ClusterQueueReconciler) Reconcile() (*kueuev1beta1.ClusterQueue, error) {
	checkResult, clusterQueue, err := r.checkExist()
	if err != nil {
		return nil, err
	}
	log.Info("ClusterQueue reconcile", "checkResult", checkResult, "err", err)

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.ClusterQueue)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.ClusterQueue)
	default:
		return clusterQueue, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.ClusterQueue, nil
}
