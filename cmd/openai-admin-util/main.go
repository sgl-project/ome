package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	watchInterval    time.Duration
	rotationInterval time.Duration
	metricsPort      int
	healthPort       int
	kubeconfigPath   string
	logger           *zap.SugaredLogger
	logLevel         string
	zapEncoder       string
	ready            bool // Track readiness state
	totalOrgs        = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "openai_admin_total_organizations",
		Help: "Total number of OpenAI organizations being monitored",
	})
	keyRotationStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "openai_admin_key_rotation_status",
		Help: "Status of key rotation attempts",
	}, []string{"status"})
	lastKeyRotationTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "openai_admin_last_key_rotation_timestamp",
		Help: "Timestamp of last key rotation attempt",
	}, []string{"org_name", "status"})
)

var rootCmd = &cobra.Command{
	Use:   "openai-admin-util",
	Short: "OpenAI Admin Utility",
	Long: `A CLI tool for managing OpenAI resources and configurations.
Supports various administrative tasks such as API key rotation, organization management, and more.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Configure logger based on log level flag
		var level zapcore.Level
		switch logLevel {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		default:
			level = zapcore.InfoLevel
		}

		// Create a new logger with the configured level
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(level)

		// Set encoder based on flag
		fmt.Printf("Using zap encoder: %s\n", zapEncoder)
		config.Encoding = zapEncoder
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		zapLogger, err := config.Build()
		if err != nil {
			fmt.Printf("Error creating logger: %v\n", err)
			os.Exit(1)
		}

		logger = zapLogger.Sugar().Named("openai-admin-util")
		logger.Info("Starting openai-admin-util")
	},
}

var rotateCmd = &cobra.Command{
	Use:   "rotate-admin-keys",
	Short: "Rotate OpenAI admin API keys",
	Long: `Automatically rotate OpenAI admin API keys for organizations.
Monitors organizations and rotates keys based on age and configuration.`,
	RunE: runRotate,
}

func init() {
	rootCmd.AddCommand(rotateCmd)
	rootCmd.PersistentFlags().StringVar(&zapEncoder, "zap-encoder", "json", "Zap encoder to use (json or console)")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")

	// Add flags to rotate command
	rotateCmd.Flags().DurationVarP(&watchInterval, "watch-interval", "w", 5*time.Minute, "Interval to scan organizations")
	rotateCmd.Flags().DurationVarP(&rotationInterval, "rotation-interval", "r", 30*24*time.Hour, "Interval after which to rotate API keys (e.g., '720h' for 30 days)")
	rotateCmd.Flags().IntVarP(&metricsPort, "metrics-port", "m", 9090, "Port for Prometheus metrics")
	rotateCmd.Flags().IntVarP(&healthPort, "health-port", "p", 8080, "Port for health check endpoints")
	rotateCmd.Flags().StringVarP(&kubeconfigPath, "kubeconfig", "k", "", "Path to kubeconfig file")
}

// initConfig is called after flags are parsed
func initConfig() {
	// This function is intentionally empty but can be used for additional configuration
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		if logger != nil {
			logger.Errorw("Failed to execute command", "error", err)
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func runRotate(cmd *cobra.Command, args []string) error {
	// Initialize Kubernetes clients
	logger.Info("Initializing Kubernetes clients")
	config, err := loadKubeConfig(kubeconfigPath)
	if err != nil {
		logger.Errorw("Error loading kubeconfig", "error", err)
		return fmt.Errorf("error loading kubeconfig: %w", err)
	}

	logger.Info("Creating Kubernetes clientset")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorw("Error creating clientset", "error", err)
		return fmt.Errorf("error creating clientset: %w", err)
	}

	logger.Info("Building scheme")
	scheme, err := v1beta1.SchemeBuilder.Build()
	if err != nil {
		logger.Errorw("Error building scheme", "error", err)
		return fmt.Errorf("error building scheme: %w", err)
	}

	logger.Info("Creating Kubernetes client")
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		logger.Errorw("Error creating client", "error", err)
		return fmt.Errorf("error creating client: %w", err)
	}

	// Initialize metrics
	logger.Info("Registering Prometheus metrics")
	prometheus.MustRegister(totalOrgs)
	prometheus.MustRegister(keyRotationStatus)
	prometheus.MustRegister(lastKeyRotationTime)

	// Create channels for communication
	orgChan := make(chan *v1beta1.Organization, 100)

	// Start health and readiness check endpoints
	logger.Info("Setting up health and metrics endpoints")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthCheck)
	mux.HandleFunc("/readyz", readinessCheck)
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthPort),
		Handler: mux,
	}

	go func() {
		logger.Infow("Starting health server", "port", healthPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw("Error starting health server", "error", err)
		}
	}()

	// Start metrics endpoint
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsPort),
		Handler: promhttp.Handler(),
	}

	go func() {
		logger.Infow("Starting metrics server", "port", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw("Error starting metrics server", "error", err)
		}
	}()

	// Create and start watcher
	logger.Info("Creating organization watcher")
	watcher := NewOrgWatcher(k8sClient, orgChan, logger)
	go watcher.Start(watchInterval)

	// Start the key rotator
	logger.Infow("Creating key rotator", "rotationInterval", rotationInterval)
	rotator := NewKeyRotator(k8sClient, clientset, logger)
	rotator.SetRotationInterval(rotationInterval)

	// Mark as ready once everything is initialized
	logger.Info("Service initialized and ready")
	ready = true

	// Start processing organizations
	logger.Info("Starting to process organizations")
	rotator.Start(orgChan)

	return nil
}

func loadKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		logger.Infow("Loading kubeconfig from path", "path", kubeconfigPath)
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	logger.Info("Loading in-cluster kubeconfig")
	return rest.InClusterConfig()
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		logger.Errorw("Error writing response", "error", err)
	}
}

func readinessCheck(w http.ResponseWriter, r *http.Request) {
	if ready {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ready")); err != nil {
			logger.Errorw("Error writing response", "error", err)
		}
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("not ready")); err != nil {
			logger.Errorw("Error writing response", "error", err)
		}
	}
}
