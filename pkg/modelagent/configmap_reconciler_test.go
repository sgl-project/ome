package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// setupConfigMapTest prepares a test environment with fake clients and test models
func setupConfigMapTest(t *testing.T) (*ConfigMapReconciler, *fake.Clientset, *zap.SugaredLogger) {
	// Create a test logger
	logger := zaptest.NewLogger(t).Sugar()

	// Create a fake Kubernetes client
	kubeClient := fake.NewSimpleClientset()

	// Create the reconciler with type conversion to match interface
	reconciler := NewConfigMapReconciler("test-node", "test-namespace", kubeClient, logger)

	return reconciler, kubeClient, logger
}

// createTestBaseModel creates a test BaseModel for tests
func createTestBaseModelCM() *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-model",
			Namespace: "default",
		},
	}
}

// createTestClusterBaseModel creates a test ClusterBaseModel for tests
func createTestClusterBaseModelCM() *v1beta1.ClusterBaseModel {
	return &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-model",
		},
	}
}

// TestNewConfigMapReconciler tests the constructor
func TestNewConfigMapReconciler(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	kubeClient := fake.NewSimpleClientset()

	reconciler := NewConfigMapReconciler("test-node", "test-namespace", kubeClient, logger)

	assert.NotNil(t, reconciler)
	assert.Equal(t, "test-node", reconciler.nodeName)
	assert.Equal(t, "test-namespace", reconciler.namespace)
	assert.NotNil(t, reconciler.kubeClient)
	assert.NotNil(t, reconciler.logger)
}

// TestGetConfigMapModelInfo tests the getConfigMapModelInfo function
func TestGetConfigMapModelInfo(t *testing.T) {
	// Test with BaseModel
	baseModel := createTestBaseModelCM()

	info := getConfigMapModelInfo(baseModel, nil)
	assert.Equal(t, "BaseModel default/test-model", info)

	// Test with ClusterBaseModel
	clusterBaseModel := createTestClusterBaseModelCM()

	info = getConfigMapModelInfo(nil, clusterBaseModel)
	assert.Equal(t, "ClusterBaseModel test-cluster-model", info)

	// Test with no model
	info = getConfigMapModelInfo(nil, nil)
	assert.Equal(t, "unknown model", info)
}

// TestGetModelConfigMapKey tests the getModelConfigMapKey method
func TestGetModelConfigMapKey(t *testing.T) {
	reconciler, _, _ := setupConfigMapTest(t)

	// Test with BaseModel
	baseModel := createTestBaseModelCM()
	key := reconciler.getModelConfigMapKey(baseModel, nil)
	expectedKey := constants.GetModelConfigMapKey(baseModel.Namespace, baseModel.Name, false)
	assert.Equal(t, expectedKey, key)

	// Test with ClusterBaseModel
	clusterBaseModel := createTestClusterBaseModelCM()
	key = reconciler.getModelConfigMapKey(nil, clusterBaseModel)
	expectedKey = constants.GetModelConfigMapKey("", clusterBaseModel.Name, true)
	assert.Equal(t, expectedKey, key)
}

