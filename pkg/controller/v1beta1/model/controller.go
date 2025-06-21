package model

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"

	omev1beta1client "github.com/sgl-project/ome/pkg/client/clientset/versioned"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/modelagent"
	"github.com/sgl-project/ome/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type ModelController struct {
	agentNamespace  string
	kubeClient      kubernetes.Interface
	omeClient       omev1beta1client.Interface
	configMapLister corelisters.ConfigMapLister
	configMapSynced cache.InformerSynced
	nodeLister      corelisters.NodeLister
	nodeSynced      cache.InformerSynced
	logger          *zap.SugaredLogger
}

func NewModelController(
	agentNamespace string,
	kubeClient kubernetes.Interface,
	omeClient omev1beta1client.Interface,
	nodeInformer coreinformers.NodeInformer,
	configMapInformer coreinformers.ConfigMapInformer,
	logger *zap.SugaredLogger) (*ModelController, error) {
	controller := &ModelController{
		agentNamespace:  agentNamespace,
		kubeClient:      kubeClient,
		omeClient:       omeClient,
		configMapLister: configMapInformer.Lister(),
		configMapSynced: configMapInformer.Informer().HasSynced,
		nodeLister:      nodeInformer.Lister(),
		nodeSynced:      nodeInformer.Informer().HasSynced,
		logger:          logger,
	}

	informers := map[string]cache.SharedInformer{
		"configMapInformer": configMapInformer.Informer(),
		"nodeInformer":      nodeInformer.Informer(),
	}

	for name, informer := range informers {
		err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
			// Pipe to default handler first, which just logs the error
			cache.DefaultWatchErrorHandler(r, err)

			if errors.IsUnauthorized(err) || errors.IsForbidden(err) {
				logger.Fatalf("Unable to sync cache for informer %s: %s. Requesting controller to exit.", name, err)
			}
		})

		if err != nil {
			return nil, fmt.Errorf("unable to set error handler for informer %s: %s", name, err)
		}
	}

	if _, err := configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleModelStatus,
		UpdateFunc: controller.handleModelStatusUpdate,
		DeleteFunc: controller.handleModelStatusDelete,
	}); err != nil {
		return nil, err
	}

	if _, err := nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.handleNodeDelete,
	}); err != nil {
		return nil, err
	}

	return controller, nil
}

func (c *ModelController) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	c.logger.Info("Starting Model controller")
	c.logger.Info("Waiting for informer caches to sync")

	synced := []cache.InformerSynced{
		c.configMapSynced,
		c.nodeSynced,
	}

	if ok := cache.WaitForCacheSync(stopCh, synced...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	<-stopCh
	c.logger.Info("Shutting down ome-model-controller")

	return nil
}

func (c *ModelController) handleModelStatus(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		c.logger.Errorf("Failed to convert %v to ConfigMap", obj)
		return
	}

	if _, ok := configMap.ObjectMeta.Labels[constants.ModelStatusConfigMapLabel]; !ok {
		return
	}

	// In case configMap get updated during the node deletion
	_, err := c.kubeClient.CoreV1().Nodes().Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.logger.Warnf("Node %s has been deleted, but got an associated configMap update.", configMap.Name)
			c.handleModelStatusDelete(obj)
		} else {
			c.logger.Warnf("Error getting node %s: %s", configMap.Name, err.Error())
		}
		return
	}

	nodeName := configMap.Name
	for modelNsName, data := range configMap.Data {
		c.logger.Infof("Processing ConfigMap data for model key %s", modelNsName)
		// Parse the ModelEntry from the ConfigMap data
		modelEntry, err := parseModelEntry(data)
		if err != nil {
			c.logger.Errorf("Failed to parse model entry for %s: %v", modelNsName, err)
			continue
		}

		// Get the status from the ModelEntry
		state := string(modelEntry.Status)
		if modelEntry.Config != nil {
			c.logger.Infof("Processing model %s with status %s and configuration data", modelNsName, state)
		} else {
			c.logger.Infof("Processing model %s with status %s (no configuration data)", modelNsName, state)
		}

		var isClusterBaseModel bool = true
		var nsName string
		var modelName string
		if strings.Contains(modelNsName, "_") {
			isClusterBaseModel = false
			splits := strings.Split(modelNsName, "_")
			if len(splits) < 2 {
				c.logger.Errorf("Failed to parse the name and namespace of the model: %s", modelNsName)
				continue
			}
			nsName = splits[0]
			modelName = splits[1]
		} else {
			modelName = modelNsName
		}

		if isClusterBaseModel {
			err := utils.Retry(3, 100*time.Millisecond, func() error {
				return c.updateClusterBaseModelState(modelName, nodeName, state, modelEntry.Config)
			})
			if err != nil {
				c.logger.Errorf("Failed to update the state of the clusterBaseModel: %s, error: %s", modelName, err.Error())
			}
		} else {
			err := utils.Retry(3, 100*time.Millisecond, func() error {
				return c.updateBaseModelState(modelName, nsName, nodeName, state, modelEntry.Config)
			})
			if err != nil {
				c.logger.Errorf("Failed to update the state of the BaseModel %s in namespace %s, error: %s", modelName, nsName, err.Error())
			}
		}
	}
}

