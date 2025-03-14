package pod

import (
	"encoding/json"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"github.com/go-playground/validator/v10"
	v1 "k8s.io/api/core/v1"
)

const (
	trainingSidecarConfigMapKeyName = "trainingSidecar"
)

// TrainingSidecarInjector represents configuration parameters for the training sidecar container.
type TrainingSidecarInjector struct {
	Image                 string `json:"image" validate:"required"`
	Region                string `json:"region"`
	Namespace             string `json:"namespace"`
	FineTunedModelBucket  string `json:"fineTunedModelBucket"`
	TrainingMetricsBucket string `json:"trainingMetricsBucket"`
	RealmDomainComponent  string `json:"realmDomainComponent"`
}

// newTrainingSidecarInjector initializes a TrainingSidecarInjector from a ConfigMap.
func newTrainingSidecarInjector(configMap *v1.ConfigMap) *TrainingSidecarInjector {
	trainingSidecarInjector := &TrainingSidecarInjector{}
	if trainingSidecarConfigVal, ok := configMap.Data[trainingSidecarConfigMapKeyName]; ok {
		if err := json.Unmarshal([]byte(trainingSidecarConfigVal), trainingSidecarInjector); err != nil {
			panic(fmt.Errorf("unable to unmarshal %v json string: %w", trainingSidecarConfigMapKeyName, err))
		}
	}
	return trainingSidecarInjector
}

// InjectTrainingSidecar injects the serving sidecar container into the pod if necessary.
func (tsi *TrainingSidecarInjector) InjectTrainingSidecar(pod *v1.Pod) error {
	if enableTrainingSidecar, ok := pod.ObjectMeta.Annotations[constants.TrainingSidecarInjectionKey]; ok && enableTrainingSidecar == "true" {
		return tsi.injectTrainingSidecar(pod)
	}
	return nil
}

func (tsi *TrainingSidecarInjector) injectTrainingSidecar(pod *v1.Pod) error {
	if tsi.containerExists(pod) {
		return nil
	}

	// general validation
	if err := tsi.validate(); err != nil {
		return err
	}

	trainingSidecarMounts := tsi.getVolumeMounts()
	trainingSidecarEnvs := tsi.getTrainingSidecarEnvs()

	securityContext, err := tsi.getMainContainerSecurityContext(pod)
	if err != nil {
		return err
	}

	trainingSidecarContainer := tsi.createTrainingSidecarContainer(trainingSidecarEnvs, trainingSidecarMounts, securityContext)

	pod.Spec.Containers = append(pod.Spec.Containers, *trainingSidecarContainer)

	return nil
}

// containerExists checks if the Training Sidecar container is already in the pod.
func (tsi *TrainingSidecarInjector) containerExists(pod *v1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.TrainingSidecarContainerName {
			return true
		}
	}
	return false
}

func (tsi *TrainingSidecarInjector) validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(tsi); err != nil {
		return fmt.Errorf("failed to validate TrainingSidecarInjector: %w", err)
	}
	return nil
}

func (tsi *TrainingSidecarInjector) getVolumeMounts() []v1.VolumeMount {
	var trainingSidecarMounts []v1.VolumeMount
	// data empty dir volume mount for sidecar container
	dataEmptyDirVolumeMount := v1.VolumeMount{
		Name:      constants.DataEmptyDirName,
		MountPath: constants.PeftTrainingDataEmptyDirMountPath,
		ReadOnly:  false,
	}
	trainingSidecarMounts = append(trainingSidecarMounts, dataEmptyDirVolumeMount)

	// Add region/ad/realm host path volume mounts
	regionADRealmHostPathVolumeMounts := []v1.VolumeMount{
		{
			Name:      constants.RegionFileVolumeName,
			MountPath: constants.RegionFileVolumeMountPath,
		},
		{
			Name:      constants.ADFileVolumeName,
			MountPath: constants.ADFileVolumeMountPath,
		},
		{
			Name:      constants.RealmFileVolumeName,
			MountPath: constants.RealmFileVolumeMountPath,
		},
	}
	trainingSidecarMounts = append(trainingSidecarMounts, regionADRealmHostPathVolumeMounts...)

	return trainingSidecarMounts
}

func (tsi *TrainingSidecarInjector) getTrainingSidecarEnvs() *[]v1.EnvVar {
	trainingSidecarEnvVars := make([]v1.EnvVar, 0)
	// Set env vars from values set in trainingSidecar config map
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.RegionEnvVarKey,
		Value: tsi.Region,
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.TrainingMetricsBucketEnvVarKey,
		Value: tsi.TrainingMetricsBucket,
	})
	trainingSidecarEnvVars = append(trainingSidecarEnvVars, v1.EnvVar{
		Name:  constants.OCIDefaultRealmEnvVarKey,
		Value: tsi.RealmDomainComponent,
	})

	trainingSidecarEnvVars = tsi.setModelStorageEnvs(trainingSidecarEnvVars)

	return &trainingSidecarEnvVars
}

// setModelStorageEnvs sets fine-tuned model bucket and namespace
func (tsi *TrainingSidecarInjector) setModelStorageEnvs(envVars []v1.EnvVar) []v1.EnvVar {
	envVars = append(envVars, v1.EnvVar{
		Name:  constants.NamespaceEnvVarKey,
		Value: tsi.Namespace,
	})
	envVars = append(envVars, v1.EnvVar{
		Name:  constants.BucketNameEnvVarKey,
		Value: tsi.FineTunedModelBucket,
	})

	return envVars
}

// getMainContainerSecurityContext finds and returns the security context of the main container.
func (tsi *TrainingSidecarInjector) getMainContainerSecurityContext(pod *v1.Pod) (*v1.SecurityContext, error) {
	for _, container := range pod.Spec.Containers {
		if container.Name == constants.TrainingMainContainerName {
			return container.SecurityContext.DeepCopy(), nil
		}
	}
	return nil, fmt.Errorf("no main container %s specified", constants.TrainingMainContainerName)
}

func (tsi *TrainingSidecarInjector) createTrainingSidecarContainer(trainingSidecarEnvs *[]v1.EnvVar, trainingSidecarMounts []v1.VolumeMount, securityContext *v1.SecurityContext) *v1.Container {
	// Create sidecar container
	return &v1.Container{
		Name:                     constants.TrainingSidecarContainerName,
		Image:                    tsi.Image,
		TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		Env:                      *trainingSidecarEnvs,
		VolumeMounts:             trainingSidecarMounts,
		SecurityContext:          securityContext,
	}
}
