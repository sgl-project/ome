package model_agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"knative.dev/pkg/kmp"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"

	omev1beta1informers "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/informers/externalversions"
	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/informers/externalversions/ome/v1beta1"
	omev1beta1lister "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/listers/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Watcher struct {
	baseModelLister        omev1beta1lister.BaseModelLister
	baseModelSynced        cache.InformerSynced
	clusterBaseModelLister omev1beta1lister.ClusterBaseModelLister
	clusterBaseModelSynced cache.InformerSynced
	informerFactory        omev1beta1informers.SharedInformerFactory
	syncerChan             chan<- *SyncerTask
	nodeName               string
	nodeInfo               *v1.Node
	nodeShape              string
	nodeShapeAlias         string
	kubeClient             *kubernetes.Clientset
	nodeLabeler            *NodeLabeler
	logger                 *zap.SugaredLogger
}

type TensorRTLLMShapeFilter struct {
	IsTensorrtLLMModel bool
	ShapeAlias         string
	ModelType          string
}

func NewWatcher(nodeShape string,
	nodeName string,
	baseModelInformer omev1beta1.BaseModelInformer,
	clusterBaseModelInformer omev1beta1.ClusterBaseModelInformer,
	informerFactory omev1beta1informers.SharedInformerFactory,
	syncerChan chan<- *SyncerTask,
	kubeClient *kubernetes.Clientset,
	nodeLabeler *NodeLabeler,
	logger *zap.SugaredLogger) (*Watcher, error) {

	nodeShapeAlias, err := utils.GetOCINodeShortVersionShape(nodeShape)
	if err != nil {
		return nil, err
	}

	// Fetch the complete node info
	nodeInfo, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node info for node %s: %w", nodeName, err)
	}

	watcher := &Watcher{
		nodeShape:              nodeShape,
		nodeShapeAlias:         nodeShapeAlias,
		nodeInfo:               nodeInfo,
		baseModelLister:        baseModelInformer.Lister(),
		baseModelSynced:        baseModelInformer.Informer().HasSynced,
		clusterBaseModelLister: clusterBaseModelInformer.Lister(),
		clusterBaseModelSynced: clusterBaseModelInformer.Informer().HasSynced,
		informerFactory:        informerFactory,
		syncerChan:             syncerChan,
		nodeName:               nodeName,
		kubeClient:             kubeClient,
		nodeLabeler:            nodeLabeler,
		logger:                 logger,
	}

	logger.Info("Setting up informer error handlers")
	informers := map[string]cache.SharedInformer{
		"baseModelInformer":        baseModelInformer.Informer(),
		"clusterBaseModelInformer": clusterBaseModelInformer.Informer(),
	}

	for name, informer := range informers {
		err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
			// Pipe to default handler first, which just logs the error
			cache.DefaultWatchErrorHandler(r, err)

			if errors.IsUnauthorized(err) || errors.IsForbidden(err) {
				logger.Fatalf("Unable to sync cache for informer %s: %s. Requesting watcher to exit.", name, err.Error())
			}
		})

		if err != nil {
			return nil, fmt.Errorf("unable to set error handler for informer %s: %s", name, err)
		}
	}

	logger.Info("Setting up event handlers")

	if _, err := baseModelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.downloadBaseModel,
		UpdateFunc: watcher.downloadIfBaseModelNeedRefresh,
		DeleteFunc: watcher.deleteBaseModel,
	}); err != nil {
		return nil, err
	}

	if _, err := clusterBaseModelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.downloadClusterBaseModel,
		UpdateFunc: watcher.downloadIfClusterBaseModelNeedRefresh,
		DeleteFunc: watcher.deleteClusterBaseModel,
	}); err != nil {
		return nil, err
	}

	return watcher, nil
}

func (w *Watcher) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	w.logger.Info("Starting watcher")
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

	<-stopCh
	close(w.syncerChan)
	w.logger.Info("Shutting down watcher")

	return nil
}

func (w *Watcher) downloadBaseModel(obj interface{}) {
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
		w.nodeInfo, err = w.kubeClient.CoreV1().Nodes().Get(context.TODO(), w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Errorf("Error getting the node info: %s, skipping download", err.Error())
			return
		}

		if state, ok := w.nodeInfo.Labels[constants.GetModelsLabelWithUid(baseModel.UID)]; ok {
			if state == string(Ready) {
				return
			}
		}

		IsTensorrtLLMModel := false
		if baseModel.Spec.ModelFormat.Name == constants.TensorRTLLM {
			IsTensorrtLLMModel = true
		}

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := baseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		w.logger.Infof("Downloading BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)
		syncerTask := &SyncerTask{
			TaskType:  Download,
			BaseModel: baseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.syncerChan <- syncerTask
	}
}

func (w *Watcher) downloadClusterBaseModel(obj interface{}) {
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
		w.nodeInfo, err = w.kubeClient.CoreV1().Nodes().Get(context.TODO(), w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Errorf("Error getting the node info: %s, skipping download", err.Error())
			return
		}

		if state, ok := w.nodeInfo.Labels[constants.GetModelsLabelWithUid(clusterBaseModel.UID)]; ok {
			if state == string(Ready) {
				return
			}
		}

		w.logger.Infof("Downloading ClusterBaseModel: %s", clusterBaseModel.Name)

		IsTensorrtLLMModel := false
		if clusterBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM {
			IsTensorrtLLMModel = true
		}

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := clusterBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		syncerTask := &SyncerTask{
			TaskType:         Download,
			ClusterBaseModel: clusterBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.syncerChan <- syncerTask
	}
}

func (w *Watcher) downloadIfBaseModelNeedRefresh(old, new interface{}) {
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
		w.logger.Infof("ignoring because of deleting BaseModel '%s'", newBaseModel.Name)
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
		w.logger.Infof("BaseModel %s need refresh in namespace %s", newBaseModel.GetName(), newBaseModel.GetNamespace())

		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Updating,
			BaseModel:        newBaseModel,
		}
		err := w.nodeLabeler.LabelNode(nodeLabelOp)
		if err != nil {
			w.logger.Fatalf("Error labeling node for BaseModels {%s in namespace %s}: %s", newBaseModel.Name, newBaseModel.Namespace, err.Error())
			return
		}

		IsTensorrtLLMModel := false
		if newBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM {
			IsTensorrtLLMModel = true
		}

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := newBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}
		syncerTask := &SyncerTask{
			TaskType:  DownloadOverride,
			BaseModel: newBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.syncerChan <- syncerTask
	}
}

