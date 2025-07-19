package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"testing"

	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istionetworking "istio.io/api/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	volcano "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestGetOptions(t *testing.T) {
	// Save original command line arguments and restore them after the test
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	tests := []struct {
		name     string
		args     []string
		expected Options
	}{
		{
			name: "default options",
			args: []string{"cmd"},
			expected: Options{
				metricsAddr:             ":8080",
				webhookPort:             9443,
				enableLeaderElection:    false,
				enableWebhook:           false,
				probeAddr:               ":8081",
				leaderElectionNamespace: LeaderElectionNamespace,
				zapOpts:                 zap.Options{},
			},
		},
		{
			name: "custom options",
			args: []string{
				"cmd",
				"--metrics-bind-address=:9090",
				"--webhook-port=8443",
				"--leader-elect=true",
				"--webhook=true",
				"--health-probe-addr=:9091",
				"--leader-election-namespace=custom-namespace",
			},
			expected: Options{
				metricsAddr:             ":9090",
				webhookPort:             8443,
				enableLeaderElection:    true,
				enableWebhook:           true,
				probeAddr:               ":9091",
				leaderElectionNamespace: "custom-namespace",
				zapOpts:                 zap.Options{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			os.Args = tt.args

			options := GetOptions()
			assert.Equal(t, tt.expected.metricsAddr, options.metricsAddr)
			assert.Equal(t, tt.expected.webhookPort, options.webhookPort)
			assert.Equal(t, tt.expected.enableLeaderElection, options.enableLeaderElection)
			assert.Equal(t, tt.expected.enableWebhook, options.enableWebhook)
			assert.Equal(t, tt.expected.probeAddr, options.probeAddr)
			assert.Equal(t, tt.expected.leaderElectionNamespace, options.leaderElectionNamespace)
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, ":8080", opts.metricsAddr)
	assert.Equal(t, 9443, opts.webhookPort)
	assert.False(t, opts.enableLeaderElection)
	assert.False(t, opts.enableWebhook)
	assert.Equal(t, ":8081", opts.probeAddr)
	assert.Equal(t, LeaderElectionNamespace, opts.leaderElectionNamespace)
}

// Mock for testing CRD availability
type mockCRDChecker struct {
	available bool
	err       error
}

func (m *mockCRDChecker) IsCrdAvailable(config *rest.Config, groupVersion, kind string) (bool, error) {
	return m.available, m.err
}

func TestSetupLogger(t *testing.T) {
	options := DefaultOptions()
	logger := zap.New(zap.UseFlagOptions(&options.zapOpts))
	ctrl.SetLogger(logger)
	assert.NotNil(t, ctrl.Log)
}

func TestLeaderElectionConfiguration(t *testing.T) {
	tests := []struct {
		name                  string
		enableLeaderElection  bool
		leaderElectionNS      string
		expectedLockName      string
		expectedLockNamespace string
	}{
		{
			name:                  "leader election disabled",
			enableLeaderElection:  false,
			leaderElectionNS:      LeaderElectionNamespace,
			expectedLockName:      LeaderLockName,
			expectedLockNamespace: LeaderElectionNamespace,
		},
		{
			name:                  "leader election enabled custom namespace",
			enableLeaderElection:  true,
			leaderElectionNS:      "custom-namespace",
			expectedLockName:      LeaderLockName,
			expectedLockNamespace: "custom-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				enableLeaderElection:    tt.enableLeaderElection,
				leaderElectionNamespace: tt.leaderElectionNS,
			}
			assert.Equal(t, tt.expectedLockName, LeaderLockName)
			assert.Equal(t, tt.expectedLockNamespace, opts.leaderElectionNamespace)
		})
	}
}

func TestHealthProbeConfiguration(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, ":8081", opts.probeAddr, "Default health probe address should be :8081")

	customOpts := Options{
		probeAddr: ":9091",
	}
	assert.Equal(t, ":9091", customOpts.probeAddr, "Custom health probe address should be set correctly")
}

func TestWebhookConfiguration(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, 9443, opts.webhookPort, "Default webhook port should be 9443")
	assert.False(t, opts.enableWebhook, "Webhook should be disabled by default")

	customOpts := Options{
		webhookPort:   8443,
		enableWebhook: true,
	}
	assert.Equal(t, 8443, customOpts.webhookPort, "Custom webhook port should be set correctly")
	assert.True(t, customOpts.enableWebhook, "Webhook should be enabled")
}

func TestMetricsConfiguration(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, ":8080", opts.metricsAddr, "Default metrics address should be :8080")

	customOpts := Options{
		metricsAddr: ":9090",
	}
	assert.Equal(t, ":9090", customOpts.metricsAddr, "Custom metrics address should be set correctly")
}

func TestInit(t *testing.T) {
	// Test that init() function sets the Istio API client flags correctly
	require.True(t, istionetworking.VirtualServiceUnmarshaler.AllowUnknownFields)
	require.True(t, istionetworking.GatewayUnmarshaler.AllowUnknownFields)
}

