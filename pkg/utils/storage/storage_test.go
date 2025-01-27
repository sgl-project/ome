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
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name:    "oci storage",
			uri:     "oci://n/myns/b/mybucket/o/mypath",
			want:    "OCI",
			wantErr: false,
		},
		{
			name:    "pvc storage",
			uri:     "pvc://my-pvc/data",
			want:    "PVC",
			wantErr: false,
		},
		{
			name:    "unknown storage type",
			uri:     "unknown://data",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty uri",
			uri:     "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetStorageType(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStorageType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetStorageType() = %v, want %v", got, tt.want)
			}
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
