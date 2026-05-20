package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	hfmodelconfig "github.com/sgl-project/ome/pkg/hfutil/modelconfig"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	defaultIntegrityCheckInterval     = 10 * time.Minute
	defaultIntegrityDeepCheckInterval = 6 * time.Hour
	defaultIntegrityStartupJitter     = 30 * time.Second

	artifactManifestDir = ".ome"
)

type IntegrityConfig struct {
	CheckInterval     time.Duration
	DeepCheckInterval time.Duration
	StartupJitter     time.Duration
}

func DefaultIntegrityConfig() IntegrityConfig {
	return IntegrityConfig{
		CheckInterval:     defaultIntegrityCheckInterval,
		DeepCheckInterval: defaultIntegrityDeepCheckInterval,
		StartupJitter:     defaultIntegrityStartupJitter,
	}
}

func (c IntegrityConfig) normalized() IntegrityConfig {
	if c.StartupJitter < 0 {
		c.StartupJitter = 0
	}
	return c
}

func (c IntegrityConfig) enabled() bool {
	return c.CheckInterval > 0
}

func (c IntegrityConfig) deepEnabled() bool {
	return c.enabled() && c.DeepCheckInterval > 0
}

type integrityCheckType string

const (
	integrityCheckBasic integrityCheckType = "basic"
	integrityCheckDeep  integrityCheckType = "deep"
)

type integrityResult string

const (
	integrityResultSuccess      integrityResult = "success"
	integrityResultFailure      integrityResult = "failure"
	integrityResultInconclusive integrityResult = "inconclusive"
	integrityResultSkipped      integrityResult = "skipped"
)

type integrityReason string

const (
	integrityReasonOK                 integrityReason = "ok"
	integrityReasonBaselineCreated    integrityReason = "baseline_created"
	integrityReasonMissingPath        integrityReason = "missing_path"
	integrityReasonPathNotDirectory   integrityReason = "path_not_directory"
	integrityReasonMissingConfig      integrityReason = "missing_config"
	integrityReasonMissingWeight      integrityReason = "missing_weight"
	integrityReasonSizeMismatch       integrityReason = "size_mismatch"
	integrityReasonChecksumMismatch   integrityReason = "checksum_mismatch"
	integrityReasonSafetensorsCorrupt integrityReason = "safetensors_corrupt"
	integrityReasonParseError         integrityReason = "parse_error"
	integrityReasonMetadataError      integrityReason = "metadata_error"
	integrityReasonManifestError      integrityReason = "manifest_error"
	integrityReasonStorageError       integrityReason = "storage_error"
	integrityReasonMarkFailedError    integrityReason = "mark_failed_error"
	integrityReasonSkippedStale       integrityReason = "skipped_stale"
)

type integrityReport struct {
	Result       integrityResult
	Reason       integrityReason
	Message      string
	BytesScanned int64
}

func successReport(reason integrityReason, bytesScanned int64) integrityReport {
	return integrityReport{Result: integrityResultSuccess, Reason: reason, BytesScanned: bytesScanned}
}

func failureReport(reason integrityReason, message string) integrityReport {
	return integrityReport{Result: integrityResultFailure, Reason: reason, Message: message}
}

func inconclusiveReport(reason integrityReason, message string) integrityReport {
	return integrityReport{Result: integrityResultInconclusive, Reason: reason, Message: message}
}

type integrityModelRef struct {
	Key              string
	BaseModel        *v1beta1.BaseModel
	ClusterBaseModel *v1beta1.ClusterBaseModel
}

func (r integrityModelRef) spec() *v1beta1.BaseModelSpec {
	if r.BaseModel != nil {
		return &r.BaseModel.Spec
	}
	if r.ClusterBaseModel != nil {
		return &r.ClusterBaseModel.Spec
	}
	return nil
}

func (r integrityModelRef) task() *GopherTask {
	return &GopherTask{
		BaseModel:        r.BaseModel,
		ClusterBaseModel: r.ClusterBaseModel,
	}
}

func (r integrityModelRef) modelTypeNamespaceName() (string, string, string) {
	return GetModelTypeNamespaceAndName(r.task())
}

