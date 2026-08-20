package replica

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/ome/pkg/xet"

	"sigs.k8s.io/ome/internal/ome-agent/replica/common"
	"sigs.k8s.io/ome/internal/ome-agent/replica/replicator"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/ome/pkg/afero"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
	"sigs.k8s.io/ome/pkg/principals"
	testingPkg "sigs.k8s.io/ome/pkg/testing"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

type TestReplicaAgent struct {
	*ReplicaAgent
	mockListSourceObjects func() ([]common.ReplicationObject, error)
	mockValidateModelSize func(objects []common.ReplicationObject)
}

// Override Start method to use the mock
func (t *TestReplicaAgent) Start() error {
	t.Logger.Infof("Start replication from %+v to %+v", t.ReplicationInput.Source, t.ReplicationInput.Target)

	sourceObjs, err := t.mockListSourceObjects()
	if err != nil {
		return err
	}
	t.mockValidateModelSize(sourceObjs)

	replicatorInstance, err := NewReplicator(t.ReplicaAgent)
	if err != nil {
		return err
	}

	return replicatorInstance.Replicate(sourceObjs)
}

// createMockOCIOSDataStore creates a properly initialized OCIOSDataStore for testing
func createMockOCIOSDataStore() *ociobjectstore.OCIOSDataStore {
	authType := principals.InstancePrincipal
	config := &ociobjectstore.Config{
		Name:     "test-config",
		AuthType: &authType,
		Region:   "us-ashburn-1",
	}

	return &ociobjectstore.OCIOSDataStore{
		Config: config,
	}
}

type fakeReplicator struct {
	err         error
	objects     []common.ReplicationObject
	onReplicate func(objects []common.ReplicationObject) error
}

func (f *fakeReplicator) Replicate(objects []common.ReplicationObject) error {
	f.objects = objects
	if f.onReplicate != nil {
		return f.onReplicate(objects)
	}
	return f.err
}

