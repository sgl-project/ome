package jobset

import (
	"maps"
	"path/filepath"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/utils"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
)

type Builder struct {
	jobsetv1alpha2.JobSet
}

var log = logf.Log.WithName("JobSetBuilder")

func NewBuilder(objectKey client.ObjectKey, jobSetTemplateSpec omev1beta1.JobSetTemplateSpec) *Builder {
	keyName := utils.GetShortTrainJobName(objectKey.Name)
	return &Builder{
		JobSet: jobsetv1alpha2.JobSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: jobsetv1alpha2.SchemeGroupVersion.String(),
				Kind:       constants.JobSetKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   objectKey.Namespace,
				Name:        keyName,
				Labels:      maps.Clone(jobSetTemplateSpec.Labels),
				Annotations: maps.Clone(jobSetTemplateSpec.Annotations),
			},
			Spec: *jobSetTemplateSpec.Spec.DeepCopy(),
		},
	}
}

// mergeInitializerEnvs merges the TrainJob and Runtime Pod envs.
func mergeInitializerEnvs(storageUri *string, containerEnv []corev1.EnvVar) []corev1.EnvVar {
	envNames := sets.New[string]()
	var envs []corev1.EnvVar
	// Add the Storage URI env.
	if storageUri != nil {
		envNames.Insert(InitializerEnvStorageUri)
		envs = append(envs, corev1.EnvVar{
			Name:  InitializerEnvStorageUri,
			Value: *storageUri,
		})
	}

	// TrainJob envs take precedence over the TrainingRuntime envs.
	for _, e := range containerEnv {
		if !envNames.Has(e.Name) {
			envs = append(envs, e)
		}
	}
	return envs
}

// Initializer updates JobSet values for the initializer Job.
func (b *Builder) Initializer(trainJob *omev1beta1.TrainingJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			// TODO: Currently, we use initContainers for the initializers.
			// Once JobSet supports execution policy for the ReplicatedJobs, we should migrate to containers.
			// Ref: https://github.com/kubernetes-sigs/jobset/issues/672
			for j, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				// Update values for the dataset initializer container.
				if container.Name == constants.ContainerDatasetInitializer && trainJob.Spec.Datasets != nil {
					// Update the dataset initializer envs.
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = mergeInitializerEnvs(
						trainJob.Spec.Datasets.StorageUri,
						container.Env,
					)
					if trainJob.Spec.Datasets.Parameters != nil {
						for k, v := range *trainJob.Spec.Datasets.Parameters {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = append(
								b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env,
								corev1.EnvVar{
									Name:  k,
									Value: v,
								},
							)
						}
					}
					// Update the dataset initializer secret reference.
					if trainJob.Spec.Datasets.StorageKey != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env,
							corev1.EnvVar{
								Name: "STORAGE_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: *trainJob.Spec.Datasets.StorageKey,
										},
										Key: "key",
									},
								},
							},
						)
					}
				}
				// TODO: Add the model exporter when we support it.
				// Update values for the model initializer container.
				if container.Name == constants.ContainerModelInitializer && trainJob.Spec.ModelConfig != nil {
					// Update the model initializer envs.
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = mergeInitializerEnvs(
						trainJob.Spec.ModelConfig.OutputModel.StorageUri,
						container.Env,
					)
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = append(
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env,
						corev1.EnvVar{
							Name:  "MODEL_NAME",
							Value: *trainJob.Spec.ModelConfig.InputModel,
						},
					)
					// Update the model initializer secret reference.
					if trainJob.Spec.ModelConfig.OutputModel.Parameters != nil {
						for k, v := range *trainJob.Spec.ModelConfig.OutputModel.Parameters {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = append(
								b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env,
								corev1.EnvVar{
									Name:  k,
									Value: v,
								},
							)
						}
					}
					// Update the model initializer secret reference.
					if trainJob.Spec.ModelConfig.OutputModel.StorageKey != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Env,
							corev1.EnvVar{
								Name: "STORAGE_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: *trainJob.Spec.ModelConfig.OutputModel.StorageKey,
										},
										Key: "key",
									},
								},
							},
						)
					}
				}
			}
		}
	}
	return b
}

