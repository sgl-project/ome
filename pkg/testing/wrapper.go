package testing

import (
	"encoding/json"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	jobsetplugin "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/jobset"
)

type JobSetWrapper struct {
	jobsetv1alpha2.JobSet
}

func MakeJobSetWrapper(namespace, name string) *JobSetWrapper {
	return &JobSetWrapper{
		JobSet: jobsetv1alpha2.JobSet{
			TypeMeta: metav1.TypeMeta{
				APIVersion: jobsetv1alpha2.SchemeGroupVersion.String(),
				Kind:       constants.JobSetKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: jobsetv1alpha2.JobSetSpec{
				ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
					{
						Name: constants.JobInitializer,
						Template: batchv1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										InitContainers: []corev1.Container{
											{
												Name: constants.ContainerDatasetInitializer,
												VolumeMounts: []corev1.VolumeMount{
													jobsetplugin.VolumeMountDatasetInitializer,
												},
											},
											{
												Name: constants.ContainerModelInitializer,
												VolumeMounts: []corev1.VolumeMount{
													jobsetplugin.VolumeMountModelInitializer,
												},
											},
										},
										Containers: []corev1.Container{
											jobsetplugin.ContainerBusyBox,
										},
										Volumes: []corev1.Volume{
											jobsetplugin.VolumeInitializer,
										},
									},
								},
							},
						},
					},
					{
						Name: constants.JobTrainerNode,
						Template: batchv1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: constants.ContainerTrainer,
												VolumeMounts: []corev1.VolumeMount{
													jobsetplugin.VolumeMountDatasetInitializer,
													jobsetplugin.VolumeMountModelInitializer,
												},
											},
										},
										Volumes: []corev1.Volume{
											jobsetplugin.VolumeInitializer,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (j *JobSetWrapper) Volumes(vendor string) *JobSetWrapper {
	var vendorPtr = &vendor
	for idx := range j.Spec.ReplicatedJobs {
		if j.Spec.ReplicatedJobs[idx].Name == constants.JobTrainerNode {
			j.Spec.ReplicatedJobs[idx].Template.Spec.Template.Spec.Volumes = append(j.Spec.ReplicatedJobs[idx].Template.Spec.Template.Spec.Volumes, getPodVolumes(vendorPtr)...)
		}
	}
	return j
}

func getPodVolumes(vendor *string) []corev1.Volume {
	var podVolumes []corev1.Volume

	emptyDirDataVolume := corev1.Volume{
		Name: constants.DataEmptyDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	podVolumes = append(podVolumes, emptyDirDataVolume)

	pvcSourceVolume := corev1.Volume{
		Name: constants.ModelStorePVCSourceName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: constants.GetPvcName("test-job", metav1.NamespaceDefault, "test-input-model"),
			},
		},
	}
	podVolumes = append(podVolumes, pvcSourceVolume)

	// Create EmptyDir volume for model, only for cohere training init container
	if *vendor == "cohere" {
		emptyDirModelVolume := corev1.Volume{
			Name: constants.ModelEmptyDirVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
		podVolumes = append(podVolumes, emptyDirModelVolume)

		baseModelNameVolume := corev1.Volume{
			Name: "test-input-model",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: constants.GetPvcName("test-job", metav1.NamespaceDefault, "test-input-model"),
				},
			},
		}
		podVolumes = append(podVolumes, baseModelNameVolume)
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

func (j *JobSetWrapper) Replicas(replicas int32) *JobSetWrapper {
	for idx := range j.Spec.ReplicatedJobs {
		j.Spec.ReplicatedJobs[idx].Replicas = replicas
	}
	return j
}

func (j *JobSetWrapper) NumNodes(numNodes int32) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			j.Spec.ReplicatedJobs[i].Template.Spec.Parallelism = &numNodes
			j.Spec.ReplicatedJobs[i].Template.Spec.Completions = &numNodes
		}
	}
	return j
}

func (j *JobSetWrapper) Labels(key, value string) *JobSetWrapper {
	j.SetLabels(map[string]string{
		key: value,
	})
	return j
}

func (j *JobSetWrapper) LabelsTrainer(key, value string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = make(map[string]string)
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels[key] = value
		}
	}
	return j
}

func (j *JobSetWrapper) AnnotationsTrainer(key, value string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations = make(map[string]string)
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Annotations[key] = value
		}
	}
	return j
}