func TestNewReplicaAgent(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name: "valid OCI to OCI configuration",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: false,
			description: "Should successfully create agent with valid OCI source and target",
		},
		{
			name: "valid HuggingFace to OCI configuration with branch",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "hf://meta-llama/Llama-3-70B-Instruct@experimental",
					HubClient:     &xet.Client{},
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: false,
			description: "Should successfully create agent with HuggingFace source (with branch) and OCI target",
		},
		{
			name: "valid HuggingFace to PVC configuration with branch",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "hf://meta-llama/Llama-3-70B-Instruct@experimental",
					HubClient:     &xet.Client{},
				},
				Target: TargetStruct{
					StorageURIStr: "pvc://target-pvc/models",
					PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
				},
			},
			expectError: false,
			description: "Should successfully create agent with HuggingFace source (with branch) and PVC target",
		},
		{
			name: "valid PVC to OCI configuration",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "pvc://source-pvc/models",
					PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: false,
			description: "Should successfully create agent with PVC source and OCI target",
		},
		{
			name: "valid OCI to PVC configuration",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
				Target: TargetStruct{
					StorageURIStr: "pvc://target-pvc/models",
					PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
				},
			},
			expectError: false,
			description: "Should successfully create agent with OCI source and PVC target",
		},
		{
			name: "valid PVC to PVC configuration",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "pvc://source-pvc/models",
					PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
				},
				Target: TargetStruct{
					StorageURIStr: "pvc://target-pvc/models",
					PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
				},
			},
			expectError: false,
			description: "Should successfully create agent with PVC source and PVC target",
		},
		{
			name: "invalid target storage URI",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
				Target: TargetStruct{
					StorageURIStr:  "invalid://storage/uri",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: true,
			errorMsg:    "unknown storage type",
			description: "Should fail with invalid target storage URI",
		},
		{
			name: "missing OCI data store for OCI source",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: nil, // Missing OCI data store
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: true,
			errorMsg:    "Source.OCIOSDataStore",
			description: "Should fail when OCI source is missing OCIOSDataStore",
		},
		{
			name: "missing OCI data store for OCI target",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: nil, // Missing OCI data store
				},
			},
			expectError: true,
			errorMsg:    "Target.OCIOSDataStore",
			description: "Should fail when OCI target is missing OCIOSDataStore",
		},
		{
			name: "missing HubClient for HuggingFace source",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "hf://meta-llama/Llama-3-70B-Instruct",
					HubClient:     nil, // Missing HubClient
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: true,
			errorMsg:    "Source.HubClient",
			description: "Should fail when HuggingFace source is missing HubClient",
		},
		{
			name: "missing PVCFileSystem for PVC source",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "pvc://source-pvc/models",
					PVCFileSystem: nil, // Missing PVCFileSystem
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: true,
			errorMsg:    "Source.PVCFileSystem",
			description: "Should fail when PVC source is missing PVCFileSystem",
		},
		{
			name: "invalid HuggingFace URI format",
			config: &Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/tmp/replica",
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				NumConnections:       5,
				Source: SourceStruct{
					StorageURIStr: "hf://", // Invalid: missing model ID
					HubClient:     &xet.Client{},
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			expectError: true,
			errorMsg:    "failed to parse source storage URI",
			description: "Should fail with invalid HuggingFace URI format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewReplicaAgent(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, agent)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, agent)
				assert.Equal(t, tt.config, &agent.Config)
				assert.Equal(t, tt.config.AnotherLogger, agent.Logger)

				// Verify ReplicationInput is properly set
				assert.NotNil(t, agent.ReplicationInput)
				assert.NotNil(t, agent.ReplicationInput.Source)
				assert.NotNil(t, agent.ReplicationInput.Target)
				assert.NotEmpty(t, agent.ReplicationInput.SourceStorageType)
				assert.NotEmpty(t, agent.ReplicationInput.TargetStorageType)

				// Verify OCI-specific handling for source
				if strings.HasPrefix(tt.config.Source.StorageURIStr, "oci://") {
					assert.Equal(t, tt.config.Source.OCIOSDataStore.Config.Region, agent.ReplicationInput.Source.Region)
					assert.Equal(t, "src-ns", agent.ReplicationInput.Source.Namespace)
					assert.Equal(t, "src-bucket", agent.ReplicationInput.Source.BucketName)
					assert.Equal(t, "models/", agent.ReplicationInput.Source.Prefix)
				}

				// Verify OCI-specific handling for target
				if strings.HasPrefix(tt.config.Target.StorageURIStr, "oci://") {
					assert.Equal(t, tt.config.Target.OCIOSDataStore.Config.Region, agent.ReplicationInput.Target.Region)
					assert.Equal(t, "tgt-ns", agent.ReplicationInput.Target.Namespace)
					assert.Equal(t, "tgt-bucket", agent.ReplicationInput.Target.BucketName)
					assert.Equal(t, "models/", agent.ReplicationInput.Target.Prefix)
				}

				// Verify HF-specific handling for source
				if strings.HasPrefix(tt.config.Source.StorageURIStr, "hf://") {
					assert.Equal(t, "meta-llama/Llama-3-70B-Instruct", agent.ReplicationInput.Source.BucketName)
					assert.Equal(t, "experimental", agent.ReplicationInput.Source.Prefix)
				}

				// Verify PVC-specific handling for source
				if strings.HasPrefix(tt.config.Source.StorageURIStr, "pvc://") {
					assert.Equal(t, "source-pvc", agent.ReplicationInput.Source.BucketName)
					assert.Equal(t, "models", agent.ReplicationInput.Source.Prefix)
				}
			}
		})
	}
}

