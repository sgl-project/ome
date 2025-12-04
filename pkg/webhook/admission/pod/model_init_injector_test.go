package pod

import (
	"reflect"
	"testing"

	"github.com/sgl-project/ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewModelInitInjector(t *testing.T) {
	tests := []struct {
		name       string
		configMap  *v1.ConfigMap
		wantErr    bool
		wantImage  string
		wantFields map[string]string
	}{
		{
			name: "valid config map",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-configmap",
				},
				Data: map[string]string{
					modelInitConfigMapKeyName: `{"image": "test-image", "compartmentId": "test-compartment-id", "authType": "test-auth-type", "vaultId": "test-vault-id"}`,
				},
			},
			wantImage: "test-image",
			wantFields: map[string]string{
				"CompartmentId": "test-compartment-id",
				"AuthType":      "test-auth-type",
				"VaultId":       "test-vault-id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInitInjector := newModelInitInjector(tt.configMap)

			if modelInitInjector.Image != tt.wantImage {
				t.Errorf("expected image to be '%s', but got '%s'", tt.wantImage, modelInitInjector.Image)
			}

			for field, want := range tt.wantFields {
				got := reflect.ValueOf(modelInitInjector).Elem().FieldByName(field).String()
				if got != want {
					t.Errorf("expected %s to be '%s', but got '%s'", field, want, got)
				}
			}
		})
	}
}

func TestModelInitInjector_injectModelInit(t *testing.T) {
	tests := []struct {
		name    string
		pod     *v1.Pod
		wantErr bool
	}{
		{
			name: "pod with model init injection enabled and main container",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.ModelInitInjectionKey: "true",
						constants.BaseModelName:         "test-base-model-name",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: constants.MainContainerName,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInitInjector := &ModelInitInjector{
				Image:         "test-image",
				CompartmentId: "test-compartment-id",
				AuthType:      "test-auth-type",
				VaultId:       "test-vault-id",
				CpuLimit:      "1",
				CpuRequest:    "1",
				MemoryLimit:   "1Gi",
				MemoryRequest: "1Gi",
			}

			err := modelInitInjector.injectModelInit(tt.pod)

			if (err != nil) != tt.wantErr {
				t.Errorf("injectModelInit() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(tt.pod.Spec.InitContainers) != 1 {
				t.Errorf("expected 1 init container, but got %d", len(tt.pod.Spec.InitContainers))
			}
		})
	}
}

