package storage

import (
	"fmt"
	"net/url"
	"strings"
)

// URIScheme defines supported URI schemes
type URIScheme string

const (
	SchemeOCI    URIScheme = "oci"
	SchemeS3     URIScheme = "s3"
	SchemeGS     URIScheme = "gs"     // Google Cloud Storage
	SchemeAzure  URIScheme = "azure"  // Azure Blob Storage
	SchemeGitHub URIScheme = "github" // GitHub LFS
)

// ParseURI parses a storage URI and returns an ObjectURI
func ParseURI(uriStr string) (*ObjectURI, error) {
	u, err := url.Parse(uriStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	scheme := URIScheme(u.Scheme)

	switch scheme {
	case SchemeOCI:
		return parseOCIURI(uriStr)
	case SchemeS3:
		return parseS3URI(u)
	case SchemeGS:
		return parseGSURI(u)
	case SchemeAzure:
		return parseAzureURI(u)
	case SchemeGitHub:
		return parseGitHubURI(u)
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}
}

// parseOCIURI parses OCI storage URIs
// Formats:
// - oci://namespace@region/bucket/prefix
// - oci://n/namespace/b/bucket/o/prefix
func parseOCIURI(uriStr string) (*ObjectURI, error) {
	if !strings.HasPrefix(uriStr, "oci://") {
		return nil, fmt.Errorf("invalid OCI URI: must start with oci://")
	}

	// Remove scheme
	path := strings.TrimPrefix(uriStr, "oci://")

	// Check for OCI specific format: n/namespace/b/bucket/o/prefix
	if strings.HasPrefix(path, "n/") {
		parts := strings.Split(path, "/")
		if len(parts) < 5 || parts[0] != "n" || parts[2] != "b" {
			return nil, fmt.Errorf("invalid OCI URI format")
		}

		namespace := parts[1]
		bucket := parts[3]

		var prefix string
		if len(parts) > 5 && parts[4] == "o" {
			prefix = strings.Join(parts[5:], "/")
		}

		return &ObjectURI{
			Provider:   ProviderOCI,
			Namespace:  namespace,
			BucketName: bucket,
			Prefix:     prefix,
		}, nil
	}

	// Handle namespace@region/bucket/prefix format
	var namespace, region, bucket, prefix string

	if strings.Contains(path, "@") {
		parts := strings.SplitN(path, "@", 2)
		namespace = parts[0]

		remainingParts := strings.SplitN(parts[1], "/", 2)
		region = remainingParts[0]

		if len(remainingParts) > 1 {
			bucketPrefixParts := strings.SplitN(remainingParts[1], "/", 2)
			bucket = bucketPrefixParts[0]

			if len(bucketPrefixParts) > 1 {
				prefix = bucketPrefixParts[1]
			}
		}
	} else {
		// Simple bucket/prefix format
		parts := strings.SplitN(path, "/", 2)
		bucket = parts[0]

		if len(parts) > 1 {
			prefix = parts[1]
		}
	}

	if bucket == "" {
		return nil, fmt.Errorf("invalid OCI URI: missing bucket name")
	}

	return &ObjectURI{
		Provider:   ProviderOCI,
		Namespace:  namespace,
		BucketName: bucket,
		Prefix:     prefix,
		Region:     region,
	}, nil
}

// parseS3URI parses S3 URIs
// Format: s3://bucket/prefix
func parseS3URI(u *url.URL) (*ObjectURI, error) {
	if u.Host == "" {
		return nil, fmt.Errorf("invalid S3 URI: missing bucket name")
	}

	prefix := strings.TrimPrefix(u.Path, "/")

	// Extract object name from prefix if it's a single object
	var objectName string
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		parts := strings.Split(prefix, "/")
		objectName = parts[len(parts)-1]
		if len(parts) > 1 {
			prefix = strings.Join(parts[:len(parts)-1], "/")
		} else {
			prefix = ""
		}
	}

	return &ObjectURI{
		Provider:   ProviderAWS,
		BucketName: u.Host,
		ObjectName: objectName,
		Prefix:     prefix,
	}, nil
}

// parseGSURI parses Google Cloud Storage URIs
// Format: gs://bucket/prefix
func parseGSURI(u *url.URL) (*ObjectURI, error) {
	if u.Host == "" {
		return nil, fmt.Errorf("invalid GCS URI: missing bucket name")
	}

	prefix := strings.TrimPrefix(u.Path, "/")

	// Extract object name from prefix if it's a single object
	var objectName string
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		parts := strings.Split(prefix, "/")
		objectName = parts[len(parts)-1]
		if len(parts) > 1 {
			prefix = strings.Join(parts[:len(parts)-1], "/")
		} else {
			prefix = ""
		}
	}

	return &ObjectURI{
		Provider:   ProviderGCP,
		BucketName: u.Host,
		ObjectName: objectName,
		Prefix:     prefix,
	}, nil
}

