package replica

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	"sigs.k8s.io/ome/internal/ome-agent/replica/common"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

const (
	GB = 1073741824

	SourceStorageConfigKeyName = "source"
	TargetStorageConfigKeyName = "target"

	targetArtifactLockPollInterval       = 30 * time.Second
	defaultArtifactUploadLockWaitTimeout = 120 * time.Hour
)

var (
	newReplicatorFunc          = NewReplicator
	uploadCompletionMarkerFunc = func(dataStore *ociobjectstore.OCIOSDataStore, source string, target ociobjectstore.ObjectURI) error {
		return dataStore.Upload(source, target)
	}
	tryAcquireArtifactUploadLockFunc = func(dataStore *ociobjectstore.OCIOSDataStore, source string, target ociobjectstore.ObjectURI) (bool, error) {
		return dataStore.UploadIfAbsent(source, target)
	}
	deleteArtifactUploadLockFunc = func(dataStore *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI) error {
		return dataStore.DeleteObject(target)
	}
	deleteArtifactCompletionMarkerFunc = func(dataStore *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI) error {
		return dataStore.DeleteObject(target)
	}
	deleteStaleArtifactUploadLockFunc = func(dataStore *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI, etag string) (bool, error) {
		return dataStore.DeleteObjectIfMatch(target, etag)
	}
	targetArtifactStateFunc = defaultTargetArtifactState
	sleepFunc               = time.Sleep
	nowFunc                 = time.Now
)

type ReplicaAgent struct {
	Logger           logging.Interface
	Config           Config
	ReplicationInput common.ReplicationInput
}

type targetArtifactState struct {
	Complete               bool
	CompletionMarked       bool
	UploadLocked           bool
	UploadLockModifiedTime *time.Time
	UploadLockETag         string
	ArtifactSizeBytes      *int64
}

// NewReplicaAgent constructs a new replica agent from the given configuration.
func NewReplicaAgent(config *Config) (*ReplicaAgent, error) {
	sourceStorageType, err := storage.GetStorageType(config.Source.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get source storage type from source storage URI %s - %w",
			config.Source.StorageURIStr, err)
	}
	targetStorageType, err := storage.GetStorageType(config.Target.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get target storage type from target storage URI %s - %w",
			config.Target.StorageURIStr, err)
	}

	if err = config.ValidateRequiredDependencies(sourceStorageType, targetStorageType); err != nil {
		return nil, fmt.Errorf("failed to validate required dependencies - %w", err)
	}

	sourceObjectURI, err := storage.NewObjectURI(config.Source.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source storage URI %s - %w", config.Source.StorageURIStr, err)
	}
	targetObjectURI, err := storage.NewObjectURI(config.Target.StorageURIStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target storage URI %s - %w", config.Target.StorageURIStr, err)
	}

	if sourceStorageType == storage.StorageTypeOCI {
		sourceObjectURI.Region = config.Source.OCIOSDataStore.Config.Region
		if !strings.HasSuffix(sourceObjectURI.Prefix, "/") && sourceObjectURI.Prefix != "" {
			sourceObjectURI.Prefix += "/"
		}
	}
	if targetStorageType == storage.StorageTypeOCI {
		targetObjectURI.Region = config.Target.OCIOSDataStore.Config.Region
		if !strings.HasSuffix(targetObjectURI.Prefix, "/") && targetObjectURI.Prefix != "" {
			targetObjectURI.Prefix += "/"
		}
	}

	return &ReplicaAgent{
		Logger: config.AnotherLogger,
		Config: *config,
		ReplicationInput: common.ReplicationInput{
			SourceStorageType: sourceStorageType,
			TargetStorageType: targetStorageType,
			Source:            *sourceObjectURI,
			Target:            *targetObjectURI,
		},
	}, nil
}

