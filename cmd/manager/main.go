package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	v1beta1benchmarkjobcontroller "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/benchmark"
	v1beta1dacccontroller "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/dac"
	v1beta1isvccontroller "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/webhook/admission/pod"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/webhook/admission/servingruntime"
	"flag"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	istionetworking "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	volcanobatch "volcano.sh/apis/pkg/apis/batch/v1alpha1"
	volcano "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

var (
	scheme   = runtime.NewScheme() //nolint: unused
	setupLog = ctrl.Log.WithName("setup")
)

const (
	LeaderLockName          = "ome-controller-manager-leader-lock"
	LeaderElectionNamespace = "ome"
)

// Options defines the program-configurable options that may be passed on the command line.
type Options struct {
	metricsAddr             string
	webhookPort             int
	enableLeaderElection    bool
	enableWebhook           bool
	probeAddr               string
	leaderElectionNamespace string
	zapOpts                 zap.Options
}

// DefaultOptions returns the default values for the program options.
func DefaultOptions() Options {
	return Options{
		metricsAddr:             ":8080",
		webhookPort:             9443,
		enableLeaderElection:    false,
		enableWebhook:           false,
		probeAddr:               ":8081",
		leaderElectionNamespace: LeaderElectionNamespace,
		zapOpts:                 zap.Options{},
	}
}

// GetOptions parses the program flags and returns them as Options.
func GetOptions() Options {
	opts := DefaultOptions()
	flag.StringVar(&opts.metricsAddr, "metrics-addr", opts.metricsAddr, "The address the metric endpoint binds to.")
	flag.IntVar(&opts.webhookPort, "webhook-port", opts.webhookPort, "The port that the webhook server binds to.")
	flag.BoolVar(&opts.enableLeaderElection, "leader-elect", opts.enableLeaderElection,
		"Enable leader election for ome controller manager. "+
			"Enabling this will ensure there is only one active ome controller manager.")
	flag.StringVar(&opts.leaderElectionNamespace, "leader-election-namespace", opts.leaderElectionNamespace, "The namespace in which the leader election configmap will be created.")
	flag.BoolVar(&opts.enableWebhook, "webhook", opts.enableWebhook, "Enable the webhook server.")
	flag.StringVar(&opts.probeAddr, "health-probe-addr", opts.probeAddr, "The address the probe endpoint binds to.")
	opts.zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()
	return opts
}

func init() {
	// Allow unknown fields in Istio API client for backwards compatibility if cluster has existing vs with deprecated fields.
	istionetworking.VirtualServiceUnmarshaler.AllowUnknownFields = true
	istionetworking.GatewayUnmarshaler.AllowUnknownFields = true
}

