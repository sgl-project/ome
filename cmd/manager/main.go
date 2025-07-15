package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	v1beta1basemodelcontroller "github.com/sgl-project/ome/pkg/controller/v1beta1/basemodel"
	v1beta1benchmarkjobcontroller "github.com/sgl-project/ome/pkg/controller/v1beta1/benchmark"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	v1beta1isvccontroller "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice"
	"github.com/sgl-project/ome/pkg/runtimeselector"
	"github.com/sgl-project/ome/pkg/utils"
	"github.com/sgl-project/ome/pkg/webhook/admission/benchmark"
	"github.com/sgl-project/ome/pkg/webhook/admission/isvc"
	"github.com/sgl-project/ome/pkg/webhook/admission/pod"
	"github.com/sgl-project/ome/pkg/webhook/admission/servingruntime"
	zaplog "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	istionetworking "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	volcanobatch "volcano.sh/apis/pkg/apis/batch/v1alpha1"
	volcano "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

const (
	LeaderLockName          = "ome-controller-manager-leader-lock"
	LeaderElectionNamespace = "ome"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	tlsOpts  []func(*tls.Config)
)

// registerOptionalScheme attempts to register a scheme if its CRD is available
func registerOptionalScheme(cfg *rest.Config, s *runtime.Scheme, groupVersion schema.GroupVersion, kind string, addToScheme func(*runtime.Scheme) error) error {
	found, err := utils.IsCrdAvailable(cfg, groupVersion.String(), kind)
	if err != nil {
		return fmt.Errorf("error checking if %s kind is available: %w", kind, err)
	}
	if found {
		setupLog.Info("Setting up scheme", "groupVersion", groupVersion.String(), "kind", kind)
		if err := addToScheme(s); err != nil {
			return fmt.Errorf("unable to add %s APIs to scheme: %w", kind, err)
		}
	}
	return nil
}

func init() {
	// Allow unknown fields in Istio API client for backwards compatibility if cluster has existing vs with deprecated fields.
	istionetworking.VirtualServiceUnmarshaler.AllowUnknownFields = true
	istionetworking.GatewayUnmarshaler.AllowUnknownFields = true

	utilruntime.Must(v1beta1.AddToScheme(scheme))
	utilruntime.Must(schedulerpluginsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kueuev1beta1.AddToScheme(scheme))
}

// Options defines the program-configurable options that may be passed on the command line.
type Options struct {
	metricsAddr             string
	secureMetrics           bool
	enableHTTP2             bool
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
		enableHTTP2:             false,
		secureMetrics:           false,
		probeAddr:               ":8081",
		leaderElectionNamespace: LeaderElectionNamespace,
		zapOpts: zap.Options{
			TimeEncoder: zapcore.RFC3339TimeEncoder,
			ZapOpts:     []zaplog.Option{zaplog.AddCaller()},
		},
	}
}