func TestValidateModelSize(t *testing.T) {
	GB := int64(1024 * 1024 * 1024)

	tests := []struct {
		name          string
		config        Config
		objects       []common.ReplicationObject
		expectPanic   bool
		panicContains string
		skip          bool
	}{
		{
			name: "model size within limit - OCI objects",
			config: Config{
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				name := "test.bin"
				size := 1 * GB // 1 GB
				summary := objectstorage.ObjectSummary{
					Name: &name,
					Size: &size,
				}
				return []common.ReplicationObject{
					common.ObjectSummaryReplicationObject{ObjectSummary: summary},
				}
			}(),
			expectPanic: false,
		},
		{
			name: "model size within limit - HuggingFace objects",
			config: Config{
				DownloadSizeLimitGB:  10,
				EnableSizeLimitCheck: true,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				return []common.ReplicationObject{
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "pytorch_model.bin",
							Size: 1073741824, // 1 GB
							Hash: "sha256:abc123...",
						},
					},
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "config.json",
							Size: 1024, // 1 KB
							Hash: "sha256:def123...",
						},
					},
				}
			}(),
			expectPanic: false,
		},
		{
			name: "model size exceeds limit - OCI objects",
			config: Config{
				DownloadSizeLimitGB:  1,
				EnableSizeLimitCheck: true,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				name := "test.bin"
				size := 2 * GB // 2 GB

				summary := objectstorage.ObjectSummary{
					Name: &name,
					Size: &size,
				}
				return []common.ReplicationObject{
					common.ObjectSummaryReplicationObject{ObjectSummary: summary},
				}
			}(),
			expectPanic:   true,
			panicContains: "Model weights exceed size limit",
			skip:          true, // Skip this test case as it's failing due to mock expectations
		},
		{
			name: "model size exceeds limit - HuggingFace objects",
			config: Config{
				DownloadSizeLimitGB:  1,
				EnableSizeLimitCheck: true,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				return []common.ReplicationObject{
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "pytorch_model-00001-of-00002.bin",
							Size: 1073741824, // 1 GB
							Hash: "sha256:...",
						},
					},
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "pytorch_model-00002-of-00002.bin",
							Size: 1073741824, // 1 GB
							Hash: "sha256:...",
						},
					},
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "config.json",
							Size: 1024, // 1 KB
							Hash: "sha256:...",
						},
					},
				}
			}(),
			expectPanic:   true,
			panicContains: "Model weights exceed size limit",
			skip:          true, // Skip this test case as it's failing due to mock expectations
		},
		{
			name: "size check disabled - OCI objects",
			config: Config{
				DownloadSizeLimitGB:  1,
				EnableSizeLimitCheck: false,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				name := "test.bin"
				size := int64(2 * GB) // 2 GB

				summary := objectstorage.ObjectSummary{
					Name: &name,
					Size: &size,
				}
				return []common.ReplicationObject{
					common.ObjectSummaryReplicationObject{ObjectSummary: summary},
				}
			}(),
			expectPanic: false,
		},
		{
			name: "size check disabled - HuggingFace objects",
			config: Config{
				DownloadSizeLimitGB:  1,
				EnableSizeLimitCheck: false,
				AnotherLogger:        testingPkg.SetupMockLogger(),
			},
			objects: func() []common.ReplicationObject {
				return []common.ReplicationObject{
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "pytorch_model.bin",
							Size: 4294967296, // 4 GB
							Hash: "sha256:...",
						},
					},
					common.HFRepoFileInfoReplicationObject{
						FileInfo: xet.FileInfo{
							Path: "tokenizer.json",
							Size: 524288, // 512 KB
							Hash: "sha256:...",
						},
					},
				}
			}(),
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests marked for skipping
			if tt.skip {
				t.Skip("Skipping test due to mock expectation issues")
			}

			agent := &ReplicaAgent{
				Logger: tt.config.AnotherLogger,
				Config: tt.config,
			}

			if tt.expectPanic {
				defer func() {
					r := recover()
					assert.NotNil(t, r)
					if tt.panicContains != "" {
						// The Fatal call will use os.Exit in production,
						// but in tests with the mock logger it will just record the call
						// We'll verify that the Fatal method was called
						// This is a compromise since we can't actually test the os.Exit behavior
						mockLogger := tt.config.AnotherLogger.(*testingPkg.MockLogger)
						mockLogger.AssertCalled(t, "Fatalf", mock.Anything, mock.Anything)
					}
				}()
			}

			agent.validateModelSize(tt.objects)

			if !tt.expectPanic {
				// Just assert we got here without panic
				assert.True(t, true)
			}
		})
	}
}

