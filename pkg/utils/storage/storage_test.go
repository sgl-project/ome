package storage

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/stretchr/testify/assert"
)

func TestParseOCIStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *OCIStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with simple path",
			uri:  "oci://n/myns/b/mybucket/o/mypath",
			want: &OCIStorageComponents{
				Namespace: "myns",
				Bucket:    "mybucket",
				Prefix:    "mypath",
			},
			wantErr: false,
		},
		{
			name: "valid uri with nested path",
			uri:  "oci://n/myns/b/mybucket/o/path/to/my/object",
			want: &OCIStorageComponents{
				Namespace: "myns",
				Bucket:    "mybucket",
				Prefix:    "path/to/my/object",
			},
			wantErr: false,
		},
		{
			name: "valid uri with special characters",
			uri:  "oci://n/my-ns.123/b/my_bucket-123/o/path.with.dots/and-dashes",
			want: &OCIStorageComponents{
				Namespace: "my-ns.123",
				Bucket:    "my_bucket-123",
				Prefix:    "path.with.dots/and-dashes",
			},
			wantErr: false,
		},
		{
			name:        "missing oci prefix",
			uri:         "n/myns/b/mybucket/o/mypath",
			wantErr:     true,
			errContains: "missing oci:// prefix",
		},
		{
			name:        "missing namespace marker",
			uri:         "oci://myns/b/mybucket/o/mypath",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
		{
			name:        "missing bucket marker",
			uri:         "oci://n/myns/mybucket/o/mypath",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
		{
			name:        "missing object marker",
			uri:         "oci://n/myns/b/mybucket/mypath",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing oci:// prefix",
		},
		{
			name:        "only prefix",
			uri:         "oci://",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
		{
			name:        "missing path after object marker",
			uri:         "oci://n/myns/b/mybucket/o",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
		{
			name:        "invalid order of markers",
			uri:         "oci://b/mybucket/n/myns/o/mypath",
			wantErr:     true,
			errContains: "invalid OCI storage URI format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOCIStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateOCIStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid uri",
			uri:     "oci://n/myns/b/mybucket/o/mypath",
			wantErr: false,
		},
		{
			name:        "invalid uri",
			uri:         "invalid://uri",
			wantErr:     true,
			errContains: "missing oci:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing oci:// prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOCIStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestParsePVCStorageURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    *PVCStorageComponents
		wantErr bool
	}{
		{
			name: "valid uri without namespace - simple path",
			uri:  "pvc://my-pvc/results",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "results",
			},
			wantErr: false,
		},
		{
			name: "valid uri without namespace - nested path",
			uri:  "pvc://my-pvc/path/to/results",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path/to/results",
			},
			wantErr: false,
		},
		{
			name: "valid uri with namespace using colon separator",
			uri:  "pvc://default:my-pvc/results",
			want: &PVCStorageComponents{
				Namespace: "default",
				PVCName:   "my-pvc",
				SubPath:   "results",
			},
			wantErr: false,
		},
		{
			name: "valid uri with namespace and nested path",
			uri:  "pvc://my-namespace:my-pvc/path/to/results",
			want: &PVCStorageComponents{
				Namespace: "my-namespace",
				PVCName:   "my-pvc",
				SubPath:   "path/to/results",
			},
			wantErr: false,
		},
		{
			name: "valid uri with special characters in pvc name",
			uri:  "pvc://my-pvc-123/path_with-special.chars",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc-123",
				SubPath:   "path_with-special.chars",
			},
			wantErr: false,
		},
		{
			name: "valid uri with namespace and special chars",
			uri:  "pvc://default:my-pvc-123/path_with-special.chars",
			want: &PVCStorageComponents{
				Namespace: "default",
				PVCName:   "my-pvc-123",
				SubPath:   "path_with-special.chars",
			},
			wantErr: false,
		},
		{
			name: "namespace with numbers and hyphens",
			uri:  "pvc://ns-123:pvc-456/model",
			want: &PVCStorageComponents{
				Namespace: "ns-123",
				PVCName:   "pvc-456",
				SubPath:   "model",
			},
			wantErr: false,
		},
		{
			name: "valid uri with ClusterBaseModel use case",
			uri:  "pvc://model-storage:shared-pvc/path/to/models/llama2-7b",
			want: &PVCStorageComponents{
				Namespace: "model-storage",
				PVCName:   "shared-pvc",
				SubPath:   "path/to/models/llama2-7b",
			},
			wantErr: false,
		},
		// Enhanced test cases for various subpath formats
		{
			name: "subpath with file extensions",
			uri:  "pvc://my-pvc/models/llama2-7b.bin",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/llama2-7b.bin",
			},
			wantErr: false,
		},
		{
			name: "subpath with multiple file extensions",
			uri:  "pvc://my-pvc/data/model.tar.gz",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "data/model.tar.gz",
			},
			wantErr: false,
		},
		{
			name: "subpath with unicode characters",
			uri:  "pvc://my-pvc/测试/模型/llama2-7b",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "测试/模型/llama2-7b",
			},
			wantErr: false,
		},
		{
			name: "subpath with spaces",
			uri:  "pvc://my-pvc/my path with spaces",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "my path with spaces",
			},
			wantErr: false,
		},
		{
			name: "subpath with special characters",
			uri:  "pvc://my-pvc/path/with/special@#$%^&*()chars",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path/with/special@#$%^&*()chars",
			},
			wantErr: false,
		},
		{
			name: "subpath with query parameters (treated as part of path)",
			uri:  "pvc://my-pvc/path?param=value",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path?param=value",
			},
			wantErr: false,
		},
		{
			name: "subpath with fragments (treated as part of path)",
			uri:  "pvc://my-pvc/path#fragment",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path#fragment",
			},
			wantErr: false,
		},
		{
			name: "subpath with percent encoding",
			uri:  "pvc://my-pvc/path%20with%20spaces",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path%20with%20spaces",
			},
			wantErr: false,
		},
		{
			name: "subpath with backslashes (treated as regular characters)",
			uri:  "pvc://my-pvc/path\\with\\backslashes",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path\\with\\backslashes",
			},
			wantErr: false,
		},
		{
			name: "subpath with newlines (treated as regular characters)",
			uri:  "pvc://my-pvc/path\nwith\nnewlines",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path\nwith\nnewlines",
			},
			wantErr: false,
		},
		{
			name: "subpath with tabs (treated as regular characters)",
			uri:  "pvc://my-pvc/path\twith\ttabs",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path\twith\ttabs",
			},
			wantErr: false,
		},
		{
			name: "subpath with only dots",
			uri:  "pvc://my-pvc/...",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "...",
			},
			wantErr: false,
		},
		{
			name: "subpath with only hyphens",
			uri:  "pvc://my-pvc/---",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "---",
			},
			wantErr: false,
		},
		{
			name: "subpath with multiple consecutive slashes (normalized)",
			uri:  "pvc://my-pvc/path//to//results",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path//to//results",
			},
			wantErr: false,
		},
		{
			name: "very long subpath",
			uri:  "pvc://my-pvc/" + strings.Repeat("a", 1000),
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   strings.Repeat("a", 1000),
			},
			wantErr: false,
		},
		{
			name: "subpath with leading and trailing slashes",
			uri:  "pvc://my-pvc//path/to/results//",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "/path/to/results//",
			},
			wantErr: false,
		},
		{
			name: "subpath with mixed case",
			uri:  "pvc://my-pvc/Path/To/Results",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "Path/To/Results",
			},
			wantErr: false,
		},
		{
			name: "subpath with numbers",
			uri:  "pvc://my-pvc/models/v1.0.0/checkpoint_123",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/v1.0.0/checkpoint_123",
			},
			wantErr: false,
		},
		{
			name: "subpath with environment-like paths",
			uri:  "pvc://my-pvc/env/prod/models/llama2-7b",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "env/prod/models/llama2-7b",
			},
			wantErr: false,
		},
		{
			name: "subpath with date-based paths",
			uri:  "pvc://my-pvc/backups/2024-01-15/models",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "backups/2024-01-15/models",
			},
			wantErr: false,
		},
		{
			name: "subpath with hash-based paths",
			uri:  "pvc://my-pvc/models/abc123def456",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/abc123def456",
			},
			wantErr: false,
		},
		{
			name: "subpath with versioned paths",
			uri:  "pvc://my-pvc/models/v2.1.3-beta/weights",
			want: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/v2.1.3-beta/weights",
			},
			wantErr: false,
		},
		// Error cases
		{
			name:    "invalid namespace with uppercase",
			uri:     "pvc://MyNamespace:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid namespace with underscore",
			uri:     "pvc://my_namespace:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "namespace starting with hyphen",
			uri:     "pvc://-namespace:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "namespace ending with hyphen",
			uri:     "pvc://namespace-:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "very long namespace (64 chars)",
			uri:     "pvc://a123456789012345678901234567890123456789012345678901234567890123:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty namespace before colon",
			uri:     "pvc://:my-pvc/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty pvc name after colon",
			uri:     "pvc://default:/models",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing pvc prefix",
			uri:     "my-pvc/results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty uri",
			uri:     "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "only prefix",
			uri:     "pvc://",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty pvc name",
			uri:     "pvc:///results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty subpath",
			uri:     "pvc://my-pvc/",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty subpath with namespace",
			uri:     "pvc://default:my-pvc/",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "only pvc name provided",
			uri:     "pvc://my-pvc",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "only namespace and pvc provided",
			uri:     "pvc://default:my-pvc",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - wrong scheme",
			uri:     "oci://my-pvc/results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "multiple colons in namespace:pvc part",
			uri:     "pvc://ns:pvc:extra/path",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "trailing slash in pvc name",
			uri:     "pvc://my-pvc-/results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "leading slash in subpath",
			uri:     "pvc://my-pvc//results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "pvc name with invalid characters (underscore)",
			uri:     "pvc://my_pvc/results",
			wantErr: true,
		},
		{
			name:    "pvc name with uppercase",
			uri:     "pvc://MyPVC/results",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePVCStorageURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePVCStorageURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePVCStorageURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{
			name:      "valid simple namespace",
			namespace: "default",
			want:      true,
		},
		{
			name:      "valid namespace with hyphens",
			namespace: "my-namespace",
			want:      true,
		},
		{
			name:      "valid namespace with numbers",
			namespace: "ns123",
			want:      true,
		},
		{
			name:      "valid namespace with hyphens and numbers",
			namespace: "test-123-namespace",
			want:      true,
		},
		{
			name:      "valid single character",
			namespace: "a",
			want:      true,
		},
		{
			name:      "valid 63 characters",
			namespace: "a123456789012345678901234567890123456789012345678901234567890a",
			want:      true,
		},
		{
			name:      "invalid empty",
			namespace: "",
			want:      false,
		},
		{
			name:      "invalid uppercase",
			namespace: "MyNamespace",
			want:      false,
		},
		{
			name:      "invalid underscore",
			namespace: "my_namespace",
			want:      false,
		},
		{
			name:      "invalid dot",
			namespace: "my.namespace",
			want:      false,
		},
		{
			name:      "invalid starting with hyphen",
			namespace: "-namespace",
			want:      false,
		},
		{
			name:      "invalid ending with hyphen",
			namespace: "namespace-",
			want:      false,
		},
		{
			name:      "invalid too long (64 chars)",
			namespace: "a1234567890123456789012345678901234567890123456789012345678901234",
			want:      false,
		},
		{
			name:      "invalid special characters",
			namespace: "name@space",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidNamespace(tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetStorageType(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        StorageType
		wantErr     bool
		errContains string
	}{
		{
			name: "oci storage",
			uri:  "oci://n/myns/b/mybucket/o/mypath",
			want: StorageTypeOCI,
		},
		{
			name: "pvc storage",
			uri:  "pvc://my-pvc/mypath",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with namespace",
			uri:  "pvc://default:my-pvc/mypath",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with complex subpath",
			uri:  "pvc://my-pvc/path/to/models/llama2-7b.bin",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with special characters in subpath",
			uri:  "pvc://my-pvc/path/with/special@#$%^&*()chars",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with unicode characters",
			uri:  "pvc://my-pvc/测试/模型/llama2-7b",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with spaces in subpath",
			uri:  "pvc://my-pvc/my path with spaces",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with file extensions",
			uri:  "pvc://my-pvc/models/llama2-7b.bin",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with query parameters",
			uri:  "pvc://my-pvc/path?param=value",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with fragments",
			uri:  "pvc://my-pvc/path#fragment",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with percent encoding",
			uri:  "pvc://my-pvc/path%20with%20spaces",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with backslashes",
			uri:  "pvc://my-pvc/path\\with\\backslashes",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with newlines",
			uri:  "pvc://my-pvc/path\nwith\nnewlines",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with tabs",
			uri:  "pvc://my-pvc/path\twith\ttabs",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with multiple consecutive slashes",
			uri:  "pvc://my-pvc/path//to//results",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with leading and trailing slashes",
			uri:  "pvc://my-pvc//path/to/results//",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with mixed case",
			uri:  "pvc://my-pvc/Path/To/Results",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with numbers in subpath",
			uri:  "pvc://my-pvc/models/v1.0.0/checkpoint_123",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with environment-like paths",
			uri:  "pvc://my-pvc/env/prod/models/llama2-7b",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with date-based paths",
			uri:  "pvc://my-pvc/backups/2024-01-15/models",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with hash-based paths",
			uri:  "pvc://my-pvc/models/abc123def456",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with versioned paths",
			uri:  "pvc://my-pvc/models/v2.1.3-beta/weights",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with only dots in subpath",
			uri:  "pvc://my-pvc/...",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with only hyphens in subpath",
			uri:  "pvc://my-pvc/---",
			want: StorageTypePVC,
		},
		{
			name: "pvc storage with very long subpath",
			uri:  "pvc://my-pvc/" + strings.Repeat("a", 1000),
			want: StorageTypePVC,
		},
		{
			name: "vendor storage",
			uri:  "vendor://openai/models/gpt-4",
			want: StorageTypeVendor,
		},
		{
			name: "hugging face storage",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct",
			want: StorageTypeHuggingFace,
		},
		{
			name: "hugging face storage with branch",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct@experimental",
			want: StorageTypeHuggingFace,
		},
		{
			name: "s3 storage",
			uri:  "s3://my-bucket/my-prefix",
			want: StorageTypeS3,
		},
		{
			name: "azure storage",
			uri:  "az://myaccount/mycontainer/myblob",
			want: StorageTypeAzure,
		},
		{
			name: "gcs storage",
			uri:  "gs://my-bucket/my-object",
			want: StorageTypeGCS,
		},
		{
			name: "github storage",
			uri:  "github://owner/repo",
			want: StorageTypeGitHub,
		},
		{
			name:        "unknown storage type",
			uri:         "unknown://something",
			wantErr:     true,
			errContains: "unknown storage type",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "unknown storage type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetStorageType(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateStorageURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name:    "valid oci uri",
			uri:     "oci://n/myns/b/mybucket/o/mypath",
			wantErr: false,
		},
		{
			name:    "valid pvc uri without namespace",
			uri:     "pvc://my-pvc/data",
			wantErr: false,
		},
		{
			name:    "valid pvc uri with namespace",
			uri:     "pvc://default:my-pvc/data",
			wantErr: false,
		},
		{
			name:    "valid vendor uri",
			uri:     "vendor://openai/models/gpt-4",
			wantErr: false,
		},
		{
			name:    "valid hugging face uri - with model ID only",
			uri:     "hf://meta-llama/Llama-3-70B-Instruct",
			wantErr: false,
		},
		{
			name:    "valid hugging face uri - with model ID and branch",
			uri:     "hf://meta-llama/Llama-3-70B-Instruct@experimental",
			wantErr: false,
		},
		{
			name:    "invalid oci uri",
			uri:     "oci://invalid",
			wantErr: true,
		},
		{
			name:    "invalid pvc uri - missing subpath without namespace",
			uri:     "pvc://my-pvc",
			wantErr: true,
		},
		{
			name:    "invalid pvc uri - missing subpath with namespace",
			uri:     "pvc://default:my-pvc",
			wantErr: true,
		},
		{
			name:    "invalid pvc uri - empty subpath",
			uri:     "pvc://my-pvc/",
			wantErr: true,
		},
		{
			name:    "invalid hugging face uri",
			uri:     "hf://",
			wantErr: true,
		},
		{
			name:    "unknown storage type",
			uri:     "unknown://data",
			wantErr: true,
		},
		{
			name:    "empty uri",
			uri:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStorageURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStorageURI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseVendorStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *VendorStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with openai model",
			uri:  "vendor://openai/models/gpt-4",
			want: &VendorStorageComponents{
				VendorName:   "openai",
				ResourceType: "models",
				ResourcePath: "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "valid uri with azure embeddings",
			uri:  "vendor://azure/embeddings/text-embedding-ada-002",
			want: &VendorStorageComponents{
				VendorName:   "azure",
				ResourceType: "embeddings",
				ResourcePath: "text-embedding-ada-002",
			},
			wantErr: false,
		},
		{
			name: "valid uri with nested path",
			uri:  "vendor://anthropic/models/v2/claude-2",
			want: &VendorStorageComponents{
				VendorName:   "anthropic",
				ResourceType: "models",
				ResourcePath: "v2/claude-2",
			},
			wantErr: false,
		},
		{
			name:        "missing vendor prefix",
			uri:         "openai/models/gpt-4",
			wantErr:     true,
			errContains: "missing vendor:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing vendor:// prefix",
		},
		{
			name:        "only prefix",
			uri:         "vendor://",
			wantErr:     true,
			errContains: "missing vendor name",
		},
		{
			name:        "missing resource type",
			uri:         "vendor://openai",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
		{
			name:        "missing resource path",
			uri:         "vendor://openai/models",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
		{
			name:        "empty vendor name",
			uri:         "vendor:///models/gpt-4",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
		{
			name:        "empty resource type",
			uri:         "vendor://openai//gpt-4",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
		{
			name:        "empty resource path",
			uri:         "vendor://openai/models/",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVendorStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.True(t, reflect.DeepEqual(got, tt.want), "expected %+v but got %+v", tt.want, got)
		})
	}
}

func TestValidateVendorStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid uri",
			uri:     "vendor://openai/models/gpt-4",
			wantErr: false,
		},
		{
			name:        "invalid uri",
			uri:         "vendor://openai",
			wantErr:     true,
			errContains: "invalid vendor storage URI format",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing vendor:// prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVendorStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestParseHuggingFaceStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *HuggingFaceStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with model ID only",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct",
			want: &HuggingFaceStorageComponents{
				ModelID: "meta-llama/Llama-3-70B-Instruct",
				Branch:  "main", // Default branch
			},
			wantErr: false,
		},
		{
			name: "valid uri with model ID and branch",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct@alternative",
			want: &HuggingFaceStorageComponents{
				ModelID: "meta-llama/Llama-3-70B-Instruct",
				Branch:  "alternative",
			},
			wantErr: false,
		},
		{
			name: "valid uri with organization and model name",
			uri:  "hf://mistralai/Mixtral-8x7B-Instruct-v0.1",
			want: &HuggingFaceStorageComponents{
				ModelID: "mistralai/Mixtral-8x7B-Instruct-v0.1",
				Branch:  "main", // Default branch
			},
			wantErr: false,
		},
		{
			name: "valid uri with special characters",
			uri:  "hf://user-name/model-version_3.1@dev-branch",
			want: &HuggingFaceStorageComponents{
				ModelID: "user-name/model-version_3.1",
				Branch:  "dev-branch",
			},
			wantErr: false,
		},
		{
			name:        "missing hf prefix",
			uri:         "meta-llama/Llama-3-70B-Instruct",
			wantErr:     true,
			errContains: "missing hf:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing hf:// prefix",
		},
		{
			name:        "only prefix",
			uri:         "hf://",
			wantErr:     true,
			errContains: "missing model ID",
		},
		{
			name:        "empty model ID",
			uri:         "hf://@main",
			wantErr:     true,
			errContains: "model ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHuggingFaceStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateHuggingFaceStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid uri with model ID only",
			uri:     "hf://meta-llama/Llama-3-70B-Instruct",
			wantErr: false,
		},
		{
			name:    "valid uri with model ID and branch",
			uri:     "hf://meta-llama/Llama-3-70B-Instruct@alternative",
			wantErr: false,
		},
		{
			name:        "invalid uri",
			uri:         "invalid://uri",
			wantErr:     true,
			errContains: "missing hf:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing hf:// prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHuggingFaceStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNewObjectURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		expect      *ociobjectstore.ObjectURI
		wantErr     bool
		errContains string
	}{
		// Hugging Face URIs
		{
			name: "valid hugging face uri with model ID only",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct",
			expect: &ociobjectstore.ObjectURI{
				Namespace:  "huggingface",
				BucketName: "meta-llama/Llama-3-70B-Instruct",
				Prefix:     "main", // Default branch when not specified
			},
			wantErr: false,
		},
		{
			name: "valid hugging face uri with model ID and branch",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct@experimental",
			expect: &ociobjectstore.ObjectURI{
				Namespace:  "huggingface",
				BucketName: "meta-llama/Llama-3-70B-Instruct",
				Prefix:     "experimental", // Specified branch
			},
			wantErr: false,
		},
		{
			name:        "invalid hugging face uri - empty model ID",
			uri:         "hf://@branch",
			wantErr:     true,
			errContains: "model ID cannot be empty",
		},
		// OCI URIs
		{
			name:    "valid n/namespace/b/bucket/o/prefix",
			uri:     "oci://n/myns/b/mybucket/o/myprefix",
			expect:  &ociobjectstore.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid n/namespace/b/bucket/o/multi/level/prefix",
			uri:     "oci://n/myns/b/mybucket/o/dir1/dir2/file.txt",
			expect:  &ociobjectstore.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: "dir1/dir2/file.txt"},
			wantErr: false,
		},
		{
			name:    "valid namespace@region/bucket/prefix",
			uri:     "oci://myns@us-phoenix-1/mybucket/myprefix",
			expect:  &ociobjectstore.ObjectURI{Namespace: "myns", Region: "us-phoenix-1", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid namespace@region/bucket with no prefix",
			uri:     "oci://myns@us-phoenix-1/mybucket",
			expect:  &ociobjectstore.ObjectURI{Namespace: "myns", Region: "us-phoenix-1", BucketName: "mybucket", Prefix: ""},
			wantErr: false,
		},
		{
			name:    "valid bucket/prefix (no namespace/region)",
			uri:     "oci://mybucket/myprefix",
			expect:  &ociobjectstore.ObjectURI{Namespace: "", Region: "", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid bucket only (no namespace/region/prefix)",
			uri:     "oci://mybucket",
			expect:  &ociobjectstore.ObjectURI{Namespace: "", Region: "", BucketName: "mybucket", Prefix: ""},
			wantErr: false,
		},
		{
			name:        "missing oci scheme",
			uri:         "n/myns/b/mybucket/o/myprefix",
			wantErr:     true,
			errContains: "unknown storage type",
		},
		{
			name:        "malformed n/namespace/b/bucket/o (too short)",
			uri:         "oci://n/myns/b/mybucket",
			wantErr:     true,
			errContains: "invalid OCI URI format",
		},
		{
			name:        "malformed n/namespace/b/bucket/x/extra",
			uri:         "oci://n/myns/b/mybucket/x/extra",
			wantErr:     true,
			errContains: "invalid OCI URI format",
		},
		{
			name:        "namespace@region missing bucket",
			uri:         "oci://myns@us-phoenix-1",
			wantErr:     true,
			errContains: "missing bucket name",
		},
		{
			name:        "empty string",
			uri:         "",
			wantErr:     true,
			errContains: "unknown storage type",
		},
		{
			name:        "oci:// only",
			uri:         "oci://",
			wantErr:     true,
			errContains: "missing bucket name",
		},
		{
			name:    "oci://n/namespace/b/bucket/o/ (empty prefix)",
			uri:     "oci://n/myns/b/mybucket/o/",
			expect:  &ociobjectstore.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewObjectURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err, "expected error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestParseS3StorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *S3StorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with bucket and prefix",
			uri:  "s3://my-bucket/path/to/object",
			want: &S3StorageComponents{
				Bucket: "my-bucket",
				Prefix: "path/to/object",
				Region: "",
			},
			wantErr: false,
		},
		{
			name: "valid uri with bucket only",
			uri:  "s3://my-bucket",
			want: &S3StorageComponents{
				Bucket: "my-bucket",
				Prefix: "",
				Region: "",
			},
			wantErr: false,
		},
		{
			name: "valid uri with region",
			uri:  "s3://my-bucket@us-east-1/path/to/object",
			want: &S3StorageComponents{
				Bucket: "my-bucket",
				Prefix: "path/to/object",
				Region: "us-east-1",
			},
			wantErr: false,
		},
		{
			name:        "missing s3 prefix",
			uri:         "my-bucket/object",
			wantErr:     true,
			errContains: "missing s3:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing s3:// prefix",
		},
		{
			name:        "only prefix",
			uri:         "s3://",
			wantErr:     true,
			errContains: "missing bucket name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseS3StorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAzureStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *AzureStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with simple format",
			uri:  "az://myaccount/mycontainer/path/to/blob",
			want: &AzureStorageComponents{
				AccountName:   "myaccount",
				ContainerName: "mycontainer",
				BlobPath:      "path/to/blob",
			},
			wantErr: false,
		},
		{
			name: "valid uri with blob endpoint format",
			uri:  "az://myaccount.blob.core.windows.net/mycontainer/path/to/blob",
			want: &AzureStorageComponents{
				AccountName:   "myaccount",
				ContainerName: "mycontainer",
				BlobPath:      "path/to/blob",
			},
			wantErr: false,
		},
		{
			name: "valid uri without blob path",
			uri:  "az://myaccount/mycontainer",
			want: &AzureStorageComponents{
				AccountName:   "myaccount",
				ContainerName: "mycontainer",
				BlobPath:      "",
			},
			wantErr: false,
		},
		{
			name:        "missing container",
			uri:         "az://myaccount",
			wantErr:     true,
			errContains: "missing container name",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing az:// prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAzureStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseGCSStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *GCSStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri with bucket and object",
			uri:  "gs://my-bucket/path/to/object",
			want: &GCSStorageComponents{
				Bucket: "my-bucket",
				Object: "path/to/object",
			},
			wantErr: false,
		},
		{
			name: "valid uri with bucket only",
			uri:  "gs://my-bucket",
			want: &GCSStorageComponents{
				Bucket: "my-bucket",
				Object: "",
			},
			wantErr: false,
		},
		{
			name:        "missing gs prefix",
			uri:         "my-bucket/object",
			wantErr:     true,
			errContains: "missing gs:// prefix",
		},
		{
			name:        "empty bucket",
			uri:         "gs://",
			wantErr:     true,
			errContains: "missing bucket name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGCSStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseGitHubStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		want        *GitHubStorageComponents
		wantErr     bool
		errContains string
	}{
		{
			name: "valid uri without tag",
			uri:  "github://owner/repo",
			want: &GitHubStorageComponents{
				Owner:      "owner",
				Repository: "repo",
				Tag:        "latest",
			},
			wantErr: false,
		},
		{
			name: "valid uri with tag",
			uri:  "github://owner/repo@v1.0.0",
			want: &GitHubStorageComponents{
				Owner:      "owner",
				Repository: "repo",
				Tag:        "v1.0.0",
			},
			wantErr: false,
		},
		{
			name:        "missing repository",
			uri:         "github://owner",
			wantErr:     true,
			errContains: "expected owner/repository",
		},
		{
			name:        "empty uri",
			uri:         "github://",
			wantErr:     true,
			errContains: "missing owner/repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGitHubStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatePVCStorageURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid uri without namespace - simple path",
			uri:     "pvc://my-pvc/results",
			wantErr: false,
		},
		{
			name:    "valid uri without namespace - nested path",
			uri:     "pvc://my-pvc/path/to/results",
			wantErr: false,
		},
		{
			name:    "valid uri with namespace using colon separator",
			uri:     "pvc://default:my-pvc/results",
			wantErr: false,
		},
		{
			name:    "valid uri with namespace and nested path",
			uri:     "pvc://my-namespace:my-pvc/path/to/results",
			wantErr: false,
		},
		{
			name:    "valid uri with special characters in pvc name",
			uri:     "pvc://my-pvc-123/path_with-special.chars",
			wantErr: false,
		},
		{
			name:    "valid uri with namespace and special chars",
			uri:     "pvc://default:my-pvc-123/path_with-special.chars",
			wantErr: false,
		},
		{
			name:    "namespace with numbers and hyphens",
			uri:     "pvc://ns-123:pvc-456/model",
			wantErr: false,
		},
		{
			name:    "valid uri with ClusterBaseModel use case",
			uri:     "pvc://model-storage:shared-pvc/path/to/models/llama2-7b",
			wantErr: false,
		},
		{
			name:        "invalid namespace with uppercase",
			uri:         "pvc://MyNamespace:my-pvc/models",
			wantErr:     true,
			errContains: "invalid namespace",
		},
		{
			name:        "invalid namespace with underscore",
			uri:         "pvc://my_namespace:my-pvc/models",
			wantErr:     true,
			errContains: "invalid namespace",
		},
		{
			name:        "namespace starting with hyphen",
			uri:         "pvc://-namespace:my-pvc/models",
			wantErr:     true,
			errContains: "invalid namespace",
		},
		{
			name:        "namespace ending with hyphen",
			uri:         "pvc://namespace-:my-pvc/models",
			wantErr:     true,
			errContains: "invalid namespace",
		},
		{
			name:        "very long namespace (64 chars)",
			uri:         "pvc://a123456789012345678901234567890123456789012345678901234567890123:my-pvc/models",
			wantErr:     true,
			errContains: "invalid namespace",
		},
		{
			name:        "empty namespace before colon",
			uri:         "pvc://:my-pvc/models",
			wantErr:     true,
			errContains: "empty namespace before colon",
		},
		{
			name:        "empty pvc name after colon",
			uri:         "pvc://default:/models",
			wantErr:     true,
			errContains: "empty PVC name after colon",
		},
		{
			name:        "missing pvc prefix",
			uri:         "my-pvc/results",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
		},
		{
			name:        "empty uri",
			uri:         "",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
		},
		{
			name:        "only prefix",
			uri:         "pvc://",
			wantErr:     true,
			errContains: "missing content after prefix",
		},
		{
			name:        "empty pvc name",
			uri:         "pvc:///results",
			wantErr:     true,
			errContains: "missing PVC name",
		},
		{
			name:        "empty subpath",
			uri:         "pvc://my-pvc/",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:        "empty subpath with namespace",
			uri:         "pvc://default:my-pvc/",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:        "only pvc name provided",
			uri:         "pvc://my-pvc",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:        "only namespace and pvc provided",
			uri:         "pvc://default:my-pvc",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:        "invalid uri - wrong scheme",
			uri:         "oci://my-pvc/results",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
		},
		{
			name:        "multiple colons in namespace:pvc part",
			uri:         "pvc://ns:pvc:extra/path",
			wantErr:     true,
			errContains: "multiple colons not allowed",
		},
		{
			name:        "trailing slash in pvc name",
			uri:         "pvc://my-pvc-/results",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:        "leading slash in subpath",
			uri:         "pvc://my-pvc//results",
			wantErr:     true,
			errContains: "missing subpath",
		},
		{
			name:    "multiple consecutive slashes in subpath",
			uri:     "pvc://my-pvc/path//to//results",
			wantErr: false, // This should be valid as it's normalized
		},
		{
			name:    "subpath with only dots",
			uri:     "pvc://my-pvc/...",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with only hyphens",
			uri:     "pvc://my-pvc/---",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with unicode characters",
			uri:     "pvc://my-pvc/测试/模型",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with spaces",
			uri:     "pvc://my-pvc/my path with spaces",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with special characters",
			uri:     "pvc://my-pvc/path/with/special@#$%^&*()chars",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with file extensions",
			uri:     "pvc://my-pvc/models/llama2-7b.bin",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with query parameters (should be treated as part of path)",
			uri:     "pvc://my-pvc/path?param=value",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with fragments (should be treated as part of path)",
			uri:     "pvc://my-pvc/path#fragment",
			wantErr: false, // This should be valid
		},
		{
			name:    "very long subpath",
			uri:     "pvc://my-pvc/" + strings.Repeat("a", 1000),
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with backslashes (should be treated as regular characters)",
			uri:     "pvc://my-pvc/path\\with\\backslashes",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with percent encoding",
			uri:     "pvc://my-pvc/path%20with%20spaces",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with newlines (should be treated as regular characters)",
			uri:     "pvc://my-pvc/path\nwith\nnewlines",
			wantErr: false, // This should be valid
		},
		{
			name:    "subpath with tabs (should be treated as regular characters)",
			uri:     "pvc://my-pvc/path\twith\ttabs",
			wantErr: false, // This should be valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestPVCStorageEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
		description string
	}{
		// Edge cases for empty or minimal URIs
		{
			name:        "completely empty URI",
			uri:         "",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
			description: "Empty URI should fail validation",
		},
		{
			name:        "only pvc prefix",
			uri:         "pvc://",
			wantErr:     true,
			errContains: "missing content after prefix",
			description: "URI with only prefix should fail",
		},
		{
			name:        "pvc prefix with whitespace",
			uri:         "pvc:// ",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with prefix and whitespace should fail",
		},
		{
			name:        "pvc prefix with single slash",
			uri:         "pvc:///",
			wantErr:     true,
			errContains: "missing PVC name",
			description: "URI with prefix and single slash should fail",
		},
		{
			name:        "pvc prefix with double slash",
			uri:         "pvc:////",
			wantErr:     true,
			errContains: "missing PVC name",
			description: "URI with prefix and double slash should fail",
		},

		// Edge cases for PVC names
		{
			name:        "PVC name with only numbers",
			uri:         "pvc://123456/path",
			wantErr:     false,
			description: "PVC name with only numbers should be valid",
		},
		{
			name:        "PVC name with only hyphens",
			uri:         "pvc://---/path",
			wantErr:     false,
			description: "PVC name with only hyphens should be valid",
		},
		{
			name:        "PVC name with only underscores",
			uri:         "pvc://___/path",
			wantErr:     false,
			description: "PVC name with only underscores should be valid",
		},
		{
			name:        "PVC name with mixed valid characters",
			uri:         "pvc://my-pvc_123/path",
			wantErr:     false,
			description: "PVC name with mixed valid characters should be valid",
		},
		{
			name:        "PVC name with uppercase letters",
			uri:         "pvc://MyPVC/path",
			wantErr:     false,
			description: "PVC name with uppercase letters should be valid",
		},
		{
			name:        "PVC name with dots",
			uri:         "pvc://my.pvc/path",
			wantErr:     false,
			description: "PVC name with dots should be valid",
		},

		// Edge cases for namespaces
		{
			name:        "namespace with only numbers",
			uri:         "pvc://123:my-pvc/path",
			wantErr:     false,
			description: "Namespace with only numbers should be valid",
		},
		{
			name:        "namespace with only hyphens",
			uri:         "pvc://---:my-pvc/path",
			wantErr:     false,
			description: "Namespace with only hyphens should be valid",
		},
		{
			name:        "namespace with mixed valid characters",
			uri:         "pvc://my-ns-123:my-pvc/path",
			wantErr:     false,
			description: "Namespace with mixed valid characters should be valid",
		},
		{
			name:        "namespace with dots",
			uri:         "pvc://my.ns:my-pvc/path",
			wantErr:     false,
			description: "Namespace with dots should be valid",
		},
		{
			name:        "namespace with maximum valid length (63 chars)",
			uri:         "pvc://a12345678901234567890123456789012345678901234567890123456789012:my-pvc/path",
			wantErr:     false,
			description: "Namespace with maximum valid length should be valid",
		},

		// Edge cases for subpaths
		{
			name:        "subpath with only forward slashes",
			uri:         "pvc://my-pvc/////",
			wantErr:     false,
			description: "Subpath with only forward slashes should be valid",
		},
		{
			name:        "subpath with only backslashes",
			uri:         "pvc://my-pvc/\\\\\\",
			wantErr:     false,
			description: "Subpath with only backslashes should be valid",
		},
		{
			name:        "subpath with mixed slashes",
			uri:         "pvc://my-pvc/path\\with/mixed\\slashes",
			wantErr:     false,
			description: "Subpath with mixed slashes should be valid",
		},
		{
			name:        "subpath with control characters",
			uri:         "pvc://my-pvc/path\x00with\x01control\x02chars",
			wantErr:     false,
			description: "Subpath with control characters should be valid",
		},
		{
			name:        "subpath with null bytes",
			uri:         "pvc://my-pvc/path\x00null\x00bytes",
			wantErr:     false,
			description: "Subpath with null bytes should be valid",
		},
		{
			name:        "subpath with unicode control characters",
			uri:         "pvc://my-pvc/path\u0000with\u0001unicode\u0002control",
			wantErr:     false,
			description: "Subpath with unicode control characters should be valid",
		},

		// Edge cases for URI format variations
		{
			name:        "URI with multiple consecutive colons in namespace",
			uri:         "pvc://ns::pvc/path",
			wantErr:     true,
			errContains: "multiple colons not allowed",
			description: "URI with multiple consecutive colons should fail",
		},
		{
			name:        "URI with colon in PVC name",
			uri:         "pvc://ns:pvc:name/path",
			wantErr:     true,
			errContains: "multiple colons not allowed",
			description: "URI with colon in PVC name should fail",
		},
		{
			name:        "URI with colon in subpath",
			uri:         "pvc://ns:pvc/path:with:colons",
			wantErr:     false,
			description: "URI with colon in subpath should be valid",
		},
		{
			name:        "URI with at symbol in namespace",
			uri:         "pvc://ns@domain:pvc/path",
			wantErr:     false,
			description: "URI with at symbol in namespace should be valid",
		},
		{
			name:        "URI with at symbol in PVC name",
			uri:         "pvc://ns:pvc@name/path",
			wantErr:     false,
			description: "URI with at symbol in PVC name should be valid",
		},
		{
			name:        "URI with at symbol in subpath",
			uri:         "pvc://ns:pvc/path@with@symbols",
			wantErr:     false,
			description: "URI with at symbol in subpath should be valid",
		},

		// Edge cases for whitespace handling
		{
			name:        "URI with leading whitespace",
			uri:         " pvc://my-pvc/path",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
			description: "URI with leading whitespace should fail",
		},
		{
			name:        "URI with trailing whitespace",
			uri:         "pvc://my-pvc/path ",
			wantErr:     false,
			description: "URI with trailing whitespace should be valid",
		},
		{
			name:        "URI with whitespace in PVC name",
			uri:         "pvc://my pvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with whitespace in PVC name should fail",
		},
		{
			name:        "URI with whitespace in namespace",
			uri:         "pvc://my namespace:my-pvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with whitespace in namespace should fail",
		},

		// Edge cases for special characters in different positions
		{
			name:        "URI with special characters in PVC name",
			uri:         "pvc://my-pvc@#$%^&*()/path",
			wantErr:     false,
			description: "URI with special characters in PVC name should be valid",
		},
		{
			name:        "URI with special characters in namespace",
			uri:         "pvc://my-ns@#$%^&*():my-pvc/path",
			wantErr:     false,
			description: "URI with special characters in namespace should be valid",
		},
		{
			name:        "URI with special characters in subpath",
			uri:         "pvc://my-pvc/path@#$%^&*()",
			wantErr:     false,
			description: "URI with special characters in subpath should be valid",
		},

		// Edge cases for very long components
		{
			name:        "very long PVC name",
			uri:         "pvc://" + strings.Repeat("a", 1000) + "/path",
			wantErr:     false,
			description: "Very long PVC name should be valid",
		},
		{
			name:        "very long namespace",
			uri:         "pvc://" + strings.Repeat("a", 63) + ":my-pvc/path",
			wantErr:     false,
			description: "Very long namespace (63 chars) should be valid",
		},
		{
			name:        "very long namespace (64 chars)",
			uri:         "pvc://" + strings.Repeat("a", 64) + ":my-pvc/path",
			wantErr:     true,
			errContains: "invalid namespace",
			description: "Very long namespace (64 chars) should fail",
		},
		{
			name:        "very long subpath",
			uri:         "pvc://my-pvc/" + strings.Repeat("a", 10000),
			wantErr:     false,
			description: "Very long subpath should be valid",
		},

		// Edge cases for case sensitivity
		{
			name:        "URI with mixed case prefix",
			uri:         "PVC://my-pvc/path",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
			description: "URI with mixed case prefix should fail",
		},
		{
			name:        "URI with uppercase prefix",
			uri:         "PVC://my-pvc/path",
			wantErr:     true,
			errContains: "missing pvc:// prefix",
			description: "URI with uppercase prefix should fail",
		},

		// Edge cases for malformed URIs
		{
			name:        "URI with missing slash after PVC name",
			uri:         "pvc://my-pvcpath",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with missing slash after PVC name should fail",
		},
		{
			name:        "URI with missing slash after namespace:pvc",
			uri:         "pvc://default:my-pvcpath",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with missing slash after namespace:pvc should fail",
		},
		{
			name:        "URI with extra slashes in namespace:pvc part",
			uri:         "pvc://default//my-pvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with extra slashes in namespace:pvc part should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ParsePVCStorageURI
			components, err := ParsePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, tt.description)
				}
				assert.Nil(t, components, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, components, tt.description)
			}

			// Test ValidatePVCStorageURI
			validateErr := ValidatePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, validateErr, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, validateErr.Error(), tt.errContains, tt.description)
				}
			} else {
				assert.NoError(t, validateErr, tt.description)
			}

			// Test GetStorageType for valid PVC URIs
			if !tt.wantErr {
				storageType, typeErr := GetStorageType(tt.uri)
				assert.NoError(t, typeErr, tt.description)
				assert.Equal(t, StorageTypePVC, storageType, tt.description)
			}
		})
	}
}