func (r integrityModelRef) logName() string {
	return getModelInfoForLogging(r.task())
}

func (s *Gopher) runIntegrityReconciliationLoop(stopCh <-chan struct{}) {
	if !s.integrityConfig.enabled() {
		s.logger.Info("Model artifact integrity reconciliation is disabled")
		return
	}

	s.logger.Infof("Starting model artifact integrity reconciliation with interval %v, deep interval %v",
		s.integrityConfig.CheckInterval, s.integrityConfig.DeepCheckInterval)

	if !cache.WaitForCacheSync(stopCh, s.baseModelSynced, s.clusterBaseModelSynced) {
		s.logger.Warn("Stopping model artifact integrity reconciliation because model informer caches did not sync")
		return
	}

	if jitter := deterministicIntegrityJitter(s.nodeLabelReconciler.nodeName, s.integrityConfig.StartupJitter); jitter > 0 {
		s.logger.Infof("Waiting %v before first model artifact integrity check", jitter)
		timer := time.NewTimer(jitter)
		select {
		case <-timer.C:
		case <-stopCh:
			timer.Stop()
			s.logger.Info("Stopping model artifact integrity reconciliation before first check")
			return
		}
	}

	ticker := time.NewTicker(s.integrityConfig.CheckInterval)
	defer ticker.Stop()

	var lastDeepCheck time.Time
	for {
		checkType := s.integrityCheckTypeForCycle(&lastDeepCheck)
		s.reconcileReadyModelIntegrity(context.Background(), checkType)

		select {
		case <-ticker.C:
		case <-stopCh:
			s.logger.Info("Stopping model artifact integrity reconciliation")
			return
		}
	}
}

func (s *Gopher) integrityCheckTypeForCycle(lastDeepCheck *time.Time) integrityCheckType {
	if !s.integrityConfig.deepEnabled() {
		return integrityCheckBasic
	}
	if lastDeepCheck.IsZero() || time.Since(*lastDeepCheck) >= s.integrityConfig.DeepCheckInterval {
		*lastDeepCheck = time.Now()
		return integrityCheckDeep
	}
	return integrityCheckBasic
}