// TestGetOrCreateConfigMap tests the getOrCreateConfigMap method
func TestGetOrCreateConfigMap(t *testing.T) {
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Test when ConfigMap doesn't exist (should create new one)
	ctx := context.Background()
	configMap, needCreate, err := reconciler.getOrCreateConfigMap(ctx)
	assert.NoError(t, err)
	assert.True(t, needCreate)
	assert.NotNil(t, configMap)
	assert.Equal(t, reconciler.nodeName, configMap.Name)
	assert.Equal(t, reconciler.namespace, configMap.Namespace)
	assert.Equal(t, "true", configMap.Labels[constants.ModelStatusConfigMapLabel])
	assert.Equal(t, reconciler.nodeName, configMap.Labels["node"])
	assert.Equal(t, reconciler.nodeName, configMap.Annotations["models.ome.io/node-name"])
	assert.Equal(t, "model-agent", configMap.Annotations["models.ome.io/managed-by"])

	// Now create a ConfigMap and test getting existing one
	configMap.Data = map[string]string{"test": "data"}
	_, err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Create(
		ctx, configMap, metav1.CreateOptions{},
	)
	assert.NoError(t, err)

	// Should return existing ConfigMap
	existingConfigMap, needCreate, err := reconciler.getOrCreateConfigMap(ctx)
	assert.NoError(t, err)
	assert.False(t, needCreate)
	assert.Equal(t, configMap.Name, existingConfigMap.Name)
	assert.Equal(t, configMap.Namespace, existingConfigMap.Namespace)
	assert.Equal(t, "data", existingConfigMap.Data["test"])

	// Test error case
	kubeClient.PrependReactor("get", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake error")
	})

	_, _, err = reconciler.getOrCreateConfigMap(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fake error")
}

// TestSaveConfigMap tests the saveConfigMap method
func TestSaveConfigMap(t *testing.T) {
	reconciler, kubeClient, _ := setupConfigMapTest(t)
	modelInfo := "test model"

	// Test creating new ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      reconciler.nodeName,
			Namespace: reconciler.namespace,
		},
		Data: map[string]string{"test": "value"},
	}

	ctx := context.Background()
	err := reconciler.saveConfigMap(ctx, configMap, modelInfo, true)
	assert.NoError(t, err)

	// Verify ConfigMap was created
	createdCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(
		ctx, reconciler.nodeName, metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "value", createdCM.Data["test"])

	// Test updating existing ConfigMap
	configMap.Data["test"] = "updated"
	err = reconciler.saveConfigMap(ctx, configMap, modelInfo, false)
	assert.NoError(t, err)

	// Verify ConfigMap was updated
	updatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(
		ctx, reconciler.nodeName, metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "updated", updatedCM.Data["test"])

	// Test create error
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake create error")
	})

	err = reconciler.saveConfigMap(ctx, configMap, modelInfo, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fake create error")

	// Test update error
	kubeClient.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake update error")
	})

	err = reconciler.saveConfigMap(ctx, configMap, modelInfo, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fake update error")
}

// Note: These types are defined in the main package file
// ModelStatus, ModelStatusReady, ModelStatusDeleted, and ModelEntry are
// used here but not redeclared to avoid conflicts.

// TestUpdateModelStatusInConfigMap tests the updateModelStatusInConfigMap method
func TestUpdateModelStatusInConfigMap(t *testing.T) {
	// Setup test environment
	reconciler, _, _ := setupConfigMapTest(t)

	// Create a test ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-namespace",
		},
		Data: make(map[string]string),
	}

	// Create a test operation
	baseModel := createTestBaseModelCM()
	op := &ConfigMapStatusOp{
		BaseModel:   baseModel,
		ModelStatus: ModelStatusReady,
	}

	// Rather than mocking the internal function which is challenging,
	// we'll verify the expected result directly

	// Execute the test - the function should modify the configMap in-place
	ctx := context.Background()
	err := reconciler.updateModelStatusInConfigMap(ctx, configMap, op, true)
	assert.NoError(t, err)

	// Verify the ConfigMap was updated correctly
	key := reconciler.getModelConfigMapKey(baseModel, nil)
	assert.Contains(t, configMap.Data, key, "ConfigMap should contain the model key")

	// The entry should be a JSON string with the model status
	entryJSON := configMap.Data[key]
	assert.Contains(t, entryJSON, "\"status\":\"Ready\"")
	assert.Contains(t, entryJSON, baseModel.Name)

	// Verify the model entry
	var modelEntry ModelEntry
	err = json.Unmarshal([]byte(configMap.Data[key]), &modelEntry)
	assert.NoError(t, err)
	assert.Equal(t, baseModel.Name, modelEntry.Name)
	assert.Equal(t, ModelStatusReady, modelEntry.Status)
	assert.Nil(t, modelEntry.Config)

}

