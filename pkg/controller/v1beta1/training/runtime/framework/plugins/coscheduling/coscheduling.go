package coscheduling

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
	runtimeindexer "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/indexer"
)

type CoScheduling struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
}

var _ framework.EnforcePodGroupPolicyPlugin = (*CoScheduling)(nil)
var _ framework.WatchExtensionPlugin = (*CoScheduling)(nil)
var _ framework.ComponentBuilderPlugin = (*CoScheduling)(nil)

var (
	ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer        = errors.New("setting index on runtimeClass for TrainingRuntime")
	ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer = errors.New("setting index on runtimeClass for ClusterTrainingRuntime")
)

const Name = "CoScheduling"

// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups,verbs=get;list;watch;create

func New(ctx context.Context, c client.Client, indexer client.FieldIndexer) (framework.Plugin, error) {
	if err := indexer.IndexField(ctx, &omev1beta1.TrainingRuntime{}, TrainingRuntimeContainerRuntimeClassKey,
		IndexTrainingRuntimeContainerRuntimeClass); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer, err)
	}
	if err := indexer.IndexField(ctx, &omev1beta1.ClusterTrainingRuntime{}, ClusterTrainingRuntimeContainerRuntimeClassKey,
		IndexClusterTrainingRuntimeContainerRuntimeClass); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer, err)
	}
	return &CoScheduling{
		client:     c,
		restMapper: c.RESTMapper(),
		scheme:     c.Scheme(),
	}, nil
}

func (c *CoScheduling) Name() string {
	return Name
}

func (c *CoScheduling) EnforcePodGroupPolicy(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || trainJob == nil {
		return nil
	}

	if info.Scheduler.PodLabels == nil {
		info.Scheduler.PodLabels = make(map[string]string, 1)
	}
	info.Scheduler.PodLabels[schedulerpluginsv1alpha1.PodGroupLabel] = trainJob.Name
	return nil
}

func (c *CoScheduling) Build(ctx context.Context, _ client.Object, info *runtime.Info, trainJob *omev1beta1.TrainingJob) (client.Object, error) {
	if info == nil || info.RuntimePolicy.PodGroupPolicy == nil || info.RuntimePolicy.PodGroupPolicy.CoschedulingPodGroupPolicyConfig == nil || trainJob == nil {
		return nil, nil
	}

	var totalMembers int32
	totalResources := make(corev1.ResourceList)
	for _, resourceRequests := range info.TotalRequests {
		totalMembers += resourceRequests.Replicas
		for resName, quantity := range resourceRequests.PodRequests {
			quantity.Mul(int64(resourceRequests.Replicas))
			current := totalResources[resName]
			current.Add(quantity)
			totalResources[resName] = current
		}
	}
	newPG := &schedulerpluginsv1alpha1.PodGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedulerpluginsv1alpha1.SchemeGroupVersion.String(),
			Kind:       constants.PodGroupKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      trainJob.Name,
			Namespace: trainJob.Namespace,
		},
		Spec: schedulerpluginsv1alpha1.PodGroupSpec{
			ScheduleTimeoutSeconds: info.RuntimePolicy.PodGroupPolicy.CoschedulingPodGroupPolicyConfig.ScheduleTimeoutSeconds,
			MinMember:              totalMembers,
			MinResources:           totalResources,
		},
	}
	if err := ctrlutil.SetControllerReference(trainJob, newPG, c.scheme); err != nil {
		return nil, err
	}
	oldPG := &schedulerpluginsv1alpha1.PodGroup{}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(newPG), oldPG); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		oldPG = nil
	}
	if needsCreateOrUpdate(oldPG, newPG, ptr.Deref(trainJob.Spec.Suspend, false)) {
		return newPG, nil
	}
	return nil, nil
}

func needsCreateOrUpdate(old, new *schedulerpluginsv1alpha1.PodGroup, suspended bool) bool {
	return old == nil ||
		suspended && (!equality.Semantic.DeepEqual(old.Spec, new.Spec) || !maps.Equal(old.Labels, new.Labels) || !maps.Equal(old.Annotations, new.Annotations))
}

type PodGroupRuntimeClassHandler struct {
	client client.Client
}

var _ handler.TypedEventHandler[*nodev1.RuntimeClass, reconcile.Request] = (*PodGroupRuntimeClassHandler)(nil)

