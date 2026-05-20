package modelagent

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	omev1beta1lister "github.com/sgl-project/ome/pkg/client/listers/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	hfmodelconfig "github.com/sgl-project/ome/pkg/hfutil/modelconfig"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

func TestValidateFilesystemArtifactDetectsSameSizeCorruption(t *testing.T) {
	modelPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(modelPath, "config.json"), []byte(`{"model_type":"llama"}`), 0644))
	weightPath := filepath.Join(modelPath, "model.safetensors")
	writeTinySafetensors(t, weightPath)

	gopher := newIntegrityTestGopher(t, nil, nil)
	report := gopher.validateFilesystemArtifact(context.Background(), integrityModelRef{
		BaseModel: testIntegrityBaseModel("default", "llama", "hf://meta-llama/llama", modelPath),
	}, modelPath, storage.StorageTypeHuggingFace, integrityCheckDeep)
	require.Equal(t, integrityResultSuccess, report.Result)
	require.Equal(t, integrityReasonBaselineCreated, report.Reason)

	data, err := os.ReadFile(weightPath)
	require.NoError(t, err)
	data[len(data)-1] ^= 0xff
	require.NoError(t, os.WriteFile(weightPath, data, 0644))

	report = gopher.validateFilesystemArtifact(context.Background(), integrityModelRef{
		BaseModel: testIntegrityBaseModel("default", "llama", "hf://meta-llama/llama", modelPath),
	}, modelPath, storage.StorageTypeHuggingFace, integrityCheckDeep)
	assert.Equal(t, integrityResultFailure, report.Result)
	assert.Equal(t, integrityReasonChecksumMismatch, report.Reason)
}

func TestReconcileReadyModelIntegrityMarksMissingLocalPathFailed(t *testing.T) {
	nodeName := "test-node"
	namespace := constants.OMENamespace
	missingPath := filepath.Join(t.TempDir(), "missing")
	baseModel := testIntegrityBaseModel("default", "llama", "local:///models/llama", missingPath)
	key := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
	entry := ModelEntry{
		Name:        baseModel.Name,
		Status:      ModelStatusReady,
		StorageURI:  *baseModel.Spec.Storage.StorageUri,
		StoragePath: missingPath,
	}
	cm := testIntegrityConfigMap(t, nodeName, namespace, key, entry)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Labels: map[string]string{}}}

	gopher := newIntegrityTestGopher(t, []*v1beta1.BaseModel{baseModel}, nil, node, cm)
	gopher.reconcileReadyModelIntegrity(context.Background(), integrityCheckBasic)

	updatedCM, err := gopher.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), nodeName, metav1.GetOptions{})
	require.NoError(t, err)
	var updatedEntry ModelEntry
	require.NoError(t, json.Unmarshal([]byte(updatedCM.Data[key]), &updatedEntry))
	assert.Equal(t, ModelStatusFailed, updatedEntry.Status)

	updatedNode, err := gopher.kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(Failed), updatedNode.Labels[constants.GetBaseModelLabel(baseModel.Namespace, baseModel.Name)])
}

func TestReconcileReadyModelIntegritySkipsStaleStorageIdentity(t *testing.T) {
	nodeName := "test-node"
	namespace := constants.OMENamespace
	missingPath := filepath.Join(t.TempDir(), "missing")
	currentStorageURI := "local:///models/current"
	baseModel := testIntegrityBaseModel("default", "llama", currentStorageURI, missingPath)
	key := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
	entry := ModelEntry{
		Name:        baseModel.Name,
		Status:      ModelStatusReady,
		StorageURI:  "local:///models/old",
		StoragePath: "/old/path",
	}
	cm := testIntegrityConfigMap(t, nodeName, namespace, key, entry)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Labels: map[string]string{}}}

	gopher := newIntegrityTestGopher(t, []*v1beta1.BaseModel{baseModel}, nil, node, cm)
	gopher.reconcileReadyModelIntegrity(context.Background(), integrityCheckBasic)

	updatedCM, err := gopher.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), nodeName, metav1.GetOptions{})
	require.NoError(t, err)
	var updatedEntry ModelEntry
	require.NoError(t, json.Unmarshal([]byte(updatedCM.Data[key]), &updatedEntry))
	assert.Equal(t, ModelStatusReady, updatedEntry.Status)
}

