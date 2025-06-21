package multimodelconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ModelSpecInfo holds the extracted storage and metadata for a model or fine-tuned weight.
type ModelSpecInfo struct {
	StorageURI            *string           `json:"storageUri,omitempty"`
	Path                  *string           `json:"path,omitempty"`
	SchemaPath            *string           `json:"schemaPath,omitempty"`
	Parameters            map[string]string `json:"parameters,omitempty"`
	StorageKey            *string           `json:"storageKey,omitempty"`
	ModelFormatName       *string           `json:"modelFormatName,omitempty"`
	ModelFormatVersion    *string           `json:"modelFormatVersion,omitempty"`
	ModelType             *string           `json:"modelType,omitempty"`
	ModelFrameworkName    *string           `json:"modelFrameworkName,omitempty"`
	ModelFrameworkVersion *string           `json:"modelFrameworkVersion,omitempty"`
}

// ModelConfigEntry defines the structure for a single model's configuration within the ConfigMap.
// This structure will be part of an array in the ConfigMap's data to support multi-model configurations in the future.
type ModelConfigEntry struct {
	ModelName           string         `json:"modelName"` // e.g., "primary" or the actual model name from ModelRef
	ModelSpec           ModelSpecInfo  `json:"modelSpec"`
	FineTunedWeightSpec *ModelSpecInfo `json:"fineTunedWeightSpec,omitempty"`
}

type ConfigMapReconciler struct {
	client    client.Client
	clientset kubernetes.Interface
	scheme    *runtime.Scheme
}

func NewModelConfigReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		client:    client,
		clientset: clientset,
		scheme:    scheme,
	}
}

func (c *ConfigMapReconciler) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	ctxLog := logf.FromContext(ctx)

	// If no model is specified, we don't need a model config ConfigMap.
	if isvc.Spec.Model == nil {
		// TODO: Consider deleting an existing ConfigMap if the model spec is removed.
		ctxLog.Info("No model specified in InferenceService, skipping ModelConfig reconciliation", "InferenceService", isvc.Name)
		return ctrl.Result{}, nil
	}

	modelConfigName := constants.ModelConfigName(isvc.Name) // Reverted to constants package

	// Generate desired ModelConfigEntry
	desiredEntry, err := c.generateModelConfigEntry(ctx, isvc)
	if err != nil {
		ctxLog.Error(err, "Failed to generate ModelConfigEntry")
		return ctrl.Result{}, err
	}

	// For now, we are managing a single model entry based on InferenceService.Spec.Model
	// In the future, this could be expanded to manage multiple models in the ConfigMap.
	desiredEntries := []ModelConfigEntry{*desiredEntry}
	desiredData, err := json.Marshal(desiredEntries)
	if err != nil {
		ctxLog.Error(err, "Failed to marshal desired ModelConfig data")
		return ctrl.Result{}, err
	}

	// Retrieve the ConfigMap
	existingCm := &corev1.ConfigMap{}
	err = c.client.Get(ctx, types.NamespacedName{Name: modelConfigName, Namespace: isvc.Namespace}, existingCm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap is not found, create a new one
			ctxLog.Info("Creating new ModelConfig", "name", modelConfigName, "InferenceService", isvc.Name)
			return c.createModelConfig(ctx, isvc, modelConfigName, desiredData)
		}
		// Unexpected error while retrieving ConfigMap
		ctxLog.Error(err, "Failed to get ConfigMap", "configmap", modelConfigName)
		return ctrl.Result{}, err
	}

	// ConfigMap exists, check if update is needed
	// Unmarshal existing data to compare semantically with desired state (array of ModelConfigEntry)
	var existingEntries []ModelConfigEntry
	existingDataJSON, ok := existingCm.Data[constants.ModelConfigKey]
	if ok && existingDataJSON != "" {
		if errUnmarshal := json.Unmarshal([]byte(existingDataJSON), &existingEntries); errUnmarshal != nil {
			ctxLog.Error(errUnmarshal, "Failed to unmarshal existing ModelConfig data, will overwrite", "name", modelConfigName)
			ok = false // Treat as if data is not there or invalid
		}
	}

	if !ok || !equality.Semantic.DeepEqual(existingEntries, desiredEntries) {
		ctxLog.Info("ModelConfig exists but needs update", "name", modelConfigName)
		updatedCm := existingCm.DeepCopy()
		if updatedCm.Data == nil {
			updatedCm.Data = make(map[string]string)
		}
		updatedCm.Data[constants.ModelConfigKey] = string(desiredData)

		err = c.client.Update(ctx, updatedCm)
		if err != nil {
			ctxLog.Error(err, "Failed to update ConfigMap", "configmap", modelConfigName)
			return ctrl.Result{}, err
		}

		ctxLog.Info("Successfully updated ModelConfig", "name", modelConfigName)
	} else {
		ctxLog.V(1).Info("ModelConfig already exists and is up-to-date", "configmap", modelConfigName)
	}

	return ctrl.Result{}, nil
}

