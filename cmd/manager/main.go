package main

import (
	"flag"
	"net/http"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	v1beta1controller "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/utils"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/webhook/admission/pod"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/webhook/admission/servingruntime"
	istionetworking "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme() //nolint: unused
	setupLog = ctrl.Log.WithName("setup")
)

const (
	LeaderLockName = "ome-controller-manager-leader-lock"
)

// Options defines the program configurable options that may be passed on the command line.
type Options struct {
	metricsAddr          string
	webhookPort          int
	enableLeaderElection bool
	probeAddr            string
	zapOpts              zap.Options
}

// DefaultOptions returns the default values for the program options.
func DefaultOptions() Options {
	return Options{
		metricsAddr:          ":8080",
		webhookPort:          9443,
		enableLeaderElection: false,
		probeAddr:            ":8081",
		zapOpts:              zap.Options{},
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
		LeaderElection:         options.enableLeaderElection,
		LeaderElectionID:       LeaderLockName,
		HealthProbeBindAddress: options.probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	setupLog.Info("Registering Components.")

	setupLog.Info("Setting up KServe v1alpha1 scheme")

	setupLog.Info("Setting up KServe v1beta1 scheme")
	if err := v1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add KServe v1beta1 to scheme")
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

	setupLog.Info("Setting up core scheme")
	if err := v1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add Core APIs to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	setupLog.Info("Setting up v1beta1 controller")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1controller.InferenceServiceReconciler{
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
		Complete(); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "v1beta1")
		os.Exit(1)
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