// Start initiates the replication process.
func (r *ReplicaAgent) Start() error {
	r.Logger.Infof("Start replication from %s %v to %s %v with checksum config %+v", r.ReplicationInput.SourceStorageType, r.ReplicationInput.Source, r.ReplicationInput.TargetStorageType, r.ReplicationInput.Target, r.Config.Target.ChecksumConfig)

	if r.Config.NumConnections <= 0 {
		err := fmt.Errorf("num_connections must be greater than 0")
		r.writeTerminationLog(err.Error())
		return err
	}

	lockAcquired, skipReplication, err := r.prepareTargetArtifactUpload()
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}
	if lockAcquired {
		defer r.releaseTargetArtifactUploadLock()
	}
	if skipReplication {
		return nil
	}
	if lockAcquired {
		skipReplication, err = r.prepareTargetArtifactAfterLockAcquired()
		if err != nil {
			r.writeTerminationLog(err.Error())
			return err
		}
		if skipReplication {
			return nil
		}
	}

	sourceObjs, err := r.listSourceObjects()
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}

	r.validateModelSize(sourceObjs)

	replicatorImp, err := newReplicatorFunc(r)
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}

	err = replicatorImp.Replicate(sourceObjs)
	if err != nil {
		r.writeTerminationLog(err.Error())
		return err
	}

	if err = r.writeCompletionMarker(); err != nil {
		err = fmt.Errorf("failed to write target artifact completion marker: %w", err)
		r.writeTerminationLog(err.Error())
		return err
	}

	return nil
}

func (r *ReplicaAgent) prepareTargetArtifactUpload() (bool, bool, error) {
	if r.ReplicationInput.TargetStorageType != storage.StorageTypeOCI {
		return false, false, nil
	}
	if r.Config.Target.OCIOSDataStore == nil {
		return false, false, fmt.Errorf("target OCI object store data store is nil")
	}
	reuseAllowed := r.Config.TargetArtifactReuseAllowed
	if !reuseAllowed {
		r.Logger.Infof("Target artifact reuse disabled; upload lock still applies")
	}
	waitDeadline := nowFunc().Add(r.targetArtifactUploadLockTimeout())

	state, err := r.targetArtifactState()
	if err != nil {
		return false, false, fmt.Errorf("failed to inspect target artifact state: %w", err)
	}
	if state.Complete {
		if reuseAllowed && r.canReuseCompleteTargetArtifact(state) {
			r.Logger.Infof("Target artifact is already complete; skipping replication")
			r.logTargetArtifactSize(state)
			return false, true, nil
		}
		if reuseAllowed {
			r.Logger.Infof("Target artifact is complete but upload lock still exists; waiting for upload lock release")
		} else {
			r.Logger.Infof("Target artifact is already complete but reuse is disabled; continuing with upload")
		}
	}

	for {
		acquired, err := r.acquireTargetArtifactUploadLock()
		if err != nil {
			return false, false, err
		}
		if acquired {
			return true, false, nil
		}

		r.Logger.Infof("Target artifact upload lock already exists; waiting for completion marker")
		state, err = r.waitForTargetArtifactStateChange(waitDeadline)
		if err != nil {
			return false, false, err
		}
		if state.Complete {
			if reuseAllowed && r.canReuseCompleteTargetArtifact(state) {
				r.Logger.Infof("Target artifact completed while waiting for upload lock; skipping replication")
				r.logTargetArtifactSize(state)
				return false, true, nil
			}
			if !reuseAllowed {
				r.Logger.Infof("Target artifact completed while waiting for upload lock but reuse is disabled; continuing with upload")
			}
		}
		if r.isTargetArtifactUploadLockStale(state) {
			if err := r.deleteStaleTargetArtifactUploadLock(state); err != nil {
				return false, false, err
			}
			continue
		}
	}
}

func (r *ReplicaAgent) canReuseCompleteTargetArtifact(state targetArtifactState) bool {
	// A complete marker is reusable once no active writer owns the prefix. A
	// stale lock can be left behind after completion and should not block reuse.
	return state.Complete && (!state.UploadLocked || r.isTargetArtifactUploadLockStale(state))
}

func (r *ReplicaAgent) logTargetArtifactSize(state targetArtifactState) {
	if state.ArtifactSizeBytes == nil {
		r.Logger.Infof("Target artifact is complete but artifact size is unavailable")
		return
	}
	r.Logger.Infof("Total model size: %d bytes", *state.ArtifactSizeBytes)
}