func (s *Gopher) reconcileReadyModelIntegrity(ctx context.Context, checkType integrityCheckType) {
	start := time.Now()
	summary := struct {
		candidates   int
		success      int
		failure      int
		inconclusive int
		skipped      int
		bytesScanned int64
	}{}

	cm, err := s.configMapReconciler.getConfigMap(ctx)
	if err != nil {
		s.logger.Warnf("Skipping model artifact integrity check: failed to read node ConfigMap: %v", err)
		return
	}

	modelRefs, err := s.buildIntegrityModelRefIndex()
	if err != nil {
		s.logger.Warnf("Skipping model artifact integrity check: failed to list model resources: %v", err)
		return
	}

	for key, raw := range cm.Data {
		var entry ModelEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			s.logger.Warnf("Skipping unparsable model status entry %s during artifact integrity check: %v", key, err)
			summary.skipped++
			continue
		}
		if entry.Status != ModelStatusReady {
			continue
		}

		ref, ok := modelRefs[key]
		if !ok {
			s.logger.Debugf("Skipping Ready model status entry %s during artifact integrity check: model no longer exists or key is unknown", key)
			summary.skipped++
			continue
		}

		spec := ref.spec()
		if spec == nil || spec.Storage == nil || spec.Storage.StorageUri == nil {
			s.recordIntegrityResult(ref, "", checkType, inconclusiveReport(integrityReasonStorageError, "model storage spec is missing"), 0)
			summary.inconclusive++
			continue
		}

		storageType, err := storage.GetStorageType(*spec.Storage.StorageUri)
		if err != nil {
			s.recordIntegrityResult(ref, "", checkType, inconclusiveReport(integrityReasonStorageError, err.Error()), 0)
			summary.inconclusive++
			continue
		}

		if entry.hasStorageIdentity() && !entry.MatchesStorageIdentity(spec) {
			report := integrityReport{Result: integrityResultSkipped, Reason: integrityReasonSkippedStale}
			s.recordIntegrityResult(ref, string(storageType), checkType, report, 0)
			s.logger.Infow("Skipping stale Ready model status entry during artifact integrity check",
				"model", ref.logName(),
				"key", key,
				"entryStorageUri", entry.StorageURI,
				"entryStoragePath", entry.StoragePath)
			summary.skipped++
			continue
		}

		summary.candidates++
		candidateStart := time.Now()
		report := s.validateReadyModelArtifact(ctx, ref, entry, storageType, checkType)
		summary.bytesScanned += report.BytesScanned
		s.recordIntegrityResult(ref, string(storageType), checkType, report, time.Since(candidateStart))

		switch report.Result {
		case integrityResultSuccess:
			summary.success++
			if !entry.hasStorageIdentity() {
				if err := s.backfillReadyStorageIdentity(ctx, ref); err != nil {
					s.logger.Warnf("Artifact integrity check succeeded for %s but failed to backfill storage identity: %v", ref.logName(), err)
				}
			}
		case integrityResultFailure:
			summary.failure++
			s.logger.Warnf("Artifact integrity check failed for %s: reason=%s message=%s", ref.logName(), report.Reason, report.Message)
			if err := s.markIntegrityFailureIfCurrent(ctx, key, ref); err != nil {
				s.logger.Warnf("Failed to mark %s as Failed after artifact integrity failure: %v", ref.logName(), err)
				markReport := integrityReport{Result: integrityResultFailure, Reason: integrityReasonMarkFailedError, Message: err.Error()}
				s.recordIntegrityResult(ref, string(storageType), checkType, markReport, 0)
			}
		case integrityResultInconclusive:
			summary.inconclusive++
			s.logger.Warnf("Artifact integrity check inconclusive for %s: reason=%s message=%s", ref.logName(), report.Reason, report.Message)
		}
	}

	s.logger.Infof("Completed model artifact integrity reconciliation checkType=%s candidates=%d success=%d failure=%d inconclusive=%d skipped=%d bytesScanned=%d duration=%v",
		checkType, summary.candidates, summary.success, summary.failure, summary.inconclusive, summary.skipped, summary.bytesScanned, time.Since(start).Round(time.Millisecond))
}

func (s *Gopher) buildIntegrityModelRefIndex() (map[string]integrityModelRef, error) {
	refs := make(map[string]integrityModelRef)

	clusterBaseModels, err := s.clusterBaseModelLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, clusterBaseModel := range clusterBaseModels {
		key := constants.GetModelConfigMapKey("", clusterBaseModel.Name, true)
		refs[key] = integrityModelRef{Key: key, ClusterBaseModel: clusterBaseModel}
	}

	baseModels, err := s.baseModelLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, baseModel := range baseModels {
		key := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
		refs[key] = integrityModelRef{Key: key, BaseModel: baseModel}
	}

	return refs, nil
}

func (s *Gopher) validateReadyModelArtifact(ctx context.Context, ref integrityModelRef, entry ModelEntry, storageType storage.StorageType, checkType integrityCheckType) integrityReport {
	spec := ref.spec()
	modelPath := entry.StoragePath
	if modelPath == "" {
		resolvedPath, err := resolveArtifactPath(spec, s.modelRootDir)
		if err != nil {
			return inconclusiveReport(integrityReasonStorageError, err.Error())
		}
		modelPath = resolvedPath
	}

	switch storageType {
	case storage.StorageTypeOCI:
		return s.validateOCIArtifact(ctx, ref, modelPath, checkType)
	case storage.StorageTypeHuggingFace, storage.StorageTypeLocal:
		return s.validateFilesystemArtifact(ctx, ref, modelPath, storageType, checkType)
	case storage.StorageTypeVendor, storage.StorageTypePVC:
		return inconclusiveReport(integrityReasonStorageError, fmt.Sprintf("periodic integrity validation is not supported for storage type %s", storageType))
	default:
		return inconclusiveReport(integrityReasonStorageError, fmt.Sprintf("unknown storage type %s", storageType))
	}
}