func (c *ConfigMapReconciler) createModelConfig(ctx context.Context, isvc *v1beta1.InferenceService, modelConfigName string, jsonData []byte) (ctrl.Result, error) {
	// Use the package-level logger or a logger derived from context if specific context is needed.
	// For now, let's ensure we are not re-declaring if 'log' is already the package-level one.
	// If we want a context-specific logger:
	// createLog := logf.FromContext(ctx).WithName("createModelConfig")
	// And then use createLog instead of log.
	// For simplicity and to avoid shadowing the package var 'log' with 'log :=' if not intended,
	// we can directly use the package 'log' or be explicit with a new variable name.
	// Let's use a new variable for the context logger to avoid confusion.
	createOpLog := logf.FromContext(ctx)
	newModelConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelConfigName,
			Namespace: isvc.Namespace,
			Labels: map[string]string{
				constants.InferenceServicePodLabelKey: isvc.Name,
			},
		},
		Data: map[string]string{
			constants.ModelConfigKey: string(jsonData),
		},
	}

	err := controllerutil.SetControllerReference(isvc, newModelConfig, c.scheme)
	if err != nil {
		createOpLog.Error(err, "Failed to set controller reference for modelConfig", "configmap", modelConfigName)
		return ctrl.Result{}, err
	}

	// Use c.client for creating corev1 types like ConfigMap
	err = c.client.Create(ctx, newModelConfig)
	if err != nil {
		createOpLog.Error(err, "Failed to create ConfigMap via clientset", "configmap", modelConfigName)
		return ctrl.Result{}, err
	}

	createOpLog.Info("Successfully created modelConfig", "configmap", modelConfigName)
	return ctrl.Result{}, nil
}

func (c *ConfigMapReconciler) getBaseModelSpec(ctx context.Context, modelRef *v1beta1.ModelRef, isvcNamespace string) (*v1beta1.BaseModelSpec, error) {
	log := logf.FromContext(ctx)
	modelKind := "ClusterBaseModel" // Default
	if modelRef.Kind != nil && *modelRef.Kind != "" {
		modelKind = *modelRef.Kind
	}

	namespacedName := types.NamespacedName{Name: modelRef.Name}
	if modelKind == "BaseModel" {
		namespacedName.Namespace = isvcNamespace
		baseModel := &v1beta1.BaseModel{}
		if err := c.client.Get(ctx, namespacedName, baseModel); err != nil {
			log.Error(err, "Failed to get referenced BaseModel", "name", modelRef.Name, "namespace", isvcNamespace)
			return nil, err
		}
		return &baseModel.Spec, nil
	} else {
		clusterBaseModel := &v1beta1.ClusterBaseModel{}
		if err := c.client.Get(ctx, namespacedName, clusterBaseModel); err != nil {
			log.Error(err, "Failed to get referenced ClusterBaseModel", "name", modelRef.Name)
			return nil, err
		}
		return &clusterBaseModel.Spec, nil
	}
}