func (r *ReplicaAgent) acquireTargetArtifactUploadLock() (bool, error) {
	lockURI := r.targetArtifactUploadLockURI()
	r.Logger.Infof("Acquiring target artifact upload lock at oci://n/%s/b/%s/o/%s", lockURI.Namespace, lockURI.BucketName, lockURI.ObjectName)
	acquired, err := tryAcquireArtifactUploadLockFunc(
		r.Config.Target.OCIOSDataStore,
		constants.ArtifactUploadLockBody,
		lockURI,
	)
	if err != nil {
		return false, fmt.Errorf("failed to acquire target artifact upload lock: %w", err)
	}
	return acquired, nil
}

func (r *ReplicaAgent) releaseTargetArtifactUploadLock() {
	lockURI := r.targetArtifactUploadLockURI()
	if err := deleteArtifactUploadLockFunc(r.Config.Target.OCIOSDataStore, lockURI); err != nil {
		r.Logger.Errorf("Failed to release target artifact upload lock at oci://n/%s/b/%s/o/%s: %v", lockURI.Namespace, lockURI.BucketName, lockURI.ObjectName, err)
	}
}

func (r *ReplicaAgent) waitForTargetArtifactStateChange(deadline time.Time) (targetArtifactState, error) {
	for {
		if !nowFunc().Before(deadline) {
			return targetArtifactState{}, fmt.Errorf("timed out waiting for target artifact completion marker")
		}
		sleepFunc(targetArtifactLockPollInterval)

		state, err := r.targetArtifactState()
		if err != nil {
			return targetArtifactState{}, fmt.Errorf("failed to inspect target artifact state while waiting for upload lock: %w", err)
		}
		if !state.UploadLocked || r.isTargetArtifactUploadLockStale(state) {
			return state, nil
		}
	}
}

func (r *ReplicaAgent) targetArtifactUploadLockTimeout() time.Duration {
	if r.Config.ArtifactUploadLockTimeout > 0 {
		return r.Config.ArtifactUploadLockTimeout
	}
	return defaultArtifactUploadLockWaitTimeout
}

func (r *ReplicaAgent) isTargetArtifactUploadLockStale(state targetArtifactState) bool {
	if !state.UploadLocked || state.UploadLockModifiedTime == nil {
		return false
	}
	return !nowFunc().Before(state.UploadLockModifiedTime.Add(r.targetArtifactUploadLockTimeout()))
}

func (r *ReplicaAgent) deleteStaleTargetArtifactUploadLock(state targetArtifactState) error {
	lockURI := r.targetArtifactUploadLockURI()
	modifiedAt := "unknown"
	if state.UploadLockModifiedTime != nil {
		modifiedAt = state.UploadLockModifiedTime.Format(time.RFC3339)
	}
	if state.UploadLockETag == "" {
		return fmt.Errorf("cannot delete stale target artifact upload lock without etag")
	}
	r.Logger.Infof(
		"Deleting stale target artifact upload lock at oci://n/%s/b/%s/o/%s; modifiedAt=%s timeout=%s",
		lockURI.Namespace,
		lockURI.BucketName,
		lockURI.ObjectName,
		modifiedAt,
		r.targetArtifactUploadLockTimeout(),
	)
	deleted, err := deleteStaleArtifactUploadLockFunc(r.Config.Target.OCIOSDataStore, lockURI, state.UploadLockETag)
	if err != nil {
		return fmt.Errorf("failed to delete stale target artifact upload lock: %w", err)
	}
	if !deleted {
		r.Logger.Infof("Stale target artifact upload lock changed before deletion; retrying lock acquisition")
	}
	return nil
}

func (r *ReplicaAgent) targetArtifactState() (targetArtifactState, error) {
	return targetArtifactStateFunc(r.Config.Target.OCIOSDataStore, r.targetArtifactPrefixURI())
}

