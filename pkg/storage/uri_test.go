package storage

import (
	"testing"
)

func TestParseOCIURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected *ObjectURI
		wantErr  bool
	}{
		{
			name: "OCI URI with namespace@region format",
			uri:  "oci://namespace@us-ashburn-1/bucket/prefix/object.txt",
			expected: &ObjectURI{
				Provider:   ProviderOCI,
				Namespace:  "namespace",
				BucketName: "bucket",
				Prefix:     "prefix/object.txt",
				Region:     "us-ashburn-1",
			},
		},
		{
			name: "OCI URI with n/namespace/b/bucket/o format",
			uri:  "oci://n/namespace/b/bucket/o/prefix/object.txt",
			expected: &ObjectURI{
				Provider:   ProviderOCI,
				Namespace:  "namespace",
				BucketName: "bucket",
				Prefix:     "prefix/object.txt",
			},
		},
		{
			name: "Simple OCI URI",
			uri:  "oci://bucket/prefix",
			expected: &ObjectURI{
				Provider:   ProviderOCI,
				BucketName: "bucket",
				Prefix:     "prefix",
			},
		},
		{
			name:    "Invalid OCI URI - wrong scheme",
			uri:     "http://bucket/prefix",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareObjectURI(got, tt.expected) {
				t.Errorf("ParseURI() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected *ObjectURI
		wantErr  bool
	}{
		{
			name: "S3 URI with object",
			uri:  "s3://bucket/path/to/object.txt",
			expected: &ObjectURI{
				Provider:   ProviderAWS,
				BucketName: "bucket",
				ObjectName: "object.txt",
				Prefix:     "path/to",
			},
		},
		{
			name: "S3 URI with prefix only",
			uri:  "s3://bucket/prefix/",
			expected: &ObjectURI{
				Provider:   ProviderAWS,
				BucketName: "bucket",
				Prefix:     "prefix/",
			},
		},
		{
			name: "S3 URI bucket only",
			uri:  "s3://bucket",
			expected: &ObjectURI{
				Provider:   ProviderAWS,
				BucketName: "bucket",
			},
		},
		{
			name:    "Invalid S3 URI - no bucket",
			uri:     "s3://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareObjectURI(got, tt.expected) {
				t.Errorf("ParseURI() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestParseGSURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected *ObjectURI
		wantErr  bool
	}{
		{
			name: "GCS URI with object",
			uri:  "gs://bucket/path/to/object.txt",
			expected: &ObjectURI{
				Provider:   ProviderGCP,
				BucketName: "bucket",
				ObjectName: "object.txt",
				Prefix:     "path/to",
			},
		},
		{
			name: "GCS URI with prefix",
			uri:  "gs://bucket/prefix/",
			expected: &ObjectURI{
				Provider:   ProviderGCP,
				BucketName: "bucket",
				Prefix:     "prefix/",
			},
		},
		{
			name:    "Invalid GCS URI - no bucket",
			uri:     "gs://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareObjectURI(got, tt.expected) {
				t.Errorf("ParseURI() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestParseAzureURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected *ObjectURI
		wantErr  bool
	}{
		{
			name: "Azure URI with account and object",
			uri:  "azure://container@storageaccount/path/to/object.txt",
			expected: &ObjectURI{
				Provider:   ProviderAzure,
				BucketName: "container",
				ObjectName: "object.txt",
				Prefix:     "path/to",
				Extra: map[string]interface{}{
					"account": "storageaccount",
				},
			},
		},
		{
			name: "Azure URI with account and prefix",
			uri:  "azure://container@storageaccount/prefix/",
			expected: &ObjectURI{
				Provider:   ProviderAzure,
				BucketName: "container",
				Prefix:     "prefix/",
				Extra: map[string]interface{}{
					"account": "storageaccount",
				},
			},
		},
		{
			name:    "Invalid Azure URI - no account",
			uri:     "azure://container/path",
			wantErr: true,
		},
		{
			name:    "Invalid Azure URI - no container",
			uri:     "azure://@account/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareObjectURI(got, tt.expected) {
				t.Errorf("ParseURI() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestParseGitHubURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected *ObjectURI
		wantErr  bool
	}{
		{
			name: "GitHub URI with branch",
			uri:  "github://owner/repo@branch/path/to/file.txt",
			expected: &ObjectURI{
				Provider:   ProviderGitHub,
				BucketName: "owner/repo",
				ObjectName: "path/to/file.txt",
				Extra: map[string]interface{}{
					"owner":  "owner",
					"repo":   "repo",
					"branch": "branch",
				},
			},
		},
		{
			name: "GitHub URI default branch",
			uri:  "github://owner/repo/path/to/file.txt",
			expected: &ObjectURI{
				Provider:   ProviderGitHub,
				BucketName: "owner/repo",
				ObjectName: "path/to/file.txt",
				Extra: map[string]interface{}{
					"owner":  "owner",
					"repo":   "repo",
					"branch": "main",
				},
			},
		},
		{
			name:    "Invalid GitHub URI - no repo",
			uri:     "github://owner",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareObjectURI(got, tt.expected) {
				t.Errorf("ParseURI() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestObjectURIToURI(t *testing.T) {
	tests := []struct {
		name string
		uri  *ObjectURI
		want string
	}{
		{
			name: "OCI URI with namespace and region",
			uri: &ObjectURI{
				Provider:   ProviderOCI,
				Namespace:  "namespace",
				BucketName: "bucket",
				Prefix:     "prefix",
				Region:     "us-ashburn-1",
			},
			want: "oci://namespace@us-ashburn-1/bucket/prefix",
		},
		{
			name: "S3 URI with object",
			uri: &ObjectURI{
				Provider:   ProviderAWS,
				BucketName: "bucket",
				ObjectName: "object.txt",
				Prefix:     "path/to",
			},
			want: "s3://bucket/path/to/object.txt",
		},
		{
			name: "Azure URI",
			uri: &ObjectURI{
				Provider:   ProviderAzure,
				BucketName: "container",
				Prefix:     "prefix",
				Extra: map[string]interface{}{
					"account": "storageaccount",
				},
			},
			want: "azure://container@storageaccount/prefix",
		},
		{
			name: "GitHub URI with custom branch",
			uri: &ObjectURI{
				Provider:   ProviderGitHub,
				BucketName: "owner/repo",
				ObjectName: "file.txt",
				Extra: map[string]interface{}{
					"owner":  "owner",
					"repo":   "repo",
					"branch": "develop",
				},
			},
			want: "github://owner/repo@develop/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.uri.ToURI(); got != tt.want {
				t.Errorf("ObjectURI.ToURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

// compareObjectURI compares two ObjectURI instances
func compareObjectURI(a, b *ObjectURI) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.Provider != b.Provider ||
		a.Namespace != b.Namespace ||
		a.BucketName != b.BucketName ||
		a.ObjectName != b.ObjectName ||
		a.Prefix != b.Prefix ||
		a.Region != b.Region {
		return false
	}

	// Compare Extra maps
	if len(a.Extra) != len(b.Extra) {
		return false
	}

	for k, v := range a.Extra {
		bv, ok := b.Extra[k]
		if !ok || v != bv {
			return false
		}
	}

	return true
}