func (s *Gopher) validateOCIArtifact(ctx context.Context, ref integrityModelRef, modelPath string, checkType integrityCheckType) integrityReport {
	spec := ref.spec()
	fsReport := s.validateFilesystemArtifact(ctx, ref, modelPath, storage.StorageTypeOCI, checkType)
	if fsReport.Result == integrityResultFailure {
		return fsReport
	}

	osUri, err := getTargetDirPath(spec)
	if err != nil {
		return inconclusiveReport(integrityReasonStorageError, err.Error())
	}

	ociOSDataStore, err := s.createOCIOSDataStore(*spec)
	if err != nil {
		return inconclusiveReport(integrityReasonStorageError, err.Error())
	}

	objects, err := ociOSDataStore.ListObjects(*osUri)
	if err != nil {
		return inconclusiveReport(integrityReasonMetadataError, err.Error())
	}

	filter := tensorRTLLMShapeFilterForSpec(spec, s.nodeShapeAlias)
	objects, _, err = filterObjectsForTensorRTLLM(objects, filter)
	if err != nil {
		return failureReport(integrityReasonMissingWeight, err.Error())
	}
	if len(objects) == 0 {
		return inconclusiveReport(integrityReasonMetadataError, "no OCI objects found for model source")
	}

	for _, obj := range objects {
		if obj.Name == nil {
			continue
		}
		source := ociobjectstore.ObjectURI{
			Namespace:  osUri.Namespace,
			BucketName: osUri.BucketName,
			ObjectName: *obj.Name,
			Prefix:     osUri.Prefix,
		}
		localPath := filepath.Join(modelPath, ociobjectstore.TrimObjectPrefix(source.ObjectName, source.Prefix))
		var result ociobjectstore.LocalCopyValidationResult
		var err error
		if checkType == integrityCheckDeep {
			result, err = ociOSDataStore.ValidateLocalCopy(source, localPath)
		} else {
			result, err = validateOCIObjectSummaryLocalCopy(*obj.Name, obj.Size, localPath)
		}
		if err != nil {
			return inconclusiveReport(integrityReasonMetadataError, err.Error())
		}
		switch result.State {
		case ociobjectstore.LocalCopyValidationInvalid:
			return failureReport(ociValidationReason(result.Reason), fmt.Sprintf("%s: %s", source.ObjectName, result.Message))
		case ociobjectstore.LocalCopyValidationInconclusive:
			if checkType == integrityCheckDeep && fsReport.BytesScanned == 0 {
				return inconclusiveReport(integrityReasonMetadataError, fmt.Sprintf("%s: %s", source.ObjectName, result.Message))
			}
			s.logger.Debugf("OCI local copy validation was inconclusive for %s: %s", source.ObjectName, result.Message)
		}
	}

	return fsReport
}

func ociValidationReason(reason ociobjectstore.LocalCopyValidationReason) integrityReason {
	switch reason {
	case ociobjectstore.LocalCopyValidationReasonMissing:
		return integrityReasonMissingWeight
	case ociobjectstore.LocalCopyValidationReasonSizeMismatch:
		return integrityReasonSizeMismatch
	case ociobjectstore.LocalCopyValidationReasonChecksumMismatch:
		return integrityReasonChecksumMismatch
	default:
		return integrityReasonMetadataError
	}
}

func validateOCIObjectSummaryLocalCopy(objectName string, objectSize *int64, localPath string) (ociobjectstore.LocalCopyValidationResult, error) {
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ociobjectstore.LocalCopyValidationResult{
				State:   ociobjectstore.LocalCopyValidationInvalid,
				Reason:  ociobjectstore.LocalCopyValidationReasonMissing,
				Message: fmt.Sprintf("local file for %s does not exist", objectName),
			}, nil
		}
		return ociobjectstore.LocalCopyValidationResult{}, err
	}
	if objectSize != nil && fileInfo.Size() != *objectSize {
		return ociobjectstore.LocalCopyValidationResult{
			State:   ociobjectstore.LocalCopyValidationInvalid,
			Reason:  ociobjectstore.LocalCopyValidationReasonSizeMismatch,
			Message: fmt.Sprintf("file size mismatch for %s: expected %d got %d", objectName, *objectSize, fileInfo.Size()),
		}, nil
	}
	return ociobjectstore.LocalCopyValidationResult{
		State:  ociobjectstore.LocalCopyValidationValid,
		Reason: ociobjectstore.LocalCopyValidationReasonOK,
	}, nil
}