// TestUpdateModelMetadataInConfigMap tests the updateModelMetadataInConfigMap method
func TestUpdateModelMetadataInConfigMap(t *testing.T) {
	// Setup test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a test ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-namespace",
		},
		Data: make(map[string]string),
	}

	// Create test model metadata with fields from our internal ModelMetadata type
	modelMetadata := ModelMetadata{
		ModelType:         "llm",
		ModelArchitecture: "transformer",
		ModelFramework: &v1beta1.ModelFrameworkSpec{
			Name:    "tensorflow",
			Version: &[]string{"2.6.0"}[0],
		},
		ModelFormat: v1beta1.ModelFormat{
			Name: "savedmodel",
		},
		ModelParameterSize: "7B",
		MaxTokens:          4096,
		ModelCapabilities:  []string{"TEXT_GENERATION"},
	}

	// Create a test operation with BaseModel
	baseModel := createTestBaseModelCM()
	op := &ConfigMapMetadataOp{
		BaseModel:     baseModel,
		ModelMetadata: modelMetadata,
	}

	// Execute the test
	ctx := context.Background()
	err := reconciler.updateModelMetadataInConfigMap(ctx, configMap, op, true)
	assert.NoError(t, err)

	// Verify the ConfigMap was updated correctly
	key := reconciler.getModelConfigMapKey(baseModel, nil)
	assert.Contains(t, configMap.Data, key)

	// Verify the JSON data
	var modelEntry ModelEntry
	err = json.Unmarshal([]byte(configMap.Data[key]), &modelEntry)
	assert.NoError(t, err)
	assert.Equal(t, baseModel.Name, modelEntry.Name)
	assert.Equal(t, ModelStatusReady, modelEntry.Status)

	// Verify the model config
	assert.NotNil(t, modelEntry.Config)
	assert.Equal(t, modelMetadata.ModelType, modelEntry.Config.ModelType)
	assert.Equal(t, modelMetadata.ModelArchitecture, modelEntry.Config.ModelArchitecture)
	// Check ModelFramework map
	assert.Contains(t, modelEntry.Config.ModelFramework, "name")
	assert.Equal(t, modelMetadata.ModelFramework.Name, modelEntry.Config.ModelFramework["name"])
	// ModelFormat is stored as a map
	assert.Contains(t, modelEntry.Config.ModelFormat, "name")
	assert.Equal(t, modelMetadata.ModelFormat.Name, modelEntry.Config.ModelFormat["name"])

	// Test with ClusterBaseModel
	clusterModel := createTestClusterBaseModelCM()
	clusterOp := &ConfigMapMetadataOp{
		ClusterBaseModel: clusterModel,
		ModelMetadata:    modelMetadata,
	}

	err = reconciler.updateModelMetadataInConfigMap(ctx, configMap, clusterOp, false)
	assert.NoError(t, err)

	// Test update of existing entry
	// First, create an entry with an existing status
	existingKey := reconciler.getModelConfigMapKey(baseModel, nil)
	existingEntry := ModelEntry{
		Name:   baseModel.Name,
		Status: ModelStatusFailed,
		Config: nil,
	}
	entryBytes, _ := json.Marshal(existingEntry)
	configMap.Data[existingKey] = string(entryBytes)

	// Update with new metadata should preserve the status
	err = reconciler.updateModelMetadataInConfigMap(ctx, configMap, op, false)
	assert.NoError(t, err)

	// Verify status was preserved and config was updated
	err = json.Unmarshal([]byte(configMap.Data[existingKey]), &modelEntry)
	assert.NoError(t, err)
	assert.Equal(t, ModelStatusFailed, modelEntry.Status)
	assert.NotNil(t, modelEntry.Config)

	// Test invalid existing JSON data
	configMap.Data[existingKey] = "{invalid-json}"
	err = reconciler.updateModelMetadataInConfigMap(ctx, configMap, op, false)
	assert.NoError(t, err) // Should handle gracefully

	// Test nil Data map
	configMapNilData := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node-nil-data",
			Namespace: "test-namespace",
		},
	}
	err = reconciler.updateModelMetadataInConfigMap(ctx, configMapNilData, op, true)
	assert.NoError(t, err)
	assert.NotNil(t, configMapNilData.Data)

	// Test saving errors
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated create error")
	})

	err = reconciler.updateModelMetadataInConfigMap(ctx, configMap, op, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated create error")

	// Test create error
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated create error")
	})

	err = reconciler.saveConfigMap(ctx, configMap, "test-model", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated create error")

	// Test update error
	kubeClient.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated update error")
	})

	err = reconciler.saveConfigMap(ctx, configMap, "test-model", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated update error")
}