func (c *ConfigMapReconciler) getFineTunedWeightSpec(ctx context.Context, ftwName string, namespace string) (*v1beta1.FineTunedWeightSpec, error) {
	log := logf.FromContext(ctx)
	ftw := &v1beta1.FineTunedWeight{}
	// FineTunedWeight is cluster-scoped, so we only use the Name (no namespace)
	namespacedName := types.NamespacedName{Name: ftwName}
	if err := c.client.Get(ctx, namespacedName, ftw); err != nil {
		log.Error(err, "Failed to get FineTunedWeight", "name", ftwName)
		return nil, err
	}
	return &ftw.Spec, nil
}

func populateStorageFields(target *ModelSpecInfo, sourceStorage *v1beta1.StorageSpec) {
	if sourceStorage == nil {
		return
	}
	target.StorageURI = sourceStorage.StorageUri
	target.Path = sourceStorage.Path
	target.SchemaPath = sourceStorage.SchemaPath
	if sourceStorage.Parameters != nil {
		target.Parameters = *sourceStorage.Parameters
	}
	target.StorageKey = sourceStorage.StorageKey
}

func populateBaseModelDetails(target *ModelSpecInfo, bmSpec *v1beta1.BaseModelSpec) {
	populateStorageFields(target, bmSpec.Storage)
	target.ModelFormatName = &bmSpec.ModelFormat.Name
	if bmSpec.ModelFormat.Version != nil {
		target.ModelFormatVersion = bmSpec.ModelFormat.Version
	}
	if bmSpec.ModelFramework != nil {
		target.ModelFrameworkName = &bmSpec.ModelFramework.Name
		if bmSpec.ModelFramework.Version != nil {
			target.ModelFrameworkVersion = bmSpec.ModelFramework.Version
		}
	}
	target.ModelType = bmSpec.ModelType
}

func populateFineTunedWeightDetails(target *ModelSpecInfo, ftwSpec *v1beta1.FineTunedWeightSpec) {
	populateStorageFields(target, ftwSpec.Storage)
	target.ModelType = ftwSpec.ModelType
}

