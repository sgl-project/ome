package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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
	configMap, needCreate, err := reconciler.getOrCreateConfigMap()
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
		context.TODO(), configMap, metav1.CreateOptions{},
	)
	assert.NoError(t, err)

	// Should return existing ConfigMap
	existingConfigMap, needCreate, err := reconciler.getOrCreateConfigMap()
	assert.NoError(t, err)
	assert.False(t, needCreate)
	assert.Equal(t, configMap.Name, existingConfigMap.Name)
	assert.Equal(t, configMap.Namespace, existingConfigMap.Namespace)
	assert.Equal(t, "data", existingConfigMap.Data["test"])

	// Test error case
	kubeClient.PrependReactor("get", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake error")
	})

	_, _, err = reconciler.getOrCreateConfigMap()
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

	err := reconciler.saveConfigMap(configMap, modelInfo, true)
	assert.NoError(t, err)

	// Verify ConfigMap was created
	createdCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(
		context.TODO(), reconciler.nodeName, metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "value", createdCM.Data["test"])

	// Test updating existing ConfigMap
	configMap.Data["test"] = "updated"
	err = reconciler.saveConfigMap(configMap, modelInfo, false)
	assert.NoError(t, err)

	// Verify ConfigMap was updated
	updatedCM, err := kubeClient.CoreV1().ConfigMaps(reconciler.namespace).Get(
		context.TODO(), reconciler.nodeName, metav1.GetOptions{},
	)
	assert.NoError(t, err)
	assert.Equal(t, "updated", updatedCM.Data["test"])

	// Test create error
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake create error")
	})

	err = reconciler.saveConfigMap(configMap, modelInfo, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fake create error")

	// Test update error
	kubeClient.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fake update error")
	})

	err = reconciler.saveConfigMap(configMap, modelInfo, false)
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
	err := reconciler.updateModelStatusInConfigMap(configMap, op, true)
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
	err := reconciler.updateModelMetadataInConfigMap(configMap, op, true)
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

	err = reconciler.updateModelMetadataInConfigMap(configMap, clusterOp, false)
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
	err = reconciler.updateModelMetadataInConfigMap(configMap, op, false)
	assert.NoError(t, err)

	// Verify status was preserved and config was updated
	err = json.Unmarshal([]byte(configMap.Data[existingKey]), &modelEntry)
	assert.NoError(t, err)
	assert.Equal(t, ModelStatusFailed, modelEntry.Status)
	assert.NotNil(t, modelEntry.Config)

	// Test invalid existing JSON data
	configMap.Data[existingKey] = "{invalid-json}"
	err = reconciler.updateModelMetadataInConfigMap(configMap, op, false)
	assert.NoError(t, err) // Should handle gracefully

	// Test nil Data map
	configMapNilData := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node-nil-data",
			Namespace: "test-namespace",
		},
	}
	err = reconciler.updateModelMetadataInConfigMap(configMapNilData, op, true)
	assert.NoError(t, err)
	assert.NotNil(t, configMapNilData.Data)

	// Test saving errors
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated create error")
	})

	err = reconciler.updateModelMetadataInConfigMap(configMap, op, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated create error")
}

// TestSaveConfigMap tests the saveConfigMap method
func TestSaveConfigMapCreate(t *testing.T) {
	// Setup test environment
	reconciler, kubeClient, _ := setupConfigMapTest(t)

	// Create a test ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "test-namespace",
		},
		Data: map[string]string{"test": "data"},
	}

	// Test create scenario
	err := reconciler.saveConfigMap(configMap, "test-model", true)
	assert.NoError(t, err)

	// Verify ConfigMap was created
	createdMap, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Get(context.TODO(), "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "data", createdMap.Data["test"])

	// Test update scenario
	configMap.Data["test"] = "updated-data"
	err = reconciler.saveConfigMap(configMap, "test-model", false)
	assert.NoError(t, err)

	// Verify ConfigMap was updated
	updatedMap, err := kubeClient.CoreV1().ConfigMaps("test-namespace").Get(context.TODO(), "test-node", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "updated-data", updatedMap.Data["test"])

	// Test create error
	kubeClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated create error")
	})

	err = reconciler.saveConfigMap(configMap, "test-model", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated create error")

	// Test update error
	kubeClient.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated update error")
	})

	err = reconciler.saveConfigMap(configMap, "test-model", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated update error")
}
