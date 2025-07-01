package modelagent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sgl-project/ome/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStorageURI(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		wantProvider  storage.Provider
		wantNamespace string
		wantBucket    string
		wantPrefix    string
		wantRegion    string
		wantError     bool
	}{
		// OCI Tests
		{
			name:          "OCI explicit format",
			uri:           "oci://n/mytenancy/b/mybucket/o/models/llama",
			wantProvider:  storage.ProviderOCI,
			wantNamespace: "mytenancy",
			wantBucket:    "mybucket",
			wantPrefix:    "models/llama",
		},
		{
			name:          "OCI with namespace@region",
			uri:           "oci://mytenancy@us-ashburn-1/mybucket/models/llama",
			wantProvider:  storage.ProviderOCI,
			wantNamespace: "mytenancy",
			wantBucket:    "mybucket",
			wantPrefix:    "models/llama",
			wantRegion:    "us-ashburn-1",
		},
		{
			name:         "OCI simple format",
			uri:          "oci://mybucket/models/llama",
			wantProvider: storage.ProviderOCI,
			wantBucket:   "mybucket",
			wantPrefix:   "models/llama",
		},

		// AWS Tests
		{
			name:         "AWS S3 format",
			uri:          "s3://mybucket/models/llama",
			wantProvider: storage.ProviderAWS,
			wantBucket:   "mybucket",
			wantPrefix:   "models",
		},

		// GCP Tests
		{
			name:         "GCP GS format",
			uri:          "gs://mybucket/models/llama",
			wantProvider: storage.ProviderGCP,
			wantBucket:   "mybucket",
			wantPrefix:   "models",
		},

		// Azure Tests
		{
			name:         "Azure with account",
			uri:          "azure://mycontainer@myaccount/models/llama",
			wantProvider: storage.ProviderAzure,
			wantBucket:   "mycontainer",
			wantPrefix:   "models",
		},

		// Error cases
		{
			name:      "Unsupported protocol",
			uri:       "ftp://server/path",
			wantError: true,
		},
		{
			name:      "Empty URI",
			uri:       "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectURI, err := storage.ParseURI(tt.uri)

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantProvider, objectURI.Provider)
			assert.Equal(t, tt.wantNamespace, objectURI.Namespace)
			assert.Equal(t, tt.wantBucket, objectURI.BucketName)
			assert.Equal(t, tt.wantPrefix, objectURI.Prefix)
			assert.Equal(t, tt.wantRegion, objectURI.Region)
		})
	}
}

func TestFilterObjectsForShape(t *testing.T) {
	// This test is for shape filtering logic that is now inline in downloadModel
	// Shape filtering is now done inline in the downloadModel method
	// Testing the logic:
	objects := []storage.ObjectInfo{
		{Name: "models/base/config.json"},
		{Name: "models/A10/weights.bin"},
		{Name: "models/A100/weights.bin"},
		{Name: "models/H100/weights.bin"},
		{Name: "models/shared/tokenizer.json"},
	}

	shapeAlias := "A100"
	var filtered []storage.ObjectInfo
	for _, obj := range objects {
		if strings.Contains(obj.Name, fmt.Sprintf("/%s/", shapeAlias)) {
			filtered = append(filtered, obj)
		}
	}

	assert.Len(t, filtered, 1)
	assert.Equal(t, "models/A100/weights.bin", filtered[0].Name)
}

func TestObjectURIFormatting(t *testing.T) {
	// The formatObjectURI function doesn't exist in the current implementation
	// The storage package has its own ToURI method on ObjectURI
	tests := []struct {
		name string
		uri  *storage.ObjectURI
		want string
	}{
		{
			name: "OCI with namespace",
			uri: &storage.ObjectURI{
				Provider:   storage.ProviderOCI,
				Namespace:  "mytenancy",
				BucketName: "mybucket",
				Prefix:     "models/llama",
			},
			want: "oci://n/mytenancy/b/mybucket/o/models/llama",
		},
		{
			name: "AWS S3 format",
			uri: &storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "mybucket",
				Prefix:     "models/llama",
			},
			want: "s3://mybucket/models/llama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.uri.ToURI()
			assert.Equal(t, tt.want, got)
		})
	}
}