func TestReconcileReadyModelIntegrityBackfillsLegacyStorageIdentity(t *testing.T) {
	nodeName := "test-node"
	namespace := constants.OMENamespace
	modelPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(modelPath, "config.json"), []byte(`{"model_type":"llama"}`), 0644))
	writeTinySafetensors(t, filepath.Join(modelPath, "model.safetensors"))

	storageURI := "local:///models/llama"
	baseModel := testIntegrityBaseModel("default", "llama", storageURI, modelPath)
	key := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
	entry := ModelEntry{
		Name:   baseModel.Name,
		Status: ModelStatusReady,
	}
	cm := testIntegrityConfigMap(t, nodeName, namespace, key, entry)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Labels: map[string]string{}}}

	gopher := newIntegrityTestGopher(t, []*v1beta1.BaseModel{baseModel}, nil, node, cm)
	gopher.reconcileReadyModelIntegrity(context.Background(), integrityCheckBasic)

	updatedCM, err := gopher.kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), nodeName, metav1.GetOptions{})
	require.NoError(t, err)
	var updatedEntry ModelEntry
	require.NoError(t, json.Unmarshal([]byte(updatedCM.Data[key]), &updatedEntry))
	assert.Equal(t, ModelStatusReady, updatedEntry.Status)
	assert.Equal(t, storageURI, updatedEntry.StorageURI)
	assert.Equal(t, modelPath, updatedEntry.StoragePath)
}

func TestBuildIntegrityModelRefIndexUsesGeneratedConfigMapKeys(t *testing.T) {
	longName := "this-is-a-very-long-model-name-that-requires-hashing-to-fit-config-map-key-limits"
	baseModel := testIntegrityBaseModel("default", longName, "local:///models/long", "/models/long")
	gopher := newIntegrityTestGopher(t, []*v1beta1.BaseModel{baseModel}, nil)

	refs, err := gopher.buildIntegrityModelRefIndex()
	require.NoError(t, err)
	key := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
	ref, ok := refs[key]
	require.True(t, ok)
	assert.Equal(t, baseModel.Name, ref.BaseModel.Name)
}

func TestIntegrityCheckTypeForCycle(t *testing.T) {
	gopher := &Gopher{
		integrityConfig: IntegrityConfig{
			CheckInterval:     time.Minute,
			DeepCheckInterval: time.Hour,
		},
	}
	var lastDeep time.Time
	assert.Equal(t, integrityCheckDeep, gopher.integrityCheckTypeForCycle(&lastDeep))
	assert.False(t, lastDeep.IsZero())
	assert.Equal(t, integrityCheckBasic, gopher.integrityCheckTypeForCycle(&lastDeep))
	lastDeep = time.Now().Add(-2 * time.Hour)
	assert.Equal(t, integrityCheckDeep, gopher.integrityCheckTypeForCycle(&lastDeep))

	gopher.integrityConfig.DeepCheckInterval = 0
	assert.Equal(t, integrityCheckBasic, gopher.integrityCheckTypeForCycle(&lastDeep))
}

