package core

import (
	"context"
	"errors"
	"fmt"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	fwkcore "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/core"
	fwkplugins "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins"
	idxer "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/indexer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

var (
	errorNotFoundSpecifiedTrainingRuntime = errors.New("TrainingRuntime specified in TrainJob is not found")
)

type TrainingRuntime struct {
	framework *fwkcore.Framework
	client    client.Client
}

var TrainingRuntimeGroupKind = schema.GroupKind{
	Group: omev1beta1.SchemeGroupVersion.Group,
	Kind:  omev1beta1.TrainingRuntimeKind,
}.String()

var _ runtime.Runtime = (*TrainingRuntime)(nil)

var log = logf.Log.WithName("TrainingRuntimeBuilder")

var trainingRuntimeFactory *TrainingRuntime

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer) (runtime.Runtime, error) {
	if err := indexer.IndexField(ctx, &omev1beta1.TrainingJob{}, idxer.TrainJobRuntimeRefKey, idxer.IndexTrainJobTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on TrainingRuntime for TrainJob: %w", err)
	}
	if err := indexer.IndexField(ctx, &omev1beta1.TrainingJob{}, idxer.TrainJobClusterRuntimeRefKey, idxer.IndexTrainJobClusterTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on ClusterTrainingRuntime for TrainJob: %w", err)
	}
	fwk, err := fwkcore.New(ctx, c, fwkplugins.NewRegistry(), indexer)
	if err != nil {
		return nil, err
	}
	trainingRuntimeFactory = &TrainingRuntime{
		framework: fwk,
		client:    c,
	}
	return trainingRuntimeFactory, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *omev1beta1.TrainingJob, vendor *string) ([]client.Object, error) {
	var trainingRuntime omev1beta1.TrainingRuntime
	err := r.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, &trainingRuntime)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errorNotFoundSpecifiedTrainingRuntime, err)
	}
	return r.buildObjects(ctx, trainJob, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy, vendor)
}

func (r *TrainingRuntime) buildObjects(
	ctx context.Context, trainJob *omev1beta1.TrainingJob, jobSetTemplateSpec omev1beta1.JobSetTemplateSpec, mlPolicy *omev1beta1.MLPolicy, podGroupPolicy *omev1beta1.PodGroupPolicy, vendor *string) ([]client.Object, error) {
	propagationLabels := jobSetTemplateSpec.Labels
	if propagationLabels == nil && trainJob.Spec.Labels != nil {
		propagationLabels = make(map[string]string, len(trainJob.Spec.Labels))
	}
	for k, v := range trainJob.Spec.Labels {
		// The JobSetTemplateSpec labels are overridden by the TrainJob Labels (.spec.labels).
		propagationLabels[k] = v
	}
	propagationAnnotations := jobSetTemplateSpec.Annotations
	if propagationAnnotations == nil && trainJob.Spec.Annotations != nil {
		propagationAnnotations = make(map[string]string, len(trainJob.Spec.Annotations))
	}
	for k, v := range trainJob.Spec.Annotations {
		// The JobSetTemplateSpec annotations are overridden by the TrainJob Annotations (.spec.annotations).
		propagationAnnotations[k] = v
	}
	opts := []runtime.InfoOption{
		runtime.WithLabels(propagationLabels),
		runtime.WithAnnotations(propagationAnnotations),
		runtime.WithMLPolicy(mlPolicy),
		runtime.WithPodGroupPolicy(podGroupPolicy),
	}

	for _, rJob := range jobSetTemplateSpec.Spec.ReplicatedJobs {
		// By default, every ReplicatedJob has only 1 replica.
		opts = append(opts, runtime.WithPodSpecReplicas(rJob.Name, 1, rJob.Template.Spec.Template.Spec))
	}

	var trainingPodVolumes = r.getPodVolumes(trainJob, vendor)
	var podAffinity *corev1.Affinity
	// Set node affinity from DAC if necessary
	dedicatedAiClusterResource, err := utils.GetDedicatedAIClusterResource(r.client, &corev1.ObjectReference{
		Name: trainJob.Namespace,
	})
	if err == nil && dedicatedAiClusterResource != nil {
		profile := &omev1beta1.DedicatedAIClusterProfile{}
		if err = r.client.Get(ctx, types.NamespacedName{Name: dedicatedAiClusterResource.Spec.Profile}, profile); err != nil {
			if apierr.IsNotFound(err) {
				log.Error(err, "Non-blocking error: DAC profile not found in DAC scheduling injector", "DAC profile name", dedicatedAiClusterResource.Spec.Profile)
			}
			log.Error(err, "Non-blocking error: failed to get DAC profile in DAC scheduling injector", "DAC profile name", dedicatedAiClusterResource.Spec.Profile)
		}

		if profile.Spec.Affinity != nil {
			podAffinity = profile.Spec.Affinity.DeepCopy()
		}
	}

	log.Info("Pod spec override", "podVolumes", trainingPodVolumes, "affinity", podAffinity)

	opts = append(opts, runtime.WithVolumes(trainingPodVolumes))
	opts = append(opts, runtime.WithAffinity(podAffinity))

	info := runtime.NewInfo(opts...)

	if err := r.framework.RunEnforceMLPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	if err := r.framework.RunEnforcePodGroupPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	jobSetTemplate := jobsetv1alpha2.JobSet{
		Spec: jobSetTemplateSpec.Spec,
	}

	log.Info("Checking runtime info", "runtime.info", info)

	return r.framework.RunComponentBuilderPlugins(ctx, jobSetTemplate.DeepCopy(), info, trainJob)
}