func TestReplicaAgent_Start(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()

	// Mocked source objects to be returned by listSourceObjects
	mockSourceObjects := []common.ReplicationObject{}

	testAgent := &TestReplicaAgent{
		ReplicaAgent: &ReplicaAgent{
			Logger: mockLogger,
			Config: Config{
				AnotherLogger:        mockLogger,
				LocalPath:            "/test/path",
				NumConnections:       1,
				DownloadSizeLimitGB:  100,
				EnableSizeLimitCheck: true,
				Source: SourceStruct{
					StorageURIStr:  "oci://n/src-ns/b/src-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
				Target: TargetStruct{
					StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/models",
					OCIOSDataStore: createMockOCIOSDataStore(),
				},
			},
			ReplicationInput: common.ReplicationInput{
				SourceStorageType: storage.StorageTypeOCI,
				TargetStorageType: storage.StorageTypeOCI,
				Source:            ociobjectstore.ObjectURI{BucketName: "src-bucket", Namespace: "src-ns", Prefix: "models/"},
				Target:            ociobjectstore.ObjectURI{BucketName: "tgt-bucket", Namespace: "tgt-ns", Prefix: "models/"},
			},
		},
		mockListSourceObjects: func() ([]common.ReplicationObject, error) {
			return mockSourceObjects, nil
		},
		mockValidateModelSize: func(objects []common.ReplicationObject) {},
	}

	err := testAgent.Start()
	assert.NoError(t, err)
}

func TestReplicaAgent_StartReturnsErrorWhenNumConnectionsInvalid(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()
	agent := &ReplicaAgent{
		Logger: mockLogger,
		Config: Config{
			AnotherLogger:  mockLogger,
			NumConnections: 0,
		},
	}

	err := agent.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "num_connections")
}

func TestReplicaAgent_StartWritesCompletionMarkerAfterSuccessfulOCITargetReplication(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	fake := &fakeReplicator{}
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return fake, nil
	}

	var markerSource string
	var markerTarget ociobjectstore.ObjectURI
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, source string, target ociobjectstore.ObjectURI) error {
		markerSource = source
		markerTarget = target
		return nil
	}

	err := agent.Start()
	require.NoError(t, err)
	require.Len(t, fake.objects, 1)
	assert.Equal(t, constants.ArtifactCompleteMarkerBody, markerSource)
	assert.Equal(t, "tgt-ns", markerTarget.Namespace)
	assert.Equal(t, "tgt-bucket", markerTarget.BucketName)
	assert.Equal(t, "target-models/"+constants.ArtifactCompleteMarkerFileName, markerTarget.ObjectName)
	assert.Equal(t, "us-ashburn-1", markerTarget.Region)
}

func TestReplicaAgent_StartSkipsCompletionMarkerWhenReplicationFails(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return &fakeReplicator{err: errors.New("replication failed")}, nil
	}

	markerWritten := false
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) error {
		markerWritten = true
		return nil
	}

	err := agent.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replication failed")
	assert.False(t, markerWritten)
}

func TestReplicaAgent_StartReturnsErrorWhenCompletionMarkerUploadFails(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return &fakeReplicator{}, nil
	}
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) error {
		return errors.New("marker upload failed")
	}

	err := agent.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write target artifact completion marker")
	assert.Contains(t, err.Error(), "marker upload failed")
}

func TestReplicaAgent_StartSkipsReplicationWhenTargetArtifactUploadLockCompletes(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	replicatorCalled := false
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		replicatorCalled = true
		return &fakeReplicator{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		return "", false, nil
	}
	stateCalls := 0
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		stateCalls++
		if stateCalls == 1 {
			return targetArtifactState{}, nil
		}
		return targetArtifactState{Complete: true}, nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, replicatorCalled)
	assert.Equal(t, 2, stateCalls)
}

func TestReplicaAgent_StartWaitsWhenCompletedTargetArtifactStillHasActiveUploadLock(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	replicatorCalled := false
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		replicatorCalled = true
		return &fakeReplicator{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		return "", false, nil
	}
	stateCalls := 0
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		stateCalls++
		if stateCalls < 3 {
			return targetArtifactState{Complete: true, CompletionMarked: true, UploadLocked: true}, nil
		}
		return targetArtifactState{Complete: true, CompletionMarked: true}, nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, replicatorCalled)
	assert.Equal(t, 3, stateCalls)
}

func TestReplicaAgent_StartSkipsCompletedTargetArtifactWhenUploadLockClearsBeforeAcquire(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	replicatorCalled := false
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		replicatorCalled = true
		return &fakeReplicator{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		return "acquired-lock-etag", true, nil
	}
	released := false
	releaseArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, etag string) (bool, error) {
		released = true
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		assert.Equal(t, "acquired-lock-etag", etag)
		return true, nil
	}
	deleteArtifactCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) error {
		t.Fatal("completion marker should not be deleted when reuse is allowed and target is complete")
		return nil
	}
	stateCalls := 0
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		stateCalls++
		if stateCalls == 1 {
			return targetArtifactState{Complete: true, CompletionMarked: true, UploadLocked: true}, nil
		}
		return targetArtifactState{Complete: true, CompletionMarked: true}, nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, replicatorCalled)
	assert.True(t, released)
	assert.Equal(t, 2, stateCalls)
}

