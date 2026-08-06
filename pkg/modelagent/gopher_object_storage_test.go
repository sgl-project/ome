package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	omev1beta1lister "sigs.k8s.io/ome/pkg/client/listers/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/modelparser"
)

func TestFilterInternalArtifactObjectSummaries(t *testing.T) {
	modelFile := "prefix/config.json"
	completeMarker := "prefix/" + constants.ArtifactCompleteMarkerFileName
	activeLock := "prefix/" + constants.ArtifactUploadLockFileName

	filtered := filterInternalArtifactObjectSummaries([]objectstorage.ObjectSummary{
		{Name: &modelFile},
		{Name: &completeMarker},
		{Name: &activeLock},
		{},
	})

	require.Len(t, filtered, 2)
	assert.Equal(t, modelFile, *filtered[0].Name)
	assert.Nil(t, filtered[1].Name)
}

func TestProcessObjectStorageDeleteUsesPersistedSharedArtifactMetadata(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Annotations = nil // Deletion must not depend on the current rollout gate.
	task := &GopherTask{TaskType: Delete, BaseModel: model}
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	parent := artifactParentForChild(identity, childPath)
	require.NoError(t, writeTestArtifact(parent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(parent.Path))
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(parent.Path, childPath))

	parentEntry, err := json.Marshal(modelEntryFromArtifactParent(ArtifactParent{
		Key:      parent.Key,
		Path:     parent.Path,
		Identity: parent.Identity,
		Status:   ModelStatusReady,
		Children: []string{childPath},
	}))
	require.NoError(t, err)
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	childEntry, err := json.Marshal(ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{Artifact: Artifact{
			Sha:           identity.HFCommitSHA,
			Origin:        identity.toOrigin(),
			ParentPath:    map[string]string{parent.Key: parent.Path},
			ChildrenPaths: []string{},
		}},
	})
	require.NoError(t, err)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{
		parent.Key: string(parentEntry),
		modelKey:   string(childEntry),
	}))
	gopher.modelRootDir = root
	gopher.baseModelLister = omev1beta1lister.NewBaseModelLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))
	gopher.clusterBaseModelLister = omev1beta1lister.NewClusterBaseModelLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))

	outcome, err := gopher.processObjectStorageDelete(context.Background(), task, model.Spec)

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	_, err = os.Lstat(childPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(parent.Path)
	assert.True(t, os.IsNotExist(err))
	configMap, err := gopher.configMapReconciler.kubeClient.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, configMap.Data, parent.Key)
}

func TestProcessObjectStorageDeleteReleasesSupersededSharedArtifactReferences(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Annotations = nil
	task := &GopherTask{TaskType: Delete, BaseModel: model}
	currentIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	oldIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", strings.Repeat("a", 40))
	require.NoError(t, err)
	currentParent := artifactParentForChild(currentIdentity, childPath)
	oldParent := artifactParentForChild(oldIdentity, childPath)
	for _, parent := range []ArtifactParent{currentParent, oldParent} {
		require.NoError(t, writeTestArtifact(parent.Path))
		require.NoError(t, writeHfArtifactReadyMarker(parent.Path))
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(currentParent.Path, childPath))

	entries := make(map[string]string)
	for _, parent := range []ArtifactParent{currentParent, oldParent} {
		encoded, marshalErr := json.Marshal(modelEntryFromArtifactParent(ArtifactParent{
			Key:      parent.Key,
			Path:     parent.Path,
			Identity: parent.Identity,
			Status:   ModelStatusReady,
			Children: []string{childPath},
		}))
		require.NoError(t, marshalErr)
		entries[parent.Key] = string(encoded)
	}
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	childEntry, err := json.Marshal(ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{Artifact: artifactForParent(currentParent)},
	})
	require.NoError(t, err)
	entries[modelKey] = string(childEntry)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", entries))
	gopher.modelRootDir = root
	gopher.baseModelLister = omev1beta1lister.NewBaseModelLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))
	gopher.clusterBaseModelLister = omev1beta1lister.NewClusterBaseModelLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))

	outcome, err := gopher.processObjectStorageDelete(context.Background(), task, model.Spec)

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	configMap, err := gopher.configMapReconciler.kubeClient.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	for _, parent := range []ArtifactParent{currentParent, oldParent} {
		assert.NotContains(t, configMap.Data, parent.Key)
		_, statErr := os.Stat(parent.Path)
		assert.True(t, os.IsNotExist(statErr))
	}
}

