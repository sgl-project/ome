package main

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/placement"
	placementendpoint "sigs.k8s.io/ome/pkg/controller/v1beta1/placement/endpoint"
	workloadcluster "sigs.k8s.io/ome/pkg/controller/v1beta1/workloadcluster"
)

// mcWiring is the multi-cluster wiring resolved from a MultiClusterConfig: the
// plain values that flow into the WorkloadCluster transport, the placement
// controller, and the endpoint publisher. It is split out of setupMultiCluster
// so the config->values mapping — the part where a mis-wired field would
// silently mis-tune the control plane — is unit-testable without a manager.
//
// Durations are already resolved (the accessors yield 0 on an omitted key, and
// each consuming option then applies its own in-package default).
type mcWiring struct {
	clientTuning      workloadcluster.ClientTuning
	cacheEnabled      bool
	healthInterval    time.Duration
	connectionGrace   time.Duration
	eventsBatchPeriod time.Duration
	reconnectBackoff  workloadcluster.ReconnectBackoffConfig

	// Placement + its status convergence and GC (control plane only).
	requeue                time.Duration
	gcInterval             time.Duration
	maxConcurrent          int
	placeTimeout           time.Duration
	winnerLostGrace        time.Duration
	statusBatchPeriod      time.Duration
	statusSafetyRequeue    time.Duration
	dispatcherMode         placement.DispatcherMode
	dispatcherStepSize     int
	dispatcherRoundTimeout time.Duration
	funnelResyncInterval   time.Duration
	funnelBufferSize       int

	endpoint placementendpoint.Config
}

// resolveMCWiring maps a loaded MultiClusterConfig to the wiring values used to
// build the multi-cluster controllers.
func resolveMCWiring(mc *controllerconfig.MultiClusterConfig) mcWiring {
	wc, pl, ep := mc.WorkloadCluster, mc.Placement, mc.Endpoint

	// Status-convergence backstop: with the cache (and thus the watch funnel) on,
	// events drive freshness and the safety requeue only recovers a missed event;
	// with it off there is no event source, so the poll cadence (requeueInterval)
	// is the backstop.
	safetyRequeue := pl.StatusSafetyRequeueDuration()
	if !wc.CacheEnabled {
		safetyRequeue = pl.RequeueIntervalDuration()
	}

	return mcWiring{
		clientTuning: workloadcluster.ClientTuning{
			QPS:            float32(wc.ClientQPS),
			Burst:          wc.ClientBurst,
			PerCallTimeout: wc.PerCallTimeoutDuration(),
		},
		cacheEnabled:      wc.CacheEnabled,
		healthInterval:    wc.HealthIntervalDuration(),
		connectionGrace:   wc.ConnectionGraceDuration(),
		eventsBatchPeriod: wc.EventsBatchPeriodDuration(),
		reconnectBackoff: workloadcluster.ReconnectBackoffConfig{
			EstablishInitial: wc.EstablishInitialDuration(),
			EstablishMax:     wc.EstablishMaxDuration(),
			RetryMax:         wc.ReconnectRetryMaxDuration(),
		},
		requeue:                pl.RequeueIntervalDuration(),
		gcInterval:             pl.GCIntervalDuration(),
		maxConcurrent:          pl.MaxConcurrentReconciles,
		placeTimeout:           pl.FanoutTimeoutDuration(),
		winnerLostGrace:        pl.WinnerLostGraceDuration(),
		statusBatchPeriod:      pl.StatusBatchPeriodDuration(),
		statusSafetyRequeue:    safetyRequeue,
		dispatcherMode:         placement.DispatcherMode(pl.DispatcherMode),
		dispatcherStepSize:     pl.DispatcherStepSize,
		dispatcherRoundTimeout: pl.DispatcherRoundTimeoutDuration(),
		funnelResyncInterval:   wc.FunnelResyncIntervalDuration(),
		funnelBufferSize:       wc.FunnelBufferSize,
		endpoint: placementendpoint.Config{
			GlobalHostTemplate: ep.GlobalHostTemplate,
			GlobalGateway:      ep.GlobalGateway,
			RouteNamespace:     ep.RouteNamespace,
			BackendPort:        int32(ep.BackendPort),
		},
	}
}