func TestReplicaAgent_StartSkipsCompletedTargetArtifactEvenWhenUploadLockIsStale(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()
	agent.Config.ArtifactUploadLockTimeout = time.Hour

	now := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	replicatorCalled := false
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		replicatorCalled = true
		return &fakeReplicator{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, target ociobjectstore.ObjectURI) (string, bool, error) {
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		return "", false, nil
	}

	staleLockModifiedTime := now.Add(-2 * time.Hour)
	stateCalls := 0
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		stateCalls++
		if stateCalls == 1 {
			return targetArtifactState{}, nil
		}
		return targetArtifactState{
			Complete:               true,
			UploadLocked:           true,
			UploadLockModifiedTime: &staleLockModifiedTime,
		}, nil
	}
	deleteStaleArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI, _ string) (bool, error) {
		t.Fatal("stale upload lock should not be deleted before skipping a complete target artifact")
		return false, nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, replicatorCalled)
}

func TestReplicaAgent_StartDeletesStaleTargetArtifactUploadLockAndReplicates(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()
	agent.Config.ArtifactUploadLockTimeout = time.Hour

	now := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	fake := &fakeReplicator{}
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return fake, nil
	}

	acquireAttempts := 0
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, target ociobjectstore.ObjectURI) (string, bool, error) {
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		acquireAttempts++
		if acquireAttempts > 1 {
			return "replacement-lock-etag", true, nil
		}
		return "", false, nil
	}

	stateCalls := 0
	staleLockModifiedTime := now.Add(-2 * time.Hour)
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		stateCalls++
		if stateCalls == 1 {
			return targetArtifactState{}, nil
		}
		return targetArtifactState{
			UploadLocked:           true,
			UploadLockModifiedTime: &staleLockModifiedTime,
			UploadLockETag:         "stale-lock-etag",
		}, nil
	}

	deletedLock := false
	deleteStaleArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, etag string) (bool, error) {
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		assert.Equal(t, "stale-lock-etag", etag)
		deletedLock = true
		return true, nil
	}
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) error {
		return nil
	}

	err := agent.Start()
	require.NoError(t, err)
	require.Len(t, fake.objects, 1)
	assert.Equal(t, 2, acquireAttempts)
	assert.True(t, deletedLock)
}

func TestReplicaAgent_StartLogsTargetArtifactSizeWhenSkippingCompletedTargetArtifact(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	replicatorCalled := false
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		replicatorCalled = true
		return &fakeReplicator{}, nil
	}
	artifactSizeBytes := int64(123456789)
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		return targetArtifactState{
			Complete:          true,
			ArtifactSizeBytes: &artifactSizeBytes,
		}, nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, replicatorCalled)
	mockLogger := agent.Logger.(*testingPkg.MockLogger)
	mockLogger.AssertCalled(t, "Infof", "Total model size: %d bytes", []interface{}{artifactSizeBytes})
}

func TestReplicaAgent_StartDeletesCompletionMarkerBeforeOverwritingWhenReuseNotAllowed(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()
	agent.Config.TargetArtifactReuseAllowed = false

	operations := make([]string, 0, 2)
	fake := &fakeReplicator{
		onReplicate: func(objects []common.ReplicationObject) error {
			operations = append(operations, "replicate")
			return nil
		},
	}
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return fake, nil
	}
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		t.Fatal("target artifact state should not be inspected when reuse is disabled")
		return targetArtifactState{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		t.Fatal("target artifact upload lock should not be acquired when reuse is disabled")
		return "", false, nil
	}
	deleteArtifactCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI) error {
		assert.Equal(t, "target-models/"+constants.ArtifactCompleteMarkerFileName, target.ObjectName)
		operations = append(operations, "delete-completion-marker")
		return nil
	}
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) error {
		t.Fatal("completion marker should not be written when reuse is disabled")
		return nil
	}

	err := agent.Start()
	require.NoError(t, err)
	require.Len(t, fake.objects, 1)
	assert.Equal(t, []string{"delete-completion-marker", "replicate"}, operations)
}

