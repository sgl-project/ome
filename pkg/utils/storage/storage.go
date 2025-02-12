package storage

import (
	"fmt"
	"strings"
)

const (
	// OCIStoragePrefix is the prefix for OCI storage URIs
	OCIStoragePrefix = "oci://"
	// PVCStoragePrefix is the prefix for PVC storage URIs
	PVCStoragePrefix = "pvc://"
	// VendorStoragePrefix is the prefix for vendor storage URIs
	VendorStoragePrefix = "vendor://"
)

// OCIStorageComponents represents the components of an OCI storage URI
type OCIStorageComponents struct {
	Namespace string
	Bucket    string
	Prefix    string
}

// PVCStorageComponents represents the components of a PVC storage URI
type PVCStorageComponents struct {
	PVCName string
	SubPath string
}

// VendorStorageComponents represents the components of a vendor storage URI
type VendorStorageComponents struct {
	VendorName   string
	ResourceType string
	ResourcePath string
}

// ParseOCIStorageURI parses an OCI storage URI and returns its components
// Format: oci://n/{namespace}/b/{bucket}/o/{object_path}
func ParseOCIStorageURI(uri string) (*OCIStorageComponents, error) {
	if !strings.HasPrefix(uri, OCIStoragePrefix) {
		return nil, fmt.Errorf("invalid OCI storage URI format: missing %s prefix", OCIStoragePrefix)
	}

	parts := strings.Split(strings.TrimPrefix(uri, OCIStoragePrefix), "/")
	if len(parts) < 6 || parts[0] != "n" || parts[2] != "b" || parts[4] != "o" {
		return nil, fmt.Errorf("invalid OCI storage URI format. Expected: oci://n/{namespace}/b/{bucket}/o/{object_path}")
	}

	return &OCIStorageComponents{
		Namespace: parts[1],
		Bucket:    parts[3],
		Prefix:    strings.Join(parts[5:], "/"),
	}, nil
}

// ValidateOCIStorageURI validates if the given URI matches OCI storage format
func ValidateOCIStorageURI(uri string) error {
	_, err := ParseOCIStorageURI(uri)
	return err
}

// ParsePVCStorageURI parses a PVC storage URI and returns its components
// Format: pvc://{pvc-name}/{sub-path}
func ParsePVCStorageURI(uri string) (*PVCStorageComponents, error) {
	if !strings.HasPrefix(uri, PVCStoragePrefix) {
		return nil, fmt.Errorf("invalid PVC storage URI format: missing %s prefix", PVCStoragePrefix)
	}

	// Remove prefix
	path := strings.TrimPrefix(uri, PVCStoragePrefix)
	if path == "" {
		return nil, fmt.Errorf("invalid PVC storage URI format: missing PVC name")
	}

	// Split into PVC name and subpath
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid PVC storage URI format: missing PVC name")
	}

	// Require both PVC name and subpath
	if len(parts) < 2 || parts[1] == "" {
		return nil, fmt.Errorf("invalid PVC storage URI format: missing subpath")
	}

	return &PVCStorageComponents{
		PVCName: parts[0],
		SubPath: parts[1],
	}, nil
}

// ValidatePVCStorageURI validates if the given URI matches PVC storage format
func ValidatePVCStorageURI(uri string) error {
	_, err := ParsePVCStorageURI(uri)
	return err
}

// ParseVendorStorageURI parses a vendor storage URI and returns its components
// Format: vendor://{vendor-name}/{resource-type}/{resource-path}
func ParseVendorStorageURI(uri string) (*VendorStorageComponents, error) {
	if !strings.HasPrefix(uri, VendorStoragePrefix) {
		return nil, fmt.Errorf("invalid vendor storage URI format: missing %s prefix", VendorStoragePrefix)
	}

	// Remove prefix
	path := strings.TrimPrefix(uri, VendorStoragePrefix)
	if path == "" {
		return nil, fmt.Errorf("invalid vendor storage URI format: missing vendor name")
	}

	// Split into components
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("invalid vendor storage URI format. Expected: vendor://{vendor-name}/{resource-type}/{resource-path}")
	}

	return &VendorStorageComponents{
		VendorName:   parts[0],
		ResourceType: parts[1],
		ResourcePath: parts[2],
	}, nil
}

// ValidateVendorStorageURI validates if the given URI matches vendor storage format
func ValidateVendorStorageURI(uri string) error {
	_, err := ParseVendorStorageURI(uri)
	return err
}

// GetStorageType determines the type of storage URI
func GetStorageType(uri string) (string, error) {
	switch {
	case strings.HasPrefix(uri, OCIStoragePrefix):
		return "OCI", nil
	case strings.HasPrefix(uri, PVCStoragePrefix):
		return "PVC", nil
	case strings.HasPrefix(uri, VendorStoragePrefix):
		return "VENDOR", nil
	default:
		return "", fmt.Errorf("unknown storage type for URI: %s", uri)
	}
}

// ValidateStorageURI validates a storage URI based on its type
func ValidateStorageURI(uri string) error {
	storageType, err := GetStorageType(uri)
	if err != nil {
		return err
	}

	switch storageType {
	case "OCI":
		return ValidateOCIStorageURI(uri)
	case "PVC":
		return ValidatePVCStorageURI(uri)
	case "VENDOR":
		return ValidateVendorStorageURI(uri)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
}