// GetOptions parses the program flags and returns them as Options.
func GetOptions() Options {
	opts := DefaultOptions()
	flag.StringVar(&opts.metricsAddr, "metrics-bind-address", opts.metricsAddr, "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.BoolVar(&opts.secureMetrics, "metrics-secure", opts.secureMetrics,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&opts.enableHTTP2, "enable-http2", opts.enableHTTP2,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
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

func main() {
	setupLog.Info("Initializing controller manager")
	options := GetOptions()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))

	// Get a config to talk to the apiserver
	setupLog.Info("Configuring API client connection")
	cfg := ctrl.GetConfigOrDie()

	// Setup clientset to directly talk to the api server
	setupLog.Info("Creating Kubernetes client set")
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "Failed to create Kubernetes client set")
		os.Exit(1)
	}

	if !options.enableHTTP2 {
		// if the enable-http2 flag is false (the default), http/2 should be disabled
		// due to its vulnerabilities. More specifically, disabling http/2 will
		// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
		// Rapid Reset CVEs. For more information see:
		// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
		// - https://github.com/advisories/GHSA-4374-p667-p6c8
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		})
	}

	// Create a new Cmd to provide shared dependencies and start components
	setupLog.Info("Initializing controller manager",
		"metricsAddr", options.metricsAddr,
		"webhookPort", options.webhookPort,
		"leaderElection", options.enableLeaderElection)
	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   options.metricsAddr,
			TLSOpts:       tlsOpts,
			SecureServing: options.secureMetrics,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    options.webhookPort,
			TLSOpts: tlsOpts,
		}),
		LeaderElection:          options.enableLeaderElection,
		LeaderElectionID:        LeaderLockName,
		LeaderElectionNamespace: options.leaderElectionNamespace,
		HealthProbeBindAddress:  options.probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "Failed to initialize controller manager")
		os.Exit(1)
	}

	deployConfig, err := controllerconfig.NewDeployConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "Failed to initialize deployment configuration")
		os.Exit(1)
	}
	ingressConfig, err := controllerconfig.NewIngressConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "Failed to initialize ingress configuration")
		os.Exit(1)
	}

	// Register optional schemes based on CRD availability
	setupLog.Info("Registering optional CRD schemes")
	optionalSchemes := []struct {
		groupVersion schema.GroupVersion
		kind         string
		addToScheme  func(*runtime.Scheme) error
	}{
		{ray.SchemeGroupVersion, constants.RayClusterKind, ray.AddToScheme},
		{knservingv1.SchemeGroupVersion, constants.KnativeServiceKind, knservingv1.AddToScheme},
		{lws.SchemeGroupVersion, constants.LWSKind, lws.AddToScheme},
		{volcano.SchemeGroupVersion, constants.VolcanoQueueKind, volcano.AddToScheme},
		{volcanobatch.SchemeGroupVersion, constants.VolcanoJobKind, volcanobatch.AddToScheme},
		{kedav1.SchemeGroupVersion, constants.KEDAScaledObjectKind, kedav1.AddToScheme},
	}

	for _, s := range optionalSchemes {
		if err := registerOptionalScheme(cfg, mgr.GetScheme(), s.groupVersion, s.kind, s.addToScheme); err != nil {
			setupLog.Error(err, "Failed to register optional scheme",
				"groupVersion", s.groupVersion.String(),
				"kind", s.kind)
			os.Exit(1)
		}
	}

	if !ingressConfig.DisableIstioVirtualHost {
		if err := registerOptionalScheme(cfg, mgr.GetScheme(), istioclientv1beta1.SchemeGroupVersion, constants.IstioVirtualServiceKind, istioclientv1beta1.AddToScheme); err != nil {
			setupLog.Error(err, "Failed to register Istio scheme")
			os.Exit(1)
		}
	}

	// Setup Event Broadcaster
	setupLog.Info("Configuring event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up InferenceService controller")
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1isvccontroller.InferenceServiceReconciler{
		Client:    mgr.GetClient(),
		Clientset: clientSet,
		Log:       ctrl.Log.WithName("InferenceService"),
		Scheme:    mgr.GetScheme(),
		Recorder:  eventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr, deployConfig, ingressConfig); err != nil {
		setupLog.Error(err, "Failed to create InferenceService controller")
		os.Exit(1)
	}

	// Setup BaseModel and ClusterBaseModel controllers with the manager
	baseModelEventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up BaseModel controller")
	baseModelEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1basemodelcontroller.BaseModelReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("BaseModel"),
		Scheme:   mgr.GetScheme(),
		Recorder: baseModelEventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create BaseModel controller")
		os.Exit(1)
	}

	clusterBaseModelEventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up ClusterBaseModel controller")
	clusterBaseModelEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1basemodelcontroller.ClusterBaseModelReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("ClusterBaseModel"),
		Scheme:   mgr.GetScheme(),
		Recorder: clusterBaseModelEventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create ClusterBaseModel controller")
		os.Exit(1)
	}

	benchmarkJobEventBroadcaster := record.NewBroadcaster()
	setupLog.Info("Setting up BenchmarkJob controller")
	benchmarkJobEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if err = (&v1beta1benchmarkjobcontroller.BenchmarkJobReconciler{
		Client:    mgr.GetClient(),
		Clientset: clientSet,
		Log:       ctrl.Log.WithName("BenchmarkJob"),
		Scheme:    mgr.GetScheme(),
		Recorder:  benchmarkJobEventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create Benchmark Job controller")
		os.Exit(1)
	}

	if options.enableWebhook {
		setupLog.Info("Configuring webhook server", "port", options.webhookPort)
		hookServer := mgr.GetWebhookServer()

		setupLog.Info("Registering InferenceService webhook to the webhook server")
		hookServer.Register("/mutate-pods", &webhook.Admission{
			Handler: &pod.Mutator{Client: mgr.GetClient(), Clientset: clientSet, Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("Registering cluster serving runtime validator webhook to the webhook server")
		hookServer.Register("/validate-ome-io-v1beta1-clusterservingruntime", &webhook.Admission{
			Handler: &servingruntime.ClusterServingRuntimeValidator{Client: mgr.GetClient(), Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("Registering serving runtime validator webhook to the webhook server")
		hookServer.Register("/validate-ome-io-v1beta1-servingruntime", &webhook.Admission{
			Handler: &servingruntime.ServingRuntimeValidator{Client: mgr.GetClient(), Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("Registering benchmark job validator webhook to the webhook server")
		hookServer.Register("/validate-ome-io-v1beta1-benchmarkjob", &webhook.Admission{
			Handler: &benchmark.BenchmarkJobValidator{Client: mgr.GetClient(), Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		if err = ctrl.NewWebhookManagedBy(mgr).
			For(&v1beta1.InferenceService{}).
			WithDefaulter(&isvc.InferenceServiceDefaulter{}).
			WithValidator(&isvc.InferenceServiceValidator{
				Client:          mgr.GetClient(),
				RuntimeSelector: runtimeselector.New(mgr.GetClient()),
			}).
			Complete(); err != nil {
			setupLog.Error(err, "Failed to create InferenceService webhook", "webhook", "v1beta1")
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
	setupLog.Info("Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}
}
