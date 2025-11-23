package k8s

import (
	"os"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes client functionality
type Client struct {
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
	Config        *rest.Config
	Logger        *zap.Logger
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

	logger.Info("Kubernetes client initialized successfully")

	return &Client{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		Config:        config,
		Logger:        logger,
	}, nil
}