func (w *Watcher) downloadIfClusterBaseModelNeedRefresh(old, new interface{}) {
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
		w.logger.Infof("ignoring because of deleting ClusterBaseModel '%s'", newClusterBaseModel.Name)
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

		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Updating,
			ClusterBaseModel: newClusterBaseModel,
		}
		err := w.nodeLabeler.LabelNode(nodeLabelOp)
		if err != nil {
			w.logger.Fatalf("Error labeling node for ClusterBaseModels {%s}: %s", newClusterBaseModel.Name, err.Error())
			return
		}

		IsTensorrtLLMModel := false
		if newClusterBaseModel.Spec.ModelFormat.Name == constants.TensorRTLLM {
			IsTensorrtLLMModel = true
		}

		modelType := string(constants.ServingBaseModel)
		if modelTypeFromMetadata, ok := newClusterBaseModel.Spec.AdditionalMetadata["type"]; ok {
			modelType = modelTypeFromMetadata
		}

		syncerTask := &SyncerTask{
			TaskType:         DownloadOverride,
			ClusterBaseModel: newClusterBaseModel,
			TensorRTLLMShapeFilter: &TensorRTLLMShapeFilter{
				IsTensorrtLLMModel: IsTensorrtLLMModel,
				ShapeAlias:         w.nodeShapeAlias,
				ModelType:          modelType,
			},
		}

		w.syncerChan <- syncerTask
	}
}

func (w *Watcher) deleteBaseModel(obj interface{}) {
	baseModel, ok := obj.(*v1beta1.BaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to BaseModel", obj)
		return
	}

	w.logger.Infof("Deleting BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)

	nodeLabelOp := &NodeLabelOp{
		ModelStateOnNode: Deleted,
		BaseModel:        baseModel,
	}
	err := w.nodeLabeler.LabelNode(nodeLabelOp)
	if err != nil {
		w.logger.Fatalf("Error labeling node for BaseModels {%s in namespace %s}: %s", baseModel.Name, baseModel.Namespace, err.Error())
		return
	}

	syncerTask := &SyncerTask{
		TaskType:  Delete,
		BaseModel: baseModel,
	}

	w.syncerChan <- syncerTask
}

func (w *Watcher) deleteClusterBaseModel(obj interface{}) {
	clusterBaseModel, ok := obj.(*v1beta1.ClusterBaseModel)
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", obj)
		return
	}

	w.logger.Infof("Deleting ClusterBaseModel: %s", clusterBaseModel.Name)
	nodeLabelOp := &NodeLabelOp{
		ModelStateOnNode: Deleted,
		ClusterBaseModel: clusterBaseModel,
	}
	err := w.nodeLabeler.LabelNode(nodeLabelOp)
	if err != nil {
		w.logger.Fatalf("Error labeling node for ClusterBaseModels {%s}: %s", clusterBaseModel.Name, err.Error())
		return
	}

	syncerTask := &SyncerTask{
		TaskType:         Delete,
		ClusterBaseModel: clusterBaseModel,
	}

	w.syncerChan <- syncerTask
}

// shouldDownloadModel checks if a model should be downloaded to this node based on node selector and node affinity
func (w *Watcher) shouldDownloadModel(storage *v1beta1.StorageSpec) bool {
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

	// If neither NodeSelector nor NodeAffinity are specified, fallback to annotation-based matching for backward compatibility
	// TODO remove this fallback in the future once we deprecate the annotation
	if len(storage.NodeSelector) == 0 &&
		(storage.NodeAffinity == nil || storage.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil ||
			len(storage.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0) {

		// Check if target shape annotation exists on the node
		if targetShapes, ok := w.nodeInfo.Annotations[constants.TargetInstanceShapes]; ok && targetShapes != "" {
			return w.matchTargetShape(targetShapes)
		}
	}

	// Default to true if no other conditions are specified
	return true
}

func (w *Watcher) nodeMatchesSelectorTerm(term v1.NodeSelectorTerm) bool {
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

func (w *Watcher) nodeMatchesExpression(expr v1.NodeSelectorRequirement) bool {
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

// Keep the legacy implementation but modify it to check annotations in the model object
func (w *Watcher) matchTargetShape(targetShapes string) bool {
	// For backward compatibility - this functionality is now integrated into shouldDownloadModel
	// matched if target shape not specified
	if len(targetShapes) == 0 {
		return true
	}

	if strings.Contains(targetShapes, w.nodeShape) {
		return true
	}

	return false
}
