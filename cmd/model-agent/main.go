package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	kubeapiserver "k8s.io/apiserver/pkg/server"
	ctrl "sigs.k8s.io/controller-runtime"

	omev1beta1client "github.com/sgl-project/sgl-ome/pkg/client/clientset/versioned"
	omev1beta1informers "github.com/sgl-project/sgl-ome/pkg/client/informers/externalversions"
	"github.com/sgl-project/sgl-ome/pkg/hfutil/hub"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/modelagent"
	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
	"github.com/sgl-project/sgl-ome/pkg/principals"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// config holds all configuration parameters for the model agent
type config struct {
	port                 int
	modelsRootDir        string
	modelsRootDirOnHost  string
	nodeName             string
	nodeLabelRetry       int
	concurrency          int
	multipartConcurrency int
	downloadRetry        int
	downloadAuthType     string
	numDownloadWorker    int
	namespace            string
	logLevel             string
}

// Logger type alias for zap.SugaredLogger
type Logger = zap.SugaredLogger

// Global variables
var (
	rootCmd = &cobra.Command{
		Use:              "model-agent",
		Short:            "Model agent for Open Model Engine",
		Run:              runCommand,
		PersistentPreRun: initConfig,
	}
	cfg = &config{}
	v   = viper.New() // Global viper instance for configuration
)

// init sets up command line flags and binds them to Viper
func init() {
	// Define command-line flags
	rootCmd.PersistentFlags().IntVar(&cfg.port, "port", 8080, "HTTP port for health checks")
	rootCmd.PersistentFlags().StringVar(&cfg.modelsRootDir, "models-root-dir", "/mnt/models", "Root directory for models")
	rootCmd.PersistentFlags().StringVar(&cfg.nodeName, "node-name", "", "Name of the node where agent is running")
	rootCmd.PersistentFlags().IntVar(&cfg.nodeLabelRetry, "node-label-retry", 5, "Number of retries for node labeling")
	rootCmd.PersistentFlags().IntVar(&cfg.downloadRetry, "download-retry", 3, "Number of retries for downloading")
	rootCmd.PersistentFlags().IntVar(&cfg.concurrency, "concurrency", 4, "Number of concurrent download workers per gopher")
	rootCmd.PersistentFlags().IntVar(&cfg.multipartConcurrency, "multipart-concurrency", 4, "Number of concurrent multipart download workers per gopher")
	rootCmd.PersistentFlags().StringVar(&cfg.downloadAuthType, "download-auth-type", "InstancePrincipal", "Auth type for downloading models")
	rootCmd.PersistentFlags().IntVar(&cfg.numDownloadWorker, "num-download-worker", 5, "Number of download workers")
	rootCmd.PersistentFlags().StringVar(&cfg.namespace, "namespace", "ome", "Kubernetes namespace to use")
	rootCmd.PersistentFlags().StringVar(&cfg.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	_ = v.BindPFlags(rootCmd.PersistentFlags())

	v.AutomaticEnv()
	// Finally, add any explicit bindings
	_ = v.BindEnv("region", "REGION") // Note: use lowercase keys for consistency
	_ = v.BindEnv("compartment_id", "COMPARTMENT_ID")
	_ = v.BindEnv("realm", "REALM")
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
	level, err := zapcore.ParseLevel(v.GetString("log-level"))
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", "info", err)
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Use a more human-friendly timestamp format for console encoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if config.Encoding == "console" {
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
	cfg := ctrl.GetConfigOrDie()
	kubeClient := createKubeClient(cfg)
	omeClient := createOmeClient(cfg)
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
	omeClient *omev1beta1client.Clientset,
	omeInformerFactory omev1beta1informers.SharedInformerFactory,
	metrics *modelagent.Metrics,
	gopherTaskChan chan *modelagent.GopherTask,
	logger *Logger,
) (*modelagent.Scount, *modelagent.Gopher, error) {
	// Create node labeler for labeling the node based on model status
	nodeLabeler := modelagent.NewNodeLabeler(cfg.nodeName, cfg.namespace, kubeClient, cfg.nodeLabelRetry, logger)

	// Set up an authentication type from Viper
	authType := principals.AuthenticationType(v.GetString("download-auth-type"))

	// Convert sugared logger back to a regular zap logger to use ForZap
	zapLogger := logger.Desugar()

	// Create Casper config with a proper logger adapter
	casperConfig, err := ociobjectstore.NewConfig(
		ociobjectstore.WithAnotherLog(logging.ForZap(zapLogger)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ociobjectstore config: %w", err)
	}

	// Set auth type (needs to be a pointer)
	casperConfig.AuthType = &authType

	// Create OCIOSDataStore
	casperDS, err := ociobjectstore.NewOCIOSDataStore(casperConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ociobjectstore data store: %w", err)
	}

	// Create a ModelConfigParser instance
	modelConfigParser := modelagent.NewModelConfigParser(omeClient, logger)

	// Create a ModelConfigUpdater instance
	modelConfigUpdater := modelagent.NewModelConfigUpdater(cfg.nodeName, cfg.namespace, kubeClient, logger)

	// Create a Scout instance
	baseModelInformer := omeInformerFactory.Ome().V1beta1().BaseModels()
	clusterBaseModelInformer := omeInformerFactory.Ome().V1beta1().ClusterBaseModels()

	scout, err := modelagent.NewScout(
		cfg.nodeName,
		baseModelInformer,
		clusterBaseModelInformer,
		omeInformerFactory,
		gopherTaskChan,
		kubeClient,
		nodeLabeler,
		logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create scout: %w", err)
	}

	// Create default Hugging Face hub config
	hfHubConfig, err := hub.NewHubConfig(
		hub.WithLogger(logging.ForZap(zapLogger)),
		hub.WithViper(v),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HuggingFace hub config: %w", err)
	}

	// Create hub client
	hfHubClient, err := hub.NewHubClient(hfHubConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HuggingFace hub client: %w", err)
	}

	logger.Infof("Configured Hugging Face hub client with max retries: %d, max workers: %d",
		hfHubConfig.MaxRetries, hfHubConfig.MaxWorkers)

	// Create a Gopher instance for downloading models
	gopher, err := modelagent.NewGopher(
		modelConfigParser,
		modelConfigUpdater,
		casperDS, // Pass the ociobjectstore data store directly
		hfHubClient,
		kubeClient, // Pass the Kubernetes client for secret access
		cfg.concurrency,
		cfg.multipartConcurrency,
		cfg.downloadRetry,
		cfg.modelsRootDir,
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
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Log all Viper config at startup for traceability
	logger.Infow("Model Agent configuration (Viper)", "allSettings", v.AllSettings())

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
		omeClient,
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
