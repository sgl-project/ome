package jobset

import (
	"maps"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
)

type Builder struct {
	jobsetv1alpha2.JobSet
}

func NewBuilder(objectKey client.ObjectKey, jobSetTemplateSpec omev1beta1.JobSetTemplateSpec) *Builder {
	return &Builder{
		JobSet: jobsetv1alpha2.JobSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: jobsetv1alpha2.SchemeGroupVersion.String(),
				Kind:       constants.JobSetKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   objectKey.Namespace,
				Name:        objectKey.Name,
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
	for i := range b.Spec.ReplicatedJobs {
		b.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = labels
	}
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