func (j *JobSetWrapper) ContainerTrainer(image string, command []string, args []string, res corev1.ResourceList) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Image = image
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Command = command
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Args = args
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Resources.Requests = res
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Env = getEnvs()
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].VolumeMounts = append(j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].VolumeMounts, getVolumeMounts()...)
				}
			}
		}
	}
	return j
}

func getEnvs() []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0)
	envs = append(envs, corev1.EnvVar{
		Name:  constants.TrainingPathPrefixEnvVarKey,
		Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
	})

	envs = append(envs, corev1.EnvVar{
		Name:  constants.TrainingBaselineModelEnvVarKey,
		Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
	})

	return envs
}

func getVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "model-empty-dir", MountPath: "/mnt/cohere/t-job"},
		{Name: "data", MountPath: "/mnt/cohere/t-job/input/data/training"},
	}
}

func (j *JobSetWrapper) ContainerTrainerPorts(ports []corev1.ContainerPort) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Ports = ports
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) ContainerTrainerEnv(env []corev1.EnvVar) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			for k, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[k].Env = env
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainerDatasetModelInitializer(image string, command []string, args []string, res corev1.ResourceList) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for k, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerDatasetInitializer || container.Name == constants.ContainerModelInitializer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Image = image
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Command = command
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Args = args
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Resources.Requests = res
				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainerDatasetInitializerEnv(env []corev1.EnvVar) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for k, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerDatasetInitializer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Env = env

				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainerDatasetInitializerEnvFrom(envFrom []corev1.EnvFromSource) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for k, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerDatasetInitializer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].EnvFrom = envFrom

				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainerModelInitializerEnv(env []corev1.EnvVar) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for k, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerModelInitializer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].Env = env

				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) InitContainerModelInitializerEnvFrom(envFrom []corev1.EnvFromSource) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for k, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerModelInitializer {
					j.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[k].EnvFrom = envFrom

				}
			}
		}
	}
	return j
}

func (j *JobSetWrapper) Suspend(suspend bool) *JobSetWrapper {
	j.Spec.Suspend = &suspend
	return j
}

