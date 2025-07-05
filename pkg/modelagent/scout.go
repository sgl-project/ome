package modelagent

import (
	"context"
	"fmt"
	"strconv"

	"knative.dev/pkg/kmp"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"

	omev1beta1informers "github.com/sgl-project/ome/pkg/client/informers/externalversions"
	omev1beta1 "github.com/sgl-project/ome/pkg/client/informers/externalversions/ome/v1beta1"
	omev1beta1lister "github.com/sgl-project/ome/pkg/client/listers/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Scout struct {
	ctx                    context.Context
	baseModelLister        omev1beta1lister.BaseModelLister
	baseModelSynced        cache.InformerSynced
	clusterBaseModelLister omev1beta1lister.ClusterBaseModelLister
	clusterBaseModelSynced cache.InformerSynced
	informerFactory        omev1beta1informers.SharedInformerFactory
	gopherChan             chan<- *GopherTask
	nodeName               string
	nodeInfo               *v1.Node
	nodeShapeAlias         string
	kubeClient             *kubernetes.Clientset
	logger                 *zap.SugaredLogger
}

type TensorRTLLMShapeFilter struct {
	IsTensorrtLLMModel bool
	ShapeAlias         string
	ModelType          string
}

func NewScout(ctx context.Context, nodeName string,
	baseModelInformer omev1beta1.BaseModelInformer,
	clusterBaseModelInformer omev1beta1.ClusterBaseModelInformer,
	informerFactory omev1beta1informers.SharedInformerFactory,
	gopherChan chan<- *GopherTask,
	kubeClient *kubernetes.Clientset,
	logger *zap.SugaredLogger) (*Scout, error) {

	// Fetch the complete node info
	nodeInfo, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node info for node %s: %w", nodeName, err)
	}
	// Try the newer label first, then fallback to the deprecated beta label
	instanceType, ok := nodeInfo.Labels[constants.NodeInstanceShapeLabel]
	if !ok {
		instanceType = nodeInfo.Labels[constants.DeprecatedNodeInstanceShapeLabel]
	}
	nodeShapeAlias, err := utils.GetOCINodeShortVersionShape(instanceType)
	if err != nil {
		return nil, err
	}

	scout := &Scout{
		ctx:                    ctx,
		nodeShapeAlias:         nodeShapeAlias,
		nodeInfo:               nodeInfo,
		baseModelLister:        baseModelInformer.Lister(),
		baseModelSynced:        baseModelInformer.Informer().HasSynced,
		clusterBaseModelLister: clusterBaseModelInformer.Lister(),
		clusterBaseModelSynced: clusterBaseModelInformer.Informer().HasSynced,
		informerFactory:        informerFactory,
		gopherChan:             gopherChan,
		nodeName:               nodeName,
		kubeClient:             kubeClient,
		logger:                 logger,
	}

	logger.Info("Setting up informer error handlers")
	informers := map[string]cache.SharedInformer{
		"baseModelInformer":        baseModelInformer.Informer(),
		"clusterBaseModelInformer": clusterBaseModelInformer.Informer(),
	}

	for name, informer := range informers {
		err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
			// Pipe to the default handler first, which just logs the error
			cache.DefaultWatchErrorHandler(ctx, r, err)

			if errors.IsUnauthorized(err) || errors.IsForbidden(err) {
				logger.Fatalf("Unable to sync cache for informer %s: %s. Requesting scout to exit.", name, err.Error())
			}
		})

		if err != nil {
			return nil, fmt.Errorf("unable to set error handler for informer %s: %s", name, err)
		}
	}

	logger.Info("Setting up event handlers")

	if _, err := baseModelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scout.downloadBaseModel,
		UpdateFunc: scout.updateBaseModel,
		DeleteFunc: scout.deleteBaseModel,
	}); err != nil {
		return nil, err
	}

	if _, err := clusterBaseModelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scout.downloadClusterBaseModel,
		UpdateFunc: scout.updateClusterBaseModel,
		DeleteFunc: scout.deleteClusterBaseModel,
	}); err != nil {
		return nil, err
	}

	return scout, nil
}