func (s *Gopher) validateFilesystemArtifact(ctx context.Context, ref integrityModelRef, modelPath string, storageType storage.StorageType, checkType integrityCheckType) integrityReport {
	if err := validateArtifactPath(modelPath); err != nil {
		if os.IsNotExist(err) {
			return failureReport(integrityReasonMissingPath, err.Error())
		}
		return failureReport(integrityReasonPathNotDirectory, err.Error())
	}

	if storageType == storage.StorageTypeHuggingFace || storageType == storage.StorageTypeLocal {
		if report := s.validateModelConfigReadOnly(modelPath, ref); report.Result == integrityResultFailure {
			return report
		}
		if report := validateWeightArtifacts(modelPath); report.Result == integrityResultFailure {
			return report
		}
	}

	report := validateArtifactManifest(ctx, ref.spec(), s.modelRootDir, modelPath, checkType == integrityCheckDeep)
	shouldCreateManifest := s.integrityConfig.deepEnabled() && checkType == integrityCheckDeep
	if report.Result == integrityResultInconclusive && report.Reason == integrityReasonManifestError && shouldCreateManifest {
		created, err := createArtifactManifest(ctx, ref.spec(), s.modelRootDir, modelPath, storageType)
		if err != nil {
			return inconclusiveReport(integrityReasonManifestError, err.Error())
		}
		return successReport(integrityReasonBaselineCreated, created.BytesScanned)
	}
	if report.Result == integrityResultInconclusive && report.Reason == integrityReasonManifestError {
		return successReport(integrityReasonOK, 0)
	}
	return report
}

func (s *Gopher) validateModelConfigReadOnly(modelPath string, ref integrityModelRef) integrityReport {
	if s.modelConfigParser == nil {
		return inconclusiveReport(integrityReasonMetadataError, "model config parser is not initialized")
	}
	if s.modelConfigParser.shouldSkipConfigParsing(ref.BaseModel, ref.ClusterBaseModel) {
		return successReport(integrityReasonOK, 0)
	}
	if _, err := s.modelConfigParser.ParseModelConfig(modelPath, nil, nil); err != nil {
		if strings.Contains(err.Error(), "no model_index.json or config.json") {
			return failureReport(integrityReasonMissingConfig, err.Error())
		}
		return failureReport(integrityReasonParseError, err.Error())
	}
	return successReport(integrityReasonOK, 0)
}

func validateArtifactPath(modelPath string) error {
	info, err := os.Stat(modelPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("model artifact path is not a directory: %s", modelPath)
	}
	return nil
}

func validateWeightArtifacts(modelPath string) integrityReport {
	indexReport, indexFound := validateWeightIndexFiles(modelPath)
	if indexFound {
		return indexReport
	}

	weightFound := false
	var parseError error
	err := filepath.WalkDir(modelPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == artifactManifestDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !isWeightFile(path) {
			return nil
		}
		weightFound = true
		if err := validateWeightFile(path); err != nil {
			parseError = err
			return err
		}
		return nil
	})
	if err != nil {
		if parseError != nil {
			return failureReport(integrityReasonSafetensorsCorrupt, parseError.Error())
		}
		return failureReport(integrityReasonParseError, err.Error())
	}
	if !weightFound {
		return failureReport(integrityReasonMissingWeight, fmt.Sprintf("no model weight files found in %s", modelPath))
	}
	return successReport(integrityReasonOK, 0)
}

