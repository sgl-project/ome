package model_agent

import (
	"context"
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	omev1beta1informers "bitbucket.oci.oraclecorp.com/gen/ome/pkg/client/informers/externalversions"
	omev1beta1 "bitbucket.oci.oraclecorp.com/gen/ome/pkg/client/informers/externalversions/serving/v1beta1"
	omev1beta1lister "bitbucket.oci.oraclecorp.com/gen/ome/pkg/client/listers/serving/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"go.uber.org/zap"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
)

type Watcher struct {
	baseModelLister   omev1beta1lister.BaseModelLister
	baseModelSynced   cache.InformerSynced
	clusterBaseModelLister omev1beta1lister.ClusterBaseModelLister
	clusterBaseModelSynced cache.InformerSynced
	informerFactory   omev1beta1informers.SharedInformerFactory
	syncerChan        chan<- *SyncerTask
	nodeName          string
	nodeShape         string
	kubeClient        *kubernetes.Clientset
	nodeLabeler       *NodeLabeler
	logger            *zap.SugaredLogger
}

func NewWatcher(nodeShape string,
				nodeName string,
				baseModelInformer omev1beta1.BaseModelInformer, 
				clusterBaseModelInformer omev1beta1.ClusterBaseModelInformer,
				informerFactory omev1beta1informers.SharedInformerFactory,
				syncerChan chan<- *SyncerTask,
				kubeClient  *kubernetes.Clientset,
				nodeLabeler *NodeLabeler,
				logger *zap.SugaredLogger) (*Watcher, error) {
	watcher := &Watcher{
		nodeShape: nodeShape,
		baseModelLister: baseModelInformer.Lister(),
		baseModelSynced: baseModelInformer.Informer().HasSynced,
		clusterBaseModelLister: clusterBaseModelInformer.Lister(),
		clusterBaseModelSynced: clusterBaseModelInformer.Informer().HasSynced,
		informerFactory: informerFactory,
		syncerChan: syncerChan,
		nodeName: nodeName,
		kubeClient: kubeClient, 
		nodeLabeler: nodeLabeler,
		logger: logger,
	}

	logger.Info("Setting up informer error handlers")
	informers := map[string]cache.SharedInformer{
		"baseModelInformer":               baseModelInformer.Informer(),
		"clusterBaseModelInformer":        clusterBaseModelInformer.Informer(),
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

	if w.matchTargetShape(baseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
		nodeInfo, err := w.kubeClient.CoreV1().Nodes().Get(context.TODO(), w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Fatalf("Error getting the node info: %s", err.Error())
		} else {
			if state, ok := nodeInfo.Labels[constants.GetModelsLabelWithUid(baseModel.UID)]; ok {
				if state == string(Ready) {
					return
				}
			}
		}

		w.logger.Infof("Downloading BaseModel: %s in namespace %s", baseModel.Name, baseModel.Namespace)
		syncerTask := &SyncerTask{
			TaskType: Download,
			BaseModel: baseModel,
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
	if !clusterBaseModel.ObjectMeta.DeletionTimestamp.IsZero(){
		w.logger.Infof("ignoring because of deleting ClusterBaseModel '%s'", clusterBaseModel.Name)
		return
	}
 
	if w.matchTargetShape(clusterBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
		nodeInfo, err := w.kubeClient.CoreV1().Nodes().Get(context.TODO(), w.nodeName, metav1.GetOptions{})
		if err != nil {
			w.logger.Fatalf("Error getting the node info: %s", err.Error())
		} else {
			if state, ok := nodeInfo.Labels[constants.GetModelsLabelWithUid(clusterBaseModel.UID)]; ok {
				if state == string(Ready) {
					return
				}
			}
		}

		w.logger.Infof("Downloading ClusterBaseModel: %s", clusterBaseModel.Name)

		syncerTask := &SyncerTask{
			TaskType: Download,
			ClusterBaseModel: clusterBaseModel,
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
	if !ok {
		w.logger.Errorf("Failed to convert %v to ClusterBaseModel", new)
		return
	}

	w.logger.Infof("Processing BaseModel update: %s in namespace %s", newBaseModel.GetName(), newBaseModel.GetNamespace())

	if w.matchTargetShape(oldBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) &&
		!w.matchTargetShape(newBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
		// shape config changed, delete from current node
		w.logger.Infof("Target shapes excluded BaseModel update: %s in namespace %s, deleting", newBaseModel.GetName(), newBaseModel.GetNamespace())
		w.deleteBaseModel(new)
		return
 	}

	if !newBaseModel.ObjectMeta.DeletionTimestamp.IsZero(){
		w.logger.Infof("ignoring because of deleting BaseModel '%s'", newBaseModel.Name)
		return
	}

	var needRefresh bool = true
	if equality.Semantic.DeepEqual(oldBaseModel.Spec.Storage.StorageUri, newBaseModel.Spec.Storage.StorageUri) &&
		equality.Semantic.DeepEqual(oldBaseModel.Spec.Storage.Path, newBaseModel.Spec.Storage.Path) &&
		equality.Semantic.DeepEqual(oldBaseModel.Spec.Storage.SchemaPath, newBaseModel.Spec.Storage.SchemaPath) &&
		equality.Semantic.DeepEqual(oldBaseModel.Spec.Storage.Parameters, newBaseModel.Spec.Storage.Parameters) &&
		equality.Semantic.DeepEqual(oldBaseModel.Spec.Storage.StorageKey, newBaseModel.Spec.Storage.StorageKey) {
		needRefresh = false
	}

	if needRefresh && w.matchTargetShape(newBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
		w.logger.Infof("BaseModel %s need refresh in namespace %s", newBaseModel.GetName(), newBaseModel.GetNamespace())

		nodeLabelOp := &NodeLabelOp{
			ModelStateOnNode: Updating,
			BaseModel: newBaseModel,
		}
		err := w.nodeLabeler.LabelNode(nodeLabelOp)
		if err != nil {
			w.logger.Fatalf("Error labeling node for BaseModels {%s in namespace %s}: %s", newBaseModel.Name, newBaseModel.Namespace, err.Error())
			return
		}

		syncerTask := &SyncerTask{
			TaskType: DownloadOverride,
			BaseModel: newBaseModel,
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

	w.logger.Infof("Processing ClusterBaseModel update: %s", newClusterBaseModel.GetName())

	if w.matchTargetShape(oldClusterBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) &&
		!w.matchTargetShape(newClusterBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
		// shape config changed, delete from current node
		w.logger.Infof("Target shapes excluded ClusterBaseModel %s, deleting", newClusterBaseModel.GetName())
		w.deleteClusterBaseModel(new)
		return
	}
 
	if !newClusterBaseModel.ObjectMeta.DeletionTimestamp.IsZero(){
		w.logger.Infof("ignoring because of deleting ClusterBaseModel '%s'", newClusterBaseModel.Name)
		return
	}

	var needRefresh bool = true
	if equality.Semantic.DeepEqual(oldClusterBaseModel.Spec.Storage.StorageUri, newClusterBaseModel.Spec.Storage.StorageUri) &&
		equality.Semantic.DeepEqual(oldClusterBaseModel.Spec.Storage.Path, newClusterBaseModel.Spec.Storage.Path) &&
		equality.Semantic.DeepEqual(oldClusterBaseModel.Spec.Storage.SchemaPath, newClusterBaseModel.Spec.Storage.SchemaPath) &&
		equality.Semantic.DeepEqual(oldClusterBaseModel.Spec.Storage.Parameters, newClusterBaseModel.Spec.Storage.Parameters) &&
		equality.Semantic.DeepEqual(oldClusterBaseModel.Spec.Storage.StorageKey, newClusterBaseModel.Spec.Storage.StorageKey) {
		needRefresh = false
	}

	if needRefresh && w.matchTargetShape(newClusterBaseModel.ObjectMeta.Annotations[constants.TargetInstanceShapes]) {
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

		syncerTask := &SyncerTask{
			TaskType: DownloadOverride,
			ClusterBaseModel: newClusterBaseModel,
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
		BaseModel: baseModel,
	}
	err := w.nodeLabeler.LabelNode(nodeLabelOp)
	if err != nil {
		w.logger.Fatalf("Error labeling node for BaseModels {%s in namespace %s}: %s", baseModel.Name, baseModel.Namespace, err.Error())
		return
	}

	syncerTask := &SyncerTask{
		TaskType: Delete,
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
		TaskType: Delete,
		ClusterBaseModel: clusterBaseModel,
	}

	w.syncerChan <- syncerTask
}

func (w *Watcher) matchTargetShape(targetShapes string) bool {
	// matched if target shape not specified
	if len(targetShapes) == 0 {
		return true
	}

	if strings.Contains(targetShapes, w.nodeShape) {
		return true
	}

	return false
}