// TestPVCStorageComprehensiveEdgeCases tests additional edge cases for PVC storage URIs
func TestPVCStorageComprehensiveEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantErr     bool
		errContains string
		description string
	}{
		// Additional edge cases for empty or minimal URIs
		{
			name:        "URI with only whitespace after prefix",
			uri:         "pvc://   ",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with only whitespace after prefix should fail",
		},
		{
			name:        "URI with tab characters in PVC name",
			uri:         "pvc://my\tpvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with tab characters in PVC name should fail",
		},
		{
			name:        "URI with newline characters in PVC name",
			uri:         "pvc://my\npvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with newline characters in PVC name should fail",
		},

		// Edge cases for namespace validation
		{
			name:        "namespace with consecutive hyphens",
			uri:         "pvc://my--namespace:my-pvc/path",
			wantErr:     false,
			description: "Namespace with consecutive hyphens should be valid",
		},
		{
			name:        "namespace with consecutive dots",
			uri:         "pvc://my..namespace:my-pvc/path",
			wantErr:     false,
			description: "Namespace with consecutive dots should be valid",
		},
		{
			name:        "namespace with mixed separators",
			uri:         "pvc://my-ns.123:my-pvc/path",
			wantErr:     false,
			description: "Namespace with mixed separators should be valid",
		},

		// Edge cases for PVC name validation
		{
			name:        "PVC name with consecutive hyphens",
			uri:         "pvc://my--pvc/path",
			wantErr:     false,
			description: "PVC name with consecutive hyphens should be valid",
		},
		{
			name:        "PVC name with consecutive underscores",
			uri:         "pvc://my__pvc/path",
			wantErr:     false,
			description: "PVC name with consecutive underscores should be valid",
		},
		{
			name:        "PVC name with consecutive dots",
			uri:         "pvc://my..pvc/path",
			wantErr:     false,
			description: "PVC name with consecutive dots should be valid",
		},

		// Edge cases for subpath validation
		{
			name:        "subpath with only dots",
			uri:         "pvc://my-pvc/...",
			wantErr:     false,
			description: "Subpath with only dots should be valid",
		},
		{
			name:        "subpath with only hyphens",
			uri:         "pvc://my-pvc/---",
			wantErr:     false,
			description: "Subpath with only hyphens should be valid",
		},
		{
			name:        "subpath with only underscores",
			uri:         "pvc://my-pvc/___",
			wantErr:     false,
			description: "Subpath with only underscores should be valid",
		},
		{
			name:        "subpath with mixed separators",
			uri:         "pvc://my-pvc/path-with.mixed_separators",
			wantErr:     false,
			description: "Subpath with mixed separators should be valid",
		},

		// Edge cases for URI format variations
		{
			name:        "URI with multiple consecutive slashes in subpath",
			uri:         "pvc://my-pvc/path///to///file",
			wantErr:     false,
			description: "URI with multiple consecutive slashes in subpath should be valid",
		},
		{
			name:        "URI with backslashes in subpath",
			uri:         "pvc://my-pvc/path\\with\\backslashes",
			wantErr:     false,
			description: "URI with backslashes in subpath should be valid",
		},
		{
			name:        "URI with mixed forward and backslashes",
			uri:         "pvc://my-pvc/path/with\\mixed/slashes",
			wantErr:     false,
			description: "URI with mixed slashes should be valid",
		},

		// Edge cases for special characters
		{
			name:        "URI with unicode characters in PVC name",
			uri:         "pvc://my-pvc-测试/path",
			wantErr:     false,
			description: "URI with unicode characters in PVC name should be valid",
		},
		{
			name:        "URI with unicode characters in namespace",
			uri:         "pvc://my-ns-测试:my-pvc/path",
			wantErr:     false,
			description: "URI with unicode characters in namespace should be valid",
		},
		{
			name:        "URI with unicode characters in subpath",
			uri:         "pvc://my-pvc/path/with/测试/characters",
			wantErr:     false,
			description: "URI with unicode characters in subpath should be valid",
		},

		// Edge cases for very long components
		{
			name:        "very long PVC name (1000 chars)",
			uri:         "pvc://" + strings.Repeat("a", 1000) + "/path",
			wantErr:     false,
			description: "Very long PVC name should be valid",
		},
		{
			name:        "very long namespace (63 chars)",
			uri:         "pvc://" + strings.Repeat("a", 63) + ":my-pvc/path",
			wantErr:     false,
			description: "Very long namespace (63 chars) should be valid",
		},
		{
			name:        "very long subpath (10000 chars)",
			uri:         "pvc://my-pvc/" + strings.Repeat("a", 10000),
			wantErr:     false,
			description: "Very long subpath should be valid",
		},

		// Edge cases for malformed URIs
		{
			name:        "URI with missing slash after PVC name",
			uri:         "pvc://my-pvcpath",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with missing slash after PVC name should fail",
		},
		{
			name:        "URI with missing slash after namespace:pvc",
			uri:         "pvc://default:my-pvcpath",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with missing slash after namespace:pvc should fail",
		},
		{
			name:        "URI with extra slashes in namespace:pvc part",
			uri:         "pvc://default//my-pvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with extra slashes in namespace:pvc part should fail",
		},
		{
			name:        "URI with colon in PVC name without namespace",
			uri:         "pvc://my:pvc/path",
			wantErr:     true,
			errContains: "missing subpath",
			description: "URI with colon in PVC name without namespace should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ParsePVCStorageURI
			components, err := ParsePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, tt.description)
				}
				assert.Nil(t, components, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, components, tt.description)
			}

			// Test ValidatePVCStorageURI
			validateErr := ValidatePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, validateErr, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, validateErr.Error(), tt.errContains, tt.description)
				}
			} else {
				assert.NoError(t, validateErr, tt.description)
			}

			// Test GetStorageType for valid PVC URIs
			if !tt.wantErr {
				storageType, typeErr := GetStorageType(tt.uri)
				assert.NoError(t, typeErr, tt.description)
				assert.Equal(t, StorageTypePVC, storageType, tt.description)
			}
		})
	}
}

