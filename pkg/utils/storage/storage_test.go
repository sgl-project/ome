package storage

import (
	"reflect"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/casper"
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
			name: "valid uri with simple path",
			uri:  "pvc://my-pvc/results",
			want: &PVCStorageComponents{
				PVCName: "my-pvc",
				SubPath: "results",
			},
			wantErr: false,
		},
		{
			name: "valid uri with nested path",
			uri:  "pvc://my-pvc/path/to/results",
			want: &PVCStorageComponents{
				PVCName: "my-pvc",
				SubPath: "path/to/results",
			},
			wantErr: false,
		},
		{
			name: "valid uri with special characters",
			uri:  "pvc://my-pvc-123/path_with-special.chars",
			want: &PVCStorageComponents{
				PVCName: "my-pvc-123",
				SubPath: "path_with-special.chars",
			},
			wantErr: false,
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
			name:    "empty pvc name with subpath",
			uri:     "pvc:///results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - missing subpath",
			uri:     "pvc://my-pvc",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - empty subpath",
			uri:     "pvc://my-pvc/",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - empty pvc name",
			uri:     "pvc:///results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - wrong scheme",
			uri:     "oci://my-pvc/results",
			want:    nil,
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
			uri:  "pvc://mypvc/mypath",
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
			name:    "valid pvc uri",
			uri:     "pvc://my-pvc/data",
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
			name:    "invalid pvc uri - missing subpath",
			uri:     "pvc://my-pvc",
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
		expect      *casper.ObjectURI
		wantErr     bool
		errContains string
	}{
		// Hugging Face URIs
		{
			name: "valid hugging face uri with model ID only",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct",
			expect: &casper.ObjectURI{
				Namespace:  "huggingface",
				BucketName: "meta-llama/Llama-3-70B-Instruct",
				Prefix:     "main", // Default branch when not specified
			},
			wantErr: false,
		},
		{
			name: "valid hugging face uri with model ID and branch",
			uri:  "hf://meta-llama/Llama-3-70B-Instruct@experimental",
			expect: &casper.ObjectURI{
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
			expect:  &casper.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid n/namespace/b/bucket/o/multi/level/prefix",
			uri:     "oci://n/myns/b/mybucket/o/dir1/dir2/file.txt",
			expect:  &casper.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: "dir1/dir2/file.txt"},
			wantErr: false,
		},
		{
			name:    "valid namespace@region/bucket/prefix",
			uri:     "oci://myns@us-phoenix-1/mybucket/myprefix",
			expect:  &casper.ObjectURI{Namespace: "myns", Region: "us-phoenix-1", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid namespace@region/bucket with no prefix",
			uri:     "oci://myns@us-phoenix-1/mybucket",
			expect:  &casper.ObjectURI{Namespace: "myns", Region: "us-phoenix-1", BucketName: "mybucket", Prefix: ""},
			wantErr: false,
		},
		{
			name:    "valid bucket/prefix (no namespace/region)",
			uri:     "oci://mybucket/myprefix",
			expect:  &casper.ObjectURI{Namespace: "", Region: "", BucketName: "mybucket", Prefix: "myprefix"},
			wantErr: false,
		},
		{
			name:    "valid bucket only (no namespace/region/prefix)",
			uri:     "oci://mybucket",
			expect:  &casper.ObjectURI{Namespace: "", Region: "", BucketName: "mybucket", Prefix: ""},
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
			expect:  &casper.ObjectURI{Namespace: "myns", BucketName: "mybucket", Prefix: ""},
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
