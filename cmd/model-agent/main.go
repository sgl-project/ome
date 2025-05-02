package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"k8s.io/apiserver/pkg/server/healthz"

	omev1beta1client "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/clientset/versioned"
	omev1beta1informers "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/informers/externalversions"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/modelagent"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	kubeapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// config holds all configuration parameters for the model agent
type config struct {
	port                int
	modelsRootDir       string
	modelsRootDirOnHost string
	nodeName            string
	nodeLabelRetry      int
	downloadRetry       int
	downloadAuthType    string
	numDownloadWorker   int
	namespace           string
}

// Logger type alias for zap.SugaredLogger
type Logger = zap.SugaredLogger

// Global variables
var (
	cfg            config
	logLevel       string
	logEncoder     string
	logDevelopment bool

	rootCmd = &cobra.Command{
		Use:              "start",
		Short:            "Starts the model agent",
		Long:             `Starts the model agent to watch the base model custom resources and update the node labels`,
		Run:              runCommand,
		PersistentPreRun: initConfig,
	}
)

// init sets up command line flags
func init() {
	// Main configuration flags
	rootCmd.Flags().IntVar(&cfg.port, "health-check-port", 8080, "Address for readiness and liveness health check")
	rootCmd.Flags().StringVar(&cfg.modelsRootDirOnHost, "models-root-dir-on-host", "/raid/models", "host's root dir for storing all models")
	rootCmd.Flags().StringVar(&cfg.modelsRootDir, "models-root-dir", "/raid/models", "folder for all models' root dir for the model-agent")
	rootCmd.Flags().IntVar(&cfg.nodeLabelRetry, "node-label-retry", 2, "number of retries for the node labeling operations")
	rootCmd.Flags().IntVar(&cfg.downloadRetry, "download-retry", 3, "number of retries for the model download operations")
	rootCmd.Flags().StringVar(&cfg.downloadAuthType, "download-auth-type", "instance-principal", "authentication method for model download")
	rootCmd.Flags().IntVar(&cfg.numDownloadWorker, "num-download-worker", 3, "number of download workers")
	rootCmd.Flags().StringVar(&cfg.namespace, "namespace", "ome", "the namespace of the ome model agents daemon set")

	// Logger flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "zap-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logEncoder, "zap-encoder", "console", "Log encoder (console, json)")
	rootCmd.PersistentFlags().BoolVar(&logDevelopment, "zap-development", false, "Development mode")
}

// initConfig validates required environment variables
func initConfig(_ *cobra.Command, _ []string) {
	nodeName, ok := os.LookupEnv("NODE_NAME")
	if !ok {
		panic("NODE_NAME environment variable is not set for model-agent")
	}
	if nodeName == "" {
		panic("NODE_NAME environment variable is empty")
	}
	cfg.nodeName = nodeName
}

// initializeLogger creates and configures a zap logger with the specified settings
func initializeLogger() (*Logger, error) {
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", logLevel, err)
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      logDevelopment,
		Encoding:         logEncoder,
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Use a more human-friendly timestamp format for console encoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if logEncoder == "console" {
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger.Sugar(), nil
}

// setupServer configures an HTTP server for health checks and metrics
func setupServer(port int, modelsRootDir string, logger *Logger) *http.Server {
	mux := http.NewServeMux()

	// Add health check endpoint
	healthz.InstallPathHandler(mux, "/healthz", modelagent.NewModelAgentHealthCheck(modelsRootDir))

	// Add liveness check
	healthz.InstallLivezHandler(mux, healthz.PingHealthz)

	// Add metrics endpoint
	modelagent.RegisterMetricsHandler(mux)
	logger.Info("Registered Prometheus metrics endpoint at /metrics")

	logger.Infof("Health check server configured with port %d", port)
	logger.Infof("Health check configured for models root dir: %s", modelsRootDir)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

// setupKubernetesClients creates the Kubernetes and OME clients
func setupKubernetesClients() (*kubernetes.Clientset, *omev1beta1client.Clientset, error) {
	kubeConfig := getKubeConfig()
	kubeClient := createKubeClient(kubeConfig)
	omeClient := createOmeClient(kubeConfig)
	return kubeClient, omeClient, nil
}

// initializePrometheusMetrics sets up Prometheus metrics and registers collectors
func initializePrometheusMetrics(logger *Logger) *modelagent.Metrics {
	// Register Go and process collectors (safely, without panicking if already registered)
	reg := prometheus.DefaultRegisterer

	// Register Go collector
	if err := reg.Register(collectors.NewGoCollector()); err != nil {
		// Ignore "already exists" errors, warn about others
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.Warnf("Error registering Go collector: %v", err)
		}
	}

	// Register Process collector
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.Warnf("Error registering Process collector: %v", err)
		}
	}

	// Initialize metrics
	metrics := modelagent.NewMetrics(nil)
	logger.Info("Initialized Prometheus metrics")
	return metrics
}