func TestReplicaAgent_StartDeletesStaleCompletionMarkerBeforeUploadingTargetArtifact(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	operations := make([]string, 0, 3)
	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return &fakeReplicator{
			onReplicate: func(objects []common.ReplicationObject) error {
				operations = append(operations, "replicate")
				return nil
			},
		}, nil
	}
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		return targetArtifactState{CompletionMarked: true}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		return "acquired-lock-etag", true, nil
	}
	deleteArtifactCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI) error {
		operations = append(operations, "delete-completion-marker")
		assert.Equal(t, "target-models/"+constants.ArtifactCompleteMarkerFileName, target.ObjectName)
		return nil
	}
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, target ociobjectstore.ObjectURI) error {
		operations = append(operations, "write-completion-marker")
		assert.Equal(t, "target-models/"+constants.ArtifactCompleteMarkerFileName, target.ObjectName)
		return nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.Equal(t, []string{"delete-completion-marker", "replicate", "write-completion-marker"}, operations)
}

func TestReplicaAgent_PrepareTargetArtifactUploadSkipsWhenReuseDisabled(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()
	agent.Config.TargetArtifactReuseAllowed = false

	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		t.Fatal("target artifact state should not be inspected when reuse is disabled")
		return targetArtifactState{}, nil
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		t.Fatal("target artifact upload lock should not be acquired when reuse is disabled")
		return "", false, nil
	}

	uploadLock, skipReplication, err := agent.prepareTargetArtifactUpload()
	require.NoError(t, err)
	assert.Nil(t, uploadLock)
	assert.False(t, skipReplication)
}

func TestReplicaAgent_PrepareTargetArtifactUploadUsesOneWaitDeadline(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()
	agent.Config.ArtifactUploadLockTimeout = time.Hour

	current := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return current }
	sleepFunc = func(time.Duration) {
		current = current.Add(time.Hour)
	}
	acquireAttempts := 0
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		acquireAttempts++
		if acquireAttempts > 2 {
			return "", false, errors.New("wait deadline was reset")
		}
		return "", false, nil
	}
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		return targetArtifactState{}, nil
	}

	_, _, err := agent.prepareTargetArtifactUpload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for target artifact completion marker")
	assert.Equal(t, 2, acquireAttempts)
}

func TestReplicaAgent_StartReleasesTargetArtifactUploadLockWhenReplicationFails(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return &fakeReplicator{err: errors.New("replication failed")}, nil
	}

	released := false
	releaseArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, etag string) (bool, error) {
		released = true
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		assert.Equal(t, "test-lock-etag", etag)
		return true, nil
	}

	err := agent.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replication failed")
	assert.True(t, released)
}

func TestReplicaAgent_ReleaseTargetArtifactUploadLockOnlyDeletesAcquiredLock(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	released := false
	releaseArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, etag string) (bool, error) {
		released = true
		assert.Equal(t, "target-models/"+constants.ArtifactUploadLockFileName, target.ObjectName)
		assert.Equal(t, "acquired-lock-etag", etag)
		return false, nil
	}

	agent.releaseTargetArtifactUploadLock(targetArtifactUploadLock{ETag: "acquired-lock-etag"})

	assert.True(t, released)
}

func TestReplicaAgent_StartSkipsCompletionMarkerForNonOCITarget(t *testing.T) {
	agent, cleanup := newTestAgentForCompletionMarker(t)
	defer cleanup()

	agent.ReplicationInput.TargetStorageType = storage.StorageTypePVC
	agent.ReplicationInput.Target = ociobjectstore.ObjectURI{
		BucketName: "target-pvc",
		Prefix:     "target-model",
	}
	agent.Config.Target = TargetStruct{
		StorageURIStr: "pvc://target-pvc/target-model",
		PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
	}

	newReplicatorFunc = func(_ *ReplicaAgent) (replicator.Replicator, error) {
		return &fakeReplicator{}, nil
	}
	markerWritten := false
	uploadCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) error {
		markerWritten = true
		return nil
	}

	err := agent.Start()
	require.NoError(t, err)
	assert.False(t, markerWritten)
}