func main() {
	options := GetOptions()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))

	// Get a config to talk to the apiserver
	setupLog.Info("Setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Setup clientset to directly talk to the api server
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "unable to create clientSet")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	setupLog.Info("Setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: options.metricsAddr},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: options.webhookPort}),
		LeaderElection:          options.enableLeaderElection,
		LeaderElectionID:        LeaderLockName,
		LeaderElectionNamespace: options.leaderElectionNamespace,
		HealthProbeBindAddress:  options.probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	setupLog.Info("Registering Components.")

	setupLog.Info("Setting up OME v1beta1 scheme")
	if err := v1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add OME v1beta1 to scheme")
		os.Exit(1)
	}

	deployConfig, err := v1beta1.NewDeployConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "unable to get deploy config.")
		os.Exit(1)
	}
	ingressConfig, err := v1beta1.NewIngressConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "unable to get ingress config.")
		os.Exit(1)
	}

	dacReconcilePolicyConfig, err := v1beta1.NewDacReconcilePolicyConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "unable to get dacReconcilePolicy config.")
		os.Exit(1)
	}

	rayFound, rayCheckErr := utils.IsCrdAvailable(cfg, ray.SchemeGroupVersion.String(), constants.RayClusterKind)
	if rayCheckErr != nil {
		setupLog.Error(rayCheckErr, "error when checking if Ray Cluster kind is available")
		os.Exit(1)
	}
	if rayFound {
		setupLog.Info("Setting up Ray scheme")
		if err := ray.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "unable to add Ray APIs to scheme")
			os.Exit(1)
		}
	}

	volcanoFound, volcanoCheckErr := utils.IsCrdAvailable(cfg, volcano.SchemeGroupVersion.String(), constants.VolcanoQueueKind)
	if volcanoCheckErr != nil {
		setupLog.Error(volcanoCheckErr, "error when checking if Volcano Queue kind is available")
		os.Exit(1)
	}
	if volcanoFound {
		setupLog.Info("Setting up Volcano scheme")
		if err := volcano.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "unable to add Volcano APIs to scheme")
			os.Exit(1)
		}
	}

	volcanoBatchFound, volcanoCheckErr := utils.IsCrdAvailable(cfg, volcanobatch.SchemeGroupVersion.String(), constants.VolcanoJobKind)
	if volcanoCheckErr != nil {
		setupLog.Error(volcanoCheckErr, "error when checking if Volcano Job kind is available")
		os.Exit(1)
	}
	if volcanoBatchFound {
		setupLog.Info("Setting up Volcano Batch scheme")
		if err := volcanobatch.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "unable to add Volcano Batch APIs to scheme")
			os.Exit(1)
		}
	}

	ksvcFound, ksvcCheckErr := utils.IsCrdAvailable(cfg, knservingv1.SchemeGroupVersion.String(), constants.KnativeServiceKind)
	if ksvcCheckErr != nil {
		setupLog.Error(ksvcCheckErr, "error when checking if Knative Service kind is available")
		os.Exit(1)
	}
	if ksvcFound {
		setupLog.Info("Setting up Knative scheme")
		if err := knservingv1.AddToScheme(mgr.GetScheme()); err != nil {
			setupLog.Error(err, "unable to add Knative APIs to scheme")
			os.Exit(1)
		}
	}
	if !ingressConfig.DisableIstioVirtualHost {
		vsFound, vsCheckErr := utils.IsCrdAvailable(cfg, istioclientv1beta1.SchemeGroupVersion.String(), constants.IstioVirtualServiceKind)
		if vsCheckErr != nil {
			setupLog.Error(vsCheckErr, "error when checking if Istio VirtualServices are available")
			os.Exit(1)
		}
		if vsFound {
			setupLog.Info("Setting up Istio schemes")
			if err := istioclientv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
				setupLog.Error(err, "unable to add Istio v1beta1 APIs to scheme")
				os.Exit(1)
			}
		}
	}

	// Register KEDA API to the scheme
	setupLog.Info("Setting up KEDA ScaledObject scheme")
	if err := kedav1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add KEDA ScaledObject to scheme")
		os.Exit(1)
	}

	setupLog.Info("Setting up core scheme")
	if err := v1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add Core APIs to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	setupLog.Info("Setting up v1beta1 controller")
	eventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up InferenceService controller")
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1isvccontroller.InferenceServiceReconciler{
		Client:    mgr.GetClient(),
		Clientset: clientSet,
		Log:       ctrl.Log.WithName("v1beta1Controllers").WithName("InferenceService"),
		Scheme:    mgr.GetScheme(),
		Recorder: eventBroadcaster.NewRecorder(
			mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr, deployConfig, ingressConfig); err != nil {
		setupLog.Error(err, "unable to create controller", "v1beta1Controller", "InferenceService")
		os.Exit(1)
	}

	dedicatedAIClusterEventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up DedicatedAICluster controller")
	dedicatedAIClusterEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1dacccontroller.DedicatedAIClusterReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("v1beta1Controllers").WithName("DedicatedAICluster"),
		Scheme:   mgr.GetScheme(),
		Recorder: dedicatedAIClusterEventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr, dacReconcilePolicyConfig); err != nil {
		setupLog.Error(err, "unable to create controller", "v1beta1Controller", "DedicatedAICluster")
		os.Exit(1)
	}

	benchmarkJobEventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up BenchmarkJob controller")
	benchmarkJobEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1benchmarkjobcontroller.BenchmarkJobReconciler{
		Client:    mgr.GetClient(),
		Clientset: clientSet,
		Log:       ctrl.Log.WithName("v1beta1Controllers").WithName("BenchmarkJob"),
		Scheme:    mgr.GetScheme(),
		Recorder: benchmarkJobEventBroadcaster.NewRecorder(
			mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "v1beta1Controller", "BenchmarkJob")
		os.Exit(1)
	}

	if options.enableWebhook {
		setupLog.Info("setting up webhook server")
		hookServer := mgr.GetWebhookServer()

		setupLog.Info("registering webhooks to the webhook server")
		hookServer.Register("/mutate-pods", &webhook.Admission{
			Handler: &pod.Mutator{Client: mgr.GetClient(), Clientset: clientSet, Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("registering cluster serving runtime validator webhook to the webhook server")
		hookServer.Register("/validate-serving-ome-io-v1alpha1-clusterservingruntime", &webhook.Admission{
			Handler: &servingruntime.ClusterServingRuntimeValidator{Client: mgr.GetClient(), Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("registering serving runtime validator webhook to the webhook server")
		hookServer.Register("/validate-serving-ome-io-v1alpha1-servingruntime", &webhook.Admission{
			Handler: &servingruntime.ServingRuntimeValidator{Client: mgr.GetClient(), Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		if err = ctrl.NewWebhookManagedBy(mgr).
			For(&v1beta1.InferenceService{}).
			WithDefaulter(&v1beta1.InferenceServiceDefaulter{}).
			WithValidator(&v1beta1.InferenceServiceValidator{}).
			Complete(); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "v1beta1")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", func(req *http.Request) error {
		return mgr.GetWebhookServer().StartedChecker()(req)
	}); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", func(req *http.Request) error {
		return mgr.GetWebhookServer().StartedChecker()(req)
	}); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	// Start the Cmd
	setupLog.Info("Starting the Cmd.")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}