func (w *Scout) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	w.logger.Info("Starting scout")
	w.logger.Info("Starting informer cache")
	go w.informerFactory.Start(stopCh)

	w.logger.Info("Waiting for informer caches to sync")
	synced := []cache.InformerSynced{
		w.clusterBaseModelSynced,
		w.baseModelSynced,
	}

	if ok := cache.WaitForCacheSync(stopCh, synced...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// After caches are synced, check for any pending deletions (resources with DeletionTimestamp)
	// This ensures we catch any deletion requests that occurred while the agent was down
	w.reconcilePendingDeletions()

	<-stopCh
	close(w.gopherChan)
	w.logger.Info("Shutting down scout")

	return nil
}

func (w *Scout) downloadBaseModel(obj interface{}) {
	baseModel, ok := obj.(*v1beta1.BaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to BaseModel", obj)
		return
	}

	w.logger.Infof("Processing BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)
	if !baseModel.ObjectMeta.DeletionTimestamp.IsZero() {
		w.logger.Infof("ignoring because of deleting of BaseModel '%s'", baseModel.Name)
		return
	}

	if w.shouldDownloadModel(baseModel.Spec.Storage) {
		// Refresh the node info
		var err error
		w.nodeInfo, err = w.kubeClient.CoreV1().Nodes().Get(w.ctx, w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Errorf("Error getting the node info: %s, skipping download", err.Error())
			return
		}

		w.logger.Infof("Downloading BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)

		IsTensorrtLLMModel := baseModel.Spec.ModelFormat.Name == constants.TensorRTLLM

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := baseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		gopherTask := &GopherTask{
			TaskType:  Download,
			BaseModel: baseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.gopherChan <- gopherTask
	}
}

func (w *Scout) downloadClusterBaseModel(obj interface{}) {
	clusterBaseModel, ok := obj.(*v1beta1.ClusterBaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to clusterBaseModel", obj)
		return
	}

	w.logger.Infof("Processing ClusterBaseModel: %s", clusterBaseModel.Name)
	if !clusterBaseModel.ObjectMeta.DeletionTimestamp.IsZero() {
		w.logger.Infof("ignoring because of deleting ClusterBaseModel '%s'", clusterBaseModel.Name)
		return
	}

	if w.shouldDownloadModel(clusterBaseModel.Spec.Storage) {
		// Refresh the node info
		var err error
		w.nodeInfo, err = w.kubeClient.CoreV1().Nodes().Get(w.ctx, w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Errorf("Error getting the node info: %s, skipping download", err.Error())
			return
		}

		w.logger.Infof("Downloading ClusterBaseModel: %s", clusterBaseModel.Name)

		IsTensorrtLLMModel := clusterBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := clusterBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		gopherTask := &GopherTask{
			TaskType:         Download,
			ClusterBaseModel: clusterBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.gopherChan <- gopherTask
	}
}

func (w *Scout) updateBaseModel(old, new interface{}) {
	oldBaseModel, ok := old.(*v1beta1.BaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", old)
		return
	}
	newBaseModel := new.(*v1beta1.BaseModel)

	if w.shouldDownloadModel(oldBaseModel.Spec.Storage) &&
		!w.shouldDownloadModel(newBaseModel.Spec.Storage) {
		// shape config changed, delete it from the current node
		w.logger.Infof("Target shapes excluded BaseModel update: %s in namespace %s, deleting", newBaseModel.GetName(), newBaseModel.GetNamespace())
		w.deleteBaseModel(new)
		return
	}

	if !newBaseModel.ObjectMeta.DeletionTimestamp.IsZero() {
		w.logger.Infof("Resource has deletion timestamp: BaseModel '%s', processing delete", newBaseModel.Name)
		w.deleteBaseModel(newBaseModel)
		return
	}

	hasChanges := false
	for _, diff := range []struct {
		name     string
		old, new interface{}
	}{
		{"Labels", oldBaseModel.Labels, newBaseModel.Labels},
		{"Annotations", oldBaseModel.Annotations, newBaseModel.Annotations},
		{"Spec", oldBaseModel.Spec, newBaseModel.Spec},
	} {
		result, err := kmp.SafeDiff(diff.old, diff.new)
		if err != nil {
			w.logger.Errorf("Failed to diff %s for BaseModel: %s in namespace %s",
				diff.name, newBaseModel.Name, newBaseModel.Namespace)
			return
		}
		hasChanges = hasChanges || (result != "")
	}

	if hasChanges && w.shouldDownloadModel(newBaseModel.Spec.Storage) {
		w.logger.Infof("BaseModel %s needs refresh in namespace %s", newBaseModel.GetName(), newBaseModel.GetNamespace())

		IsTensorrtLLMModel := newBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := newBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}
		gopherTask := &GopherTask{
			TaskType:  DownloadOverride,
			BaseModel: newBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.gopherChan <- gopherTask
	}
}

func (w *Scout) updateClusterBaseModel(old, new interface{}) {
	oldClusterBaseModel, ok := old.(*v1beta1.ClusterBaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", old)
		return
	}

	newClusterBaseModel, ok := new.(*v1beta1.ClusterBaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", new)
		return
	}

	if w.shouldDownloadModel(oldClusterBaseModel.Spec.Storage) &&
		!w.shouldDownloadModel(newClusterBaseModel.Spec.Storage) {
		// shape config changed, delete it from the current node
		w.logger.Infof("Target shapes excluded ClusterBaseModel %s, deleting", newClusterBaseModel.GetName())
		w.deleteClusterBaseModel(new)
		return
	}

	if !newClusterBaseModel.ObjectMeta.DeletionTimestamp.IsZero() {
		w.logger.Infof("Resource has deletion timestamp: ClusterBaseModel '%s', processing delete", newClusterBaseModel.Name)
		w.deleteClusterBaseModel(newClusterBaseModel)
		return
	}

	hasChanges := false
	for _, diff := range []struct {
		name     string
		old, new interface{}
	}{
		{"Labels", oldClusterBaseModel.Labels, newClusterBaseModel.Labels},
		{"Annotations", oldClusterBaseModel.Annotations, newClusterBaseModel.Annotations},
		{"Spec", oldClusterBaseModel.Spec, newClusterBaseModel.Spec},
	} {
		result, err := kmp.SafeDiff(diff.old, diff.new)
		if err != nil {
			w.logger.Errorf("Failed to diff %s for BaseModel: %s in namespace %s",
				diff.name, newClusterBaseModel.Name, newClusterBaseModel.Namespace)
			return
		}
		hasChanges = hasChanges || (result != "")
	}

	if hasChanges && w.shouldDownloadModel(newClusterBaseModel.Spec.Storage) {
		w.logger.Infof("ClusterBaseModel %s need refresh", newClusterBaseModel.GetName())

		IsTensorrtLLMModel := newClusterBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := newClusterBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		gopherTask := &GopherTask{
			TaskType:         DownloadOverride,
			ClusterBaseModel: newClusterBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.gopherChan <- gopherTask
	}
}

func (w *Scout) deleteBaseModel(obj interface{}) {
	baseModel, ok := obj.(*v1beta1.BaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to BaseModel", obj)
		return
	}

	w.logger.Infof("Deleting BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)

	gopherTask := &GopherTask{
		TaskType:  Delete,
		BaseModel: baseModel,
	}

	w.gopherChan <- gopherTask
}

func (w *Scout) deleteClusterBaseModel(obj interface{}) {
	clusterBaseModel, ok := obj.(*v1beta1.ClusterBaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", obj)
		return
	}

	w.logger.Infof("Deleting ClusterBaseModel: %s", clusterBaseModel.Name)

	gopherTask := &GopherTask{
		TaskType:         Delete,
		ClusterBaseModel: clusterBaseModel,
	}
	w.gopherChan <- gopherTask
}

// reconcilePendingDeletions checks for any resources with deletion timestamps
// and processes them to ensure no deletions are missed if the model agent was down
// when the deletion request was made
func (w *Scout) reconcilePendingDeletions() {
	w.logger.Info("Checking for pending deletions on startup...")

	// Check BaseModels with deletionTimestamp
	baseModels, err := w.baseModelLister.List(labels.Everything())
	if err != nil {
		w.logger.Errorf("Failed to list BaseModels during reconciliation: %v", err)
	} else {
		for _, baseModel := range baseModels {
			if !baseModel.ObjectMeta.DeletionTimestamp.IsZero() {
				w.logger.Infof("Found BaseModel with deletion timestamp during startup: %s in namespace %s",
					baseModel.Name, baseModel.Namespace)
				w.deleteBaseModel(baseModel)
			}
		}
	}

	// Check ClusterBaseModels with deletionTimestamp
	clusterBaseModels, err := w.clusterBaseModelLister.List(labels.Everything())
	if err != nil {
		w.logger.Errorf("Failed to list ClusterBaseModels during reconciliation: %v", err)
	} else {
		for _, clusterBaseModel := range clusterBaseModels {
			if !clusterBaseModel.ObjectMeta.DeletionTimestamp.IsZero() {
				w.logger.Infof("Found ClusterBaseModel with deletion timestamp during startup: %s",
					clusterBaseModel.Name)
				w.deleteClusterBaseModel(clusterBaseModel)
			}
		}
	}

	w.logger.Info("Finished checking for pending deletions")
}

// shouldDownloadModel checks if a model should be downloaded to this node based on node selector and node affinity
func (w *Scout) shouldDownloadModel(storage *v1beta1.StorageSpec) bool {
	if storage == nil {
		// If storage is nil, default to true (backward compatibility)
		return true
	}

	// Check NodeSelector if specified
	if len(storage.NodeSelector) > 0 {
		for key, value := range storage.NodeSelector {
			nodeValue, exists := w.nodeInfo.Labels[key]
			if !exists || nodeValue != value {
				return false
			}
		}
	}

	// Check NodeAffinity if specified
	if storage.NodeAffinity != nil && storage.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeSelectorTerms := storage.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		if len(nodeSelectorTerms) > 0 {
			matches := false
			for _, term := range nodeSelectorTerms {
				if w.nodeMatchesSelectorTerm(term) {
					matches = true
					break
				}
			}
			if !matches {
				return false
			}
		}
	}

	// Default to true if no other conditions are specified
	return true
}

func (w *Scout) nodeMatchesSelectorTerm(term v1.NodeSelectorTerm) bool {
	// Check match expressions
	for _, expr := range term.MatchExpressions {
		if !w.nodeMatchesExpression(expr) {
			return false
		}
	}

	// Check match fields
	for _, field := range term.MatchFields {
		if !w.nodeMatchesExpression(field) {
			return false
		}
	}

	return true
}

func (w *Scout) nodeMatchesExpression(expr v1.NodeSelectorRequirement) bool {
	// Get the field value based on whether it's a label or field selector
	var values []string
	var exists bool

	// For label selectors, get the label values
	labelValue, labelExists := w.nodeInfo.Labels[expr.Key]
	if labelExists {
		values = []string{labelValue}
		exists = true
	}

	// If not found in labels, try fields (only for special fields)
	if !exists {
		switch expr.Key {
		case "metadata.name":
			values = []string{w.nodeInfo.Name}
			exists = true
			// Add other field cases as needed
		}
	}

	if !exists {
		return expr.Operator == v1.NodeSelectorOpDoesNotExist
	}

	switch expr.Operator {
	case v1.NodeSelectorOpIn:
		for _, v := range values {
			for _, requiredValue := range expr.Values {
				if v == requiredValue {
					return true
				}
			}
		}
		return false
	case v1.NodeSelectorOpNotIn:
		for _, v := range values {
			for _, requiredValue := range expr.Values {
				if v == requiredValue {
					return false
				}
			}
		}
		return true
	case v1.NodeSelectorOpExists:
		return true
	case v1.NodeSelectorOpDoesNotExist:
		return false
	case v1.NodeSelectorOpGt:
		if len(values) == 0 || len(expr.Values) == 0 {
			return false
		}
		// Try to convert to integers for numeric comparison
		nodeVal, nodeErr := strconv.Atoi(values[0])
		requiredVal, reqErr := strconv.Atoi(expr.Values[0])
		if nodeErr == nil && reqErr == nil {
			// If both values can be parsed as integers, do numeric comparison
			return nodeVal > requiredVal
		}
		// Fall back to string comparison if not numeric
		return values[0] > expr.Values[0]
	case v1.NodeSelectorOpLt:
		if len(values) == 0 || len(expr.Values) == 0 {
			return false
		}
		// Try to convert to integers for numeric comparison
		nodeVal, nodeErr := strconv.Atoi(values[0])
		requiredVal, reqErr := strconv.Atoi(expr.Values[0])
		if nodeErr == nil && reqErr == nil {
			// If both values can be parsed as integers, do numeric comparison
			return nodeVal < requiredVal
		}
		// Fall back to string comparison if not numeric
		return values[0] < expr.Values[0]
	}

	return false
}