func (c *ModelController) handleModelStatusUpdate(old, new interface{}) {
	c.handleModelStatus(new)
}

func (c *ModelController) handleModelStatusDelete(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		c.logger.Errorf("Failed to convert %v to ConfigMap", obj)
		return
	}

	if _, ok := configMap.ObjectMeta.Labels[constants.ModelStatusConfigMapLabel]; !ok {
		return
	}

	nodeName := configMap.Name
	for modelNsName := range configMap.Data {
		var isClusterBaseModel bool = true
		var nsName string
		var modelName string
		if strings.Contains(modelNsName, "_") {
			isClusterBaseModel = false
			splits := strings.Split(modelNsName, "_")
			if len(splits) < 2 {
				c.logger.Errorf("Failed to parse the name and namespace of the model: %s", modelNsName)
			}
			nsName = splits[0]
			modelName = splits[1]
		} else {
			modelName = modelNsName
		}

		// For deletion, we don't need to pass config as it's not relevant
		if isClusterBaseModel {
			err := utils.Retry(3, 100*time.Millisecond, func() error {
				return c.updateClusterBaseModelState(modelName, nodeName, string(modelagent.Deleted), nil)
			})
			if err != nil {
				c.logger.Errorf("Failed to update the state of the clusterBaseModel: %s, error: %s", modelName, err.Error())
			}
		} else {
			err := utils.Retry(3, 100*time.Millisecond, func() error {
				return c.updateBaseModelState(modelName, nsName, nodeName, string(modelagent.Deleted), nil)
			})
			if err != nil {
				c.logger.Errorf("Failed to update the state of the BaseModel %s in namespace %s, error: %s", modelName, nsName, err.Error())
			}
		}
	}
}

func (c *ModelController) handleNodeDelete(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		c.logger.Errorf("Failed to convert %v to node", obj)
		return
	}

	err := utils.Retry(3, 100*time.Millisecond, func() error {
		configMap, err := c.kubeClient.CoreV1().ConfigMaps(c.agentNamespace).Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if _, ok := configMap.ObjectMeta.Labels[constants.ModelStatusConfigMapLabel]; !ok {
			return nil
		}

		return c.kubeClient.CoreV1().ConfigMaps(c.agentNamespace).Delete(context.TODO(), node.Name, metav1.DeleteOptions{})
	})
	if err != nil {
		c.logger.Errorf("Error deleting the configMap %s in namespace %s, error: %s", node.Name, c.agentNamespace, err.Error())
	}
}