// setupMultiCluster wires the multi-cluster control/transport layer onto mgr:
// the WorkloadCluster registry/transport always, plus — on the control plane —
// the placement (fan-out) controller, its orphan GC, and the global endpoint
// publisher, all sharing the one WorkloadCluster Manager so the placement
// controllers read the live per-cluster clients it connects. Tunables load from
// the inferenceservice-config ConfigMap; topology, identity, and security come
// from options (flags).
func setupMultiCluster(mgr manager.Manager, clientSet kubernetes.Interface, options Options, isControlPlane bool) error {
	mcConfig, err := controllerconfig.NewMultiClusterConfig(clientSet)
	if err != nil {
		return fmt.Errorf("load multi-cluster configuration: %w", err)
	}
	w := resolveMCWiring(mcConfig)

	clusterManager := workloadcluster.NewManager(mgr.GetScheme())
	execPolicy := workloadcluster.ExecCredentialPolicy{
		Allowed:         options.allowExecCredentials,
		AllowedCommands: splitAndTrim(options.execCredentialAllowedCmds),
	}
	clusterManager.SetExecCredentialPolicy(execPolicy)
	clusterManager.SetClientTuning(w.clientTuning)
	if w.cacheEnabled {
		// Scope the cached derived-InferenceService informer to exactly the set the
		// watch funnel watches and resolves: this control plane's deriveds (origin
		// marker, control-plane-scoped when an identity is configured). Sharing one
		// selector keeps the cache from holding another control plane's deriveds on
		// a shared workload cluster and guarantees every object the funnel's cache
		// handler resolves is actually cached.
		clusterManager.SetCacheOptions(workloadcluster.CacheOptions{
			CachedKinds:     sets.New(v1beta1.SchemeGroupVersion.WithKind("InferenceService").GroupKind()),
			DefaultSelector: placement.FunnelConfigFor(options.placementControlPlaneID).WatchSelector,
		})
	}
	// Run the cluster Manager as a Runnable so it captures the controller-manager's
	// long-lived context as the base for remote-client (and cache-informer)
	// contexts, and disconnects everything on shutdown (no leaked watches).
	if err := mgr.Add(clusterManager); err != nil {
		return fmt.Errorf("add WorkloadCluster manager runnable: %w", err)
	}
	setupLog.Info("Setting up WorkloadCluster controller")
	if err := (&workloadcluster.Reconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Log:                   ctrl.Log.WithName("controllers").WithName("WorkloadCluster"),
		Manager:               clusterManager,
		ExecPolicy:            execPolicy,
		HealthInterval:        w.healthInterval,
		ConnectionGracePeriod: w.connectionGrace,
	}).SetupWithManager(mgr,
		workloadcluster.WithEventsBatchPeriod(w.eventsBatchPeriod),
		workloadcluster.WithReconnectBackoff(w.reconnectBackoff),
	); err != nil {
		return fmt.Errorf("create WorkloadCluster controller: %w", err)
	}

	if !isControlPlane {
		return nil
	}

	setupLog.Info("Setting up multi-cluster placement (fan-out) controller")
	// Cross-cluster status convergence. The status batch period and safety requeue
	// always apply; when the cache is enabled the watch funnel additionally feeds
	// events so a derived's status change re-reconciles its source on an event
	// rather than waiting for the safety requeue (which is then only the
	// missed-event backstop). Without the cache there is no event source, so
	// convergence stays on the poll cadence (already folded into statusSafetyRequeue).
	convergeOpts := []placement.ConvergeOption{
		placement.WithStatusBatchPeriod(w.statusBatchPeriod),
		placement.WithStatusSafetyRequeue(w.statusSafetyRequeue),
	}
	if w.cacheEnabled {
		funnelCfg := placement.FunnelConfigFor(options.placementControlPlaneID)
		funnelCfg.ResyncInterval = w.funnelResyncInterval
		funnelCfg.BufferSize = w.funnelBufferSize
		funnel := workloadcluster.NewStatusFunnel(clusterManager, funnelCfg)
		if err := mgr.Add(funnel); err != nil {
			return fmt.Errorf("add multi-cluster status funnel runnable: %w", err)
		}
		convergeOpts = append(convergeOpts, placement.WithStatusEvents(funnel.Events()))
	}

	if err := (&placement.Reconciler{
		Client:                  mgr.GetClient(),
		Scheme:                  mgr.GetScheme(),
		Log:                     ctrl.Log.WithName("controllers").WithName("Placement"),
		Clusters:                clusterManager,
		Requeue:                 w.requeue,
		ControlPlaneID:          options.placementControlPlaneID,
		MaxConcurrentReconciles: w.maxConcurrent,
		PlaceTimeout:            w.placeTimeout,
		WinnerLostGracePeriod:   w.winnerLostGrace,
		DispatcherMode:          w.dispatcherMode,
		DispatcherStepSize:      w.dispatcherStepSize,
		DispatcherRoundTimeout:  w.dispatcherRoundTimeout,
	}).SetupWithManager(mgr, convergeOpts...); err != nil {
		return fmt.Errorf("create Placement controller: %w", err)
	}
	if err := mgr.Add(&placement.GCReconciler{
		APIReader:      mgr.GetAPIReader(),
		Log:            ctrl.Log.WithName("controllers").WithName("PlacementGC"),
		Clusters:       clusterManager,
		Interval:       w.gcInterval,
		ControlPlaneID: options.placementControlPlaneID,
	}); err != nil {
		return fmt.Errorf("add placement GC runnable: %w", err)
	}

	// Program the global endpoint to the placement winner. The Gateway API scheme
	// is required for the HTTPRoute the publisher writes; register it here
	// (idempotent) since the control plane may not have EnableGatewayAPI set for
	// its own (empty) ingress config.
	utilruntime.Must(gatewayapiv1.Install(mgr.GetScheme()))
	setupLog.Info("Setting up multi-cluster endpoint publisher")
	if err := (&placementendpoint.Reconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("PlacementEndpoint"),
		Publisher: placementendpoint.NewGatewayAPIPublisher(mgr.GetClient(), w.endpoint),
		Config:    w.endpoint,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("create PlacementEndpoint controller: %w", err)
	}
	return nil
}
