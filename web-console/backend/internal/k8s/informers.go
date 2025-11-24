package k8s

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// SetupInformers configures informers for all resources with event handlers
func (c *Client) SetupInformers() {
	c.Logger.Info("Setting up informers with event handlers...")

	// Setup ClusterBaseModel informer
	c.setupClusterBaseModelInformer()

	// Setup BaseModel informer
	c.setupBaseModelInformer()

	// Setup ClusterServingRuntime informer
	c.setupClusterServingRuntimeInformer()

	// Setup ServingRuntime informer
	c.setupServingRuntimeInformer()

	// Setup InferenceService informer
	c.setupInferenceServiceInformer()

	// Setup AcceleratorClass informer
	c.setupAcceleratorClassInformer()

	// Setup Namespace informer
	c.setupNamespaceInformer()

	c.Logger.Info("Informers setup completed")
}

// setupClusterBaseModelInformer sets up the ClusterBaseModel informer with event handlers
func (c *Client) setupClusterBaseModelInformer() {
	informer := c.DynamicInformerFactory.ForResource(ClusterBaseModelGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterBaseModel added", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "models",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterBaseModel updated", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "models",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterBaseModel deleted", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "models",
				Name:     u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for ClusterBaseModel", zap.Error(err))
	}
}

// setupBaseModelInformer sets up the BaseModel informer with event handlers
func (c *Client) setupBaseModelInformer() {
	informer := c.DynamicInformerFactory.ForResource(BaseModelGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("BaseModel added", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "models",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("BaseModel updated", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "models",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("BaseModel deleted", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "models",
				Name:     u.GetNamespace() + "/" + u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for BaseModel", zap.Error(err))
	}
}

// setupClusterServingRuntimeInformer sets up the ClusterServingRuntime informer
func (c *Client) setupClusterServingRuntimeInformer() {
	informer := c.DynamicInformerFactory.ForResource(ClusterServingRuntimeGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterServingRuntime added", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "runtimes",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterServingRuntime updated", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "runtimes",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ClusterServingRuntime deleted", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "runtimes",
				Name:     u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for ClusterServingRuntime", zap.Error(err))
	}
}

// setupServingRuntimeInformer sets up the ServingRuntime informer
func (c *Client) setupServingRuntimeInformer() {
	informer := c.DynamicInformerFactory.ForResource(ServingRuntimeGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ServingRuntime added", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "runtimes",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("ServingRuntime updated", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "runtimes",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("ServingRuntime deleted", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "runtimes",
				Name:     u.GetNamespace() + "/" + u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for ServingRuntime", zap.Error(err))
	}
}

// setupInferenceServiceInformer sets up the InferenceService informer
func (c *Client) setupInferenceServiceInformer() {
	informer := c.DynamicInformerFactory.ForResource(InferenceServiceGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("InferenceService added", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "services",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("InferenceService updated", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "services",
				Name:     u.GetNamespace() + "/" + u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("InferenceService deleted", zap.String("namespace", u.GetNamespace()), zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "services",
				Name:     u.GetNamespace() + "/" + u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for InferenceService", zap.Error(err))
	}
}

// setupAcceleratorClassInformer sets up the AcceleratorClass informer
func (c *Client) setupAcceleratorClassInformer() {
	informer := c.DynamicInformerFactory.ForResource(AcceleratorClassGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("AcceleratorClass added", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "add",
				Resource: "accelerators",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.Debug("AcceleratorClass updated", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "update",
				Resource: "accelerators",
				Name:     u.GetName(),
				Data:     u.Object,
			})
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.Debug("AcceleratorClass deleted", zap.String("name", u.GetName()))
			c.Broadcaster.Broadcast(ResourceEvent{
				Type:     "delete",
				Resource: "accelerators",
				Name:     u.GetName(),
			})
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for AcceleratorClass", zap.Error(err))
	}
}

// setupNamespaceInformer sets up the Namespace informer
func (c *Client) setupNamespaceInformer() {
	informer := c.InformerFactory.Core().V1().Namespaces().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Namespace events don't need broadcasting as they're not frequently changed
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Namespace events don't need broadcasting as they're not frequently changed
		},
		DeleteFunc: func(obj interface{}) {
			// Namespace events don't need broadcasting as they're not frequently changed
		},
	})

	if err != nil {
		c.Logger.Error("Failed to add event handler for Namespace", zap.Error(err))
	}
}