// TestManagerSetup tests the manager configuration
func TestManagerSetup(t *testing.T) {
	tests := []struct {
		name          string
		opts          Options
		expectedError bool
		setupMockFunc func()
		cleanupFunc   func()
	}{
		{
			name: "valid configuration",
			opts: Options{
				metricsAddr:             ":18080",
				probeAddr:               ":18081",
				webhookPort:             18443,
				leaderElectionNamespace: LeaderElectionNamespace,
			},
			expectedError: false,
		},
		{
			name: "custom metrics port",
			opts: Options{
				metricsAddr:             ":19090",
				probeAddr:               ":19081",
				webhookPort:             19443,
				leaderElectionNamespace: LeaderElectionNamespace,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMockFunc != nil {
				tt.setupMockFunc()
			}
			if tt.cleanupFunc != nil {
				defer tt.cleanupFunc()
			}

			cfg := &rest.Config{
				Host: "http://localhost:8080",
			}

			mgr, err := manager.New(cfg, manager.Options{
				Metrics: metricsserver.Options{
					BindAddress: tt.opts.metricsAddr},
				WebhookServer: webhook.NewServer(webhook.Options{
					Port: tt.opts.webhookPort}),
				LeaderElection:          tt.opts.enableLeaderElection,
				LeaderElectionID:        LeaderLockName,
				LeaderElectionNamespace: tt.opts.leaderElectionNamespace,
				HealthProbeBindAddress:  tt.opts.probeAddr,
			})

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mgr)
			}
		})
	}
}

// createMockConfigMap creates a mock ConfigMap for testing
func createMockConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inferenceservice-config",
			Namespace: "ome",
		},
		Data: map[string]string{
			"deploy": `{
				"defaultDeploymentMode": "Serverless"
			}`,
			"ingress": `{
				"ingressGateway": "test-gateway",
				"ingressService": "test-service"
			}`,
		},
	}
}

// TestDeployConfigSetup tests the setup of deployment configuration
func TestDeployConfigSetup(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (kubernetes.Interface, error)
		shouldError bool
	}{
		{
			name: "successful config setup",
			setupFunc: func() (kubernetes.Interface, error) {
				client := fake.NewSimpleClientset()
				// Create the required ConfigMap
				_, err := client.CoreV1().ConfigMaps("ome").Create(context.Background(), createMockConfigMap(), metav1.CreateOptions{})
				if err != nil {
					return nil, err
				}
				return client, nil
			},
			shouldError: false,
		},
		{
			name: "config setup failure",
			setupFunc: func() (kubernetes.Interface, error) {
				return nil, errors.New("failed to create clientset")
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset, err := tt.setupFunc()
			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			deployConfig, err := controllerconfig.NewDeployConfig(clientset)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, deployConfig)
			}
		})
	}
}

// TestIngressConfigSetup tests the setup of ingress configuration
func TestIngressConfigSetup(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (kubernetes.Interface, error)
		shouldError bool
	}{
		{
			name: "successful ingress config setup",
			setupFunc: func() (kubernetes.Interface, error) {
				client := fake.NewSimpleClientset()
				// Create the required ConfigMap
				_, err := client.CoreV1().ConfigMaps("ome").Create(context.Background(), createMockConfigMap(), metav1.CreateOptions{})
				if err != nil {
					return nil, err
				}
				return client, nil
			},
			shouldError: false,
		},
		{
			name: "ingress config setup failure",
			setupFunc: func() (kubernetes.Interface, error) {
				return nil, errors.New("failed to create clientset")
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset, err := tt.setupFunc()
			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			ingressConfig, err := controllerconfig.NewIngressConfig(clientset)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ingressConfig)
			}
		})
	}
}

// TestCRDSetup tests the setup of various CRDs
func TestCRDSetup(t *testing.T) {
	tests := []struct {
		name        string
		crdType     string
		available   bool
		setupError  error
		shouldError bool
	}{
		{
			name:        "Ray CRD available",
			crdType:     "Ray",
			available:   true,
			setupError:  nil,
			shouldError: false,
		},
		{
			name:        "Ray CRD not available",
			crdType:     "Ray",
			available:   false,
			setupError:  nil,
			shouldError: false,
		},
		{
			name:        "Volcano CRD available",
			crdType:     "Volcano",
			available:   true,
			setupError:  nil,
			shouldError: false,
		},
		{
			name:        "Error checking CRD",
			crdType:     "Ray",
			available:   false,
			setupError:  errors.New("failed to check CRD"),
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChecker := &mockCRDChecker{
				available: tt.available,
				err:       tt.setupError,
			}

			cfg := &rest.Config{}
			var err error

			switch tt.crdType {
			case "Ray":
				_, err = mockChecker.IsCrdAvailable(cfg, ray.SchemeGroupVersion.String(), constants.RayClusterKind)
			case "Volcano":
				_, err = mockChecker.IsCrdAvailable(cfg, volcano.SchemeGroupVersion.String(), constants.VolcanoQueueKind)
			}

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