func TestValidateOCIObjectSummaryLocalCopy(t *testing.T) {
	localPath := filepath.Join(t.TempDir(), "model.safetensors")
	require.NoError(t, os.WriteFile(localPath, []byte("abc"), 0644))

	size := int64(3)
	result, err := validateOCIObjectSummaryLocalCopy("model.safetensors", &size, localPath)
	require.NoError(t, err)
	assert.Equal(t, ociobjectstore.LocalCopyValidationValid, result.State)

	size = 4
	result, err = validateOCIObjectSummaryLocalCopy("model.safetensors", &size, localPath)
	require.NoError(t, err)
	assert.Equal(t, ociobjectstore.LocalCopyValidationInvalid, result.State)
	assert.Equal(t, ociobjectstore.LocalCopyValidationReasonSizeMismatch, result.Reason)

	result, err = validateOCIObjectSummaryLocalCopy("missing.safetensors", nil, filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Equal(t, ociobjectstore.LocalCopyValidationInvalid, result.State)
	assert.Equal(t, ociobjectstore.LocalCopyValidationReasonMissing, result.Reason)
}

func TestEnsureArtifactManifestReturnsCreateError(t *testing.T) {
	modelPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(modelPath, "config.json"), []byte(`{"model_type":"llama"}`), 0644))
	writeTinySafetensors(t, filepath.Join(modelPath, "model.safetensors"))

	modelRootFile := filepath.Join(t.TempDir(), "model-root-file")
	require.NoError(t, os.WriteFile(modelRootFile, []byte("not a directory"), 0644))

	baseModel := testIntegrityBaseModel("default", "llama", "local:///models/llama", modelPath)
	gopher := newIntegrityTestGopher(t, []*v1beta1.BaseModel{baseModel}, nil)
	gopher.modelRootDir = modelRootFile

	err := gopher.ensureArtifactManifest(context.Background(), (&integrityModelRef{BaseModel: baseModel}).task(), &baseModel.Spec, storage.StorageTypeLocal, modelPath)
	require.Error(t, err)
}

func newIntegrityTestGopher(t *testing.T, baseModels []*v1beta1.BaseModel, clusterBaseModels []*v1beta1.ClusterBaseModel, objects ...runtime.Object) *Gopher {
	t.Helper()
	logger := zaptest.NewLogger(t).Sugar()
	kubeClient := fake.NewSimpleClientset(objects...)

	baseIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, baseModel := range baseModels {
		require.NoError(t, baseIndexer.Add(baseModel))
	}
	clusterIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, clusterBaseModel := range clusterBaseModels {
		require.NoError(t, clusterIndexer.Add(clusterBaseModel))
	}

	parser := &ModelConfigParser{
		logger: logger,
		loadModelConfig: func(_ string) (hfmodelconfig.HuggingFaceModel, error) {
			return createDefaultMockModel(), nil
		},
	}
	return &Gopher{
		modelConfigParser:      parser,
		configMapReconciler:    NewConfigMapReconciler("test-node", constants.OMENamespace, kubeClient, logger),
		kubeClient:             kubeClient,
		nodeLabelReconciler:    NewNodeLabelReconciler("test-node", kubeClient, 1, logger),
		metrics:                NewMetrics(prometheus.NewRegistry()),
		logger:                 logger,
		modelRootDir:           t.TempDir(),
		baseModelLister:        omev1beta1lister.NewBaseModelLister(baseIndexer),
		clusterBaseModelLister: omev1beta1lister.NewClusterBaseModelLister(clusterIndexer),
		nodeShapeAlias:         "H100",
		integrityConfig:        DefaultIntegrityConfig(),
	}
}

func testIntegrityBaseModel(namespace, name, storageURI, storagePath string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			Storage: &v1beta1.StorageSpec{
				StorageUri: &storageURI,
				Path:       &storagePath,
			},
			ModelFormat: v1beta1.ModelFormat{Name: "safetensors"},
		},
	}
}

func testIntegrityConfigMap(t *testing.T, name, namespace, key string, entry ModelEntry) *corev1.ConfigMap {
	t.Helper()
	entryJSON, err := json.Marshal(entry)
	require.NoError(t, err)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				constants.ModelStatusConfigMapLabel: "true",
			},
		},
		Data: map[string]string{
			key: string(entryJSON),
		},
	}
}

func writeTinySafetensors(t *testing.T, path string) {
	t.Helper()
	header := []byte(`{"weight":{"dtype":"F32","shape":[1],"data_offsets":[0,4]}}`)
	content := make([]byte, 8+len(header)+4)
	binary.LittleEndian.PutUint64(content[:8], uint64(len(header)))
	copy(content[8:], header)
	copy(content[8+len(header):], []byte{1, 2, 3, 4})
	require.NoError(t, os.WriteFile(path, content, 0644))
}