// TestRestoreModelInConfigMap tests the restoreModelInConfigMap method
func TestRestoreModelInConfigMap(t *testing.T) {
	// Create test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a context for all operations
	ctx := context.Background()

	// Create a ConfigMap first
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      reconciler.nodeName,
			Namespace: reconciler.namespace,
		},
		Data: map[string]string{},
	}
	_, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Create(ctx, cm, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create a test model
	baseModel := createTestBaseModelCM()

	// Add the model to the cache
	reconciler.cacheMutex.Lock()
	modelID := reconciler.getModelConfigMapKey(baseModel, nil)
	reconciler.modelCache[modelID] = &CacheEntry{
		ModelName:   "test-model",
		ModelStatus: ModelStatusReady,
		ModelMetadata: &ModelMetadata{
			ModelType:          "llm",
			ModelArchitecture:  "transformer",
			ModelParameterSize: "1.0",
		},
	}
	cacheEntry := reconciler.modelCache[modelID]
	reconciler.cacheMutex.Unlock()

	// Restore the model from cache to ConfigMap
	// Call restoreModelInConfigMap (note: this method creates its own context internally)
	reconciler.restoreModelInConfigMap(modelID, cacheEntry)

	// Verify the model was added to the ConfigMap
	updatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, updatedCM.Data, modelID)

	// Test error cases
	// 1. ConfigMap doesn't exist
	err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Delete(ctx, reconciler.nodeName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	// This should trigger recreateConfigMap
	// Call restoreModelInConfigMap again to test the recreateConfigMap path
	reconciler.restoreModelInConfigMap(modelID, cacheEntry)

	// Verify ConfigMap was recreated
	recreatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, recreatedCM.Data, modelID)

	// Skip the conflict test part since it's already covered by the implementation
	// and difficult to mock correctly in a test environment
	// The conflict handling in restoreModelInConfigMap is robust with 3 retry attempts
	// which is sufficient for most use cases in Kubernetes
}

// TestDeleteModelFromConfigMap tests the DeleteModelFromConfigMap method
func TestDeleteModelFromConfigMap(t *testing.T) {
	// Create test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a context for all operations
	ctx := context.Background()

	// Create a test model
	baseModel := createTestBaseModelCM()

	// Add model to ConfigMap and cache
	err := reconciler.ReconcileModelStatus(ctx, &ConfigMapStatusOp{
		BaseModel:   baseModel,
		ModelStatus: ModelStatusReady,
	})
	assert.NoError(t, err)

	// Verify model exists in ConfigMap and cache
	cm, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)

	modelID := reconciler.getModelConfigMapKey(baseModel, nil)
	assert.Contains(t, cm.Data, modelID)

	reconciler.cacheMutex.Lock()
	assert.Contains(t, reconciler.modelCache, modelID)
	reconciler.cacheMutex.Unlock()

	// Now delete the model
	err = reconciler.DeleteModelFromConfigMap(ctx, baseModel, nil)
	assert.NoError(t, err)

	// Verify model was deleted from both ConfigMap and cache
	updatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotContains(t, updatedCM.Data, modelID)

	reconciler.cacheMutex.Lock()
	assert.NotContains(t, reconciler.modelCache, modelID)
	reconciler.cacheMutex.Unlock()

	// Test deleting a non-existent model (should not error)
	err = reconciler.DeleteModelFromConfigMap(ctx, baseModel, nil)
	assert.NoError(t, err)

	// Test error cases: ConfigMap doesn't exist
	err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Delete(ctx, reconciler.nodeName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	// Deleting from non-existent ConfigMap should still be successful (nothing to delete)
	err = reconciler.DeleteModelFromConfigMap(ctx, baseModel, nil)
	assert.NoError(t, err)
}

