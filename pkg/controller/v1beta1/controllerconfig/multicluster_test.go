package controllerconfig

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/ome/pkg/constants"
)

func TestNewMultiClusterConfig(t *testing.T) {
	tests := []struct {
		name          string
		configMapData map[string]string
		expectedError bool
		validate      func(*testing.T, *MultiClusterConfig)
	}{
		{
			name:          "missing configmap",
			configMapData: nil,
			expectedError: true,
		},
		{
			name:          "absent multicluster block yields zero config",
			configMapData: map[string]string{},
			validate: func(t *testing.T, cfg *MultiClusterConfig) {
				// Every knob zero/empty: each consumer applies its own default.
				assert.False(t, cfg.WorkloadCluster.CacheEnabled)
				assert.Equal(t, time.Duration(0), cfg.WorkloadCluster.HealthIntervalDuration())
				assert.Equal(t, time.Duration(0), cfg.Placement.RequeueIntervalDuration())
				assert.Equal(t, 0, cfg.Placement.MaxConcurrentReconciles)
				assert.Equal(t, "", cfg.Placement.DispatcherMode)
				assert.Equal(t, "", cfg.Endpoint.GlobalGateway)
			},
		},
		{
			name: "full block parses every field",
			configMapData: map[string]string{
				MultiClusterConfigName: `{
					"workloadCluster": {
						"clientQPS": 50,
						"clientBurst": 100,
						"perCallTimeout": "15s",
						"cacheEnabled": true,
						"healthInterval": "30s",
						"connectionGrace": "90s",
						"eventsBatchPeriod": "200ms",
						"establishInitial": "2m",
						"establishMax": "20m",
						"reconnectRetryMax": "5m",
						"funnelResyncInterval": "250ms",
						"funnelBufferSize": 64
					},
					"placement": {
						"requeueInterval": "45s",
						"gcInterval": "10m",
						"maxConcurrentReconciles": 8,
						"fanoutTimeout": "20s",
						"winnerLostGrace": "2m",
						"statusBatchPeriod": "500ms",
						"statusSafetyRequeue": "5m",
						"dispatcherMode": "Incremental",
						"dispatcherStepSize": 4,
						"dispatcherRoundTimeout": "30s"
					},
					"endpoint": {
						"globalHostTemplate": "{{.Name}}.{{.Namespace}}.global.example",
						"globalGateway": "ome/global-gateway",
						"routeNamespace": "ome-routes",
						"backendPort": 8080
					}
				}`,
			},
			validate: func(t *testing.T, cfg *MultiClusterConfig) {
				wc := cfg.WorkloadCluster
				assert.Equal(t, 50.0, wc.ClientQPS)
				assert.Equal(t, 100, wc.ClientBurst)
				assert.Equal(t, 15*time.Second, wc.PerCallTimeoutDuration())
				assert.True(t, wc.CacheEnabled)
				assert.Equal(t, 30*time.Second, wc.HealthIntervalDuration())
				assert.Equal(t, 90*time.Second, wc.ConnectionGraceDuration())
				assert.Equal(t, 200*time.Millisecond, wc.EventsBatchPeriodDuration())
				assert.Equal(t, 2*time.Minute, wc.EstablishInitialDuration())
				assert.Equal(t, 20*time.Minute, wc.EstablishMaxDuration())
				assert.Equal(t, 5*time.Minute, wc.ReconnectRetryMaxDuration())
				assert.Equal(t, 250*time.Millisecond, wc.FunnelResyncIntervalDuration())
				assert.Equal(t, 64, wc.FunnelBufferSize)

				pl := cfg.Placement
				assert.Equal(t, 45*time.Second, pl.RequeueIntervalDuration())
				assert.Equal(t, 10*time.Minute, pl.GCIntervalDuration())
				assert.Equal(t, 8, pl.MaxConcurrentReconciles)
				assert.Equal(t, 20*time.Second, pl.FanoutTimeoutDuration())
				assert.Equal(t, 2*time.Minute, pl.WinnerLostGraceDuration())
				assert.Equal(t, 500*time.Millisecond, pl.StatusBatchPeriodDuration())
				assert.Equal(t, 5*time.Minute, pl.StatusSafetyRequeueDuration())
				assert.Equal(t, "Incremental", pl.DispatcherMode)
				assert.Equal(t, 4, pl.DispatcherStepSize)
				assert.Equal(t, 30*time.Second, pl.DispatcherRoundTimeoutDuration())

				ep := cfg.Endpoint
				assert.Equal(t, "{{.Name}}.{{.Namespace}}.global.example", ep.GlobalHostTemplate)
				assert.Equal(t, "ome/global-gateway", ep.GlobalGateway)
				assert.Equal(t, "ome-routes", ep.RouteNamespace)
				assert.Equal(t, 8080, ep.BackendPort)
			},
		},
		{
			name: "empty and malformed durations resolve to zero",
			configMapData: map[string]string{
				MultiClusterConfigName: `{
					"workloadCluster": {"healthInterval": "", "connectionGrace": "not-a-duration"},
					"placement": {"requeueInterval": "0s"}
				}`,
			},
			validate: func(t *testing.T, cfg *MultiClusterConfig) {
				assert.Equal(t, time.Duration(0), cfg.WorkloadCluster.HealthIntervalDuration())
				assert.Equal(t, time.Duration(0), cfg.WorkloadCluster.ConnectionGraceDuration())
				// non-positive parses but is treated as "use the package default".
				assert.Equal(t, time.Duration(0), cfg.Placement.RequeueIntervalDuration())
			},
		},
		{
			name: "invalid json errors",
			configMapData: map[string]string{
				MultiClusterConfigName: `{"workloadCluster": {`,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			if tt.configMapData != nil {
				_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.TODO(), &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.InferenceServiceConfigMapName,
						Namespace: constants.OMENamespace,
					},
					Data: tt.configMapData,
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			cfg, err := NewMultiClusterConfig(clientset)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}