// TestPVCStorageTypeDetectionComprehensive tests comprehensive storage type detection for PVC URIs
func TestPVCStorageTypeDetectionComprehensive(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		expectedType StorageType
		wantErr      bool
		errContains  string
		description  string
	}{
		{
			name:         "valid PVC URI without namespace",
			uri:          "pvc://my-pvc/models/llama2",
			expectedType: StorageTypePVC,
			wantErr:      false,
			description:  "PVC URI without namespace should be detected as PVC type",
		},
		{
			name:         "valid PVC URI with namespace",
			uri:          "pvc://default:my-pvc/models/llama2",
			expectedType: StorageTypePVC,
			wantErr:      false,
			description:  "PVC URI with namespace should be detected as PVC type",
		},
		{
			name:         "PVC URI with complex subpath",
			uri:          "pvc://my-pvc/path/to/models/llama2-7b-chat-hf",
			expectedType: StorageTypePVC,
			wantErr:      false,
			description:  "PVC URI with complex subpath should be detected as PVC type",
		},
		{
			name:         "PVC URI with special characters in subpath",
			uri:          "pvc://my-pvc/models/llama2@7b#chat$hf",
			expectedType: StorageTypePVC,
			wantErr:      false,
			description:  "PVC URI with special characters should be detected as PVC type",
		},
		{
			name:         "PVC URI with unicode characters",
			uri:          "pvc://my-pvc/models/测试模型",
			expectedType: StorageTypePVC,
			wantErr:      false,
			description:  "PVC URI with unicode characters should be detected as PVC type",
		},
		{
			name:        "invalid URI format",
			uri:         "invalid://storage/uri",
			wantErr:     true,
			errContains: "unknown storage type",
			description: "Invalid URI format should return error",
		},
		{
			name:        "empty URI",
			uri:         "",
			wantErr:     true,
			errContains: "empty URI",
			description: "Empty URI should return error",
		},
		{
			name:        "URI with only prefix",
			uri:         "pvc://",
			wantErr:     true,
			errContains: "unknown storage type",
			description: "URI with only prefix should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageType, err := GetStorageType(tt.uri)
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, tt.description)
				}
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedType, storageType, tt.description)
			}
		})
	}
}

