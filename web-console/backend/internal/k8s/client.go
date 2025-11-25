package k8s

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// EventBroadcaster handles real-time event broadcasting
type EventBroadcaster struct {
	mu        sync.RWMutex
	listeners map[chan ResourceEvent]struct{}
}

// ResourceEvent represents a change to a Kubernetes resource
type ResourceEvent struct {
	Type     string      `json:"type"`     // "add", "update", "delete"
	Resource string      `json:"resource"` // "models", "runtimes", "services", etc.
	Name     string      `json:"name"`
	Data     interface{} `json:"data,omitempty"`
}

// Client wraps Kubernetes client functionality with informers
type Client struct {
	Clientset              *kubernetes.Clientset
	DynamicClient          dynamic.Interface
	Config                 *rest.Config
	Logger                 *zap.Logger
	DynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory
	InformerFactory        informers.SharedInformerFactory
	Broadcaster            *EventBroadcaster
	stopCh                 chan struct{}
}

// NewClient creates a new Kubernetes client
func NewClient(logger *zap.Logger) (*Client, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	if os.Getenv("KUBERNETES_IN_CLUSTER") == "true" {
		logger.Info("Using in-cluster Kubernetes configuration")
		config, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig from environment or default location
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			homeDir, _ := os.UserHomeDir()
			kubeconfig = homeDir + "/.kube/config"
		}
		logger.Info("Using kubeconfig file", zap.String("path", kubeconfig))
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, err
	}

	// Create typed clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Create dynamic client for CRDs
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Create informer factories with 30 second resync period
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second)
	informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)

	// Create event broadcaster
	broadcaster := &EventBroadcaster{
		listeners: make(map[chan ResourceEvent]struct{}),
	}

	logger.Info("Kubernetes client initialized successfully")

	return &Client{
		Clientset:              clientset,
		DynamicClient:          dynamicClient,
		Config:                 config,
		Logger:                 logger,
		DynamicInformerFactory: dynamicInformerFactory,
		InformerFactory:        informerFactory,
		Broadcaster:            broadcaster,
		stopCh:                 make(chan struct{}),
	}, nil
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		listeners: make(map[chan ResourceEvent]struct{}),
	}
}

// Subscribe adds a listener to receive events
func (eb *EventBroadcaster) Subscribe() chan ResourceEvent {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan ResourceEvent, 100)
	eb.listeners[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a listener
func (eb *EventBroadcaster) Unsubscribe(ch chan ResourceEvent) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.listeners, ch)
	close(ch)
}

// Broadcast sends an event to all listeners
func (eb *EventBroadcaster) Broadcast(event ResourceEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for ch := range eb.listeners {
		select {
		case ch <- event:
		default:
			// Skip slow consumers
		}
	}
}

// StartInformers starts all informers and waits for cache sync
func (c *Client) StartInformers() error {
	c.Logger.Info("Starting Kubernetes informers...")

	// Start dynamic informer factory for CRDs
	c.DynamicInformerFactory.Start(c.stopCh)

	// Start standard informer factory for core resources
	c.InformerFactory.Start(c.stopCh)

	// Wait for all caches to sync
	c.Logger.Info("Waiting for informer caches to sync...")

	// Sync dynamic informers (CRDs)
	for gvr, informer := range c.DynamicInformerFactory.WaitForCacheSync(c.stopCh) {
		if !informer {
			c.Logger.Error("Failed to sync cache for resource", zap.String("gvr", gvr.String()))
			return fmt.Errorf("failed to sync cache for resource %s", gvr.String())
		}
	}

	// Sync standard informers (core resources)
	synced := c.InformerFactory.WaitForCacheSync(c.stopCh)
	for informerType, isSynced := range synced {
		if !isSynced {
			c.Logger.Error("Failed to sync cache", zap.String("type", informerType.String()))
			return fmt.Errorf("failed to sync cache for type %s", informerType.String())
		}
	}

	c.Logger.Info("All informer caches synced successfully")
	return nil
}

// Stop stops all informers
func (c *Client) Stop() {
	close(c.stopCh)
}
