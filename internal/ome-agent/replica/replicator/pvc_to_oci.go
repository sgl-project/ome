package replicator

import (
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type PVCToOCIReplicator struct {
	Logger           logging.Interface
	Config           PVCToOCIReplicatorConfig
	ReplicationInput common.ReplicationInput
}

type PVCToOCIReplicatorConfig struct {
	Logger logging.Interface

	OCIOSDataStore *ociobjectstore.OCIOSDataStore
}

func (r *PVCToOCIReplicator) Replicate(objects []common.ReplicationObject) error {
	return nil
}