func (r *TrainingRuntime) TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	return r.framework.RunTerminalConditionPlugins(ctx, trainJob)
}

func (r *TrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	var builders []runtime.ReconcilerBuilder
	for _, ex := range r.framework.WatchExtensionPlugins() {
		builders = append(builders, ex.ReconcilerBuilders()...)
	}
	return builders
}

func (r *TrainingRuntime) ValidateObjects(ctx context.Context, old, new *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: old.Namespace,
		Name:      old.Spec.RuntimeRef.Name,
	}, &omev1beta1.TrainingRuntime{}); err != nil {
		return nil, field.ErrorList{
			field.Invalid(field.NewPath("spec", "runtimeRef"), old.Spec.RuntimeRef,
				fmt.Sprintf("%v: specified trainingRuntime must be created before the TrainJob is created", err)),
		}
	}
	return r.framework.RunCustomValidationPlugins(old, new)
}

func (r *TrainingRuntime) getPodVolumes(trainJob *omev1beta1.TrainingJob, vendor *string) []corev1.Volume {
	var podVolumes []corev1.Volume

	emptyDirDataVolume := corev1.Volume{
		Name: constants.DataEmptyDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	podVolumes = append(podVolumes, emptyDirDataVolume)

	if *vendor == "cohere" {
		emptyDirModelVolumeInitContainer := corev1.Volume{
			Name: constants.ModelEmptyDirVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
		podVolumes = append(podVolumes, emptyDirModelVolumeInitContainer)

		baseModelNameVolume := corev1.Volume{
			Name: *trainJob.Spec.ModelConfig.InputModel,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: constants.GetPvcName(trainJob.Name, trainJob.Namespace, *trainJob.Spec.ModelConfig.InputModel),
				},
			},
		}
		podVolumes = append(podVolumes, baseModelNameVolume)
	} else {
		pvcSourceVolume := corev1.Volume{
			Name: constants.ModelStorePVCSourceName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: constants.GetPvcName(trainJob.Name, trainJob.Namespace, *trainJob.Spec.ModelConfig.InputModel),
				},
			},
		}
		podVolumes = append(podVolumes, pvcSourceVolume)
	}

	regionFileVolume := corev1.Volume{
		Name: constants.RegionFileVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: constants.RegionFileVolumeMountPath,
			},
		},
	}
	podVolumes = append(podVolumes, regionFileVolume)

	adFileVolume := corev1.Volume{
		Name: constants.ADFileVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: constants.ADFileVolumeMountPath,
			},
		},
	}
	podVolumes = append(podVolumes, adFileVolume)

	realmFileVolume := corev1.Volume{
		Name: constants.RealmFileVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: constants.RealmFileVolumeMountPath,
			},
		},
	}
	podVolumes = append(podVolumes, realmFileVolume)

	return podVolumes
}
