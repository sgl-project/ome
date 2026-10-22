package pod

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

func TestNewServingSidecarInjector_ValidMissingOrBadJSON_OnlyBadJSONErrors(t *testing.T) {
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
func TestInjectServingSidecar_AgentConfig_SetsAgentEnvWithoutWeightInfoPath(t *testing.T) {
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

// newTestServingSidecarInjector builds an injector from a fully-populated
// servingSidecar config block, the shape a production
// inferenceservice-config ConfigMap carries.
func newTestServingSidecarInjector(t *testing.T, authType string) *ServingSidecarInjector {
	t.Helper()
	injector, err := newServingSidecarInjector(&v1.ConfigMap{Data: map[string]string{
		servingSidecarConfigMapKeyName: `{
  "image": "ghcr.io/test/serving-agent:test",
  "compartmentId": "compartment-1",
  "authType": "` + authType + `",
  "region": "region-1",
  "memoryRequest": "100Mi",
  "memoryLimit": "1Gi",
  "cpuRequest": "100m",
  "cpuLimit": "1"
}`,
	}})
	require.NoError(t, err)
	return injector
}

// sidecarTestPod returns a pod carrying the sidecar trigger annotations
// and a main container, the minimum surface injectServingSidecar reads.
func sidecarTestPod(annotations map[string]string) *v1.Pod {
	base := map[string]string{
		constants.ServingSidecarInjectionKey:   "true",
		constants.FineTunedWeightFTStrategyKey: "lora",
	}
	for k, v := range annotations {
		base[k] = v
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "sidecar-pod",
			Annotations: base,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: constants.MainContainerName,
				SecurityContext: &v1.SecurityContext{
					RunAsUser: ptrTo(int64(1001)),
				},
			}},
		},
	}
}

func TestInjectServingSidecar_AnnotationGating(t *testing.T) {
	injector := newTestServingSidecarInjector(t, "InstancePrincipal")

	t.Run("absent annotation is a no-op", func(t *testing.T) {
		pod := sidecarTestPod(nil)
		delete(pod.Annotations, constants.ServingSidecarInjectionKey)
		assert.NoError(t, injector.InjectServingSidecar(pod))
		assert.Len(t, pod.Spec.Containers, 1)
	})

	t.Run("annotation false is a no-op", func(t *testing.T) {
		pod := sidecarTestPod(map[string]string{constants.ServingSidecarInjectionKey: "false"})
		assert.NoError(t, injector.InjectServingSidecar(pod))
		assert.Len(t, pod.Spec.Containers, 1)
	})
}

func TestInjectServingSidecar_HappyPath(t *testing.T) {
	injector := newTestServingSidecarInjector(t, "InstancePrincipal")
	pod := sidecarTestPod(nil)

	require.NoError(t, injector.InjectServingSidecar(pod))
	require.Len(t, pod.Spec.Containers, 2, "sidecar must be appended after the main container")

	sidecar := pod.Spec.Containers[1]
	assert.Equal(t, constants.ServingSidecarContainerName, sidecar.Name)
	assert.Equal(t, "ghcr.io/test/serving-agent:test", sidecar.Image)
	assert.Equal(t, []string{"serving-agent", "--config", "/ome-agent.yaml", "--debug"}, sidecar.Args)
	assert.Equal(t, v1.TerminationMessageFallbackToLogsOnError, sidecar.TerminationMessagePolicy)

	// Resources come straight from the config block.
	assert.Equal(t, resource.MustParse("1"), sidecar.Resources.Limits[v1.ResourceCPU])
	assert.Equal(t, resource.MustParse("1Gi"), sidecar.Resources.Limits[v1.ResourceMemory])
	assert.Equal(t, resource.MustParse("100m"), sidecar.Resources.Requests[v1.ResourceCPU])
	assert.Equal(t, resource.MustParse("100Mi"), sidecar.Resources.Requests[v1.ResourceMemory])

	// Env carries the agent auth/config surface plus the strategy-derived
	// weight directories.
	env := map[string]string{}
	for _, e := range sidecar.Env {
		env[e.Name] = e.Value
	}
	assert.Equal(t, "InstancePrincipal", env[constants.AgentAuthTypeEnvVarKey])
	assert.Equal(t, "compartment-1", env[constants.AgentCompartmentIDEnvVarKey])
	assert.Equal(t, "region-1", env[constants.AgentRegionEnvVarKey])
	assert.Equal(t, "/opt/ml/lora", env[constants.AgentUnzippedFineTunedWeightDirectory],
		"unzipped dir must be the strategy-suffixed model mount prefix")
	assert.Equal(t, constants.FineTunedWeightDownloadMountPath, env[constants.AgentZippedFineTunedWeightDirectory])

	// Mount order: download dir first, then the strategy-scoped weight dir,
	// both backed by the shared model emptyDir.
	require.Len(t, sidecar.VolumeMounts, 2)
	assert.Equal(t, constants.ModelEmptyDirVolumeName, sidecar.VolumeMounts[0].Name)
	assert.Equal(t, constants.FineTunedWeightDownloadMountPath, sidecar.VolumeMounts[0].MountPath)
	assert.Equal(t, constants.FineTunedWeightDownloadVolumeMountSubPath, sidecar.VolumeMounts[0].SubPath)
	assert.Equal(t, constants.ModelEmptyDirVolumeName, sidecar.VolumeMounts[1].Name)
	assert.Equal(t, "/opt/ml/lora", sidecar.VolumeMounts[1].MountPath)
	assert.Equal(t, constants.FineTunedWeightVolumeMountSubPath, sidecar.VolumeMounts[1].SubPath)

	// Security context is inherited from the main container as a deep copy:
	// same values, distinct pointer.
	require.NotNil(t, sidecar.SecurityContext)
	assert.Equal(t, pod.Spec.Containers[0].SecurityContext, sidecar.SecurityContext)
	assert.NotSame(t, pod.Spec.Containers[0].SecurityContext, sidecar.SecurityContext)
}