func validateWeightIndexFiles(modelPath string) (integrityReport, bool) {
	foundIndex := false
	var validationErr error
	err := filepath.WalkDir(modelPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == artifactManifestDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !isWeightIndexFile(entry.Name()) {
			return nil
		}
		shards, err := readWeightMapShards(path)
		if err != nil {
			validationErr = err
			return err
		}
		if len(shards) == 0 {
			return nil
		}
		foundIndex = true
		for _, shard := range shards {
			shardPath := filepath.Join(filepath.Dir(path), shard)
			if err := validateWeightFile(shardPath); err != nil {
				validationErr = err
				return err
			}
		}
		return nil
	})
	if err != nil {
		reason := integrityReasonParseError
		if validationErr != nil && strings.Contains(validationErr.Error(), "safetensors") {
			reason = integrityReasonSafetensorsCorrupt
		}
		if validationErr == nil {
			validationErr = err
		}
		return failureReport(reason, validationErr.Error()), foundIndex
	}
	if foundIndex {
		return successReport(integrityReasonOK, 0), true
	}
	return successReport(integrityReasonOK, 0), false
}

func readWeightMapShards(indexPath string) ([]string, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	var index struct {
		WeightMap map[string]string `json:"weight_map"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	if len(index.WeightMap) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	for _, shard := range index.WeightMap {
		if shard == "" {
			return nil, fmt.Errorf("empty shard filename found in %s", indexPath)
		}
		seen[shard] = struct{}{}
	}
	shards := make([]string, 0, len(seen))
	for shard := range seen {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	return shards, nil
}

func validateWeightFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("weight artifact is not a regular file: %s", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("weight artifact is empty: %s", path)
	}
	if strings.HasSuffix(path, ".safetensors") {
		if _, err := hfmodelconfig.ParseSafetensors(path); err != nil {
			return err
		}
	}
	return nil
}

func isWeightFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".safetensors", ".bin", ".gguf", ".pt", ".pth", ".onnx", ".ckpt", ".msgpack", ".h5", ".pb", ".engine", ".plan":
		return true
	default:
		return false
	}
}

func isWeightIndexFile(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".safetensors.index.json") || strings.HasSuffix(lowerName, ".bin.index.json")
}

func (s *Gopher) ensureArtifactManifest(ctx context.Context, task *GopherTask, spec *v1beta1.BaseModelSpec, storageType storage.StorageType, modelPath string) error {
	if !s.integrityConfig.deepEnabled() {
		return nil
	}
	if storageType == storage.StorageTypeVendor || storageType == storage.StorageTypePVC {
		return nil
	}
	if modelPath == "" {
		resolvedPath, err := resolveArtifactPath(spec, s.modelRootDir)
		if err != nil {
			return err
		}
		modelPath = resolvedPath
	}
	if err := validateArtifactPath(modelPath); err != nil {
		return err
	}
	if storageType == storage.StorageTypeHuggingFace || storageType == storage.StorageTypeLocal {
		ref := integrityModelRef{}
		if task != nil {
			ref.BaseModel = task.BaseModel
			ref.ClusterBaseModel = task.ClusterBaseModel
		}
		if report := s.validateModelConfigReadOnly(modelPath, ref); report.Result == integrityResultFailure {
			return fmt.Errorf("artifact manifest validation failed: %s", report.Message)
		}
		if report := validateWeightArtifacts(modelPath); report.Result == integrityResultFailure {
			return fmt.Errorf("artifact manifest validation failed: %s", report.Message)
		}
	}
	if _, err := createArtifactManifest(ctx, spec, s.modelRootDir, modelPath, storageType); err != nil {
		return err
	}
	return nil
}

func (s *Gopher) backfillReadyStorageIdentity(ctx context.Context, ref integrityModelRef) error {
	return s.configMapReconciler.ReconcileModelStatus(ctx, &ConfigMapStatusOp{
		ModelStatus:      ModelStatusReady,
		BaseModel:        ref.BaseModel,
		ClusterBaseModel: ref.ClusterBaseModel,
	})
}

func (s *Gopher) markIntegrityFailureIfCurrent(ctx context.Context, key string, ref integrityModelRef) error {
	s.configMapMutex.Lock()
	defer s.configMapMutex.Unlock()

	cm, err := s.configMapReconciler.getConfigMap(ctx)
	if err != nil {
		return err
	}
	raw, ok := cm.Data[key]
	if !ok {
		return fmt.Errorf("model entry %s disappeared before marking Failed", key)
	}
	var current ModelEntry
	if err := json.Unmarshal([]byte(raw), &current); err != nil {
		return err
	}
	if current.Status != ModelStatusReady {
		return fmt.Errorf("model entry %s is no longer Ready; current status is %s", key, current.Status)
	}
	if !current.MatchesStorageIdentity(ref.spec()) {
		return fmt.Errorf("model entry %s storage identity changed before marking Failed", key)
	}
	op := &NodeLabelOp{
		ModelStateOnNode: Failed,
		BaseModel:        ref.BaseModel,
		ClusterBaseModel: ref.ClusterBaseModel,
	}
	if err := s.nodeLabelReconciler.ReconcileNodeLabels(op); err != nil {
		return err
	}
	return s.configMapReconciler.ReconcileModelStatus(ctx, &ConfigMapStatusOp{
		ModelStatus:      ModelStatusFailed,
		BaseModel:        ref.BaseModel,
		ClusterBaseModel: ref.ClusterBaseModel,
	})
}

func (s *Gopher) recordIntegrityResult(ref integrityModelRef, storageType string, checkType integrityCheckType, report integrityReport, duration time.Duration) {
	modelType, namespace, name := ref.modelTypeNamespaceName()
	s.metrics.RecordIntegrityCheck(modelType, namespace, name, storageType, string(checkType), string(report.Result), string(report.Reason), duration, report.BytesScanned)
}

func resolveArtifactPath(spec *v1beta1.BaseModelSpec, modelRootDir string) (string, error) {
	if spec == nil || spec.Storage == nil || spec.Storage.StorageUri == nil {
		return "", fmt.Errorf("model storage URI is missing")
	}
	storageURI := *spec.Storage.StorageUri
	storageType, err := storage.GetStorageType(storageURI)
	if err != nil {
		return "", err
	}
	if spec.Storage.Path != nil && *spec.Storage.Path != "" {
		return *spec.Storage.Path, nil
	}
	if storageType == storage.StorageTypeLocal {
		localComponents, err := storage.ParseLocalStorageURI(storageURI)
		if err != nil {
			return "", err
		}
		return localComponents.Path, nil
	}
	if strings.HasSuffix(modelRootDir, "/") {
		return modelRootDir + storageURI, nil
	}
	return modelRootDir + "/" + storageURI, nil
}

func tensorRTLLMShapeFilterForSpec(spec *v1beta1.BaseModelSpec, nodeShapeAlias string) *TensorRTLLMShapeFilter {
	if spec == nil {
		return nil
	}
	modelType := string(constants.ServingBaseModel)
	if spec.AdditionalMetadata != nil {
		if modelTypeFromMetadata, ok := spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}
	}
	return &TensorRTLLMShapeFilter{
		IsTensorrtLLMModel: spec.ModelFormat.Name == constants.TensorRTLLM,
		ShapeAlias:         nodeShapeAlias,
		ModelType:          modelType,
	}
}

func filterObjectsForTensorRTLLM(objects []objectstorage.ObjectSummary, filter *TensorRTLLMShapeFilter) ([]objectstorage.ObjectSummary, bool, error) {
	if filter == nil || !filter.IsTensorrtLLMModel || filter.ModelType != string(constants.ServingBaseModel) {
		return objects, false, nil
	}

	filtered := make([]objectstorage.ObjectSummary, 0)
	for _, object := range objects {
		if object.Name != nil && strings.Contains(*object.Name, fmt.Sprintf("/%s/", filter.ShapeAlias)) {
			filtered = append(filtered, object)
		}
	}
	if len(filtered) == 0 {
		return nil, true, fmt.Errorf("no suitable objects found for shape %s", filter.ShapeAlias)
	}
	return filtered, true, nil
}

func deterministicIntegrityJitter(nodeName string, maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 || nodeName == "" {
		return 0
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(nodeName))
	return time.Duration(hash.Sum64() % uint64(maxJitter))
}