// TestPVCStorageURIParsingWithVariousSubpaths tests PVC URI parsing with various subpath formats
func TestPVCStorageURIParsingWithVariousSubpaths(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		expected    *PVCStorageComponents
		wantErr     bool
		errContains string
		description string
	}{
		{
			name: "simple subpath",
			uri:  "pvc://my-pvc/models",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models",
			},
			wantErr:     false,
			description: "Simple subpath should be parsed correctly",
		},
		{
			name: "nested subpath",
			uri:  "pvc://my-pvc/path/to/models/llama2",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path/to/models/llama2",
			},
			wantErr:     false,
			description: "Nested subpath should be parsed correctly",
		},
		{
			name: "subpath with file extension",
			uri:  "pvc://my-pvc/models/config.json",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/config.json",
			},
			wantErr:     false,
			description: "Subpath with file extension should be parsed correctly",
		},
		{
			name: "subpath with special characters",
			uri:  "pvc://my-pvc/models/llama2@7b#chat$hf",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/llama2@7b#chat$hf",
			},
			wantErr:     false,
			description: "Subpath with special characters should be parsed correctly",
		},
		{
			name: "subpath with unicode characters",
			uri:  "pvc://my-pvc/models/测试模型",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/测试模型",
			},
			wantErr:     false,
			description: "Subpath with unicode characters should be parsed correctly",
		},
		{
			name: "subpath with spaces",
			uri:  "pvc://my-pvc/models/my model with spaces",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/my model with spaces",
			},
			wantErr:     false,
			description: "Subpath with spaces should be parsed correctly",
		},
		{
			name: "subpath with query parameters",
			uri:  "pvc://my-pvc/models/config.json?version=1.0",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/config.json?version=1.0",
			},
			wantErr:     false,
			description: "Subpath with query parameters should be parsed correctly",
		},
		{
			name: "subpath with fragments",
			uri:  "pvc://my-pvc/models/config.json#section1",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "models/config.json#section1",
			},
			wantErr:     false,
			description: "Subpath with fragments should be parsed correctly",
		},
		{
			name: "subpath with mixed separators",
			uri:  "pvc://my-pvc/path-with.mixed_separators",
			expected: &PVCStorageComponents{
				Namespace: "",
				PVCName:   "my-pvc",
				SubPath:   "path-with.mixed_separators",
			},
			wantErr:     false,
			description: "Subpath with mixed separators should be parsed correctly",
		},
		{
			name: "subpath with namespace",
			uri:  "pvc://default:my-pvc/models/llama2",
			expected: &PVCStorageComponents{
				Namespace: "default",
				PVCName:   "my-pvc",
				SubPath:   "models/llama2",
			},
			wantErr:     false,
			description: "Subpath with namespace should be parsed correctly",
		},
		{
			name: "subpath with complex namespace",
			uri:  "pvc://my-namespace-123:my-pvc/path/to/models/llama2-7b",
			expected: &PVCStorageComponents{
				Namespace: "my-namespace-123",
				PVCName:   "my-pvc",
				SubPath:   "path/to/models/llama2-7b",
			},
			wantErr:     false,
			description: "Subpath with complex namespace should be parsed correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components, err := ParsePVCStorageURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains, tt.description)
				}
				assert.Nil(t, components, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expected, components, tt.description)
			}
		})
	}
}

