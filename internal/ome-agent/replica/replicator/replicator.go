package replicator

import (
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
)

type Replicator interface {
	Replicate(objects []common.ReplicationObject) error
}
