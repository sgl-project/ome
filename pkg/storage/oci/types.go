package oci

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// Config represents OCI storage configuration
type Config struct {
	CompartmentID  string `json:"compartment_id"`
	Region         string `json:"region"`
	EnableOboToken bool   `json:"enable_obo_token"`
	OboToken       string `json:"obo_token"`

	// HTTP client configuration
	HTTPTimeout         time.Duration `json:"http_timeout"`
	MaxIdleConns        int           `json:"max_idle_conns"`
	MaxIdleConnsPerHost int           `json:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
}

// DefaultConfig returns default OCI storage configuration
func DefaultConfig() *Config {
	return &Config{
		HTTPTimeout:         20 * time.Minute,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		MaxConnsPerHost:     200,
	}
}

// PrepareDownloadPart holds info needed to construct a GetObjectRequest at download time
type PrepareDownloadPart struct {
	Namespace string
	Bucket    string
	Object    string
	ByteRange string
	Offset    int64
	PartNum   int
	Size      int64
}

// DownloadedPart contains the data downloaded from object storage
type DownloadedPart struct {
	Size         int64
	TempFilePath string // Path to temporary file containing the data
	Offset       int64
	PartNum      int
	Err          error
}

// FileToDownload represents a file to be downloaded
type FileToDownload struct {
	Namespace      string
	BucketName     string
	ObjectName     string
	TargetFilePath string
}

// DownloadedFile represents a downloaded file result
type DownloadedFile struct {
	Source         FileToDownload
	TargetFilePath string
	Err            error
}

// createObjectStorageClient creates an OCI Object Storage client
func createObjectStorageClient(configProvider common.ConfigurationProvider, config *Config) (*objectstorage.ObjectStorageClient, error) {
	common.EnableInstanceMetadataServiceLookup()

	var client objectstorage.ObjectStorageClient
	var err error

	if config.EnableOboToken {
		if config.OboToken == "" {
			return nil, fmt.Errorf("oboToken is empty but EnableOboToken is true")
		}
		client, err = objectstorage.NewObjectStorageClientWithOboToken(configProvider, config.OboToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create ObjectStorageClient with OBO token: %w", err)
		}
	} else {
		client, err = objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to create ObjectStorageClient: %w", err)
		}
	}

	// Configure HTTP client
	client.BaseClient.HTTPClient = &http.Client{
		Timeout: config.HTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        config.MaxIdleConns,
			MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
			MaxConnsPerHost:     config.MaxConnsPerHost,
		},
	}

	if config.Region != "" {
		client.SetRegion(config.Region)
	}

	return &client, nil
}