func defaultTargetArtifactState(dataStore *ociobjectstore.OCIOSDataStore, target ociobjectstore.ObjectURI) (targetArtifactState, error) {
	objects, err := dataStore.ListObjects(target)
	if err != nil {
		return targetArtifactState{}, err
	}

	completeMarkerName := normalizeObjectPrefix(target.Prefix) + constants.ArtifactCompleteMarkerFileName
	uploadLockName := normalizeObjectPrefix(target.Prefix) + constants.ArtifactUploadLockFileName
	var hasCompleteMarker bool
	var hasArtifactObject bool
	var hasUploadLock bool
	var artifactSizeBytes int64
	state := targetArtifactState{}
	for _, object := range objects {
		if object.Name == nil {
			continue
		}
		switch *object.Name {
		case completeMarkerName:
			hasCompleteMarker = true
		case uploadLockName:
			hasUploadLock = true
			stateTime := objectSummaryTime(object)
			if stateTime != nil {
				lockModifiedTime := *stateTime
				state.UploadLockModifiedTime = &lockModifiedTime
			}
			if object.Etag != nil {
				state.UploadLockETag = *object.Etag
			}
		default:
			if !constants.IsInternalArtifactObjectName(*object.Name) {
				hasArtifactObject = true
				if object.Size != nil {
					artifactSizeBytes += *object.Size
				}
			}
		}
	}

	state.CompletionMarked = hasCompleteMarker
	state.Complete = hasCompleteMarker && hasArtifactObject
	state.UploadLocked = hasUploadLock
	if artifactSizeBytes > 0 {
		state.ArtifactSizeBytes = &artifactSizeBytes
	}
	return state, nil
}

func objectSummaryTime(object objectstorage.ObjectSummary) *time.Time {
	if object.TimeModified != nil {
		return &object.TimeModified.Time
	}
	if object.TimeCreated != nil {
		return &object.TimeCreated.Time
	}
	return nil
}

func (r *ReplicaAgent) prepareTargetArtifactAfterLockAcquired() (bool, error) {
	if r.ReplicationInput.TargetStorageType != storage.StorageTypeOCI {
		return false, nil
	}
	if r.Config.Target.OCIOSDataStore == nil {
		return false, fmt.Errorf("target OCI object store data store is nil")
	}

	state, err := r.targetArtifactState()
	if err != nil {
		return false, fmt.Errorf("failed to inspect target artifact state before upload: %w", err)
	}
	if r.Config.TargetArtifactReuseAllowed && state.Complete {
		r.Logger.Infof("Target artifact completed before upload started; skipping replication")
		r.logTargetArtifactSize(state)
		return true, nil
	}
	if !state.CompletionMarked {
		return false, nil
	}

	markerURI := r.targetArtifactCompleteMarkerURI()
	r.Logger.Infof("Deleting target artifact completion marker before upload at oci://n/%s/b/%s/o/%s", markerURI.Namespace, markerURI.BucketName, markerURI.ObjectName)
	if err := deleteArtifactCompletionMarkerFunc(r.Config.Target.OCIOSDataStore, markerURI); err != nil {
		return false, fmt.Errorf("failed to delete target artifact completion marker before upload: %w", err)
	}
	return false, nil
}

func (r *ReplicaAgent) writeCompletionMarker() error {
	if r.ReplicationInput.TargetStorageType != storage.StorageTypeOCI {
		r.Logger.Infof("Skipping target artifact completion marker for non-OCI target storage type %s", r.ReplicationInput.TargetStorageType)
		return nil
	}
	if r.Config.Target.OCIOSDataStore == nil {
		return fmt.Errorf("target OCI object store data store is nil")
	}

	markerURI := r.targetArtifactCompleteMarkerURI()
	r.Logger.Infof("Writing target artifact completion marker to oci://n/%s/b/%s/o/%s", markerURI.Namespace, markerURI.BucketName, markerURI.ObjectName)
	return uploadCompletionMarkerFunc(
		r.Config.Target.OCIOSDataStore,
		constants.ArtifactCompleteMarkerBody,
		markerURI,
	)
}

func (r *ReplicaAgent) targetArtifactCompleteMarkerURI() ociobjectstore.ObjectURI {
	return ociobjectstore.ObjectURI{
		Namespace:  r.ReplicationInput.Target.Namespace,
		BucketName: r.ReplicationInput.Target.BucketName,
		ObjectName: normalizeObjectPrefix(r.ReplicationInput.Target.Prefix) + constants.ArtifactCompleteMarkerFileName,
		Region:     r.ReplicationInput.Target.Region,
	}
}

func (r *ReplicaAgent) targetArtifactPrefixURI() ociobjectstore.ObjectURI {
	return ociobjectstore.ObjectURI{
		Namespace:  r.ReplicationInput.Target.Namespace,
		BucketName: r.ReplicationInput.Target.BucketName,
		Prefix:     normalizeObjectPrefix(r.ReplicationInput.Target.Prefix),
		Region:     r.ReplicationInput.Target.Region,
	}
}

