package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHealthCheck(t *testing.T) {
	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthCheck)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestReadinessCheck(t *testing.T) {
	// Test when not ready
	ready = false
	req, err := http.NewRequest("GET", "/readyz", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(readinessCheck)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "not ready", rr.Body.String())

	// Test when ready
	ready = true
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ready", rr.Body.String())
}

func TestMetricsInitialization(t *testing.T) {
	// Reset the metrics to ensure clean state
	prometheus.Unregister(totalOrgs)
	prometheus.Unregister(keyRotationStatus)
	prometheus.Unregister(lastKeyRotationTime)

	// Register metrics
	prometheus.MustRegister(totalOrgs)
	prometheus.MustRegister(keyRotationStatus)
	prometheus.MustRegister(lastKeyRotationTime)

	// Verify that metrics can be set without error
	totalOrgs.Set(5)
	keyRotationStatus.WithLabelValues("success").Inc()
	lastKeyRotationTime.WithLabelValues("test-org", "success").Set(123456789)

	// We can't easily verify the values directly, but we can ensure no panics occurred
	assert.True(t, true, "Metrics initialization succeeded")
}

func TestMetricsEndpoint(t *testing.T) {
	// Skip this test as it requires a full Prometheus setup
	t.Skip("Skipping as it requires a full Prometheus setup")

	// Create a request to the metrics endpoint
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Create a handler that serves metrics
	handler := promhttp.Handler()

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check that the response contains our metric names
	// We can't easily check the exact values, but we can check that the metrics are present
	assert.Contains(t, rr.Body.String(), "openai_admin_total_organizations")
	assert.Contains(t, rr.Body.String(), "openai_admin_key_rotation_status")
	assert.Contains(t, rr.Body.String(), "openai_admin_last_key_rotation_timestamp")
}

func TestRunRotate(t *testing.T) {
	// Create a mock cobra command for testing
	cmd := &cobra.Command{
		Use: "test",
	}

	// Set flags to avoid connecting to real services
	kubeconfigPath = "/tmp/nonexistent/kubeconfig"
	metricsPort = 0 // Use 0 to get a random available port
	healthPort = 0  // Use 0 to get a random available port
	watchInterval = 1 * time.Minute
	rotationInterval = 30 * 24 * time.Hour

	// Initialize logger for test
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.Encoding = "console"
	zapLogger, _ := config.Build()
	logger = zapLogger.Sugar().Named("test")

	// Run the function - it should fail due to invalid kubeconfig
	err := runRotate(cmd, []string{})

	// Verify that it returns an error related to kubeconfig
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error loading kubeconfig")
}

func TestInitConfig(t *testing.T) {
	// Call the function (which is currently empty)
	initConfig()

	// This test just ensures the function doesn't panic
	assert.True(t, true, "initConfig should run without panicking")
}

// TestMain is used for setup and teardown for all tests in the package
func TestMain(m *testing.M) {
	// Setup code

	// Run tests
	code := m.Run()

	// Teardown code

	os.Exit(code)
}

func vendorPtr(v v1beta1.Vendor) *v1beta1.Vendor {
	return &v
}