func (j *JobSetWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *JobSetWrapper {
	j.OwnerReferences = append(j.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return j
}

func (j *JobSetWrapper) PodLabel(key, value string) *JobSetWrapper {
	for i, rJob := range j.Spec.ReplicatedJobs {
		if rJob.Template.Spec.Template.Labels == nil {
			j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels = make(map[string]string, 1)
		}
		j.Spec.ReplicatedJobs[i].Template.Spec.Template.Labels[key] = value
	}
	return j
}

func (j *JobSetWrapper) Label(key, value string) *JobSetWrapper {
	if j.ObjectMeta.Labels == nil {
		j.ObjectMeta.Labels = make(map[string]string, 1)
	}
	j.ObjectMeta.Labels[key] = value
	return j
}

func (j *JobSetWrapper) Annotation(key, value string) *JobSetWrapper {
	if j.ObjectMeta.Annotations == nil {
		j.ObjectMeta.Annotations = make(map[string]string, 1)
	}
	j.ObjectMeta.Annotations[key] = value
	return j
}

func (j *JobSetWrapper) Conditions(conditions ...metav1.Condition) *JobSetWrapper {
	if len(conditions) != 0 {
		j.Status.Conditions = append(j.Status.Conditions, conditions...)
	}
	return j
}

func (j *JobSetWrapper) Obj() *jobsetv1alpha2.JobSet {
	return &j.JobSet
}

type TrainJobWrapper struct {
	omev1beta1.TrainingJob
}

func MakeTrainJobWrapper(namespace, name string) *TrainJobWrapper {
	return &TrainJobWrapper{
		TrainingJob: omev1beta1.TrainingJob{
			TypeMeta: metav1.TypeMeta{
				APIVersion: omev1beta1.SchemeGroupVersion.Version,
				Kind:       omev1beta1.TrainingJobKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: omev1beta1.TrainingJobSpec{},
		},
	}
}

func (t *TrainJobWrapper) Suspend(suspend bool) *TrainJobWrapper {
	t.Spec.Suspend = &suspend
	return t
}

func (t *TrainJobWrapper) UID(uid string) *TrainJobWrapper {
	t.ObjectMeta.UID = types.UID(uid)
	return t
}

func (t *TrainJobWrapper) SpecLabel(key, value string) *TrainJobWrapper {
	if t.Spec.Labels == nil {
		t.Spec.Labels = make(map[string]string, 1)
	}
	t.Spec.Labels[key] = value
	return t
}

func (t *TrainJobWrapper) SpecAnnotation(key, value string) *TrainJobWrapper {
	if t.Spec.Annotations == nil {
		t.Spec.Annotations = make(map[string]string, 1)
	}
	t.Spec.Annotations[key] = value
	return t
}

func (t *TrainJobWrapper) RuntimeRef(gvk schema.GroupVersionKind, name string) *TrainJobWrapper {
	runtimeRef := omev1beta1.RuntimeRef{
		Name: name,
	}
	if gvk.Group != "" {
		runtimeRef.APIGroup = &gvk.Group
	}
	if gvk.Kind != "" {
		runtimeRef.Kind = &gvk.Kind
	}
	t.Spec.RuntimeRef = runtimeRef
	return t
}

func (t *TrainJobWrapper) Trainer(trainer *omev1beta1.TrainerSpec) *TrainJobWrapper {
	t.Spec.Trainer = trainer
	return t
}

func (t *TrainJobWrapper) DatasetConfig(datasetConfig *omev1beta1.StorageSpec) *TrainJobWrapper {
	if t.Spec.Datasets == nil {
		t.Spec.Datasets = &omev1beta1.StorageSpec{}
	}
	t.Spec.Datasets = datasetConfig
	return t
}

func (t *TrainJobWrapper) ModelConfig(modelConfig *omev1beta1.ModelConfig) *TrainJobWrapper {
	t.Spec.ModelConfig = modelConfig
	return t
}

func (t *TrainJobWrapper) HyperParameterTuningConfig(hyperParameterTuningConfig *omev1beta1.HyperparameterTuningConfig) *TrainJobWrapper {
	t.Spec.HyperParameterTuningConfig = hyperParameterTuningConfig
	return t
}

func (t *TrainJobWrapper) Obj() *omev1beta1.TrainingJob {
	return &t.TrainingJob
}

type TrainJobTrainerWrapper struct {
	omev1beta1.TrainerSpec
}

func MakeTrainJobTrainerWrapper() *TrainJobTrainerWrapper {
	return &TrainJobTrainerWrapper{
		TrainerSpec: omev1beta1.TrainerSpec{},
	}
}

func (t *TrainJobTrainerWrapper) NumNodes(numNodes int32) *TrainJobTrainerWrapper {
	t.TrainerSpec.NumNodes = &numNodes
	return t
}

func (t *TrainJobTrainerWrapper) NumProcPerNode(numProcPerNode string) *TrainJobTrainerWrapper {
	t.TrainerSpec.NumProcPerNode = &numProcPerNode
	return t
}

func (t *TrainJobTrainerWrapper) Container(image string, command []string, args []string, resRequests corev1.ResourceList) *TrainJobTrainerWrapper {
	t.TrainerSpec.Image = &image
	t.TrainerSpec.Command = command
	t.TrainerSpec.Args = args
	t.TrainerSpec.ResourcesPerNode = &corev1.ResourceRequirements{
		Requests: resRequests,
	}
	return t
}

func (t *TrainJobTrainerWrapper) ContainerEnv(env []corev1.EnvVar) *TrainJobTrainerWrapper {
	t.TrainerSpec.Env = env
	return t
}

func (t *TrainJobTrainerWrapper) Obj() *omev1beta1.TrainerSpec {
	return &t.TrainerSpec
}

type TrainJobDatasetConfigWrapper struct {
	omev1beta1.StorageSpec
}

func MakeTrainJobDatasetConfigWrapper() *TrainJobDatasetConfigWrapper {
	return &TrainJobDatasetConfigWrapper{
		StorageSpec: omev1beta1.StorageSpec{},
	}
}

func (t *TrainJobDatasetConfigWrapper) Path(path string) *TrainJobDatasetConfigWrapper {
	t.StorageSpec.Path = &path
	return t
}

func (t *TrainJobDatasetConfigWrapper) StorageUri(storageUri string) *TrainJobDatasetConfigWrapper {
	t.StorageSpec.StorageUri = &storageUri
	return t
}

func (t *TrainJobDatasetConfigWrapper) StorageKey(key string) *TrainJobDatasetConfigWrapper {
	t.StorageSpec.StorageKey = &key
	return t
}

func (t *TrainJobDatasetConfigWrapper) Parameters(params map[string]string) *TrainJobDatasetConfigWrapper {
	t.StorageSpec.Parameters = &params
	return t
}

func (t *TrainJobDatasetConfigWrapper) SchemaPath(schemaPath string) *TrainJobDatasetConfigWrapper {
	t.StorageSpec.SchemaPath = &schemaPath
	return t
}

func (t *TrainJobDatasetConfigWrapper) Obj() *omev1beta1.StorageSpec {
	return &t.StorageSpec
}

type TrainJobModelConfigWrapper struct {
	omev1beta1.ModelConfig
}

func MakeTrainJobModelConfigWrapper() *TrainJobModelConfigWrapper {
	return &TrainJobModelConfigWrapper{
		ModelConfig: omev1beta1.ModelConfig{},
	}
}

func (t *TrainJobModelConfigWrapper) InputModel(inputModel string) *TrainJobModelConfigWrapper {
	t.ModelConfig.InputModel = &inputModel
	return t
}

func (t *TrainJobModelConfigWrapper) OutputModel(outputModel *omev1beta1.StorageSpec) *TrainJobModelConfigWrapper {
	t.ModelConfig.OutputModel = outputModel
	return t
}

func (t *TrainJobModelConfigWrapper) Obj() *omev1beta1.ModelConfig {
	return &t.ModelConfig
}

type HyperparameterTuningConfigWrapper struct {
	omev1beta1.HyperparameterTuningConfig
}

func MakeTrainJobHyperparameterTuningConfigWrapper() *HyperparameterTuningConfigWrapper {
	return &HyperparameterTuningConfigWrapper{
		HyperparameterTuningConfig: omev1beta1.HyperparameterTuningConfig{},
	}
}

func (h *HyperparameterTuningConfigWrapper) TrainJobHyperparameterTuningConfig() *HyperparameterTuningConfigWrapper {
	hyperparameters := make(map[string]interface{})
	hyperparameters[constants.BatchSizeConfigKey] = 8
	hyperparameters[constants.LoraTrainingConfig] = "lora"
	hyperparametersRaw, _ := json.Marshal(hyperparameters)

	h.HyperparameterTuningConfig = omev1beta1.HyperparameterTuningConfig{
		Method: "bayes",
		Metric: omev1beta1.MetricConfig{
			Name: "test-metric",
			Goal: "maximize",
		},
		Parameters: runtime.RawExtension{
			Raw: hyperparametersRaw,
		},
	}
	return h
}

func (h *HyperparameterTuningConfigWrapper) Obj() *omev1beta1.HyperparameterTuningConfig {
	return &h.HyperparameterTuningConfig
}

type TrainingRuntimeWrapper struct {
	omev1beta1.TrainingRuntime
}

func MakeTrainingRuntimeWrapper(namespace, name string) *TrainingRuntimeWrapper {
	return &TrainingRuntimeWrapper{
		TrainingRuntime: omev1beta1.TrainingRuntime{
			TypeMeta: metav1.TypeMeta{
				APIVersion: omev1beta1.SchemeGroupVersion.String(),
				Kind:       omev1beta1.TrainingRuntimeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: omev1beta1.TrainingRuntimeSpec{
				Template: omev1beta1.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name: constants.JobInitializer,
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												InitContainers: []corev1.Container{
													{
														Name: constants.ContainerDatasetInitializer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountDatasetInitializer,
														},
													},
													{
														Name: constants.ContainerModelInitializer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountModelInitializer,
														},
													},
												},
												Containers: []corev1.Container{
													jobsetplugin.ContainerBusyBox,
												},
												Volumes: []corev1.Volume{
													jobsetplugin.VolumeInitializer,
												},
											},
										},
									},
								},
							},
							{
								Name: constants.JobTrainerNode,
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.ContainerTrainer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountDatasetInitializer,
															jobsetplugin.VolumeMountModelInitializer,
														},
													},
												},
												Volumes: []corev1.Volume{
													jobsetplugin.VolumeInitializer,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *TrainingRuntimeWrapper) Label(key, value string) *TrainingRuntimeWrapper {
	if r.ObjectMeta.Labels == nil {
		r.ObjectMeta.Labels = make(map[string]string, 1)
	}
	r.ObjectMeta.Labels[key] = value
	return r
}

