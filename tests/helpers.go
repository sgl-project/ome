package integration_tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/gomega"
)

// TestLogger provides a simple logging function for tests
var TestLogger = struct {
	Enabled bool
	Debug   func(format string, args ...interface{})
	Info    func(format string, args ...interface{})
	Error   func(format string, args ...interface{})
}{
	Enabled: true,
	Debug: func(format string, args ...interface{}) {
		if os.Getenv("TEST_DEBUG") == "true" {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	},
	Info: func(format string, args ...interface{}) {
		fmt.Printf("[INFO] "+format+"\n", args...)
	},
	Error: func(format string, args ...interface{}) {
		fmt.Printf("[ERROR] "+format+"\n", args...)
	},
}

// RunAgent runs the agent with the given arguments and returns stdout, stderr, and error
func RunAgent(binaryPath string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	TestLogger.Info("Running command: %s %v", binaryPath, args)
	err := cmd.Run()
	if err != nil {
		TestLogger.Error("Command failed with error: %v", err)
	} else {
		TestLogger.Info("Command completed successfully")
	}

	return stdout.String(), stderr.String(), err
}

// RunAgentWithTimeout runs the agent with a timeout and returns stdout, stderr, and error
func RunAgentWithTimeout(timeout time.Duration, binaryPath string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// RunAgentWithEnv runs the agent with the given environment variables and returns stdout, stderr, and error
func RunAgentWithEnv(binaryPath string, env map[string]string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binaryPath, args...)

	// Set up environment variables
	cmd.Env = os.Environ() // Start with current environment
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// CreateTempConfig creates a temporary config file with the given content
func CreateTempConfig(content string) string {
	tempFile, err := os.CreateTemp("", "ome-agent-config-*.yaml")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp config file")

	_, err = tempFile.WriteString(content)
	Expect(err).NotTo(HaveOccurred(), "Failed to write to temp config file")

	err = tempFile.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to close temp config file")

	return tempFile.Name()
}

// CreateTempFile creates a temporary file with the given content and extension
func CreateTempFile(content, extension string) string {
	tempFile, err := os.CreateTemp("", fmt.Sprintf("ome-agent-test-*%s", extension))
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file")

	_, err = tempFile.WriteString(content)
	Expect(err).NotTo(HaveOccurred(), "Failed to write to temp file")

	err = tempFile.Close()
	Expect(err).NotTo(HaveOccurred(), "Failed to close temp file")

	return tempFile.Name()
}

// CreateTempDir creates a temporary directory and returns its path
func CreateTempDir(prefix string) string {
	tempDir, err := os.MkdirTemp("", prefix)
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp directory")
	return tempDir
}

// CreateFileInDir creates a file with the given name and content in the specified directory
func CreateFileInDir(dir, name, content string) string {
	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	Expect(err).NotTo(HaveOccurred(), "Failed to create file in directory")
	return filePath
}

// CreateMockConfig creates a mock config file for the specified agent
func CreateMockConfig(agentType string) string {
	var content string

	switch agentType {
	case "enigma":
		content = `
debug: true
# Mock enigma agent config
encryption:
  key_id: mock-key-id
  region: us-west-2
`
	case "hf-download":
		content = `
debug: true
# Mock HF download agent config
endpoint: https://huggingface.co
model_name: gpt2
local_path: /tmp/models
`
	case "replica":
		content = `
debug: true
# Mock replica agent config
source:
  bucket: source-bucket
  region: us-west-2
destination:
  bucket: dest-bucket
  region: us-east-1
`

	case "serving-agent":
		content = `
debug: true
# Mock serving sidecar config
model_id: gpt2
port: 8080
`
	case "fine-tuned-adapter":
		content = `
debug: true
# Mock fine-tuned adapter config
base_model: /path/to/base
adapter: /path/to/adapter
output: /path/to/output
`
	default:
		content = `
debug: true
# Generic mock config
`
	}

	return CreateTempConfig(content)
}

// CreateDetailedMockConfig creates a more detailed mock config file for the specified agent
// with real paths and additional configuration options
func CreateDetailedMockConfig(agentType string, mockDataDir string) string {
	var content string

	switch agentType {
	case "enigma":
		content = fmt.Sprintf(`
debug: true
# Detailed mock enigma agent config
encryption:
  key_id: mock-key-id
  region: us-west-2
  provider: mock
input_file: %s/input.bin
output_file: %s/output.bin
`, mockDataDir, mockDataDir)

	case "hf-download":
		content = fmt.Sprintf(`
debug: true
# Detailed mock HF download agent config
endpoint: https://huggingface.co
model_name: gpt2
local_path: %s
revision: main
use_auth: false
`, mockDataDir)

	case "replica":
		content = fmt.Sprintf(`
debug: true
# Detailed mock replica agent config
source:
  bucket: source-bucket
  region: us-west-2
  namespace: mock-namespace
  prefix: models/
destination:
  bucket: dest-bucket
  region: us-east-1
  namespace: mock-namespace
  prefix: models/
temp_dir: %s
`, mockDataDir)

	case "serving-agent":
		content = fmt.Sprintf(`
debug: true
# Detailed mock serving sidecar config
model_id: gpt2
port: 8080
model_path: %s/model
cache_dir: %s/cache
max_batch_size: 4
`, mockDataDir, mockDataDir)

	case "fine-tuned-adapter":
		content = fmt.Sprintf(`
debug: true
# Detailed mock fine-tuned adapter config
base_model: %s/base
adapter: %s/adapter
output: %s/output
format: safetensors
`, mockDataDir, mockDataDir, mockDataDir)

	default:
		content = fmt.Sprintf(`
debug: true
# Generic detailed mock config
temp_dir: %s
`, mockDataDir)
	}

	return CreateTempConfig(content)
}

// SetupMockDataForAgent creates necessary mock data files for testing a specific agent
func SetupMockDataForAgent(agentType string, mockDataDir string) {
	switch agentType {
	case "enigma":
		// Create mock input file for encryption/decryption
		CreateFileInDir(mockDataDir, "input.bin", "mock model data for encryption")

	case "hf-download":
		// Create mock model structure
		modelDir := filepath.Join(mockDataDir, "models")
		err := os.MkdirAll(modelDir, 0755)
		Expect(err).NotTo(HaveOccurred(), "Failed to create model directory")

		// Create a mock model file
		CreateFileInDir(modelDir, "config.json", `{"model_type": "gpt2", "vocab_size": 50257}`)

	case "fine-tuned-adapter":
		// Create mock base model and adapter directories
		baseDir := filepath.Join(mockDataDir, "base")
		adapterDir := filepath.Join(mockDataDir, "adapter")
		outputDir := filepath.Join(mockDataDir, "output")

		for _, dir := range []string{baseDir, adapterDir, outputDir} {
			err := os.MkdirAll(dir, 0755)
			Expect(err).NotTo(HaveOccurred(), "Failed to create directory")
		}

		// Create mock model files
		CreateFileInDir(baseDir, "model.safetensors", "mock base model weights")
		CreateFileInDir(adapterDir, "adapter_model.safetensors", "mock adapter weights")
	}
}