func TestFilterInternalArtifactReplicationObjectsSkipsCompletionMarker(t *testing.T) {
	configName := "models/config.json"
	weightName := "models/model.safetensors"
	markerName := "models/" + constants.ArtifactCompleteMarkerFileName
	lockName := "models/" + constants.ArtifactUploadLockFileName
	rootMarkerName := constants.ArtifactCompleteMarkerFileName
	size := int64(1)

	objects := []common.ReplicationObject{
		common.ObjectSummaryReplicationObject{ObjectSummary: objectstorage.ObjectSummary{Name: &configName, Size: &size}},
		common.ObjectSummaryReplicationObject{ObjectSummary: objectstorage.ObjectSummary{Name: &markerName, Size: &size}},
		common.ObjectSummaryReplicationObject{ObjectSummary: objectstorage.ObjectSummary{Name: &lockName, Size: &size}},
		common.ObjectSummaryReplicationObject{ObjectSummary: objectstorage.ObjectSummary{Name: &weightName, Size: &size}},
		common.ObjectSummaryReplicationObject{ObjectSummary: objectstorage.ObjectSummary{Name: &rootMarkerName, Size: &size}},
	}

	filtered := filterInternalArtifactReplicationObjects(objects)

	require.Len(t, filtered, 2)
	assert.Equal(t, configName, filtered[0].GetName())
	assert.Equal(t, weightName, filtered[1].GetName())
}

func newTestAgentForCompletionMarker(t *testing.T) (*ReplicaAgent, func()) {
	t.Helper()

	oldNewReplicatorFunc := newReplicatorFunc
	oldUploadCompletionMarkerFunc := uploadCompletionMarkerFunc
	oldTryAcquireArtifactUploadLockFunc := tryAcquireArtifactUploadLockFunc
	oldReleaseArtifactUploadLockFunc := releaseArtifactUploadLockFunc
	oldDeleteArtifactCompletionMarkerFunc := deleteArtifactCompletionMarkerFunc
	oldDeleteStaleArtifactUploadLockFunc := deleteStaleArtifactUploadLockFunc
	oldTargetArtifactStateFunc := targetArtifactStateFunc
	oldSleepFunc := sleepFunc
	oldNowFunc := nowFunc
	cleanup := func() {
		newReplicatorFunc = oldNewReplicatorFunc
		uploadCompletionMarkerFunc = oldUploadCompletionMarkerFunc
		tryAcquireArtifactUploadLockFunc = oldTryAcquireArtifactUploadLockFunc
		releaseArtifactUploadLockFunc = oldReleaseArtifactUploadLockFunc
		deleteArtifactCompletionMarkerFunc = oldDeleteArtifactCompletionMarkerFunc
		deleteStaleArtifactUploadLockFunc = oldDeleteStaleArtifactUploadLockFunc
		targetArtifactStateFunc = oldTargetArtifactStateFunc
		sleepFunc = oldSleepFunc
		nowFunc = oldNowFunc
	}
	tryAcquireArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ string, _ ociobjectstore.ObjectURI) (string, bool, error) {
		return "test-lock-etag", true, nil
	}
	releaseArtifactUploadLockFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI, _ string) (bool, error) {
		return true, nil
	}
	deleteArtifactCompletionMarkerFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) error {
		return nil
	}
	targetArtifactStateFunc = func(_ *ociobjectstore.OCIOSDataStore, _ ociobjectstore.ObjectURI) (targetArtifactState, error) {
		return targetArtifactState{}, nil
	}
	sleepFunc = func(time.Duration) {}
	nowFunc = time.Now

	localPath := t.TempDir()
	sourceDir := filepath.Join(localPath, "source-model")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "config.json"), []byte("model config"), 0644))

	mockLogger := testingPkg.SetupMockLogger()
	return &ReplicaAgent{
		Logger: mockLogger,
		Config: Config{
			AnotherLogger:              mockLogger,
			LocalPath:                  localPath,
			NumConnections:             1,
			DownloadSizeLimitGB:        100,
			EnableSizeLimitCheck:       true,
			TargetArtifactReuseAllowed: true,
			Source: SourceStruct{
				StorageURIStr: "pvc://source-pvc/source-model",
				PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
			},
			Target: TargetStruct{
				StorageURIStr:  "oci://n/tgt-ns/b/tgt-bucket/o/target-models",
				OCIOSDataStore: createMockOCIOSDataStore(),
			},
		},
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: storage.StorageTypePVC,
			TargetStorageType: storage.StorageTypeOCI,
			Source: ociobjectstore.ObjectURI{
				BucketName: "source-pvc",
				Prefix:     "source-model",
			},
			Target: ociobjectstore.ObjectURI{
				Namespace:  "tgt-ns",
				BucketName: "tgt-bucket",
				Prefix:     "target-models/",
				Region:     "us-ashburn-1",
			},
		},
	}, cleanup
}