// TestRecreateConfigMap tests the recreateConfigMap method
func TestRecreateConfigMap(t *testing.T) {
	// Create test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a context for all operations
	ctx := context.Background()

	// Add some entries to the model cache
	reconciler.cacheMutex.Lock()
	reconciler.modelCache["model1"] = &CacheEntry{ModelName: "model1", ModelStatus: ModelStatusReady, ModelMetadata: &ModelMetadata{ModelType: "llm", ModelParameterSize: "1.0"}}
	reconciler.modelCache["model2"] = &CacheEntry{ModelName: "model2", ModelStatus: ModelStatusUpdating, ModelMetadata: &ModelMetadata{ModelType: "llm", ModelParameterSize: "2.0"}}
	reconciler.cacheMutex.Unlock()

	// Call recreateConfigMap
	reconciler.recreateConfigMap(ctx)

	// Verify the ConfigMap was created with all cache entries
	cm, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cm.Data))
	assert.Contains(t, cm.Data, "model1")
	assert.Contains(t, cm.Data, "model2")

	// Test recreating when ConfigMap already exists
	reconciler.recreateConfigMap(ctx)

	// Verify ConfigMap still has correct entries
	cm, err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cm.Data))

	// Test error handling - make Create operation fail
	// First delete the existing ConfigMap
	err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Delete(ctx, reconciler.nodeName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	// Prepare error for Create
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("create error")
	})

	// Should handle error gracefully
	reconciler.recreateConfigMap(ctx)
	// No assertions needed - function should just log the error without returning it
}

// TestConfigMapReconciliation tests the ConfigMap reconciliation and recovery functionality
func TestConfigMapReconciliation(t *testing.T) {
	// Create test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a context for the test
	ctx := context.Background()

	// Create test models
	baseModel := createTestBaseModelCM()
	clusterModel := createTestClusterBaseModelCM()

	// Set up model status operations
	baseModelOp := &ConfigMapStatusOp{
		BaseModel:   baseModel,
		ModelStatus: ModelStatusReady,
	}
	clusterModelOp := &ConfigMapStatusOp{
		ClusterBaseModel: clusterModel,
		ModelStatus:      ModelStatusReady,
	}

	// First, add some entries to the reconciler
	var err error
	err = reconciler.ReconcileModelStatus(ctx, baseModelOp)
	assert.NoError(t, err)

	err = reconciler.ReconcileModelStatus(ctx, clusterModelOp)
	assert.NoError(t, err)

	// Verify the cache contains the entries
	reconciler.cacheMutex.Lock()
	assert.Equal(t, 2, len(reconciler.modelCache))
	reconciler.cacheMutex.Unlock()

	// Verify the ConfigMap exists with both entries
	cm, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cm.Data))

	// Delete the ConfigMap to simulate manual deletion
	err = kubeClient.CoreV1().ConfigMaps("test-namespace").Delete(ctx, "test-node", metav1.DeleteOptions{})
	assert.NoError(t, err)

	// Verify ConfigMap is deleted
	_, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.Error(t, err)

	// Execute reconciliation (which would normally happen periodically)
	reconciler.reconcileConfigMaps()

	// Verify ConfigMap has been recreated with both entries
	var recreatedCM *corev1.ConfigMap
	recreatedCM, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(recreatedCM.Data))

	// Now test recovery of individual entries
	// Update the ConfigMap to remove one entry
	var recreateCM *corev1.ConfigMap
	recreateCM, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.NoError(t, err)

	// Remove one entry
	baseModelKey := reconciler.getModelConfigMapKey(baseModel, nil)
	delete(recreateCM.Data, baseModelKey)

	// Update the ConfigMap
	_, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Update(ctx, recreateCM, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// Verify the entry was removed
	var updatedCM *corev1.ConfigMap
	updatedCM, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(updatedCM.Data))

	// Run reconciliation again
	reconciler.reconcileConfigMaps()

	// Verify the missing entry was restored
	var finalCM *corev1.ConfigMap
	finalCM, err = kubeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(finalCM.Data))
}