func TestModelInitInjector_containerExists(t *testing.T) {
	tests := []struct {
		name string
		pod  *v1.Pod
		want bool
	}{
		{
			name: "pod with model init container",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						{
							Name: constants.ModelInitContainerName,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod without model init container",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelInitInjector := &ModelInitInjector{}

			got := modelInitInjector.containerExists(tt.pod)

			if got != tt.want {
				t.Errorf("containerExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelInitInjector_validateAuth(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		injector *ModelInitInjector
		wantErr  bool
	}{
		{
			name: "pod with OKE workload identity auth type and service account name",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					ServiceAccountName: "test-service-account-name",
				},
			},
			injector: &ModelInitInjector{
				AuthType: constants.AuthtypeOKEWorkloadIdentity,
			},
		},
		{
			name: "pod with OKE workload identity auth type but without service account name",
			pod: &v1.Pod{
				Spec: v1.PodSpec{},
			},
			injector: &ModelInitInjector{
				AuthType: constants.AuthtypeOKEWorkloadIdentity,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.injector.validateAuth(tt.pod)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateAuth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModelInitInjector_getVolumeMounts(t *testing.T) {
	tests := []struct {
		name string
		pod  *v1.Pod
		mi   *ModelInitInjector
		want []v1.VolumeMount
	}{
		{
			name: "test getVolumeMounts",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.BaseModelName: "test-base-model-name",
					},
				},
			},
			mi: &ModelInitInjector{
				ExtraVolumeMounts: &[]v1.VolumeMount{
					{
						Name:      "test-volume-mount-name",
						MountPath: "test-mount-path",
					},
				},
			},
			want: []v1.VolumeMount{
				{
					Name:      constants.ModelEmptyDirVolumeName,
					MountPath: constants.ModelDefaultMountPath,
					SubPath:   "",
				},
				{
					Name:      "test-base-model-name",
					MountPath: constants.ModelDefaultSourcePath,
				},
				{
					Name:      "test-volume-mount-name",
					MountPath: "test-mount-path",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mi.getVolumeMounts(tt.pod)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getVolumeMounts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelInitInjector_getMainContainerSecurityContext(t *testing.T) {
	tests := []struct {
		name    string
		pod     *v1.Pod
		want    *v1.SecurityContext
		wantErr bool
	}{
		{
			name: "test getMainContainerSecurityContext",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: constants.MainContainerName,
							SecurityContext: &v1.SecurityContext{
								RunAsUser: func(i int64) *int64 { return &i }(1000),
							},
						},
					},
				},
			},
			want: &v1.SecurityContext{
				RunAsUser: func(i int64) *int64 { return &i }(1000),
			},
		},
		{
			name: "test getMainContainerSecurityContext with no main container",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := &ModelInitInjector{}
			got, err := mi.getMainContainerSecurityContext(tt.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMainContainerSecurityContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMainContainerSecurityContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelInitInjector_getModelInitEnvs(t *testing.T) {
	tests := []struct {
		name    string
		pod     *v1.Pod
		mi      *ModelInitInjector
		want    []v1.EnvVar
		wantErr bool
	}{
		{
			name: "test getModelInitEnvs with TensorRTLLM model format",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.BaseModelName:                 "test-base-model-name",
						constants.BaseModelFormat:               constants.TensorRTLLM,
						constants.BaseModelFormatVersion:        "test-format-version",
						constants.BaseModelDecryptionKeyName:    "test-decryption-key-name",
						constants.BaseModelDecryptionSecretName: "test-decryption-secret-name",
					},
					Labels: map[string]string{
						constants.BaseModelTypeLabelKey: string(constants.ServingBaseModel),
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: constants.MainContainerName,
							Resources: v1.ResourceRequirements{
								Limits: map[v1.ResourceName]resource.Quantity{
									constants.NvidiaGPUResourceType: resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			mi: &ModelInitInjector{
				CompartmentId: "test-compartment-id",
				AuthType:      "test-auth-type",
				VaultId:       "test-vault-id",
				Region:        "test-region",
			},
			want: []v1.EnvVar{
				{
					Name:  constants.AgentAuthTypeEnvVarKey,
					Value: "test-auth-type",
				},
				{
					Name:  constants.AgentCompartmentIDEnvVarKey,
					Value: "test-compartment-id",
				},
				{
					Name:  constants.AgentVaultIDEnvVarKey,
					Value: "test-vault-id",
				},
				{
					Name:  constants.AgentModelNameEnvVarKey,
					Value: "test-base-model-name",
				},
				{
					Name:  constants.AgentKeyNameEnvVarKey,
					Value: "test-decryption-key-name",
				},
				{
					Name:  constants.AgentSecretNameEnvVarKey,
					Value: "test-decryption-secret-name",
				},
				{
					Name:  constants.AgentDisableModelDecryptionEnvVarKey,
					Value: "false",
				},
				{
					Name:  constants.AgentBaseModelTypeEnvVarKey,
					Value: string(constants.ServingBaseModel),
				},
				{
					Name:  constants.AgentLocalPathEnvVarKey,
					Value: constants.ModelDefaultSourcePath,
				},
				{
					Name:  constants.AgentModelStoreDirectoryEnvVarKey,
					Value: constants.ModelDefaultMountPath,
				},
				{
					Name:  constants.AgentRegionEnvVarKey,
					Value: "test-region",
				},
				{
					Name:  constants.AgentModelFrameworkEnvVarKey,
					Value: constants.TensorRTLLM,
				},
				{
					Name:  constants.AgentTensorRTLLMVersionsEnvVarKey,
					Value: "test-format-version",
				},
				{
					Name:  constants.AgentNumOfGPUEnvVarKey,
					Value: "1",
				},
			},
		},
		{
			name: "test getModelInitEnvs with other model format",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.BaseModelName:                 "test-base-model-name",
						constants.BaseModelFormat:               "other-format",
						constants.BaseModelFormatVersion:        "test-format-version",
						constants.BaseModelDecryptionKeyName:    "test-decryption-key-name",
						constants.BaseModelDecryptionSecretName: "test-decryption-secret-name",
					},
				},
			},
			mi: &ModelInitInjector{
				CompartmentId: "test-compartment-id",
				AuthType:      "test-auth-type",
				VaultId:       "test-vault-id",
				Region:        "test-region",
			},
			want: []v1.EnvVar{
				{
					Name:  constants.AgentAuthTypeEnvVarKey,
					Value: "test-auth-type",
				},
				{
					Name:  constants.AgentCompartmentIDEnvVarKey,
					Value: "test-compartment-id",
				},
				{
					Name:  constants.AgentVaultIDEnvVarKey,
					Value: "test-vault-id",
				},
				{
					Name:  constants.AgentModelNameEnvVarKey,
					Value: "test-base-model-name",
				},
				{
					Name:  constants.AgentKeyNameEnvVarKey,
					Value: "test-decryption-key-name",
				},
				{
					Name:  constants.AgentSecretNameEnvVarKey,
					Value: "test-decryption-secret-name",
				},
				{
					Name:  constants.AgentDisableModelDecryptionEnvVarKey,
					Value: "false",
				},
				{
					Name:  constants.AgentBaseModelTypeEnvVarKey,
					Value: string(constants.ServingBaseModel),
				},
				{
					Name:  constants.AgentLocalPathEnvVarKey,
					Value: constants.ModelDefaultSourcePath,
				},
				{
					Name:  constants.AgentModelStoreDirectoryEnvVarKey,
					Value: constants.ModelDefaultMountPath,
				},
				{
					Name:  constants.AgentRegionEnvVarKey,
					Value: "test-region",
				},
				{
					Name:  constants.AgentModelFrameworkEnvVarKey,
					Value: "otherformat",
				},
			},
		},
		{
			name: "test getModelInitEnvs with other model format and extra env vars",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.BaseModelName:                 "test-base-model-name",
						constants.BaseModelFormat:               "other-format",
						constants.BaseModelFormatVersion:        "test-format-version",
						constants.BaseModelDecryptionKeyName:    "test-decryption-key-name",
						constants.BaseModelDecryptionSecretName: "test-decryption-secret-name",
					},
				},
			},
			mi: &ModelInitInjector{
				CompartmentId: "test-compartment-id",
				AuthType:      "test-auth-type",
				VaultId:       "test-vault-id",
				Region:        "test-region",
				ExtraEnvVars: &[]v1.EnvVar{
					{
						Name:  "EXTRA_ENV_VAR_NAME",
						Value: "extra-env-var-value",
					},
				},
			},
			want: []v1.EnvVar{
				{
					Name:  constants.AgentAuthTypeEnvVarKey,
					Value: "test-auth-type",
				},
				{
					Name:  constants.AgentCompartmentIDEnvVarKey,
					Value: "test-compartment-id",
				},
				{
					Name:  constants.AgentVaultIDEnvVarKey,
					Value: "test-vault-id",
				},
				{
					Name:  constants.AgentModelNameEnvVarKey,
					Value: "test-base-model-name",
				},
				{
					Name:  constants.AgentKeyNameEnvVarKey,
					Value: "test-decryption-key-name",
				},
				{
					Name:  constants.AgentSecretNameEnvVarKey,
					Value: "test-decryption-secret-name",
				},
				{
					Name:  constants.AgentDisableModelDecryptionEnvVarKey,
					Value: "false",
				},
				{
					Name:  constants.AgentBaseModelTypeEnvVarKey,
					Value: string(constants.ServingBaseModel),
				},
				{
					Name:  constants.AgentLocalPathEnvVarKey,
					Value: constants.ModelDefaultSourcePath,
				},
				{
					Name:  constants.AgentModelStoreDirectoryEnvVarKey,
					Value: constants.ModelDefaultMountPath,
				},
				{
					Name:  constants.AgentRegionEnvVarKey,
					Value: "test-region",
				},
				{
					Name:  constants.AgentModelFrameworkEnvVarKey,
					Value: "otherformat",
				},
				{
					Name:  "EXTRA_ENV_VAR_NAME",
					Value: "extra-env-var-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.mi.getModelInitEnvs(tt.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("getModelInitEnvs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getModelInitEnvs() did not return expected env vars, expected %s, got %s", tt.want, got)
			}
		})
	}
}