// Trainer updates JobSet values for the trainer Job.
func (b *Builder) Trainer(info *runtime.Info, trainJob *omev1beta1.TrainingJob) *Builder {
	for i, rJob := range b.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			// Update the Parallelism and Completions values for the Trainer Job.
			b.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = info.Trainer.NumNodes
			b.Spec.ReplicatedJobs[i].Template.Spec.Completions = info.Trainer.NumNodes

			if b.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations == nil {
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations = make(map[string]string)
			}
			for k, v := range info.Annotations {
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations[k] = v
			}

			if b.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels == nil {
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = make(map[string]string)
			}
			for k, v := range info.Labels {
				b.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels[k] = v
			}

			log.Info("Pod spec overrides", "volumes", info.Volumes, "affinity", info.Affinity)

			b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes = append(b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Volumes, info.Volumes...)
			b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Affinity = info.Affinity

			log.Info("Added pod spec overrides", "podspec", b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec)

			// Update values for the Trainer container.
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					// Update values from the TrainJob trainer.
					if trainJob.Spec.Trainer != nil {
						if trainJob.Spec.Trainer.Image != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Image = *trainJob.Spec.Trainer.Image
						}
						if trainJob.Spec.Trainer.Command != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Command = trainJob.Spec.Trainer.Command
						}
						if trainJob.Spec.Trainer.Args != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Args = trainJob.Spec.Trainer.Args
						}
						if trainJob.Spec.Trainer.ResourcesPerNode != nil {
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Resources = *trainJob.Spec.Trainer.ResourcesPerNode
						}
					}
					// Update values from the Info object.
					if info.Trainer.Env != nil {
						// Update JobSet envs from the Info.
						envNames := sets.New[string]()
						for _, env := range info.Trainer.Env {
							envNames.Insert(env.Name)
						}
						trainerEnvs := info.Trainer.Env
						// Info envs take precedence over the TrainingRuntime envs.
						for _, env := range container.Env {
							if !envNames.Has(env.Name) {
								trainerEnvs = append(trainerEnvs, env)
							}
						}
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = trainerEnvs
					}

					pathPrefixEnv := getPathPrefixEnv(trainJob.Spec.Annotations[constants.TrainingRuntimeTypeAnnotationKey], trainJob)
					baselineModelEnv := getBaselineModelEnv(trainJob.Spec.Annotations[constants.TrainingRuntimeTypeAnnotationKey], trainJob)

					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = append(b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env, pathPrefixEnv, baselineModelEnv)

					volumeMounts := getVolumeMounts(trainJob.Spec.Annotations[constants.TrainingRuntimeTypeAnnotationKey], trainJob)
					b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts = append(b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].VolumeMounts, volumeMounts...)

					// Update the Trainer container port.
					if info.Trainer.ContainerPort != nil {
						b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports = append(
							b.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Ports, *info.Trainer.ContainerPort)
					}
				}
			}
		}
	}
	return b
}

// TODO: Supporting merge labels would be great.
func (b *Builder) PodLabels(labels map[string]string) *Builder {
	return b
}

func (b *Builder) Suspend(suspend *bool) *Builder {
	b.Spec.Suspend = suspend
	return b
}

// TODO: Need to support all TrainJob fields.

func (b *Builder) Build() *jobsetv1alpha2.JobSet {
	return &b.JobSet
}

func getPathPrefixEnv(runtime string, trainJob *omev1beta1.TrainingJob) corev1.EnvVar {
	if runtime == "peft" {
		return corev1.EnvVar{
			Name:  constants.TrainingPathPrefixEnvVarKey,
			Value: constants.TrainingDataEmptyDirMountPath,
		}
	} else {
		return corev1.EnvVar{
			Name:  constants.TrainingPathPrefixEnvVarKey,
			Value: filepath.Join(constants.CohereStorePathPrefix, utils.GetFineTunedModelName(trainJob.Name)),
		}
	}
}

func getBaselineModelEnv(runtime string, trainJob *omev1beta1.TrainingJob) corev1.EnvVar {
	if runtime == "peft" {
		return corev1.EnvVar{
			Name:  constants.TrainingBaselineModelEnvVarKey,
			Value: constants.ModelStorePVCMountPath,
		}
	} else {
		finetunedModelName := utils.GetFineTunedModelName(trainJob.Name)
		return corev1.EnvVar{
			Name:  constants.TrainingBaselineModelEnvVarKey,
			Value: getCohereBaselineModelEnvValue(finetunedModelName, runtime),
		}
	}
}

func getCohereBaselineModelEnvValue(fineTunedModelName string, runtime string) string {
	if runtime == "cohere" {
		return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName, "ckpt-0")
	} else {
		return filepath.Join(constants.CohereStorePathPrefix, fineTunedModelName)
	}
}

func getVolumeMounts(runtime string, trainJob *omev1beta1.TrainingJob) []corev1.VolumeMount {
	var vms []corev1.VolumeMount

	if runtime == "peft" {
		modelPVCSourceVolumeMount := corev1.VolumeMount{
			Name:      constants.ModelStorePVCSourceName,
			MountPath: constants.ModelStorePVCMountPath,
			ReadOnly:  false,
		}
		vms = append(vms, modelPVCSourceVolumeMount)

		dataEmptyDirVolumeMount := corev1.VolumeMount{
			Name:      constants.DataEmptyDirName,
			MountPath: constants.TrainingDataEmptyDirMountPath,
			ReadOnly:  false,
		}
		vms = append(vms, dataEmptyDirVolumeMount)
	} else {
		finetunedModelName := utils.GetFineTunedModelName(trainJob.Name)
		modelEmptyDirVolumeMount := corev1.VolumeMount{
			Name:      constants.ModelEmptyDirVolumeName,
			MountPath: filepath.Join(constants.CohereStorePathPrefix, finetunedModelName),
			ReadOnly:  false,
		}
		vms = append(vms, modelEmptyDirVolumeMount)

		dataEmptyDirVolumeMount := corev1.VolumeMount{
			Name:      constants.DataEmptyDirName,
			MountPath: filepath.Join(constants.CohereStorePathPrefix, finetunedModelName, "/input/data/training/"),
			ReadOnly:  false,
		}
		vms = append(vms, dataEmptyDirVolumeMount)
	}

	return vms
}
