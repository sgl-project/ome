package pvc

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/shared"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// Backend handles PVC-backed BaseModels: spawns a metadata-extraction
// Job, reads the per-PVC ConfigMap output. Bypasses the per-node
// ConfigMap flow entirely (no Status.NodesReady entries). A nil
// OmeAgentConfig surfaces as PVCConfigMissing via status.
type Backend struct {
	omeAgentConfig *controllerconfig.OmeAgentConfig
}

func New(omeAgentConfig *controllerconfig.OmeAgentConfig) Backend {
	return Backend{omeAgentConfig: omeAgentConfig}
}

func (Backend) Name() string { return "pvc" }

func (Backend) Matches(spec *v1beta1.BaseModelSpec) bool { return IsPVCStorage(spec) }

func (b Backend) Reconcile(ctx context.Context, a shared.BackendArgs) (ctrl.Result, error) {
	return Reconcile(ctx, a.Client, a.Scheme, a.Log, a.Obj, a.Spec, a.IsClusterScoped, omeAgentConfigToMetadataJobConfig(b.omeAgentConfig))
}

func (Backend) HandleDeletion(ctx context.Context, a shared.BackendArgs) (ctrl.Result, error) {
	return HandleModelDeletion(ctx, a.Client, a.Log, a.Obj, a.IsClusterScoped, a.Finalizer)
}

func omeAgentConfigToMetadataJobConfig(c *controllerconfig.OmeAgentConfig) MetadataJobConfig {
	if c == nil {
		return MetadataJobConfig{}
	}
	return MetadataJobConfig{
		Image:                   c.Image,
		ServiceAccount:          c.ServiceAccount,
		CPURequest:              c.CPURequest,
		MemoryRequest:           c.MemoryRequest,
		CPULimit:                c.CPULimit,
		MemoryLimit:             c.MemoryLimit,
		BackoffLimit:            c.BackoffLimit,
		TTLSecondsAfterFinished: c.TTLSecondsAfterFinished,
		NodeSelector:            c.NodeSelector,
		Tolerations:             c.Tolerations,
		Affinity:                c.Affinity,
		PriorityClassName:       c.PriorityClassName,
	}
}