// parseAzureURI parses Azure Blob Storage URIs
// Format: azure://container@account/prefix
func parseAzureURI(u *url.URL) (*ObjectURI, error) {
	// URL parser treats container@account as userinfo@host
	// So container is in u.User and account is in u.Host
	var account, container, prefix string

	if u.User != nil {
		container = u.User.Username()
		account = u.Host
		prefix = strings.TrimPrefix(u.Path, "/")
	} else if u.Host != "" {
		// Fallback: check if @ is in the host
		if strings.Contains(u.Host, "@") {
			parts := strings.SplitN(u.Host, "@", 2)
			container = parts[0]
			account = parts[1]
			prefix = strings.TrimPrefix(u.Path, "/")
		} else {
			return nil, fmt.Errorf("invalid Azure URI: must contain account (container@account)")
		}
	} else {
		return nil, fmt.Errorf("invalid Azure URI: must contain account (container@account)")
	}

	if container == "" || account == "" {
		return nil, fmt.Errorf("invalid Azure URI: missing container or account")
	}

	// Extract object name from prefix if it's a single object
	var objectName string
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		parts := strings.Split(prefix, "/")
		objectName = parts[len(parts)-1]
		if len(parts) > 1 {
			prefix = strings.Join(parts[:len(parts)-1], "/")
		} else {
			prefix = ""
		}
	}

	return &ObjectURI{
		Provider:   ProviderAzure,
		BucketName: container,
		ObjectName: objectName,
		Prefix:     prefix,
		Extra: map[string]interface{}{
			"account": account,
		},
	}, nil
}

// parseGitHubURI parses GitHub LFS URIs
// Format: github://owner/repo@branch/path
func parseGitHubURI(u *url.URL) (*ObjectURI, error) {
	path := u.Host + u.Path

	// Extract owner/repo and branch
	var owner, repo, branch, filePath string

	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URI: must contain owner/repo")
	}

	owner = parts[0]

	// Check for branch
	if strings.Contains(parts[1], "@") {
		repoParts := strings.SplitN(parts[1], "@", 2)
		repo = repoParts[0]

		if len(repoParts) > 1 {
			pathParts := strings.SplitN(repoParts[1], "/", 2)
			branch = pathParts[0]

			if len(pathParts) > 1 {
				filePath = pathParts[1]
			}
		}
	} else {
		pathParts := strings.SplitN(parts[1], "/", 2)
		repo = pathParts[0]
		branch = "main" // Default branch

		if len(pathParts) > 1 {
			filePath = pathParts[1]
		}
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid GitHub URI: missing owner or repo")
	}

	return &ObjectURI{
		Provider:   ProviderGitHub,
		BucketName: fmt.Sprintf("%s/%s", owner, repo),
		ObjectName: filePath,
		Extra: map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"branch": branch,
		},
	}, nil
}

// ToURI converts an ObjectURI back to a URI string
func (u *ObjectURI) ToURI() string {
	switch u.Provider {
	case ProviderOCI:
		if u.Namespace != "" && u.Region != "" {
			return fmt.Sprintf("oci://%s@%s/%s/%s", u.Namespace, u.Region, u.BucketName, u.Prefix)
		} else if u.Namespace != "" {
			return fmt.Sprintf("oci://n/%s/b/%s/o/%s", u.Namespace, u.BucketName, u.Prefix)
		}
		return fmt.Sprintf("oci://%s/%s", u.BucketName, u.Prefix)

	case ProviderAWS:
		if u.ObjectName != "" {
			return fmt.Sprintf("s3://%s/%s/%s", u.BucketName, u.Prefix, u.ObjectName)
		}
		return fmt.Sprintf("s3://%s/%s", u.BucketName, u.Prefix)

	case ProviderGCP:
		if u.ObjectName != "" {
			return fmt.Sprintf("gs://%s/%s/%s", u.BucketName, u.Prefix, u.ObjectName)
		}
		return fmt.Sprintf("gs://%s/%s", u.BucketName, u.Prefix)

	case ProviderAzure:
		if account, ok := u.Extra["account"].(string); ok {
			if u.ObjectName != "" {
				return fmt.Sprintf("azure://%s@%s/%s/%s", u.BucketName, account, u.Prefix, u.ObjectName)
			}
			return fmt.Sprintf("azure://%s@%s/%s", u.BucketName, account, u.Prefix)
		}
		return fmt.Sprintf("azure://%s/%s", u.BucketName, u.Prefix)

	case ProviderGitHub:
		if owner, ok := u.Extra["owner"].(string); ok {
			if repo, ok := u.Extra["repo"].(string); ok {
				if branch, ok := u.Extra["branch"].(string); ok && branch != "main" {
					return fmt.Sprintf("github://%s/%s@%s/%s", owner, repo, branch, u.ObjectName)
				}
				return fmt.Sprintf("github://%s/%s/%s", owner, repo, u.ObjectName)
			}
		}
		return fmt.Sprintf("github://%s/%s", u.BucketName, u.ObjectName)

	default:
		return ""
	}
}
