package replicator

import (
	"sigs.k8s.io/ome/internal/ome-agent/replica/common"
)

type Replicator interface {
	Replicate(objects []common.ReplicationObject) error
}