func (h *PodGroupRuntimeClassHandler) Create(ctx context.Context, e event.TypedCreateEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	containerRuntimeClass := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(containerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, containerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	newContainerRuntimeClass := e.ObjectNew
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(newContainerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, newContainerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*nodev1.RuntimeClass], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	containerRuntimeClass := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("runtimeClass", klog.KObj(containerRuntimeClass))
	if err := h.queueSuspendedTrainJobs(ctx, containerRuntimeClass, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupRuntimeClassHandler) Generic(context.Context, event.TypedGenericEvent[*nodev1.RuntimeClass], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *PodGroupRuntimeClassHandler) queueSuspendedTrainJobs(ctx context.Context, runtimeClass *nodev1.RuntimeClass, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	var trainingRuntimes omev1beta1.TrainingRuntimeList
	if err := h.client.List(ctx, &trainingRuntimes, client.MatchingFields{TrainingRuntimeContainerRuntimeClassKey: runtimeClass.Name}); err != nil {
		return err
	}
	var clusterTrainingRuntimes omev1beta1.ClusterTrainingRuntimeList
	if err := h.client.List(ctx, &clusterTrainingRuntimes, client.MatchingFields{ClusterTrainingRuntimeContainerRuntimeClassKey: runtimeClass.Name}); err != nil {
		return err
	}

	var trainJobs []omev1beta1.TrainingJob
	for _, trainingRuntime := range trainingRuntimes.Items {
		var trainJobsWithTrainingRuntime omev1beta1.TrainingJobList
		err := h.client.List(ctx, &trainJobsWithTrainingRuntime, client.MatchingFields{runtimeindexer.TrainJobRuntimeRefKey: trainingRuntime.Name})
		if err != nil {
			return err
		}
		trainJobs = append(trainJobs, trainJobsWithTrainingRuntime.Items...)
	}
	for _, clusterTrainingRuntime := range clusterTrainingRuntimes.Items {
		var trainJobsWithClTrainingRuntime omev1beta1.TrainingJobList
		err := h.client.List(ctx, &trainJobsWithClTrainingRuntime, client.MatchingFields{runtimeindexer.TrainJobClusterRuntimeRefKey: clusterTrainingRuntime.Name})
		if err != nil {
			return err
		}
		trainJobs = append(trainJobs, trainJobsWithClTrainingRuntime.Items...)
	}
	trainJobs = slices.CompactFunc(trainJobs, func(a, b omev1beta1.TrainingJob) bool {
		return a.Name == b.Name
	})
	for _, trainJob := range trainJobs {
		if ptr.Deref(trainJob.Spec.Suspend, false) {
			q.Add(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&trainJob)})
		}
	}
	return nil
}

type PodGroupLimitRangeHandler struct {
	client client.Client
}

var _ handler.TypedEventHandler[*corev1.LimitRange, reconcile.Request] = (*PodGroupLimitRangeHandler)(nil)

func (h *PodGroupLimitRangeHandler) Create(ctx context.Context, e event.TypedCreateEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	limitRange := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(limitRange))
	if err := h.queueSuspendedTrainJob(ctx, limitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	newLimitRange := e.ObjectNew
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(newLimitRange))
	if err := h.queueSuspendedTrainJob(ctx, newLimitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*corev1.LimitRange], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	limitRange := e.Object
	log := ctrl.LoggerFrom(ctx).WithValues("limitRange", klog.KObj(limitRange))
	if err := h.queueSuspendedTrainJob(ctx, limitRange.Namespace, q); err != nil {
		log.Error(err, "could not queue suspended TrainJob to reconcile queue")
	}
}

func (h *PodGroupLimitRangeHandler) Generic(context.Context, event.TypedGenericEvent[*corev1.LimitRange], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *PodGroupLimitRangeHandler) queueSuspendedTrainJob(ctx context.Context, ns string, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	var trainJobs omev1beta1.TrainingJobList
	if err := h.client.List(ctx, &trainJobs, client.InNamespace(ns)); err != nil {
		return err
	}
	for _, trainJob := range trainJobs.Items {
		if ptr.Deref(trainJob.Spec.Suspend, false) {
			q.Add(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&trainJob)})
		}
	}
	return nil
}

func (c *CoScheduling) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	if _, err := c.restMapper.RESTMapping(
		schema.GroupKind{Group: schedulerpluginsv1alpha1.SchemeGroupVersion.Group, Kind: "PodGroup"},
		schedulerpluginsv1alpha1.SchemeGroupVersion.Version,
	); err != nil {
		return nil
	}
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Owns(&schedulerpluginsv1alpha1.PodGroup{})
		},
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.WatchesRawSource(source.TypedKind[*corev1.LimitRange, reconcile.Request](cache, &corev1.LimitRange{}, &PodGroupLimitRangeHandler{
				client: cl,
			}))
		},
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.WatchesRawSource(source.TypedKind[*nodev1.RuntimeClass, reconcile.Request](cache, &nodev1.RuntimeClass{}, &PodGroupRuntimeClassHandler{
				client: cl,
			}))
		},
	}
}
