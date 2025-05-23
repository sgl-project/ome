package hub

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Environment variable helpers
var envVarsTrueValues = map[string]bool{
	"1":    true,
	"ON":   true,
	"YES":  true,
	"TRUE": true,
}

func isTrue(value string) bool {
	if value == "" {
		return false
	}
	return envVarsTrueValues[strings.ToUpper(value)]
}

func asInt(value string, defaultVal int) int {
	if value == "" {
		return defaultVal
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultVal
}

// Default values and constants following huggingface_hub patterns
const (
	// Default endpoints
	DefaultEndpoint = "https://huggingface.co"
	DefaultRevision = "main"

	// Cache directory
	DefaultCacheDir = ".cache/huggingface/hub"

	// Request timeouts
	DefaultRequestTimeout = 10 * time.Second
	DefaultEtagTimeout    = 10 * time.Second
	DownloadTimeout       = 10 * time.Minute

	// Download configuration
	DefaultMaxWorkers    = 8
	DefaultChunkSize     = 10 * 1024 * 1024 // 10MB
	DefaultMaxRetries    = 5
	DefaultRetryInterval = 10 * time.Second

	// File size thresholds
	LfsFileSizeThreshold = 10 * 1024 * 1024 // 10MB

	// Repository types
	RepoTypeModel   = "model"
	RepoTypeDataset = "dataset"
	RepoTypeSpace   = "space"

	// Headers
	UserAgentHeader              = "User-Agent"
	AuthorizationHeader          = "Authorization"
	HuggingfaceHeaderXRepoCommit = "X-Repo-Commit"
	HuggingfaceHeaderXLinkedEtag = "X-Linked-Etag"
	HuggingfaceHeaderXLinkedSize = "X-Linked-Size"

	// URL patterns
	RepoIdSeparator = "--"

	// URL construction helpers
	HuggingfaceCoURLTemplate = "%s/%s/resolve/%s/%s"
	ApiModelsURL             = "%s/api/models/%s"
	ApiDatasetsURL           = "%s/api/datasets/%s"
	ApiSpacesURL             = "%s/api/spaces/%s"
	ApiRepoTreeURL           = "%s/api/models/%s/tree/%s"

	// File download constants
	PytorchWeightsName    = "pytorch_model.bin"
	TF2WeightsName        = "tf_model.h5"
	TFWeightsName         = "model.ckpt"
	FlaxWeightsName       = "flax_model.msgpack"
	ConfigName            = "config.json"
	RepocardName          = "README.md"
	DownloadChunkSize     = 10 * 1024 * 1024 // 10MB
	HFTransferConcurrency = 100
	MaxHTTPDownloadSize   = 50 * 1000 * 1000 * 1000 // 50GB

	// Serialization constants
	PytorchWeightsFilePattern     = "pytorch_model{suffix}.bin"
	SafetensorsWeightsFilePattern = "model{suffix}.safetensors"
	TF2WeightsFilePattern         = "tf_model{suffix}.h5"

	// Safetensors constants
	SafetensorsSingleFile      = "model.safetensors"
	SafetensorsIndexFile       = "model.safetensors.index.json"
	SafetensorsMaxHeaderLength = 25_000_000
)

// Repository types
var RepoTypes = []string{RepoTypeModel, RepoTypeDataset, RepoTypeSpace}

// Environment variables
const (
	EnvHfToken              = "HF_TOKEN"
	EnvHfHome               = "HF_HOME"
	EnvHuggingfaceHubCache  = "HUGGINGFACE_HUB_CACHE"
	EnvHfHubCache           = "HF_HUB_CACHE"
	EnvHfHubOffline         = "HF_HUB_OFFLINE"
	EnvHfHubDisableProgress = "HF_HUB_DISABLE_PROGRESS_BARS"
)

// Repo URL prefixes for different types
var RepoTypesURLPrefixes = map[string]string{
	RepoTypeDataset: "datasets/",
	RepoTypeSpace:   "spaces/",
}

// GetCacheDir returns the cache directory, checking environment variables first
func GetCacheDir() string {
	// Check HF_HUB_CACHE first
	if cacheDir := os.Getenv(EnvHfHubCache); cacheDir != "" {
		return cacheDir
	}

	// Check HUGGINGFACE_HUB_CACHE
	if cacheDir := os.Getenv(EnvHuggingfaceHubCache); cacheDir != "" {
		return cacheDir
	}

	// Check HF_HOME
	if hfHome := os.Getenv(EnvHfHome); hfHome != "" {
		return filepath.Join(hfHome, "hub")
	}

	// Default to user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultCacheDir
	}

	return filepath.Join(homeDir, DefaultCacheDir)
}

// GetHfToken returns the HF token from environment
func GetHfToken() string {
	return os.Getenv(EnvHfToken)
}

// IsOfflineMode checks if offline mode is enabled
func IsOfflineMode() bool {
	return os.Getenv(EnvHfHubOffline) == "1" || os.Getenv(EnvHfHubOffline) == "true"
}