func (c *ModelController) updateClusterBaseModelState(name, nodeName, state string, config *modelagent.ModelConfig) error {
	queriedModel, err := c.omeClient.OmeV1beta1().ClusterBaseModels().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	model := queriedModel.DeepCopy()
	if !model.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	if model.Status.NodesReady == nil {
		model.Status.NodesReady = make([]string, 0)
	}

	if model.Status.NodesFailed == nil {
		model.Status.NodesFailed = make([]string, 0)
	}

	model.Status.NodesReady = removeFromSlice(model.Status.NodesReady, nodeName)
	model.Status.NodesFailed = removeFromSlice(model.Status.NodesFailed, nodeName)

	if state == string(modelagent.Ready) {
		nodesReady := addToSlice(model.Status.NodesReady, nodeName)
		slices.Sort(nodesReady)
		model.Status.NodesReady = nodesReady
	}

	if state == string(modelagent.Failed) {
		nodesFailed := addToSlice(model.Status.NodesFailed, nodeName)
		slices.Sort(nodesFailed)
		model.Status.NodesFailed = nodesFailed
	}

	if len(model.Status.NodesReady) > 0 {
		model.Status.State = v1beta1.LifeCycleStateReady
	} else if len(model.Status.NodesReady) == 0 && len(model.Status.NodesFailed) > 0 {
		model.Status.State = v1beta1.LifeCycleStateFailed
	} else {
		model.Status.State = v1beta1.LifeCycleStateInTransit
	}

	// If we have configuration data, update the model spec
	if config != nil {
		c.logger.Debugf("Updating ClusterBaseModel %s with configuration data", name)
		updateModelWithConfig(model, config)

		// Update the spec first if we changed it
		_, err = c.omeClient.OmeV1beta1().ClusterBaseModels().Update(context.TODO(), model, metav1.UpdateOptions{})
		if err != nil {
			c.logger.Warnf("Failed to update spec for ClusterBaseModel %s: %v", name, err)
			// Continue with status update even if spec update fails
			// Get the latest version again
			model, err = c.omeClient.OmeV1beta1().ClusterBaseModels().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			// Reapply the status changes
			model.Status.NodesReady = removeFromSlice(model.Status.NodesReady, nodeName)
			model.Status.NodesFailed = removeFromSlice(model.Status.NodesFailed, nodeName)

			if state == string(modelagent.Ready) {
				nodesReady := addToSlice(model.Status.NodesReady, nodeName)
				slices.Sort(nodesReady)
				model.Status.NodesReady = nodesReady
			}

			if state == string(modelagent.Failed) {
				nodesFailed := addToSlice(model.Status.NodesFailed, nodeName)
				slices.Sort(nodesFailed)
				model.Status.NodesFailed = nodesFailed
			}

			if len(model.Status.NodesReady) > 0 {
				model.Status.State = v1beta1.LifeCycleStateReady
			} else if len(model.Status.NodesReady) == 0 && len(model.Status.NodesFailed) > 0 {
				model.Status.State = v1beta1.LifeCycleStateFailed
			} else {
				model.Status.State = v1beta1.LifeCycleStateInTransit
			}
		}
	}

	// Update the status
	_, err = c.omeClient.OmeV1beta1().ClusterBaseModels().UpdateStatus(context.TODO(), model, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *ModelController) updateBaseModelState(name, namespace, nodeName, state string, config *modelagent.ModelConfig) error {
	queriedModel, err := c.omeClient.OmeV1beta1().BaseModels(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	model := queriedModel.DeepCopy()
	if !model.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	if model.Status.NodesReady == nil {
		model.Status.NodesReady = make([]string, 0)
	}

	if model.Status.NodesFailed == nil {
		model.Status.NodesFailed = make([]string, 0)
	}

	model.Status.NodesReady = removeFromSlice(model.Status.NodesReady, nodeName)
	model.Status.NodesFailed = removeFromSlice(model.Status.NodesFailed, nodeName)

	if state == string(modelagent.Ready) {
		nodesReady := addToSlice(model.Status.NodesReady, nodeName)
		slices.Sort(nodesReady)
		model.Status.NodesReady = nodesReady
	}

	if state == string(modelagent.Failed) {
		nodesFailed := addToSlice(model.Status.NodesFailed, nodeName)
		slices.Sort(nodesFailed)
		model.Status.NodesFailed = nodesFailed
	}

	if len(model.Status.NodesReady) > 0 {
		model.Status.State = v1beta1.LifeCycleStateReady
	} else if len(model.Status.NodesReady) == 0 && len(model.Status.NodesFailed) > 0 {
		model.Status.State = v1beta1.LifeCycleStateFailed
	} else {
		model.Status.State = v1beta1.LifeCycleStateInTransit
	}

	// If we have configuration data, update the model spec
	if config != nil {
		c.logger.Debugf("Updating BaseModel %s/%s with configuration data", namespace, name)
		updateModelWithConfig(model, config)

		// Update the spec first if we changed it
		_, err = c.omeClient.OmeV1beta1().BaseModels(namespace).Update(context.TODO(), model, metav1.UpdateOptions{})
		if err != nil {
			c.logger.Warnf("Failed to update spec for BaseModel %s/%s: %v", namespace, name, err)
			// Continue with status update even if spec update fails
			// Get the latest version again
			model, err = c.omeClient.OmeV1beta1().BaseModels(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			// Reapply the status changes
			model.Status.NodesReady = removeFromSlice(model.Status.NodesReady, nodeName)
			model.Status.NodesFailed = removeFromSlice(model.Status.NodesFailed, nodeName)

			if state == string(modelagent.Ready) {
				nodesReady := addToSlice(model.Status.NodesReady, nodeName)
				slices.Sort(nodesReady)
				model.Status.NodesReady = nodesReady
			}

			if state == string(modelagent.Failed) {
				nodesFailed := addToSlice(model.Status.NodesFailed, nodeName)
				slices.Sort(nodesFailed)
				model.Status.NodesFailed = nodesFailed
			}

			if len(model.Status.NodesReady) > 0 {
				model.Status.State = v1beta1.LifeCycleStateReady
			} else if len(model.Status.NodesReady) == 0 && len(model.Status.NodesFailed) > 0 {
				model.Status.State = v1beta1.LifeCycleStateFailed
			} else {
				model.Status.State = v1beta1.LifeCycleStateInTransit
			}
		}
	}

	// Update the status
	_, err = c.omeClient.OmeV1beta1().BaseModels(namespace).UpdateStatus(context.TODO(), model, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func removeFromSlice(s []string, t string) []string {
	var index int = -1
	for i, e := range s {
		if e == t {
			index = i
			break
		}
	}

	if index == -1 {
		return s
	}

	return append(s[:index], s[index+1:]...)
}

func addToSlice(s []string, t string) []string {
	for _, e := range s {
		if e == t {
			return s
		}
	}

	return append(s, t)
}