func (c *ConfigMapReconciler) generateModelConfigEntry(ctx context.Context, isvc *v1beta1.InferenceService) (*ModelConfigEntry, error) {
	log := logf.FromContext(ctx)
	isvcModelRef := isvc.Spec.Model // This is the ModelRef from the InferenceService Spec

	entry := &ModelConfigEntry{}
	var baseModelSpecToUse *v1beta1.BaseModelSpec
	var err error

	// Determine the kind of the primary model reference in the InferenceService Spec
	primaryModelKind := "" // Default to empty, getBaseModelSpec might default to ClusterBaseModel
	if isvcModelRef.Kind != nil && *isvcModelRef.Kind != "" {
		primaryModelKind = *isvcModelRef.Kind
	}

	if primaryModelKind == "FineTunedWeight" {
		// Case 1: InferenceService directly references a FineTunedWeight
		log.V(1).Info("InferenceService references a FineTunedWeight directly", "ftwName", isvcModelRef.Name)

		// 1a. Fetch the FineTunedWeight resource itself.
		// Note: isvc.Namespace is passed but getFineTunedWeightSpec ignores it as FTW is cluster-scoped.
		ftwSpec, errFtw := c.getFineTunedWeightSpec(ctx, isvcModelRef.Name, isvc.Namespace)
		if errFtw != nil {
			return nil, fmt.Errorf("failed to get referenced fine tuned weight spec for %s: %w", isvcModelRef.Name, errFtw)
		}

		// 1b. Get the BaseModelRef from ftwSpec.BaseModelRef. This points to the actual base model.
		if ftwSpec.BaseModelRef.Name == nil || *ftwSpec.BaseModelRef.Name == "" {
			return nil, fmt.Errorf("FineTunedWeight %s has no BaseModelRef.Name specified", isvcModelRef.Name)
		}

		actualBaseModelName := *ftwSpec.BaseModelRef.Name
		entry.ModelName = actualBaseModelName // ModelName in ConfigMap is the name of the actual base model

		// 1c. Construct a ModelRef to fetch this actual base model.
		refToActualBase := &v1beta1.ModelRef{Name: actualBaseModelName}
		namespaceForActualBaseLookup := isvc.Namespace // Default to ISVC namespace for base model lookup context

		if ftwSpec.BaseModelRef.Namespace != nil && *ftwSpec.BaseModelRef.Namespace != "" {
			// If BaseModelRef in FTW specifies a namespace, it's a namespaced BaseModel in that specific namespace.
			refToActualBase.Kind = func(s string) *string { return &s }("BaseModel")
			namespaceForActualBaseLookup = *ftwSpec.BaseModelRef.Namespace
		} else {
			// If BaseModelRef in FTW does not specify a namespace, it implies a ClusterBaseModel.
			refToActualBase.Kind = func(s string) *string { return &s }("ClusterBaseModel")
			// For ClusterBaseModel, the namespaceForActualBaseLookup will be effectively ignored by getBaseModelSpec.
		}

		// 1d. Fetch the spec of the actual base model.
		baseModelSpecToUse, err = c.getBaseModelSpec(ctx, refToActualBase, namespaceForActualBaseLookup)
		if err != nil {
			return nil, fmt.Errorf("failed to get base model spec for %s (referenced by FTW %s): %w", actualBaseModelName, isvcModelRef.Name, err)
		}

		// 1e. Populate the FineTunedWeightSpec part of the entry using the initially fetched ftwSpec.
		entry.FineTunedWeightSpec = &ModelSpecInfo{}
		populateFineTunedWeightDetails(entry.FineTunedWeightSpec, ftwSpec)

	} else {
		// Case 2: InferenceService references a BaseModel or ClusterBaseModel directly (or Kind is empty/nil)
		log.V(1).Info("InferenceService references a BaseModel/ClusterBaseModel directly", "modelName", isvcModelRef.Name, "kind", primaryModelKind)
		entry.ModelName = isvcModelRef.Name // ModelName in ConfigMap is the name of this directly referenced model

		baseModelSpecToUse, err = c.getBaseModelSpec(ctx, isvcModelRef, isvc.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get base model spec for %s: %w", isvcModelRef.Name, err)
		}

		// 2a. Handle any FineTunedWeights listed in isvcModelRef.FineTunedWeights array.
		// This applies if the primary reference is a BaseModel/ClusterBaseModel that itself uses a FTW.
		if len(isvcModelRef.FineTunedWeights) > 0 && isvcModelRef.FineTunedWeights[0] != "" {
			ftwNameFromArray := isvcModelRef.FineTunedWeights[0]
			log.V(1).Info("Processing FineTunedWeight specified in ModelRef.FineTunedWeights", "ftwName", ftwNameFromArray)

			// Note: isvc.Namespace is passed but getFineTunedWeightSpec ignores it as FTW is cluster-scoped.
			ftwSpecFromArray, errFtwArray := c.getFineTunedWeightSpec(ctx, ftwNameFromArray, isvc.Namespace)
			if errFtwArray != nil {
				return nil, fmt.Errorf("failed to get fine tuned weight spec %s (listed in ModelRef.FineTunedWeights): %w", ftwNameFromArray, errFtwArray)
			}
			entry.FineTunedWeightSpec = &ModelSpecInfo{}
			populateFineTunedWeightDetails(entry.FineTunedWeightSpec, ftwSpecFromArray)
		}
	}

	// Common step: Populate ModelSpec part of the entry using the determined baseModelSpecToUse.
	if baseModelSpecToUse == nil {
		// This should ideally not happen if errors are handled above, but as a safeguard:
		return nil, fmt.Errorf("baseModelSpecToUse is nil for model %s, cannot populate ModelSpecInfo", entry.ModelName)
	}
	populateBaseModelDetails(&entry.ModelSpec, baseModelSpecToUse)

	return entry, nil
}
