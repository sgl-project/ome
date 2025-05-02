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

var rootCmd = &cobra.Command{
	Use:              "start",
	Short:            "Starts the model agent",
	Long:             `Starts the model agent to watch the base model custom resources and update the node labels`,
	Run:              runCommand,
	PersistentPreRun: initConfig,
}

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

var cfg config

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

var (
	logLevel       string
	logEncoder     string
	logDevelopment bool
)

func init() {
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

type Logger = zap.SugaredLogger

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

	zapLogger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return zapLogger.Sugar(), nil
}

func setupServer(port int, modelsRootDir string, logger *Logger) *http.Server {
	mux := http.NewServeMux()
	healthz.InstallPathHandler(mux, "/healthz", modelagent.NewModelAgentHealthCheck(modelsRootDir))
	healthz.InstallLivezHandler(mux, healthz.PingHealthz)

	// Register Prometheus metrics handler
	modelagent.RegisterMetricsHandler(mux)
	logger.Info("Registered Prometheus metrics endpoint at /metrics")

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

func runCommand(cmd *cobra.Command, args []string) {
	logger, err := initializeLogger()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	inClusterKubeConfig := getKubeConfig()
	kubeClient := createKubeClient(inClusterKubeConfig)

	omev1beta1ClientSet := createOmeClient(inClusterKubeConfig)
	var omev1beta1InformerFactoryOpts []omev1beta1informers.SharedInformerOption
	omev1beta1InformerFactory := omev1beta1informers.NewSharedInformerFactoryWithOptions(omev1beta1ClientSet, 0, omev1beta1InformerFactoryOpts...)
	baseModelsInformer := omev1beta1InformerFactory.Ome().V1beta1().BaseModels()
	clusterBaseModelsInformer := omev1beta1InformerFactory.Ome().V1beta1().ClusterBaseModels()

	// global stop signal
	stopCh := kubeapiserver.SetupSignalHandler()
	ctx, cancel := context.WithCancel(context.TODO())
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Register Go and process collectors (safely, without panicking if already registered)
	reg := prometheus.DefaultRegisterer
	if err := reg.Register(collectors.NewGoCollector()); err != nil {
		// Ignore "already exists" errors, warn about others
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.Warnf("Error registering Go collector: %v", err)
		}
	}
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.Warnf("Error registering Process collector: %v", err)
		}
	}

	// Initialize metrics
	metrics := modelagent.NewMetrics(nil)
	logger.Info("Initialized Prometheus metrics")

	// create download task communication channel
	gopherTaskChan := make(chan *modelagent.GopherTask)

	// create node labeler
	nodeLabeler := modelagent.NewNodeLabeler(cfg.nodeName, cfg.namespace, kubeClient, cfg.nodeLabelRetry)

	// create scout
	scout, err := modelagent.NewScout(
		cfg.nodeName,
		baseModelsInformer,
		clusterBaseModelsInformer,
		omev1beta1InformerFactory,
		gopherTaskChan,
		kubeClient,
		nodeLabeler,
		logger)
	if err != nil {
		logger.Fatalf("Failed to create scout: %v", err)
	}

	// create gopher
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
		logger.Fatalf("Failed to create gopher: %v", err)
	}

	// setup server for health check and metrics
	server := setupServer(cfg.port, cfg.modelsRootDir, logger)
	go func() {
		logger.Infof("Starting health check server on port %d", cfg.port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("Health check server error: %v", err)
		}
	}()

	// start gopher
	go gopher.Run(stopCh, cfg.numDownloadWorker)

	// start scout
	if err := scout.Run(stopCh); err != nil {
		logger.Fatalf("Error running scout: %v", err)
	}
}

func createKubeClient(kubeConfig *rest.Config) *kubernetes.Clientset {
	return kubernetes.NewForConfigOrDie(kubeConfig)
}

func createOmeClient(kubeConfig *rest.Config) *omev1beta1client.Clientset {
	return omev1beta1client.NewForConfigOrDie(kubeConfig)
}

func getKubeConfig() *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	return config
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
