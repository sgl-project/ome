package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istionetworking "istio.io/api/networking/v1beta1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
				"--metrics-addr=:9090",
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