func TestHfOriginOCIPlanIsDisabledWithoutReusePolicy(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Spec.Storage.DownloadPolicy = nil
	task := &GopherTask{TaskType: Download, BaseModel: model}

	_, shared := planHfOriginOCIArtifact(task, model.Spec, filepath.Dir(childPath), childPath)

	assert.False(t, shared)
}

func TestHfOriginOCIPlanRejectsNonOCIStorage(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Spec.Storage.StorageUri = stringPtr("hf://Qwen/Qwen3-8B")
	task := &GopherTask{TaskType: Download, BaseModel: model}

	_, shared := planHfOriginOCIArtifact(task, model.Spec, filepath.Dir(childPath), childPath)

	assert.False(t, shared)
}

func TestLegacyObjectStorageDownloadReleasesPersistedSharedArtifact(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.UID = "uid-model-1"
	model.Annotations = nil
	model.Spec.Storage.DownloadPolicy = nil
	task := &GopherTask{TaskType: DownloadOverride, BaseModel: model}
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	parent := artifactParentForChild(identity, childPath)
	require.NoError(t, writeTestArtifact(parent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(parent.Path))
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(parent.Path, childPath))

	parentEntry, err := json.Marshal(modelEntryFromArtifactParent(ArtifactParent{
		Key:      parent.Key,
		Path:     parent.Path,
		Identity: parent.Identity,
		Status:   ModelStatusReady,
		Children: []string{childPath},
	}))
	require.NoError(t, err)
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	childEntry, err := json.Marshal(ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{Artifact: artifactForParent(parent)},
	})
	require.NoError(t, err)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{
		parent.Key: string(parentEntry),
		modelKey:   string(childEntry),
	}))
	gopher.modelRootDir = root

	outcome, err := gopher.processLegacyObjectStorageModel(
		context.Background(), task, model.Spec, childPath, true,
		func(path string) error {
			info, statErr := os.Lstat(path)
			if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("legacy download still points at shared artifact parent")
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				return statErr
			}
			return writeTestArtifact(path)
		},
	)

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	childInfo, err := os.Lstat(childPath)
	require.NoError(t, err)
	assert.Zero(t, childInfo.Mode()&os.ModeSymlink)
	_, err = os.Stat(parent.Path)
	assert.True(t, os.IsNotExist(err))
}

func TestLegacyObjectStorageDownloadWithoutSymlinkDoesNotRequireSharedStateLookup(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Annotations = nil
	model.Spec.Storage.DownloadPolicy = nil
	task := &GopherTask{TaskType: DownloadOverride, BaseModel: model}
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{}))
	gopher.configMapReconciler.kubeClient.(*fake.Clientset).PrependReactor(
		"get", "configmaps", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("ConfigMap unavailable")
		},
	)

	outcome, err := gopher.processLegacyObjectStorageModel(
		context.Background(), task, model.Spec, childPath, true, writeTestArtifact,
	)

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	info, err := os.Stat(childPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLegacyObjectStorageDemotionPreservesSharedArtifactUntilDownload(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.UID = "uid-model-1"
	model.Annotations = nil
	model.Spec.Storage.DownloadPolicy = nil
	task := &GopherTask{TaskType: Download, BaseModel: model}
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	parent := artifactParentForChild(identity, childPath)
	require.NoError(t, writeTestArtifact(parent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(parent.Path))
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(parent.Path, childPath))

	parentEntry, err := json.Marshal(modelEntryFromArtifactParent(ArtifactParent{
		Key:      parent.Key,
		Path:     parent.Path,
		Identity: parent.Identity,
		Status:   ModelStatusReady,
		Children: []string{childPath},
	}))
	require.NoError(t, err)
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	childEntry, err := json.Marshal(ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{Artifact: artifactForParent(parent)},
	})
	require.NoError(t, err)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{
		parent.Key: string(parentEntry),
		modelKey:   string(childEntry),
	}))
	gopher.modelRootDir = root
	downloaded := false

	outcome, err := gopher.processLegacyObjectStorageModel(
		context.Background(), task, model.Spec, childPath, false,
		func(string) error {
			downloaded = true
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, modelTaskDeferred, outcome)
	assert.False(t, downloaded)
	assertSymlinkTarget(t, childPath, parent.Path)
	_, err = os.Stat(parent.Path)
	assert.NoError(t, err)
}

