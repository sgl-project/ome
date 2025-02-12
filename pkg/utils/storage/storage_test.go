package storage

import (
	"reflect"
	"testing"

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
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "oci storage",
			uri:  "oci://n/myns/b/mybucket/o/mypath",
			want: "OCI",
		},
		{
			name: "pvc storage",
			uri:  "pvc://mypvc/mypath",
			want: "PVC",
		},
		{
			name: "vendor storage",
			uri:  "vendor://openai/models/gpt-4",
			want: "VENDOR",
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
