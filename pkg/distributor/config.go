package distributor

import (
	"fmt"
)

// Config holds the configuration for the P2P model distributor.
type Config struct {
	// DataDir is the root directory for storing model files.
	// This should be a hostPath mount to enable resume across pod restarts.
	DataDir string

	// Namespace is the Kubernetes namespace where the model-agent runs.
	Namespace string

	// PodName is the name of the current pod (used as lease holder identity).
	PodName string

	// PodIP is the IP address of the current pod.
	PodIP string

	// PeersService is the DNS name of the headless service for peer discovery.
	// Format: service-name.namespace.svc.cluster.local
	PeersService string

	// TorrentPort is the port for BitTorrent peer connections.
	TorrentPort int

	// MetainfoPort is the HTTP port for serving .torrent metainfo files.
	MetainfoPort int

	// MaxDownloadRate is the maximum download rate in bytes per second.
	// 0 means unlimited.
	MaxDownloadRate int64

	// MaxUploadRate is the maximum upload rate in bytes per second.
	// 0 means unlimited.
	MaxUploadRate int64

	// EnableEncryption enables BitTorrent header obfuscation.
	EnableEncryption bool

	// RequireEncryption requires all peer connections to use encryption.
	RequireEncryption bool

	// LeaseDurationSeconds is the duration of the download coordination lease.
	LeaseDurationSeconds int32

	// LeaseRenewIntervalSeconds is how often to renew the lease.
	LeaseRenewIntervalSeconds int

	// P2PTimeoutSeconds is the timeout for P2P download attempts.
	P2PTimeoutSeconds int

	// EnableP2P controls whether P2P distribution is enabled.
	// When disabled, downloads go directly to HuggingFace.
	EnableP2P bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		DataDir:                   "/mnt/models",
		Namespace:                 "ome",
		TorrentPort:               6881,
		MetainfoPort:              8081,
		MaxDownloadRate:           2 * 1024 * 1024 * 1024, // 2 GB/s
		MaxUploadRate:             2 * 1024 * 1024 * 1024, // 2 GB/s
		EnableEncryption:          false,
		RequireEncryption:         false,
		LeaseDurationSeconds:      120, // 2 minutes
		LeaseRenewIntervalSeconds: 30,
		P2PTimeoutSeconds:         30,
		EnableP2P:                 true,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("DataDir is required")
	}
	if c.Namespace == "" {
		return fmt.Errorf("Namespace is required")
	}
	if c.PodName == "" {
		return fmt.Errorf("PodName is required")
	}
	if c.PodIP == "" {
		return fmt.Errorf("PodIP is required")
	}
	if c.TorrentPort <= 0 || c.TorrentPort > 65535 {
		return fmt.Errorf("TorrentPort must be between 1 and 65535")
	}
	if c.MetainfoPort <= 0 || c.MetainfoPort > 65535 {
		return fmt.Errorf("MetainfoPort must be between 1 and 65535")
	}
	if c.TorrentPort == c.MetainfoPort {
		return fmt.Errorf("TorrentPort and MetainfoPort must be different")
	}
	if c.MaxDownloadRate < 0 {
		return fmt.Errorf("MaxDownloadRate must be non-negative")
	}
	if c.MaxUploadRate < 0 {
		return fmt.Errorf("MaxUploadRate must be non-negative")
	}
	if c.LeaseDurationSeconds <= 0 {
		return fmt.Errorf("LeaseDurationSeconds must be positive")
	}

	return nil
}

// WithDefaults returns a copy of the config with missing values filled in from defaults.
// Note: MaxDownloadRate and MaxUploadRate are NOT set here because 0 means "unlimited".
// Callers should explicitly set rate limits if they want non-unlimited rates.
func (c Config) WithDefaults() Config {
	defaults := DefaultConfig()

	if c.DataDir == "" {
		c.DataDir = defaults.DataDir
	}
	if c.Namespace == "" {
		c.Namespace = defaults.Namespace
	}
	if c.TorrentPort == 0 {
		c.TorrentPort = defaults.TorrentPort
	}
	if c.MetainfoPort == 0 {
		c.MetainfoPort = defaults.MetainfoPort
	}
	// Do not set default for MaxDownloadRate and MaxUploadRate if they are 0,
	// as 0 is a valid value indicating "unlimited".
	if c.LeaseDurationSeconds == 0 {
		c.LeaseDurationSeconds = defaults.LeaseDurationSeconds
	}
	if c.LeaseRenewIntervalSeconds == 0 {
		c.LeaseRenewIntervalSeconds = defaults.LeaseRenewIntervalSeconds
	}
	if c.P2PTimeoutSeconds == 0 {
		c.P2PTimeoutSeconds = defaults.P2PTimeoutSeconds
	}

	return c
}