func TestProcessSharedArtifactPlanResumesCleanupWithoutRepeatingRepair(t *testing.T) {
	root := filepath.Join(t.TempDir(), "models")
	childPath := filepath.Join(root, "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.UID = "uid-model-1"
	task := &GopherTask{TaskType: DownloadOverride, BaseModel: model, ArtifactCleanupPending: true}
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{}))
	gopher.modelRootDir = root
	manager := gopher.getHfArtifactManager()

	oldIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", strings.Repeat("a", 40))
	require.NoError(t, err)
	oldPlan := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	oldPlan.Parent = artifactParentForChild(oldIdentity, childPath)
	_, err = manager.Ensure(context.Background(), oldPlan, true, writeTestArtifact)
	require.NoError(t, err)

	currentPlan := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	_, err = manager.Ensure(context.Background(), currentPlan, true, writeTestArtifact)
	require.NoError(t, err)
	repairPlan := currentPlan
	repairPlan.Operation = ArtifactOperationRepair
	downloads := 0

	result, outcome, err := gopher.processSharedArtifactPlan(context.Background(), task, repairPlan, true, func(string) error {
		downloads++
		return errors.New("repair must not repeat while cleanup is pending")
	})

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	assert.Zero(t, downloads)
	assert.False(t, task.ArtifactCleanupPending)
	assertSymlinkTarget(t, childPath, currentPlan.Parent.Path)
	_, found, err := manager.repository.Get(context.Background(), oldIdentity)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestConsumeArtifactLifecycleResultFailsWhenWaitCannotBeRequeued(t *testing.T) {
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{}))
	task := &GopherTask{TaskType: Download, BaseModel: newHfOriginOCIModel("model-1", "/models/model-1")}

	outcome, err := gopher.consumeArtifactLifecycleResult(task, ArtifactLifecycleResult{
		Outcome: ArtifactLifecycleDeferred,
		WaitKey: "artifact-parent",
	})

	assert.ErrorContains(t, err, "timed out waiting")
	assert.Equal(t, modelTaskCompleted, outcome)
}

func TestSafeParseAndUpdateModelConfigPersistsArtifactWhenParsingIsSkipped(t *testing.T) {
	model := newHfOriginOCIModel("model-1", filepath.Join(t.TempDir(), "model-1"))
	model.Annotations[ConfigParsingAnnotation] = "true"
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	existing, err := json.Marshal(ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{ModelType: "existing-model-type"},
	})
	require.NoError(t, err)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{modelKey: string(existing)}))
	gopher.modelConfigParser = modelparser.NewModelConfigParser(nil, zap.NewNop().Sugar())
	artifact := &Artifact{
		Sha:        testHFCommitSHA,
		Origin:     (&ArtifactIdentity{OriginType: ArtifactOriginTypeHf, HFModelID: "Qwen/Qwen3-8B", HFCommitSHA: testHFCommitSHA}).toOrigin(),
		ParentPath: map[string]string{"parent": "/models/_artifacts/Qwen/Qwen3-8B/" + testHFCommitSHA},
	}

	err = gopher.safeParseAndUpdateModelConfig(t.TempDir(), model, nil, artifact)

	require.NoError(t, err)
	entry := readModelEntryFromTestConfigMap(t, gopher, modelKey)
	require.NotNil(t, entry.Config)
	assert.Equal(t, "existing-model-type", entry.Config.ModelType)
	assert.Equal(t, testHFCommitSHA, entry.Config.Artifact.Sha)
}

func TestSafeParseAndUpdateModelConfigPersistsArtifactWhenParsingFails(t *testing.T) {
	model := newHfOriginOCIModel("model-1", filepath.Join(t.TempDir(), "model-1"))
	modelKey := constants.GetModelConfigMapKey(model.Namespace, model.Name, false)
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{}))
	gopher.modelConfigParser = modelparser.NewModelConfigParser(nil, zap.NewNop().Sugar())
	artifact := &Artifact{
		Sha:        testHFCommitSHA,
		Origin:     (&ArtifactIdentity{OriginType: ArtifactOriginTypeHf, HFModelID: "Qwen/Qwen3-8B", HFCommitSHA: testHFCommitSHA}).toOrigin(),
		ParentPath: map[string]string{"parent": "/models/_artifacts/Qwen/Qwen3-8B/" + testHFCommitSHA},
	}

	err := gopher.safeParseAndUpdateModelConfig(t.TempDir(), model, nil, artifact)

	assert.Error(t, err)
	entry := readModelEntryFromTestConfigMap(t, gopher, modelKey)
	require.NotNil(t, entry.Config)
	assert.Equal(t, testHFCommitSHA, entry.Config.Artifact.Sha)
}

func readModelEntryFromTestConfigMap(t *testing.T, gopher *Gopher, key string) ModelEntry {
	t.Helper()
	configMap, err := gopher.configMapReconciler.kubeClient.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	var entry ModelEntry
	require.NoError(t, json.Unmarshal([]byte(configMap.Data[key]), &entry))
	return entry
}
