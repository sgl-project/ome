package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	omefake "github.com/sgl-project/sgl-ome/pkg/client/clientset/versioned/fake"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/rest"
)

func TestConfigInitialization(t *testing.T) {
	// Create a new command instance for testing
	testCmd := &cobra.Command{
		Use:   "start",
		Short: "Starts model controller",
		Long:  `Starts the model controller to watch and updates all the baseModels`,
		Run:   runCommand,
	}

	// Reset and initialize flags
	testCmd.Flags().StringVar(&namespace, "namespace", "ome", "namespace to create the leader election lock")
	testCmd.Flags().StringVar(&controllerName, "controller-name", "ome-model-controller", "the name of this controller")
	testCmd.Flags().StringVar(&agentNamespace, "agent-namespace", "ome", "the namespace of the model agents")

	// Verify default values
	assert.Equal(t, "ome", namespace)
	assert.Equal(t, "ome-model-controller", controllerName)
	assert.Equal(t, "ome", agentNamespace)
}

func TestInitializeLogger(t *testing.T) {
	logger, err := initializeLogger()
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestCreateKubeClient(t *testing.T) {
	config := &rest.Config{
		Host: "http://localhost:8080",
	}
	client := createKubeClient(config)
	require.NotNil(t, client)
}

func TestCreateOmeClient(t *testing.T) {
	config := &rest.Config{
		Host: "http://localhost:8080",
	}
	client := createOmeClient(config)
	require.NotNil(t, client)
}

type mockHealthCheck struct {
	healthy bool
}

func (m *mockHealthCheck) Name() string {
	return "mock-health-check"
}

func (m *mockHealthCheck) Check(_ *http.Request) error {
	if !m.healthy {
		return fmt.Errorf("health check failed")
	}
	return nil
}

func TestHealthCheckEndpoint(t *testing.T) {
	// Create health checker
	checker := &mockHealthCheck{healthy: true}

	mux := http.NewServeMux()
	healthz.InstallHandler(mux, checker)
	healthz.InstallReadyzHandler(mux, healthz.PingHealthz)

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test health check endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test readiness endpoint
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/readyz", port))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, server.Shutdown(ctx))

	// Check if there were any server errors
	select {
	case err := <-errChan:
		t.Fatalf("server error: %v", err)
	default:
	}
}

func TestLeaderElectionConfig(t *testing.T) {
	// Test leader election configuration
	assert.Equal(t, 15*time.Second, leaseDuration)
	assert.Equal(t, 5*time.Second, renewDuration)
	assert.Equal(t, 3*time.Second, retryPeriod)
	assert.Equal(t, 8080, healthCheckPort)
	assert.Equal(t, 20*time.Second, leaderHealthzAdaptorTimeout)
}

func TestGetKubeConfig(t *testing.T) {
	// Check if we're running in a cluster by looking for the service account token file
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	inCluster := err == nil

	if inCluster {
		// When running in cluster, getKubeConfig should return a valid config
		config := getKubeConfig()
		assert.NotNil(t, config)
	} else {
		// When not running in cluster, getKubeConfig should panic
		assert.Panics(t, func() {
			getKubeConfig()
		}, "getKubeConfig() should panic when not running in a cluster")
	}
}

func TestCheckCRDExists(t *testing.T) {
	logger, err := initializeLogger()
	require.NoError(t, err)
	client := omefake.NewSimpleClientset()

	// Test CRD check with mock client
	result := checkCRDExists(client, logger)
	assert.True(t, result, "CRD should exist in mock client")
}
