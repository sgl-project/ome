package pod

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

func TestNewServingSidecarInjector(t *testing.T) {
	tests := []struct {
		name        string
		configData  map[string]string
		expectError bool
	}{
		{
			name: "valid config",
			configData: map[string]string{
				servingSidecarConfigMapKeyName: `{"image":"ome/agent:1","compartmentId":"c1","authType":"InstancePrincipal"}`,
			},
			expectError: false,
		},
		{
			name:        "missing key yields zero-value injector",
			configData:  map[string]string{},
			expectError: false,
		},
		{
			name: "malformed JSON returns error instead of panicking",
			configData: map[string]string{
				servingSidecarConfigMapKeyName: `{"image": not-json`,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap := &v1.ConfigMap{Data: tt.configData}
			var injector *ServingSidecarInjector
			var err error
			assert.NotPanics(t, func() {
				injector, err = newServingSidecarInjector(configMap)
			})
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, injector)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, injector)
			}
		})
	}
}

// The sidecar reads its weight-info file path from its own config file;
// the injector must not set the corresponding env var, which would
// override that value in every pod.
func TestInjectServingSidecarEnvVars(t *testing.T) {
	configMap := &v1.ConfigMap{Data: map[string]string{
		servingSidecarConfigMapKeyName: `{
			"image": "ome/agent:1",
			"memoryRequest": "100Mi",
			"memoryLimit": "200Mi",
			"cpuRequest": "100m",
			"cpuLimit": "200m",
			"compartmentId": "c1",
			"authType": "InstancePrincipal",
			"region": "us-test-1"
		}`,
	}}
	injector, err := newServingSidecarInjector(configMap)
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment",
			Namespace: "default",
			Annotations: map[string]string{
				constants.ServingSidecarInjectionKey:   "true",
				constants.FineTunedWeightFTStrategyKey: "tfew",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: constants.MainContainerName}},
		},
	}
	assert.NoError(t, injector.InjectServingSidecar(pod))

	var sidecar *v1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == constants.ServingSidecarContainerName {
			sidecar = &pod.Spec.Containers[i]
		}
	}
	assert.NotNil(t, sidecar, "serving sidecar container not injected")

	envByName := map[string]string{}
	for _, env := range sidecar.Env {
		assert.NotEqual(t, env.Name, env.Value, "env var %s carries its own name as value", env.Name)
		envByName[env.Name] = env.Value
	}
	assert.NotContains(t, envByName, constants.AgentFineTunedWeightInfoFilePath)
	assert.Equal(t, "InstancePrincipal", envByName[constants.AgentAuthTypeEnvVarKey])
	assert.Equal(t, "c1", envByName[constants.AgentCompartmentIDEnvVarKey])
	assert.Equal(t, "us-test-1", envByName[constants.AgentRegionEnvVarKey])
	assert.Equal(t, filepath.Join(constants.ModelDefaultMountPathPrefix, "tfew"), envByName[constants.AgentUnzippedFineTunedWeightDirectory])
	assert.Equal(t, constants.FineTunedWeightDownloadMountPath, envByName[constants.AgentZippedFineTunedWeightDirectory])
}
