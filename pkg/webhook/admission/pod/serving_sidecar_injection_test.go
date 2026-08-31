package pod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

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

func ptrTo[T any](v T) *T { return &v }

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
