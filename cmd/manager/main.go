package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
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

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	v1beta1acceleratorclasscontroller "sigs.k8s.io/ome/pkg/controller/v1beta1/acceleratorclass"
	v1beta1basemodelcontroller "sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel"
	v1beta1benchmarkjobcontroller "sigs.k8s.io/ome/pkg/controller/v1beta1/benchmark"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	v1beta1isvccontroller "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/traffic"
	trafficfactory "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/traffic/factory"
	v1beta1runtimerevisioncontroller "sigs.k8s.io/ome/pkg/controller/v1beta1/runtimerevision"
	"sigs.k8s.io/ome/pkg/runtimeselector"
	"sigs.k8s.io/ome/pkg/utils"
	"sigs.k8s.io/ome/pkg/version"
	basemodelwebhook "sigs.k8s.io/ome/pkg/webhook/admission/basemodel"
	"sigs.k8s.io/ome/pkg/webhook/admission/benchmark"
	inferencereplicawebhook "sigs.k8s.io/ome/pkg/webhook/admission/inferencereplica"
	"sigs.k8s.io/ome/pkg/webhook/admission/isvc"
	"sigs.k8s.io/ome/pkg/webhook/admission/pod"
	runtimerevisionwebhook "sigs.k8s.io/ome/pkg/webhook/admission/runtimerevision"
	"sigs.k8s.io/ome/pkg/webhook/admission/servingruntime"
)

const (
	LeaderLockName          = "ome-controller-manager-leader-lock"
	LeaderElectionNamespace = "ome"

	// multiClusterRoleControlPlane is the --multicluster-role value that makes
	// this manager the multi-cluster fan-out control plane: it runs the placement
	// controller and disables the local InferenceService reconcilers.
	multiClusterRoleControlPlane = "control-plane"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	tlsOpts  []func(*tls.Config)
)

// splitAndTrim splits a comma-separated list, trims spaces, and drops empties.
func splitAndTrim(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

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
	metricsAddr                string
	secureMetrics              bool
	enableHTTP2                bool
	webhookPort                int
	enableLeaderElection       bool
	enableWebhook              bool
	probeAddr                  string
	leaderElectionNamespace    string
	runtimeRevisionRetention   int
	runtimeRevisionGracePeriod time.Duration
	// Multi-cluster topology, identity, and security: these decide which
	// controllers run and the manager's identity, so they are deploy-time flags.
	// The tunables live in the inferenceservice-config ConfigMap
	// (controllerconfig.MultiClusterConfig), loaded at startup.
	enableMultiCluster        bool
	multiClusterRole          string
	allowExecCredentials      bool
	execCredentialAllowedCmds string
	placementControlPlaneID   string
	zapOpts                   zap.Options
}