// TestPVCStorageValidationComprehensive tests comprehensive PVC storage validation scenarios
// extracted from controller tests to improve test organization
func TestPVCStorageValidationComprehensive(t *testing.T) {
	testCases := []struct {
		name          string
		storageUri    string
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:        "PVC storage with simple subpath",
			storageUri:  "pvc://my-pvc/models",
			expectError: false,
			description: "PVC storage with simple subpath should be valid",
		},
		{
			name:        "PVC storage with nested subpath",
			storageUri:  "pvc://my-pvc/path/to/models/llama2",
			expectError: false,
			description: "PVC storage with nested subpath should be valid",
		},
		{
			name:        "PVC storage with namespace",
			storageUri:  "pvc://storage-ns:my-pvc/models",
			expectError: false,
			description: "PVC storage with namespace should be valid",
		},
		{
			name:        "PVC storage with special characters in subpath",
			storageUri:  "pvc://my-pvc/models/llama2@7b#chat$hf",
			expectError: false,
			description: "PVC storage with special characters should be valid",
		},
		{
			name:          "invalid PVC URI format",
			storageUri:    "pvc://",
			expectError:   true,
			errorContains: "invalid PVC storage URI",
			description:   "Invalid PVC URI format should return error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test PVC URI validation
			err := ValidatePVCStorageURI(tc.storageUri)
			if tc.expectError {
				assert.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, tc.description)
				}
				return
			}
			assert.NoError(t, err, tc.description)

			// Test storage type detection
			storageType, err := GetStorageType(tc.storageUri)
			assert.NoError(t, err, tc.description)
			assert.Equal(t, StorageTypePVC, storageType, tc.description)

			// Test PVC URI parsing
			components, err := ParsePVCStorageURI(tc.storageUri)
			assert.NoError(t, err, tc.description)
			assert.NotNil(t, components, tc.description)
			assert.NotEmpty(t, components.PVCName, tc.description)
			assert.NotEmpty(t, components.SubPath, tc.description)
		})
	}
}
