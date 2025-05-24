package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	omev1beta1client "github.com/sgl-project/sgl-ome/pkg/client/clientset/versioned"
	modelcontroller "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/model"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	election "k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var (
	// leader election config
	leaseDuration = 15 * time.Second
	renewDuration = 5 * time.Second
	retryPeriod   = 3 * time.Second
	// leader election health check
	healthCheckPort = 8080
	// This is the timeout that determines the time beyond the lease expiry to be
	// allowed for timeout. Checks within the timeout period after the lease
	// expires will still return healthy.
	leaderHealthzAdaptorTimeout = time.Second * 20

	// logging config
	logLevel       string
	logEncoder     string
	logDevelopment bool

	// controller config
	namespace      string
	controllerName string
	agentNamespace string
)

var rootCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts model controller",
	Long:  `Starts the model controller to watch and updates all the baseModels`,
	Run:   runCommand,
}

type Logger = zap.SugaredLogger

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "zap-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logEncoder, "zap-encoder", "console", "Log encoder (console, json)")
	rootCmd.PersistentFlags().BoolVar(&logDevelopment, "zap-development", false, "Development mode")

	rootCmd.Flags().StringVar(&namespace, "namespace", "ome", "namespace to create the leader election lock")
	rootCmd.Flags().StringVar(&controllerName, "controller-name", "ome-model-controller", "the name of this controller")
	rootCmd.Flags().StringVar(&agentNamespace, "agent-namespace", "ome", "the namespace of the model agents")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runCommand(cmd *cobra.Command, args []string) {
	logger, err := initializeLogger()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cfg := ctrl.GetConfigOrDie()
	kubeClient := createKubeClient(cfg)
	omev1beta1ClientSet := createOmeClient(cfg)

	if !checkCRDExists(omev1beta1ClientSet, logger) {
		logger.Info("CRD doesn't exist. Exiting")
		os.Exit(1)
	}

	stopCh := kubeapiserver.SetupSignalHandler()
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	run := func(ctx context.Context) {
		var kubeInformerFactoryOpts []kubeinformers.SharedInformerOption
		kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeInformerFactoryOpts...)

		var namespacedkubeInformerFactoryOpts []kubeinformers.SharedInformerOption
		namespacedkubeInformerFactoryOpts = append(namespacedkubeInformerFactoryOpts, kubeinformers.WithNamespace(agentNamespace))
		namespacedkubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, namespacedkubeInformerFactoryOpts...)

		controller, _ := modelcontroller.NewModelController(
			agentNamespace,
			kubeClient,
			omev1beta1ClientSet,
			kubeInformerFactory.Core().V1().Nodes(),
			namespacedkubeInformerFactory.Core().V1().ConfigMaps(),
			logger,
		)

		go kubeInformerFactory.Start(ctx.Done())
		go namespacedkubeInformerFactory.Start(ctx.Done())

		if err = controller.Run(stopCh); err != nil {
			logger.Fatalf("Error running controller: %s", err.Error())
		}
	}

	// Set up leader election
	id, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %v", err))
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	var electionChecker = election.NewLeaderHealthzAdaptor(leaderHealthzAdaptorTimeout)

	mux := http.NewServeMux()
	healthz.InstallPathHandler(mux, "/healthz", electionChecker)
	healthz.InstallReadyzHandler(mux, healthz.PingHealthz)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthCheckPort),
		Handler: mux,
	}

	go func() {
		logger.Infof("Start listening to %d for health check", healthCheckPort)

		if err := server.ListenAndServe(); err != nil {
			logger.Fatalf("Error starting server for health check: %v", err)
		}
	}()

	rl := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      controllerName,
		},
		Client: kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	// Start leader election.
	election.RunOrDie(ctx, election.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDuration,
		RetryPeriod:   retryPeriod,
		Callbacks: election.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logger.Infof("Leading started")
				run(ctx)
			},
			OnStoppedLeading: func() {
				logger.Fatalf("Leader election stopped")
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				logger.Infof("New leader has been elected: %s", identity)
			},
		},
		Name:     controllerName,
		WatchDog: electionChecker,
	})

	logger.Fatalf("finished without leader elect")
}

func initializeLogger() (*Logger, error) {
	level, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	config := zap.Config{
		Level:            level,
		Development:      logDevelopment,
		Encoding:         logEncoder,
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Use more human-friendly timestamp format for console encoder
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
func getKubeConfig() *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	return config
}

func checkCRDExists(client omev1beta1client.Interface, logger *Logger) bool {
	_, err := client.OmeV1beta1().BaseModels("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("BaseModel CRD not found")
			return false
		}
		logger.Errorf("Error checking CRD: %v", err)
		return false
	}
	return true
}

// createKubeClient creates a Kubernetes client from the provided config
func createKubeClient(kubeConfig *rest.Config) *kubernetes.Clientset {
	return kubernetes.NewForConfigOrDie(kubeConfig)
}

// createOmeClient creates an OME client from the provided config
func createOmeClient(kubeConfig *rest.Config) *omev1beta1client.Clientset {
	return omev1beta1client.NewForConfigOrDie(kubeConfig)
}