func (r *ReplicaAgent) targetArtifactUploadLockURI() ociobjectstore.ObjectURI {
	return ociobjectstore.ObjectURI{
		Namespace:  r.ReplicationInput.Target.Namespace,
		BucketName: r.ReplicationInput.Target.BucketName,
		ObjectName: normalizeObjectPrefix(r.ReplicationInput.Target.Prefix) + constants.ArtifactUploadLockFileName,
		Region:     r.ReplicationInput.Target.Region,
	}
}

func normalizeObjectPrefix(prefix string) string {
	if prefix == "" || strings.HasSuffix(prefix, "/") {
		return prefix
	}
	return prefix + "/"
}

func (r *ReplicaAgent) writeTerminationLog(message string) {
	f, err := os.OpenFile("/dev/termination-log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to open /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
		return
	}

	if _, err = fmt.Fprintln(f, message); err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to write to /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
	}

	if err = f.Close(); err != nil {
		if _, ferr := fmt.Fprintf(os.Stderr, "Failed to close /dev/termination-log: %v\n", err); ferr != nil {
			r.Logger.Errorf("Failed to write error to os.Stderr: %v", ferr)
		}
	}
}

func (r *ReplicaAgent) listSourceObjects() ([]common.ReplicationObject, error) {
	switch r.ReplicationInput.SourceStorageType {
	case storage.StorageTypeOCI:
		listOfObjectSummary, err := r.Config.Source.OCIOSDataStore.ListObjects(r.ReplicationInput.Source)
		if err != nil {
			return nil, err
		}
		sourceObjects := common.ConvertToReplicationObjectsFromObjectSummary(listOfObjectSummary)
		sourceObjects = filterInternalArtifactReplicationObjects(sourceObjects)
		r.Logger.Infof("Listed %d model weight objects under prefix %s", len(sourceObjects), r.ReplicationInput.Source.Prefix)
		return sourceObjects, nil
	case storage.StorageTypeHuggingFace:
		repoFiles, err := r.Config.Source.HubClient.ListFiles(r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under model %s with %s branch", len(repoFiles), r.ReplicationInput.Source.BucketName, r.ReplicationInput.Source.Prefix)
		return common.ConvertToReplicationObjectsFromHFRepoFileInfo(repoFiles), nil
	case storage.StorageTypePVC:
		sourceDirPath := filepath.Join(r.Config.LocalPath, r.ReplicationInput.Source.Prefix)
		files, err := r.Config.Source.PVCFileSystem.ListFiles(sourceDirPath)
		if err != nil {
			return nil, err
		}
		r.Logger.Infof("Listed %d model weight files under path %s", len(files), sourceDirPath)
		return common.ConvertToReplicationObjectsFromPVCFileEntry(files), nil
	default:
		return nil, fmt.Errorf("unsupported source storage type: %s", string(r.ReplicationInput.SourceStorageType))
	}
}

func filterInternalArtifactReplicationObjects(objects []common.ReplicationObject) []common.ReplicationObject {
	filtered := make([]common.ReplicationObject, 0, len(objects))
	for _, object := range objects {
		if constants.IsInternalArtifactObjectName(object.GetName()) {
			continue
		}
		filtered = append(filtered, object)
	}
	return filtered
}

func (r *ReplicaAgent) validateModelSize(objects []common.ReplicationObject) {
	r.Logger.Info("Calculating model size from source")

	sizeLimit := int64(r.Config.DownloadSizeLimitGB) * GB
	var totalSize int64

	for _, object := range objects {
		if object.GetName() == "" || object.GetSize() == 0 {
			r.Logger.Errorf("Invalid object with missing name or size: %+v", object)
			continue
		}

		totalSize += object.GetSize()
		if r.Config.EnableSizeLimitCheck && totalSize > sizeLimit {
			r.Logger.Fatalf("Model weights exceed size limit of %d bytes", sizeLimit)
		}
	}

	if totalSize == 0 {
		r.Logger.Fatal("No model weights exist in the model folder")
	}
	r.Logger.Infof("Total model size: %d bytes", totalSize)
}
