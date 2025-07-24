package replica

import (
	"github.com/sgl-project/ome/internal/ome-agent/replica/common"
	"github.com/sgl-project/ome/internal/ome-agent/replica/replicator"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

func TestNewReplicator(t *testing.T) {
	dummyLogger := logging.Discard()
	dummyConfig := Config{}
	dummyObj := ociobjectstore.ObjectURI{}

	tests := []struct {
		name              string
		sourceType        storage.StorageType
		targetType        storage.StorageType
		expectType        interface{}
		expectErrContains string
	}{
		{
			name:       "HF to OCI",
			sourceType: storage.StorageTypeHuggingFace,
			targetType: storage.StorageTypeOCI,
			expectType: &replicator.HFToOCIReplicator{},
		},
		{
			name:       "OCI to OCI",
			sourceType: storage.StorageTypeOCI,
			targetType: storage.StorageTypeOCI,
			expectType: &replicator.OCIToOCIReplicator{},
		},
		{
			name:              "Unsupported",
			sourceType:        storage.StorageTypeHuggingFace,
			targetType:        "UNKNOWNSTORAGE",
			expectType:        nil,
			expectErrContains: "unsupported replication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &ReplicaAgent{
				Logger: dummyLogger,
				Config: dummyConfig,
				ReplicationInput: common.ReplicationInput{
					SourceStorageType: tt.sourceType,
					TargetStorageType: tt.targetType,
					Source:            dummyObj,
					Target:            dummyObj,
				},
			}
			rep, err := NewReplicator(agent)
			if tt.expectErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErrContains)
				require.Nil(t, rep)
			} else {
				require.NoError(t, err)
				require.IsType(t, tt.expectType, rep)
			}
		})
	}
}