// DefaultOptions returns the default values for the program options.
func DefaultOptions() Options {
	return Options{
		metricsAddr:                ":8080",
		webhookPort:                9443,
		enableLeaderElection:       false,
		enableWebhook:              false,
		enableHTTP2:                false,
		secureMetrics:              false,
		probeAddr:                  ":8081",
		leaderElectionNamespace:    LeaderElectionNamespace,
		runtimeRevisionRetention:   10,
		runtimeRevisionGracePeriod: 24 * time.Hour,
		enableMultiCluster:         false,
		multiClusterRole:           "",
		allowExecCredentials:       false,
		execCredentialAllowedCmds:  "aws,gke-gcloud-auth-plugin,kubelogin",
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
	flag.IntVar(&opts.runtimeRevisionRetention, "runtime-revision-retention", opts.runtimeRevisionRetention,
		"Number of ControllerRevision snapshots to retain per source runtime before garbage collection.")
	flag.DurationVar(&opts.runtimeRevisionGracePeriod, "runtime-revision-grace-period", opts.runtimeRevisionGracePeriod,
		"How long a runtime-revision snapshot must stay unreferenced and over-retention before GC deletes it.")
	flag.BoolVar(&opts.enableMultiCluster, "enable-multicluster", opts.enableMultiCluster,
		"Enable the multi-cluster controllers (WorkloadCluster registry). Alpha; default off.")
	flag.StringVar(&opts.multiClusterRole, "multicluster-role", opts.multiClusterRole,
		"Multi-cluster role. \"control-plane\" makes this the fan-out placer: it runs "+
			"the placement controller (which clones InferenceServices onto workload clusters) and "+
			"DISABLES the local InferenceService reconcilers. Empty (default) is a normal "+
			"single-cluster / workload-cluster manager. \"control-plane\" implies --enable-multicluster.")
	flag.BoolVar(&opts.allowExecCredentials, "allow-exec-credentials", opts.allowExecCredentials,
		"Allow WorkloadCluster kubeconfigs to use an exec credential plugin (e.g. aws, gke-gcloud-auth-plugin) "+
			"for short-lived cloud tokens. Default false. The plugin command must also appear in "+
			"--exec-credential-allowed-commands, and its binary must be present in the manager image. "+
			"SECURITY: an exec block runs a command in the controller pod; keep WorkloadCluster kubeconfig Secrets admin-only.")
	flag.StringVar(&opts.execCredentialAllowedCmds, "exec-credential-allowed-commands", opts.execCredentialAllowedCmds,
		"Comma-separated allowlist of exec credential plugin command basenames permitted when --allow-exec-credentials is set.")
	flag.StringVar(&opts.placementControlPlaneID, "placement-control-plane-id", opts.placementControlPlaneID,
		"identity stamped on every derived InferenceService this control plane creates. "+
			"Required when multiple control planes share a workload cluster so each control plane's GC "+
			"only reaps its OWN deriveds; the chart supplies a per-control-plane value. Empty (default) "+
			"keeps single-control-plane behavior (no identity stamping or GC scoping).")
	opts.zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()
	return opts
}

func main() {
	options := GetOptions()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))

	setupLog.Info("Initializing", "gitVersion", version.GitVersion, "gitCommit", version.GitCommit)

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
	omeAgentConfig, err := controllerconfig.NewOmeAgentConfig(clientSet)
	if err != nil {
		setupLog.Error(err, "Failed to initialize ome-agent configuration")
		os.Exit(1)
	}

	// Register optional schemes based on CRD availability
	setupLog.Info("Registering optional CRD schemes")
	optionalSchemes := []struct {
		groupVersion schema.GroupVersion
		kind         string
		addToScheme  func(*runtime.Scheme) error
	}{
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

	// Multi-cluster role resolution. On the control plane the placement (fan-out)
	// controller owns InferenceServices — it clones them onto workload clusters —
	// so the local ISVC and model/runtime reconcilers must NOT run here.
	isControlPlane := options.multiClusterRole == multiClusterRoleControlPlane
	if options.multiClusterRole != "" && !isControlPlane {
		setupLog.Error(fmt.Errorf("invalid --multicluster-role %q (want %q or empty)", options.multiClusterRole, multiClusterRoleControlPlane), "bad flag")
		os.Exit(1)
	}

	// Setup Event Broadcaster
	// Select the active traffic translator based on
	// installed backend-policy CRDs (Envoy Gateway BackendTrafficPolicy,
	// Istio DestinationRule, else Noop). The selection is process-
	// level — the translator stays stable for the controller's
	// lifetime — so we build it once at startup.
	setupLog.Info("Selecting traffic translator")
	trafficTranslator, err := trafficfactory.New(cfg)
	if err != nil {
		setupLog.Error(err, "Failed to select traffic translator")
		os.Exit(1)
	}
	setupLog.Info("Selected traffic translator", "translator", trafficTranslator.Name())
	trafficReconciler := traffic.NewReconciler(mgr.GetClient(), mgr.GetScheme(), trafficTranslator)

	setupLog.Info("Configuring event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	if isControlPlane {
		setupLog.Info("control-plane role: local InferenceService reconciler disabled; placement controller owns ISVCs")
	} else {
		setupLog.Info("Setting up InferenceService controller")
		if err = (&v1beta1isvccontroller.InferenceServiceReconciler{
			Client:            mgr.GetClient(),
			Clientset:         clientSet,
			Log:               ctrl.Log.WithName("InferenceService"),
			Scheme:            mgr.GetScheme(),
			Recorder:          eventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
			TrafficReconciler: trafficReconciler,
		}).SetupWithManager(mgr, deployConfig, ingressConfig); err != nil {
			setupLog.Error(err, "Failed to create InferenceService controller")
			os.Exit(1)
		}
	}

	// The control plane owns no workloads, models, or runtimes — those live on the
	// workload clusters. Running the model/runtime lifecycle controllers here would
	// reconcile against an empty local catalog (no-op at best, churn at worst), so
	// gate them off under the control-plane role.
	if isControlPlane {
		setupLog.Info("control-plane role: BaseModel/ClusterBaseModel/BenchmarkJob/AcceleratorClass/RuntimeRevisionGC controllers disabled")
	} else {
		// Setup BaseModel and ClusterBaseModel controllers with the manager
		setupLog.Info("Setting up BaseModel controller")
		if err = (&v1beta1basemodelcontroller.BaseModelReconciler{
			Client:         mgr.GetClient(),
			Log:            ctrl.Log.WithName("BaseModel"),
			Scheme:         mgr.GetScheme(),
			OmeAgentConfig: omeAgentConfig,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create BaseModel controller")
			os.Exit(1)
		}

		setupLog.Info("Setting up ClusterBaseModel controller")
		if err = (&v1beta1basemodelcontroller.ClusterBaseModelReconciler{
			Client:         mgr.GetClient(),
			Log:            ctrl.Log.WithName("ClusterBaseModel"),
			Scheme:         mgr.GetScheme(),
			OmeAgentConfig: omeAgentConfig,
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

		// Setup AcceleratorClass controller
		acceleratorClassEventBroadcaster := record.NewBroadcaster()
		setupLog.Info("Setting up AcceleratorClass controller")
		acceleratorClassEventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
		if err = (&v1beta1acceleratorclasscontroller.AcceleratorClassReconciler{
			Client:   mgr.GetClient(),
			Log:      ctrl.Log.WithName("AcceleratorClass"),
			Scheme:   mgr.GetScheme(),
			Recorder: acceleratorClassEventBroadcaster.NewRecorder(mgr.GetScheme(), v1.EventSource{Component: "v1beta1Controllers"}),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create AcceleratorClass controller")
			os.Exit(1)
		}

		setupLog.Info("Setting up runtime-revision GC controller",
			"retention", options.runtimeRevisionRetention,
			"gracePeriod", options.runtimeRevisionGracePeriod)
		if err = (&v1beta1runtimerevisioncontroller.GCReconciler{
			Client:              mgr.GetClient(),
			Log:                 ctrl.Log.WithName("RuntimeRevisionGC"),
			Scheme:              mgr.GetScheme(),
			OMENamespace:        constants.OMENamespace,
			RetentionPerRuntime: options.runtimeRevisionRetention,
			GracePeriod:         options.runtimeRevisionGracePeriod,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create runtime-revision GC controller")
			os.Exit(1)
		}
	}

	// Multi-cluster: the WorkloadCluster registry/transport runs when multi-cluster
	// is enabled OR this is the control plane (which requires it). The control plane
	// additionally runs the placement (fan-out) controller, its GC, and the endpoint
	// publisher — see setupMultiCluster.
	if options.enableMultiCluster || isControlPlane {
		if err = setupMultiCluster(mgr, clientSet, options, isControlPlane); err != nil {
			setupLog.Error(err, "Failed to set up multi-cluster")
			os.Exit(1)
		}
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

		setupLog.Info("Registering ControllerRevision immutability validator webhook to the webhook server")
		hookServer.Register("/validate-apps-v1-controllerrevision", &webhook.Admission{
			Handler: &runtimerevisionwebhook.ImmutabilityValidator{Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("Registering BaseModel validator webhook to the webhook server")
		hookServer.Register("/validate-ome-io-v1beta1-basemodel", &webhook.Admission{
			Handler: &basemodelwebhook.BaseModelValidator{Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		setupLog.Info("Registering ClusterBaseModel validator webhook to the webhook server")
		hookServer.Register("/validate-ome-io-v1beta1-clusterbasemodel", &webhook.Admission{
			Handler: &basemodelwebhook.ClusterBaseModelValidator{Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		hookServer.Register("/validate-ome-io-v1beta1-inferencereplica", &webhook.Admission{
			Handler: &inferencereplicawebhook.Validator{Decoder: admission.NewDecoder(mgr.GetScheme())},
		})

		if err = ctrl.NewWebhookManagedBy(mgr).
			For(&v1beta1.InferenceService{}).
			WithDefaulter(&isvc.InferenceServiceDefaulter{
				Client:    mgr.GetClient(),
				ClientSet: clientSet,
			}).
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