// TestInjectServingSidecar_Reinvocation pins webhook-reinvocation safety
// (reinvocationPolicy=IfNeeded): a second pass over an already-injected
// pod must not append a duplicate sidecar.
func TestInjectServingSidecar_Reinvocation(t *testing.T) {
	injector := newTestServingSidecarInjector(t, "InstancePrincipal")
	pod := sidecarTestPod(nil)

	require.NoError(t, injector.InjectServingSidecar(pod))
	require.Len(t, pod.Spec.Containers, 2)
	first := pod.DeepCopy()

	require.NoError(t, injector.InjectServingSidecar(pod))
	assert.Equal(t, first.Spec, pod.Spec, "re-invocation must leave the pod unchanged")
}

// TestInjectServingSidecar_ExistingSidecarShortCircuits proves the
// presence check runs before config validation: a pod that already has
// the sidecar container is accepted even when the injector config is
// incomplete.
func TestInjectServingSidecar_ExistingSidecarShortCircuits(t *testing.T) {
	injector, err := newServingSidecarInjector(&v1.ConfigMap{Data: map[string]string{}})
	require.NoError(t, err)

	pod := sidecarTestPod(nil)
	pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
		Name: constants.ServingSidecarContainerName,
	})

	assert.NoError(t, injector.InjectServingSidecar(pod))
	assert.Len(t, pod.Spec.Containers, 2)
}

func TestInjectServingSidecar_MissingFTStrategy_Errors(t *testing.T) {
	injector := newTestServingSidecarInjector(t, "InstancePrincipal")
	pod := sidecarTestPod(nil)
	delete(pod.Annotations, constants.FineTunedWeightFTStrategyKey)

	err := injector.InjectServingSidecar(pod)
	assert.Error(t, err)
	assert.Len(t, pod.Spec.Containers, 1, "no sidecar on error")
}

func TestInjectServingSidecar_MissingMainContainer_Errors(t *testing.T) {
	injector := newTestServingSidecarInjector(t, "InstancePrincipal")
	pod := sidecarTestPod(nil)
	pod.Spec.Containers = []v1.Container{{Name: "other-container"}}

	err := injector.InjectServingSidecar(pod)
	assert.Error(t, err, "security-context inheritance requires the main container")
	assert.Len(t, pod.Spec.Containers, 1)
}

func TestInjectServingSidecar_IncompleteConfig_Errors(t *testing.T) {
	// Image/compartment/auth are declared required; a config block missing
	// them must be rejected at injection time, not silently render an
	// empty-image sidecar.
	injector, err := newServingSidecarInjector(&v1.ConfigMap{Data: map[string]string{
		servingSidecarConfigMapKeyName: `{"image": "ghcr.io/test/serving-agent:test"}`,
	}})
	require.NoError(t, err)

	pod := sidecarTestPod(nil)
	assert.Error(t, injector.InjectServingSidecar(pod))
	assert.Len(t, pod.Spec.Containers, 1)
}

func TestInjectServingSidecar_OKEWorkloadIdentity(t *testing.T) {
	injector := newTestServingSidecarInjector(t, constants.AuthtypeOKEWorkloadIdentity)

	t.Run("without service account is rejected", func(t *testing.T) {
		pod := sidecarTestPod(nil)
		assert.Error(t, injector.InjectServingSidecar(pod))
		assert.Len(t, pod.Spec.Containers, 1)
	})

	t.Run("with service account injects and enables token automount", func(t *testing.T) {
		pod := sidecarTestPod(nil)
		pod.Spec.ServiceAccountName = "workload-identity-sa"
		require.NoError(t, injector.InjectServingSidecar(pod))
		assert.Len(t, pod.Spec.Containers, 2)
		require.NotNil(t, pod.Spec.AutomountServiceAccountToken)
		assert.True(t, *pod.Spec.AutomountServiceAccountToken)
	})
}