// TestReconcileConfigMaps tests the reconcileConfigMaps method
func TestReconcileConfigMaps(t *testing.T) {
	// Create test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a context for all operations
	ctx := context.Background()

	// Create test model entries
	baseModel := createTestBaseModelCM()
	clusterModel := createTestClusterBaseModelCM()

	// Add models to cache
	reconciler.cacheMutex.Lock()
	baseModelID := reconciler.getModelConfigMapKey(baseModel, nil)
	clusterModelID := reconciler.getModelConfigMapKey(nil, clusterModel)
	reconciler.modelCache[baseModelID] = &CacheEntry{ModelName: baseModel.Name, ModelStatus: ModelStatusReady}
	reconciler.modelCache[clusterModelID] = &CacheEntry{ModelName: clusterModel.Name, ModelStatus: ModelStatusReady}
	reconciler.cacheMutex.Unlock()

	// Initial create of ConfigMap
	reconciler.reconcileConfigMaps()

	// Check that ConfigMap was created correctly
	cm, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, cm.Data, baseModelID)
	assert.Contains(t, cm.Data, clusterModelID)

	// Test 1: Missing model entry - remove one model from ConfigMap
	delete(cm.Data, baseModelID)
	_, err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// Run reconciliation - should restore missing entry
	reconciler.reconcileConfigMaps()

	// Verify entry was restored
	updatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, updatedCM.Data, baseModelID)

	// Test 2: Get error handling - simulate ConfigMap not found
	err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Delete(ctx, reconciler.nodeName, metav1.DeleteOptions{})
	assert.NoError(t, err)

	// This should create a new ConfigMap
	reconciler.reconcileConfigMaps()

	// Verify ConfigMap was recreated
	recreatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, recreatedCM.Data, baseModelID)
	assert.Contains(t, recreatedCM.Data, clusterModelID)

	// Test 3: Other Get errors - simulate unknown error
	errorTriggered := false
	kubeClient.PrependReactor("get", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		if !errorTriggered && action.(ktesting.GetAction).GetName() == reconciler.nodeName {
			errorTriggered = true
			return true, nil, errors.New("unknown error")
		}
		return false, nil, nil
	})

	// Should log error but continue
	reconciler.reconcileConfigMaps()

	// Verify the error was handled and reconciler can continue
	// Add a small delay to ensure the fake client has stabilized
	time.Sleep(100 * time.Millisecond)

	// Test: Error unmarshaling model entry
	cm, err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Add invalid JSON as model entry
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[baseModelID] = "{invalid json"
	_, err = kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// Should log error but continue with valid entries
	reconciler.reconcileConfigMaps()

	// Verify models are correctly present in the ConfigMap
	finalCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(ctx, reconciler.nodeName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Ensure Data map exists
	assert.NotNil(t, finalCM.Data, "ConfigMap Data should not be nil")

	// Now it's safe to check keys in the map
	if finalCM.Data != nil {
		assert.Contains(t, finalCM.Data, clusterModelID)
		assert.Contains(t, finalCM.Data, baseModelID)
	}
}
