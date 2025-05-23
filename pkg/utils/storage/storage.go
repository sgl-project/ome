package storage

import (
	"fmt"
	"strings"

	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
)

const (
	// OCIStoragePrefix is the prefix for OCI storage URIs
	OCIStoragePrefix = "oci://"
	// PVCStoragePrefix is the prefix for PVC storage URIs
	PVCStoragePrefix = "pvc://"
	// VendorStoragePrefix is the prefix for vendor storage URIs
	VendorStoragePrefix = "vendor://"
	// HuggingFaceStoragePrefix is the prefix for Hugging Face model storage URIs
	HuggingFaceStoragePrefix = "hf://"
)

// StorageType is a string enum for storage type
type StorageType string

const (
	// StorageTypePVC is the value for PVC storage
	StorageTypePVC StorageType = "PVC"
	// StorageTypeOCI is the value for OCI storage
	StorageTypeOCI StorageType = "OCI"
	// StorageTypeVendor is the value for Vendor storage
	StorageTypeVendor StorageType = "VENDOR"
	// StorageTypeHuggingFace is the value for Hugging Face model storage
	StorageTypeHuggingFace StorageType = "HUGGINGFACE"
)

// OCIStorageComponents represents the components of an OCI storage URI
type OCIStorageComponents struct {
	Namespace  string
	Bucket     string
	Prefix     string
	ObjectName string
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

// HuggingFaceStorageComponents represents the components of a Hugging Face model URI
type HuggingFaceStorageComponents struct {
	ModelID string
	Branch  string
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

// ParseHuggingFaceStorageURI parses a Hugging Face model URI and returns its components
// Format: hf://{model-id}[@{branch}]
func ParseHuggingFaceStorageURI(uri string) (*HuggingFaceStorageComponents, error) {
	if !strings.HasPrefix(uri, HuggingFaceStoragePrefix) {
		return nil, fmt.Errorf("invalid Hugging Face storage URI format: missing %s prefix", HuggingFaceStoragePrefix)
	}

	// Remove prefix
	path := strings.TrimPrefix(uri, HuggingFaceStoragePrefix)
	if path == "" {
		return nil, fmt.Errorf("invalid Hugging Face storage URI format: missing model ID")
	}

	// Split into model ID and branch
	var modelID, branch string
	if strings.Contains(path, "@") {
		parts := strings.SplitN(path, "@", 2)
		modelID = parts[0]
		branch = parts[1]
	} else {
		modelID = path
		branch = "main" // Default to 'main' branch if not specified
	}

	if modelID == "" {
		return nil, fmt.Errorf("invalid Hugging Face storage URI format: model ID cannot be empty")
	}

	return &HuggingFaceStorageComponents{
		ModelID: modelID,
		Branch:  branch,
	}, nil
}

// ValidateHuggingFaceStorageURI validates if the given URI matches Hugging Face model storage format
func ValidateHuggingFaceStorageURI(uri string) error {
	_, err := ParseHuggingFaceStorageURI(uri)
	return err
}

// GetStorageType determines the type of storage URI
func GetStorageType(uri string) (StorageType, error) {
	switch {
	case strings.HasPrefix(uri, OCIStoragePrefix):
		return StorageTypeOCI, nil
	case strings.HasPrefix(uri, PVCStoragePrefix):
		return StorageTypePVC, nil
	case strings.HasPrefix(uri, VendorStoragePrefix):
		return StorageTypeVendor, nil
	case strings.HasPrefix(uri, HuggingFaceStoragePrefix):
		return StorageTypeHuggingFace, nil
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
	case StorageTypeOCI:
		return ValidateOCIStorageURI(uri)
	case StorageTypePVC:
		return ValidatePVCStorageURI(uri)
	case StorageTypeVendor:
		return ValidateVendorStorageURI(uri)
	case StorageTypeHuggingFace:
		return ValidateHuggingFaceStorageURI(uri)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// NewObjectURI creates a new ObjectURI from a storage URI string
// Example URI formats:
// - oci://namespace@region/bucket/prefix
// - oci://n/namespace/b/bucket/o/prefix
// - hf://model-id[@branch]
// NewObjectURI creates a new ObjectURI from a storage URI string
func NewObjectURI(uriStr string) (*ociobjectstore.ObjectURI, error) {
	storageType, err := GetStorageType(uriStr)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case StorageTypeOCI:
		return parseOCIObjectURI(uriStr)
	case StorageTypeHuggingFace:
		return parseHuggingFaceObjectURI(uriStr)
	default:
		return nil, fmt.Errorf("unsupported storage type for object URI: %s", storageType)
	}
}

// parseHuggingFaceObjectURI parses a Hugging Face URI into an ObjectURI
func parseHuggingFaceObjectURI(uriStr string) (*ociobjectstore.ObjectURI, error) {
	hfComponents, err := ParseHuggingFaceStorageURI(uriStr)
	if err != nil {
		return nil, err
	}

	// For Hugging Face models:
	// - Use BucketName field to store the model ID
	// - Use Prefix field to store the branch
	// - Use Namespace to identify this as a Hugging Face resource
	return &ociobjectstore.ObjectURI{
		Namespace:  "huggingface", // Identifies this as a Hugging Face model
		BucketName: hfComponents.ModelID,
		Prefix:     hfComponents.Branch,
	}, nil
}

// parseOCIObjectURI parses an OCI URI string into an ObjectURI
func parseOCIObjectURI(uriStr string) (*ociobjectstore.ObjectURI, error) {
	if !strings.HasPrefix(uriStr, OCIStoragePrefix) {
		return nil, fmt.Errorf("URI must start with '%s'", OCIStoragePrefix)
	}

	// Remove the scheme
	uriStr = strings.TrimPrefix(uriStr, "oci://")

	// Check for the OCI specific format: n/namespace/b/bucket/o/prefix
	if strings.HasPrefix(uriStr, "n/") {
		// Format: n/namespace/b/bucket/o/prefix
		parts := strings.Split(uriStr, "/")
		if len(parts) < 5 || parts[0] != "n" || parts[2] != "b" {
			return nil, fmt.Errorf("invalid OCI URI format, expected 'oci://n/namespace/b/bucket/o/prefix': %s", uriStr)
		}

		namespace := parts[1]
		bucketName := parts[3]

		// Validate the marker for the object prefix
		if len(parts) > 4 && parts[4] != "o" {
			return nil, fmt.Errorf("invalid OCI URI format, expected 'oci://n/namespace/b/bucket/o/prefix': %s", uriStr)
		}

		// Extract object prefix (everything after "o/")
		var prefix string
		if len(parts) > 5 && parts[4] == "o" {
			prefix = strings.Join(parts[5:], "/")
		}

		return &ociobjectstore.ObjectURI{
			Namespace:  namespace,
			BucketName: bucketName,
			Prefix:     prefix,
		}, nil
	}

	// Handle different standard formats (from previous fix)
	var namespace, region, bucketName, prefix string

	// Check if we have a namespace@region format
	if strings.Contains(uriStr, "@") {
		parts := strings.SplitN(uriStr, "@", 2)
		namespace = parts[0]

		// Split the rest into region and bucket/prefix
		remainingParts := strings.SplitN(parts[1], "/", 2)
		region = remainingParts[0]

		if len(remainingParts) > 1 {
			// Split remaining into bucket and prefix
			bucketPrefixParts := strings.SplitN(remainingParts[1], "/", 2)
			bucketName = bucketPrefixParts[0]

			if len(bucketPrefixParts) > 1 {
				prefix = bucketPrefixParts[1]
			}
		}
	} else {
		// Handle simpler oci://bucket/prefix format
		parts := strings.SplitN(uriStr, "/", 2)
		bucketName = parts[0]

		if len(parts) > 1 {
			prefix = parts[1]
		}
	}

	// Ensure we have at least a bucket name
	if bucketName == "" {
		return nil, fmt.Errorf("invalid URI format, missing bucket name: %s", uriStr)
	}

	return &ociobjectstore.ObjectURI{
		Namespace:  namespace,
		BucketName: bucketName,
		Prefix:     prefix,
		Region:     region,
	}, nil
}