// setupInformers initializes the Kubernetes informers for watching resources
func setupInformers(omeClient *omev1beta1client.Clientset) (omev1beta1informers.SharedInformerFactory, error) {
	var omeInformerOpts []omev1beta1informers.SharedInformerOption
	omeInformerFactory := omev1beta1informers.NewSharedInformerFactoryWithOptions(omeClient, 0, omeInformerOpts...)
	return omeInformerFactory, nil
}

// initializeComponents creates and initializes all the model agent components
func initializeComponents(
	kubeClient *kubernetes.Clientset,
	omeInformerFactory omev1beta1informers.SharedInformerFactory,
	metrics *modelagent.Metrics,
	gopherTaskChan chan *modelagent.GopherTask,
	logger *Logger,
) (*modelagent.Scount, *modelagent.Gopher, error) {
	// Get informers
	baseModelsInformer := omeInformerFactory.Ome().V1beta1().BaseModels()
	clusterBaseModelsInformer := omeInformerFactory.Ome().V1beta1().ClusterBaseModels()

	// Create node labeler
	nodeLabeler := modelagent.NewNodeLabeler(cfg.nodeName, cfg.namespace, kubeClient, cfg.nodeLabelRetry)

	// Create scout
	scout, err := modelagent.NewScout(
		cfg.nodeName,
		baseModelsInformer,
		clusterBaseModelsInformer,
		omeInformerFactory,
		gopherTaskChan,
		kubeClient,
		nodeLabeler,
		logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create scout: %w", err)
	}

	// Create gopher
	gopher, err := modelagent.NewGopher(
		cfg.downloadAuthType,
		cfg.downloadRetry,
		cfg.modelsRootDir,
		cfg.modelsRootDirOnHost,
		gopherTaskChan,
		nodeLabeler,
		metrics,
		logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gopher: %w", err)
	}

	return scout, gopher, nil
}

// runCommand is the main entry point executed by Cobra
func runCommand(cmd *cobra.Command, args []string) {
	// Initialize logger
	logger, err := initializeLogger()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Setup Kubernetes clients
	kubeClient, omeClient, err := setupKubernetesClients()
	if err != nil {
		logger.Fatalf("Failed to setup Kubernetes clients: %v", err)
	}

	// Setup informers
	omeInformerFactory, err := setupInformers(omeClient)
	if err != nil {
		logger.Fatalf("Failed to setup informers: %v", err)
	}

	// Setup metrics
	metrics := initializePrometheusMetrics(logger)

	// Setup signal handling
	stopCh := kubeapiserver.SetupSignalHandler()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Create a download task communication channel
	gopherTaskChan := make(chan *modelagent.GopherTask)

	// Initialize components
	scout, gopher, err := initializeComponents(
		kubeClient,
		omeInformerFactory,
		metrics,
		gopherTaskChan,
		logger,
	)
	if err != nil {
		logger.Fatalf("Failed to initialize components: %v", err)
	}

	// Set up a health check server
	server := setupServer(cfg.port, cfg.modelsRootDir, logger)
	go func() {
		logger.Infof("Starting health check server on port %d", cfg.port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("Health check server error: %v", err)
		}
	}()

	// Start gopher (download workers)
	go gopher.Run(stopCh, cfg.numDownloadWorker)

	// Start scout (watchers)
	if err := scout.Run(stopCh); err != nil {
		logger.Fatalf("Error running scout: %v", err)
	}
}

// createKubeClient creates a Kubernetes client from the provided config
func createKubeClient(kubeConfig *rest.Config) *kubernetes.Clientset {
	return kubernetes.NewForConfigOrDie(kubeConfig)
}

// createOmeClient creates an OME client from the provided config
func createOmeClient(kubeConfig *rest.Config) *omev1beta1client.Clientset {
	return omev1beta1client.NewForConfigOrDie(kubeConfig)
}

// getKubeConfig creates and returns a Kubernetes REST config
func getKubeConfig() *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to create in-cluster Kubernetes config: %v", err))
	}
	return config
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
