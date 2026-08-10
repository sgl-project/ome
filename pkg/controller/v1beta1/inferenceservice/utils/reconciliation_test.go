package utils

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func ptrInt(i int) *int { return &i }

// TestMergeRuntimeSpecs_SchedulerNamePrecedence pins the schedulerName
// resolution order across the full merge pipeline (component merge +
// top-level back-fill): ISVC pod-spec levels beat
// runtime component-config levels, which beat the runtime's top-level
// schedulerName; with nothing set anywhere the field stays empty.
func TestMergeRuntimeSpecs_SchedulerNamePrecedence(t *testing.T) {
	log := logr.Discard()

	t.Run("top-level name reaches component, leader, worker, and router levels", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{SchedulerName: "custom-scheduler"},
			EngineConfig: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
			},
			DecoderConfig: &v1beta1.DecoderSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{Size: ptrInt(1)},
			},
			RouterConfig: &v1beta1.RouterSpec{},
		}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Engine:  &v1beta1.EngineSpec{},
				Decoder: &v1beta1.DecoderSpec{},
				Router:  &v1beta1.RouterSpec{},
			},
		}

		engine, decoder, router, err := MergeRuntimeSpecs(isvc, runtime, log)
		require.NoError(t, err)
		require.NotNil(t, engine)
		require.NotNil(t, decoder)
		require.NotNil(t, router)

		assert.Equal(t, "custom-scheduler", engine.SchedulerName)
		require.NotNil(t, engine.Leader)
		assert.Equal(t, "custom-scheduler", engine.Leader.SchedulerName,
			"rendered leader pods source their template from Leader.PodSpec — it must carry the runtime-level name")
		require.NotNil(t, engine.Worker)
		assert.Equal(t, "custom-scheduler", engine.Worker.SchedulerName,
			"rendered worker pods source their template from Worker.PodSpec — it must carry the runtime-level name")
		assert.Equal(t, "custom-scheduler", decoder.SchedulerName)
		assert.Equal(t, "custom-scheduler", decoder.Leader.SchedulerName)
		assert.Equal(t, "custom-scheduler", decoder.Worker.SchedulerName)
		assert.Equal(t, "custom-scheduler", router.SchedulerName)
	})

	t.Run("runtime leader and worker levels override top-level", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{SchedulerName: "top-scheduler"},
			EngineConfig: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{PodSpec: v1beta1.PodSpec{SchedulerName: "leader-scheduler"}},
				Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
			},
		}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}},
		}

		engine, _, _, err := MergeRuntimeSpecs(isvc, runtime, log)
		require.NoError(t, err)

		assert.Equal(t, "leader-scheduler", engine.Leader.SchedulerName)
		assert.Equal(t, "top-scheduler", engine.Worker.SchedulerName,
			"a worker level left unset must still fall back to the top-level name")
	})

	t.Run("ISVC levels override every runtime level", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{SchedulerName: "top-scheduler"},
			EngineConfig: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{PodSpec: v1beta1.PodSpec{SchedulerName: "runtime-leader-scheduler"}},
				Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
			},
			RouterConfig: &v1beta1.RouterSpec{},
		}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Engine: &v1beta1.EngineSpec{
					Leader: &v1beta1.LeaderSpec{PodSpec: v1beta1.PodSpec{SchedulerName: "isvc-leader-scheduler"}},
					Worker: &v1beta1.WorkerSpec{PodSpec: v1beta1.PodSpec{SchedulerName: "isvc-worker-scheduler"}},
				},
				Router: &v1beta1.RouterSpec{
					PodSpec: v1beta1.PodSpec{SchedulerName: "isvc-router-scheduler"},
				},
			},
		}

		engine, _, router, err := MergeRuntimeSpecs(isvc, runtime, log)
		require.NoError(t, err)

		assert.Equal(t, "isvc-leader-scheduler", engine.Leader.SchedulerName)
		assert.Equal(t, "isvc-worker-scheduler", engine.Worker.SchedulerName)
		assert.Equal(t, "isvc-router-scheduler", router.SchedulerName)
	})

	t.Run("nothing set leaves every level empty", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			EngineConfig: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
			},
		}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}},
		}

		engine, _, _, err := MergeRuntimeSpecs(isvc, runtime, log)
		require.NoError(t, err)

		assert.Empty(t, engine.SchedulerName)
		assert.Empty(t, engine.Leader.SchedulerName)
		assert.Empty(t, engine.Worker.SchedulerName)
	})

	t.Run("top-level name applies when the runtime has no engine config", func(t *testing.T) {
		runtime := &v1beta1.ServingRuntimeSpec{
			ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{SchedulerName: "custom-scheduler"},
		}
		isvc := &v1beta1.InferenceService{
			Spec: v1beta1.InferenceServiceSpec{
				Engine: &v1beta1.EngineSpec{
					Leader: &v1beta1.LeaderSpec{},
					Worker: &v1beta1.WorkerSpec{Size: ptrInt(2)},
				},
			},
		}

		engine, _, _, err := MergeRuntimeSpecs(isvc, runtime, log)
		require.NoError(t, err)

		assert.Equal(t, "custom-scheduler", engine.SchedulerName)
		assert.Equal(t, "custom-scheduler", engine.Leader.SchedulerName)
		assert.Equal(t, "custom-scheduler", engine.Worker.SchedulerName)
	})
}