func (r *TrainingRuntimeWrapper) Annotation(key, value string) *TrainingRuntimeWrapper {
	if r.ObjectMeta.Annotations == nil {
		r.ObjectMeta.Annotations = make(map[string]string, 1)
	}
	r.ObjectMeta.Annotations[key] = value
	return r
}

func (r *TrainingRuntimeWrapper) RuntimeSpec(spec omev1beta1.TrainingRuntimeSpec) *TrainingRuntimeWrapper {
	r.Spec = spec
	return r
}

func (r *TrainingRuntimeWrapper) Obj() *omev1beta1.TrainingRuntime {
	return &r.TrainingRuntime
}

type ClusterTrainingRuntimeWrapper struct {
	omev1beta1.ClusterTrainingRuntime
}

func MakeClusterTrainingRuntimeWrapper(name string) *ClusterTrainingRuntimeWrapper {
	return &ClusterTrainingRuntimeWrapper{
		ClusterTrainingRuntime: omev1beta1.ClusterTrainingRuntime{
			TypeMeta: metav1.TypeMeta{
				APIVersion: omev1beta1.SchemeGroupVersion.String(),
				Kind:       omev1beta1.ClusterTrainingRuntimeKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: omev1beta1.TrainingRuntimeSpec{
				Template: omev1beta1.JobSetTemplateSpec{
					Spec: jobsetv1alpha2.JobSetSpec{
						ReplicatedJobs: []jobsetv1alpha2.ReplicatedJob{
							{
								Name: constants.JobInitializer,
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												InitContainers: []corev1.Container{
													{
														Name: constants.ContainerDatasetInitializer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountDatasetInitializer,
														},
													},
													{
														Name: constants.ContainerModelInitializer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountModelInitializer,
														},
													},
												},
												Containers: []corev1.Container{
													jobsetplugin.ContainerBusyBox,
												},
												Volumes: []corev1.Volume{
													jobsetplugin.VolumeInitializer,
												},
											},
										},
									},
								},
							},
							{
								Name: constants.JobTrainerNode,
								Template: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name: constants.ContainerTrainer,
														VolumeMounts: []corev1.VolumeMount{
															jobsetplugin.VolumeMountDatasetInitializer,
															jobsetplugin.VolumeMountModelInitializer,
														},
													},
												},
												Volumes: []corev1.Volume{
													jobsetplugin.VolumeInitializer,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ClusterTrainingRuntimeWrapper) RuntimeSpec(spec omev1beta1.TrainingRuntimeSpec) *ClusterTrainingRuntimeWrapper {
	r.Spec = spec
	return r
}

func (r *ClusterTrainingRuntimeWrapper) Obj() *omev1beta1.ClusterTrainingRuntime {
	return &r.ClusterTrainingRuntime
}

type TrainingRuntimeSpecWrapper struct {
	omev1beta1.TrainingRuntimeSpec
}

func MakeTrainingRuntimeSpecWrapper(spec omev1beta1.TrainingRuntimeSpec) *TrainingRuntimeSpecWrapper {
	return &TrainingRuntimeSpecWrapper{
		TrainingRuntimeSpec: spec,
	}
}

func (s *TrainingRuntimeSpecWrapper) NumNodes(numNodes int32) *TrainingRuntimeSpecWrapper {
	s.MLPolicy = &omev1beta1.MLPolicy{
		NumNodes: &numNodes,
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) TorchPolicy(numNodes int32, numProcPerNode string) *TrainingRuntimeSpecWrapper {
	s.MLPolicy = &omev1beta1.MLPolicy{
		NumNodes: &numNodes,
		MLPolicyConfig: omev1beta1.MLPolicyConfig{
			Torch: &omev1beta1.TorchMLPolicyConfig{
				NumProcPerNode: &numProcPerNode,
			},
		},
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) ContainerTrainer(image string, command []string, args []string, res corev1.ResourceList) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Image = image
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Command = command
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Args = args
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Resources.Requests = res
				}
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) ContainerTrainerEnv(env []corev1.EnvVar) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobTrainerNode {
			for j, container := range rJob.Template.Spec.Template.Spec.Containers {
				if container.Name == constants.ContainerTrainer {
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.Containers[j].Env = env
				}
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) InitContainerDatasetModelInitializer(image string, command []string, args []string, res corev1.ResourceList) *TrainingRuntimeSpecWrapper {
	for i, rJob := range s.Template.Spec.ReplicatedJobs {
		if rJob.Name == constants.JobInitializer {
			for j, container := range rJob.Template.Spec.Template.Spec.InitContainers {
				if container.Name == constants.ContainerDatasetInitializer || container.Name == constants.ContainerModelInitializer {
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Image = image
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Command = command
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Args = args
					s.Template.Spec.ReplicatedJobs[i].Template.Spec.Template.Spec.InitContainers[j].Resources.Requests = res
				}
			}
		}
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) PodGroupPolicyCoscheduling(timeout int32) *TrainingRuntimeSpecWrapper {
	s.PodGroupPolicy = &omev1beta1.PodGroupPolicy{
		CoschedulingPodGroupPolicyConfig: &omev1beta1.CoschedulingPodGroupPolicyConfig{
			ScheduleTimeoutSeconds: &timeout,
		},
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) PodGroupPolicyCoschedulingSchedulingTimeout(timeout int32) *TrainingRuntimeSpecWrapper {
	if s.PodGroupPolicy == nil || s.PodGroupPolicy.CoschedulingPodGroupPolicyConfig == nil {
		return s.PodGroupPolicyCoscheduling(timeout)
	}
	s.PodGroupPolicy.CoschedulingPodGroupPolicyConfig.ScheduleTimeoutSeconds = &timeout
	return s
}

func (s *TrainingRuntimeSpecWrapper) TorchElasticPolicy(maxRestarts, minNodes, maxNodes int32) *TrainingRuntimeSpecWrapper {
	if s.MLPolicy == nil || s.MLPolicy.Torch == nil {
		s.MLPolicy = &omev1beta1.MLPolicy{
			MLPolicyConfig: omev1beta1.MLPolicyConfig{
				Torch: &omev1beta1.TorchMLPolicyConfig{},
			},
		}
	}
	s.MLPolicy.Torch.ElasticPolicy = &omev1beta1.TorchElasticPolicy{
		MaxRestarts: &maxRestarts,
		MinNodes:    &minNodes,
		MaxNodes:    &maxNodes,
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) MPIPolicy(numNodes int32, numProcPerNode int32) *TrainingRuntimeSpecWrapper {
	s.MLPolicy = &omev1beta1.MLPolicy{
		NumNodes: &numNodes,
		MLPolicyConfig: omev1beta1.MLPolicyConfig{
			MPI: &omev1beta1.MPIMLPolicyConfig{
				NumProcPerNode: &numProcPerNode,
			},
		},
	}
	return s
}

func (s *TrainingRuntimeSpecWrapper) Obj() omev1beta1.TrainingRuntimeSpec {
	return s.TrainingRuntimeSpec
}

type SchedulerPluginsPodGroupWrapper struct {
	schedulerpluginsv1alpha1.PodGroup
}

func MakeSchedulerPluginsPodGroup(namespace, name string) *SchedulerPluginsPodGroupWrapper {
	return &SchedulerPluginsPodGroupWrapper{
		PodGroup: schedulerpluginsv1alpha1.PodGroup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: schedulerpluginsv1alpha1.SchemeGroupVersion.String(),
				Kind:       constants.PodGroupKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}

func (p *SchedulerPluginsPodGroupWrapper) MinMember(members int32) *SchedulerPluginsPodGroupWrapper {
	p.PodGroup.Spec.MinMember = members
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) MinResources(resources corev1.ResourceList) *SchedulerPluginsPodGroupWrapper {
	p.PodGroup.Spec.MinResources = resources
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) SchedulingTimeout(timeout int32) *SchedulerPluginsPodGroupWrapper {
	p.PodGroup.Spec.ScheduleTimeoutSeconds = &timeout
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) ControllerReference(gvk schema.GroupVersionKind, name, uid string) *SchedulerPluginsPodGroupWrapper {
	p.OwnerReferences = append(p.OwnerReferences, metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                types.UID(uid),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	})
	return p
}

func (p *SchedulerPluginsPodGroupWrapper) Obj() *schedulerpluginsv1alpha1.PodGroup {
	return &p.PodGroup
}
