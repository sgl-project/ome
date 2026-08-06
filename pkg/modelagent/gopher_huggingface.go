package modelagent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils"
	"sigs.k8s.io/ome/pkg/utils/storage"
	"sigs.k8s.io/ome/pkg/xet"
)

var (
	resolveHfRevision            = resolveHfRevisionWithEndpoint
	snapshotDownloadWithProgress = xet.SnapshotDownloadWithProgress
)

func (s *Gopher) processDirectHfModel(
	ctx context.Context,
	task *GopherTask,
	spec v1beta1.BaseModelSpec,
	modelInfo string,
	modelType string,
	namespace string,
	name string,
	allowFallbackDownload bool,
) (modelTaskOutcome, error) {
	components, err := storage.ParseHuggingFaceStorageURI(*spec.Storage.StorageUri)
	if err != nil {
		s.metrics.RecordFailedDownload(modelType, namespace, name, "invalid_hf_uri")
		s.markModelOnNodeFailed(task)
		return modelTaskCompleted, err
	}
	childPath := getDestPath(&spec, s.modelRootDir)
	token := effectiveHfToken(s.getHfToken(task, spec, modelInfo), s.xetConfig)
	endpoint := DefaultEndpoint
	if s.xetConfig != nil && strings.TrimSpace(s.xetConfig.Endpoint) != "" {
		endpoint = strings.TrimSpace(s.xetConfig.Endpoint)
	}
	identity, resolved := resolveDirectHfIdentity(
		ctx, components.ModelID, components.Branch, token, endpoint, resolveHfRevision,
	)
	if plan, shared := planDirectHfArtifact(task, spec, s.modelRootDir, childPath, identity); resolved && shared {
		download := func(path string) error {
			return s.downloadHfSnapshot(
				ctx, task, components.ModelID, identity.HFCommitSHA, token, path, modelInfo, modelType, namespace, name,
			)
		}
		result, outcome, err := s.processSharedArtifactPlan(ctx, task, plan, allowFallbackDownload, download)
		if err != nil || outcome == modelTaskDeferred {
			return outcome, err
		}
		s.parseModelConfigForTask(childPath, task, result.Artifact)
		return modelTaskCompleted, nil
	}
	if !resolved && spec.Storage.DownloadPolicy != nil && *spec.Storage.DownloadPolicy == v1beta1.ReuseIfExists {
		if !allowFallbackDownload {
			s.demoteToNormalPriority(task)
			return modelTaskDeferred, nil
		}
		s.logger.Warnf("Cannot resolve immutable Hugging Face revision for %s; using the legacy download path without artifact reuse", modelInfo)
	}

	outcome, children, err := s.prepareLegacyHfDestination(ctx, task, childPath)
	if err != nil || outcome == modelTaskDeferred {
		return outcome, err
	}
	downloadRevision := hfDownloadRevision(components.Branch, identity, resolved)
	if err := s.downloadHfSnapshot(
		ctx, task, components.ModelID, downloadRevision, token, childPath, modelInfo, modelType, namespace, name,
	); err != nil {
		return modelTaskCompleted, err
	}
	var artifact *Artifact
	if resolved {
		artifact = s.modelConfigParser.BuildArtifactAttribute(
			identity.HFCommitSHA,
			s.configMapReconciler.getModelConfigMapKey(task.BaseModel, task.ClusterBaseModel),
			childPath,
			children,
		)
	}
	s.parseModelConfigForTask(childPath, task, artifact)
	return modelTaskCompleted, nil
}

func effectiveHfToken(modelToken string, config *xet.Config) string {
	if token := strings.TrimSpace(modelToken); token != "" {
		return token
	}
	if config == nil {
		return ""
	}
	return strings.TrimSpace(config.Token)
}

func hfDownloadRevision(requested string, identity ArtifactIdentity, resolved bool) string {
	if resolved && identity.isValid() {
		return identity.HFCommitSHA
	}
	return requested
}

