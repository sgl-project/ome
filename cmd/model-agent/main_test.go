package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func setupTestEnv(t *testing.T) {
	t.Helper()
	// Save original env and restore after test
	originalNodeName := os.Getenv("NODE_NAME")
	t.Cleanup(func() {
		os.Setenv("NODE_NAME", originalNodeName)
	})
	os.Setenv("NODE_NAME", "test-node")
}

func TestConfigInitialization(t *testing.T) {
	setupTestEnv(t)
	// Reset config before each test
	cfg = config{}

	tests := []struct {
		name          string
		setupEnv      func()
		expectedPanic bool
	}{
		{
			name: "valid NODE_NAME",
			setupEnv: func() {
				os.Setenv("NODE_NAME", "test-node")
				initConfig(nil, nil)
			},
			expectedPanic: false,
		},
		{
			name: "missing NODE_NAME",
			setupEnv: func() {
				os.Unsetenv("NODE_NAME")
			},
			expectedPanic: true,
		},
		{
			name: "empty NODE_NAME",
			setupEnv: func() {
				os.Setenv("NODE_NAME", "")
			},
			expectedPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedPanic {
				assert.Panics(t, func() {
					tt.setupEnv()
					initConfig(nil, nil)
				})
			} else {
				assert.NotPanics(t, func() {
					tt.setupEnv()
					initConfig(nil, nil)
				})
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	setupTestEnv(t)
	// Create a new command instance for testing
	testCmd := &cobra.Command{
		Use:   "start",
		Short: "Starts the model agent",
		Long:  `Starts the model agent to watch the base model custom resources and update the node labels`,
		Run:   runCommand,
	}

	// Reset config and initialize
	cfg = config{}

	// Set up default flags
	testCmd.Flags().IntVar(&cfg.port, "health-check-port", 8080, "Address for readiness and liveness health check")
	testCmd.Flags().StringVar(&cfg.modelsRootDirOnHost, "models-root-dir-on-host", "/raid/models", "host's root dir for storing all models")
	testCmd.Flags().StringVar(&cfg.modelsRootDir, "models-root-dir", "/raid/models", "container's root dir for storing all models")
	testCmd.Flags().IntVar(&cfg.nodeLabelRetry, "node-label-retry", 2, "retry times for node label update")
	testCmd.Flags().IntVar(&cfg.downloadRetry, "download-retry", 3, "retry times for model download")
	testCmd.Flags().StringVar(&cfg.downloadAuthType, "download-auth-type", "instance-principal", "authentication method for model download")
	testCmd.Flags().IntVar(&cfg.numDownloadWorker, "num-download-worker", 3, "number of download workers")
	testCmd.Flags().StringVar(&cfg.namespace, "namespace", "ome", "the namespace of the ome model agents daemon set")

	// Call initConfig directly
	initConfig(testCmd, nil)

	// Verify default values
	assert.Equal(t, 8080, cfg.port)
	assert.Equal(t, "/raid/models", cfg.modelsRootDir)
	assert.Equal(t, "/raid/models", cfg.modelsRootDirOnHost)
	assert.Equal(t, 2, cfg.nodeLabelRetry)
	assert.Equal(t, 3, cfg.downloadRetry)
	assert.Equal(t, "instance-principal", cfg.downloadAuthType)
	assert.Equal(t, 3, cfg.numDownloadWorker)
	assert.Equal(t, "ome", cfg.namespace)
	assert.Equal(t, "test-node", cfg.nodeName)
}

func TestInitializeLogger(t *testing.T) {
	setupTestEnv(t)
	logger, err := initializeLogger()
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestSetupHealthServer(t *testing.T) {
	setupTestEnv(t)
	logger, _ := initializeLogger()
	server := setupServer(8080, "/tmp", logger)
	require.NotNil(t, server)
	assert.Equal(t, ":8080", server.Addr)

	// Test server starts and handles requests
	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Close()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get("http://localhost:8080/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateKubeClient(t *testing.T) {
	setupTestEnv(t)
	config := &rest.Config{
		Host: "http://localhost:8080",
	}
	client := createKubeClient(config)
	require.NotNil(t, client)
}

func TestGetNodeShape(t *testing.T) {
	setupTestEnv(t)
	tests := []struct {
		name          string
		nodeName      string
		setupNode     func() *corev1.Node
		expectedShape string
		shouldPanic   bool
	}{
		{
			name:     "valid node shape",
			nodeName: "test-node",
			setupNode: func() *corev1.Node {
				return &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							constants.NodeInstanceShapeLabel: "VM.Standard.E4.Flex",
						},
					},
				}
			},
			expectedShape: "VM.Standard.E4.Flex",
			shouldPanic:   false,
		},
		{
			name:     "missing shape label",
			nodeName: "test-node",
			setupNode: func() *corev1.Node {
				return &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test-node",
						Labels: map[string]string{},
					},
				}
			},
			shouldPanic: true,
		},
		{
			name:     "empty shape label",
			nodeName: "test-node",
			setupNode: func() *corev1.Node {
				return &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							constants.NodeInstanceShapeLabel: "",
						},
					},
				}
			},
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test node
			client := fake.NewSimpleClientset(tt.setupNode())

			if tt.shouldPanic {
				assert.Panics(t, func() {
					getNodeShape(client, tt.nodeName)
				})
			} else {
				shape := getNodeShape(client, tt.nodeName)
				assert.Equal(t, tt.expectedShape, shape)
			}
		})
	}
}

func TestCreateOmeClient(t *testing.T) {
	setupTestEnv(t)
	config := &rest.Config{
		Host: "http://localhost:8080",
	}
	client := createOmeClient(config)
	require.NotNil(t, client)
}

func TestHealthCheckEndpoint(t *testing.T) {
	setupTestEnv(t)
	logger, _ := initializeLogger()
	server := setupServer(18080, "/tmp", logger)
	require.NotNil(t, server)

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test health check endpoint
	resp, err := http.Get("http://localhost:18080/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test livez endpoint
	resp, err = http.Get("http://localhost:18080/livez")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, server.Shutdown(ctx))
}
