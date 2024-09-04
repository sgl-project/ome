package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	omev1beta1client "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/clientset/versioned"
	omev1beta1informers "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client/informers/externalversions"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	modelagent "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/model-agent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var rootCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the model agent",
	Long:  `Starts the model agent to watch the basemodel custom resources and update the node labels`,
	Run:   rumCommand,
}

var (
	healthCheckPort     int
	modelsRootDir       string
	modelsRootDirOnHost string
	nodeName            string
	nodelabelRetry      int
	downloadRetry       int
	downloadAuthType    string
	numDownloadWorker   int
	namespace           string
)

type Logger = zap.SugaredLogger

func initializeLogger() *Logger {
	zaplogger, _ := zap.NewProduction()
	return zaplogger.Sugar()
}

func init() {
	rootCmd.Flags().IntVar(&healthCheckPort, "health-check-port", 8080, "Address for readiness and liveness health check")
	rootCmd.Flags().StringVar(&modelsRootDirOnHost, "models-root-dir-on-host", "/raid/models", "host's root dir for storing all models")
	rootCmd.Flags().StringVar(&modelsRootDir, "models-root-dir", "/raid/models", "folder for all models' root dir for the model-agent")
	rootCmd.Flags().IntVar(&nodelabelRetry, "node-label-retry", 2, "number of retries for the node labeling operations")
	rootCmd.Flags().IntVar(&downloadRetry, "download-retry", 3, "number of retries for the model download operations")
	rootCmd.Flags().StringVar(&downloadAuthType, "download-auth-type", "instance-principal", "authentication method for model download")
	rootCmd.Flags().IntVar(&numDownloadWorker, "num-download-worker", 3, "number of download workers")
	rootCmd.Flags().StringVar(&namespace, "namespace", "ome", "the namespace of the ome model agents daemonset")
	if nName, ok := os.LookupEnv("NODE_NAME"); !ok {
		panic(fmt.Errorf("NODE_NAME is not set for model-agent"))
	} else {
		if len(nName) == 0 {
			panic(fmt.Errorf("NODE_NAME is set as empty"))
		} else {
			nodeName = nName
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func rumCommand(cmd *cobra.Command, args []string) {
	logger := initializeLogger()
	inclusterKubeConfig := getKubeConfig()
	kubeClient := createKubeClient(inclusterKubeConfig)
	nodeShape := getNodeShape(kubeClient, nodeName)

	omev1beta1ClientSet := createOmeClient(inclusterKubeConfig)
	var omev1beta1InformerFactoryOpts []omev1beta1informers.SharedInformerOption
	omev1beta1informerFactory := omev1beta1informers.NewSharedInformerFactoryWithOptions(omev1beta1ClientSet, 0, omev1beta1InformerFactoryOpts...)
	baseModelsInformer := omev1beta1informerFactory.Ome().V1beta1().BaseModels()
	clusterBaseModelsInformer := omev1beta1informerFactory.Ome().V1beta1().ClusterBaseModels()

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

	// create download task communication channel
	syncerTaskChan := make(chan *modelagent.SyncerTask)

	// create node labeler
	nodeLabler := modelagent.NewNodeLabler(nodeName, namespace, kubeClient, nodelabelRetry)

	// create watcher
	watcher, err := modelagent.NewWatcher(
		nodeShape,
		nodeName,
		baseModelsInformer,
		clusterBaseModelsInformer,
		omev1beta1informerFactory,
		syncerTaskChan,
		kubeClient,
		nodeLabler,
		logger)
	if err != nil {
		panic(fmt.Errorf("error creating watcher: %s", err.Error()))
	}

	// create syncer
	syncer, err := modelagent.NewSyncer(
		downloadAuthType,
		downloadRetry,
		modelsRootDir,
		modelsRootDirOnHost,
		syncerTaskChan,
		nodeLabler,
		logger)
	if err != nil {
		panic(fmt.Errorf("error creating syncer: %s", err.Error()))
	}

	// create health checker
	mux := http.NewServeMux()
	healthz.InstallPathHandler(mux, "/healthz", modelagent.NewModelAgentHealthCheck(modelsRootDir))
	healthz.InstallLivezHandler(mux, healthz.PingHealthz)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthCheckPort),
		Handler: mux,
	}

	// start syncer
	go syncer.Run(stopCh, numDownloadWorker)

	// start health checker
	go func() {
		logger.Infof("Start listening to %d for health check", healthCheckPort)

		if err := server.ListenAndServe(); err != nil {
			logger.Infof("Error starting server for health check: %v", err)
		}
	}()

	// start watcher
	if err := watcher.Run(stopCh); err != nil {
		logger.Fatalf("Error running watcher: %s", err.Error())
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

func getNodeShape(client *kubernetes.Clientset, nodeName string) string {
	opts := metav1.GetOptions{}
	node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeName, opts)
	if err != nil {
		panic(err.Error())
	}

	if nodeShape, ok := node.ObjectMeta.Labels[constants.NodeInstanceShapeLabel]; ok {
		if len(nodeShape) > 0 {
			return nodeShape
		} else {
			panic(fmt.Errorf("%s label is empty for node %s", constants.NodeInstanceShapeLabel, nodeName))
		}
	} else {
		panic(fmt.Errorf("%s label not found for node %s", constants.NodeInstanceShapeLabel, nodeName))
	}
}