func (s *Gopher) prepareLegacyHfDestination(
	ctx context.Context,
	task *GopherTask,
	childPath string,
) (modelTaskOutcome, []string, error) {
	modelKey := s.configMapReconciler.getModelConfigMapKey(task.BaseModel, task.ClusterBaseModel)
	outcome, shared, err := s.releasePersistedSharedArtifactForLegacyDownload(ctx, task, childPath)
	if err != nil || outcome == modelTaskDeferred {
		return outcome, nil, err
	}
	if shared {
		return modelTaskCompleted, nil, nil
	}

	children, parentName, _, parseErr := s.parseModelConfigDataEntry(ctx, modelKey)
	isSymlink, linkErr := utils.IsSymbolicLink(childPath)
	if linkErr != nil {
		return modelTaskCompleted, children, nil
	}
	if !isSymlink {
		return modelTaskCompleted, children, nil
	}
	if err := utils.RemoveSymbolicLink(childPath); err != nil {
		return modelTaskCompleted, nil, err
	}
	if parentName != "" && !hasChildrenPaths(children, parseErr) {
		s.removeChildPathFromParentConfigMapIfNecessary(ctx, false, parentName, modelKey, childPath)
	}
	return modelTaskCompleted, children, nil
}

func (s *Gopher) downloadHfSnapshot(
	ctx context.Context,
	task *GopherTask,
	modelID string,
	revision string,
	token string,
	destPath string,
	modelInfo string,
	modelType string,
	namespace string,
	name string,
) error {
	config := s.xetConfig.ToDownloadConfig()
	config.LocalDir = destPath
	config.RepoID = modelID
	config.Revision = revision
	if token != "" {
		config.Token = token
	}

	var lastBytes atomic.Uint64
	var lastTimeNano atomic.Int64
	lastTimeNano.Store(time.Now().UnixNano())
	const (
		progressThrottle     = 30 * time.Second
		progressFlushTimeout = 5 * time.Second
	)
	var latestProgress atomic.Pointer[DownloadProgress]
	stopWorker := make(chan struct{})
	workerDone := make(chan struct{})
	flushProgress := func(progress *DownloadProgress, timeout time.Duration) {
		if progress == nil {
			return
		}
		flushCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := s.configMapReconciler.ReconcileModelProgress(flushCtx, &ConfigMapProgressOp{
			Progress:         progress,
			BaseModel:        task.BaseModel,
			ClusterBaseModel: task.ClusterBaseModel,
		}); err != nil {
			s.logger.Warnf("Failed to update download progress for %s: %v", modelInfo, err)
		}
	}
	go func() {
		defer close(workerDone)
		ticker := time.NewTicker(progressThrottle)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flushProgress(latestProgress.Swap(nil), progressThrottle)
			case <-stopWorker:
				flushProgress(latestProgress.Swap(nil), progressFlushTimeout)
				return
			}
		}
	}()
	defer func() {
		close(stopWorker)
		<-workerDone
	}()

	progressHandler := func(update xet.ProgressUpdate) {
		now := time.Now()
		previousBytes := lastBytes.Load()
		previousTime := lastTimeNano.Load()
		var speed float64
		elapsed := float64(now.UnixNano()-previousTime) / float64(time.Second)
		if elapsed > 0 && update.CompletedBytes > previousBytes {
			speed = float64(update.CompletedBytes-previousBytes) / elapsed
		}
		lastBytes.Store(update.CompletedBytes)
		lastTimeNano.Store(now.UnixNano())
		latestProgress.Store(&DownloadProgress{
			Phase:            update.Phase.String(),
			TotalBytes:       update.TotalBytes,
			CompletedBytes:   update.CompletedBytes,
			TotalFiles:       update.TotalFiles,
			CompletedFiles:   update.CompletedFiles,
			SpeedBytesPerSec: speed,
			LastUpdated:      now.Format(time.RFC3339),
		})
	}

	_, err := snapshotDownloadWithProgress(ctx, config, progressHandler, progressThrottle)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "rate limit") {
		s.metrics.RecordRateLimit(modelType, namespace, name, progressThrottle)
		s.metrics.RecordFailedDownload(modelType, namespace, name, "rate_limit_error")
	} else {
		s.metrics.RecordFailedDownload(modelType, namespace, name, "hf_download_error")
	}
	s.markModelOnNodeFailed(task)
	return fmt.Errorf("failed to download Hugging Face model %s: %w", modelInfo, err)
}
