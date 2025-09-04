package gcs

import (
	"testing"
)

func TestParseGCSURI(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantObject string
		wantErr    bool
	}{
		{
			name:       "valid URI with object",
			uri:        "gs://my-bucket/path/to/object.txt",
			wantBucket: "my-bucket",
			wantObject: "path/to/object.txt",
			wantErr:    false,
		},
		{
			name:       "valid URI without object",
			uri:        "gs://my-bucket",
			wantBucket: "my-bucket",
			wantObject: "",
			wantErr:    false,
		},
		{
			name:       "invalid URI - not GCS",
			uri:        "s3://my-bucket/object",
			wantBucket: "",
			wantObject: "",
			wantErr:    true,
		},
		{
			name:       "invalid URI - no bucket",
			uri:        "gs://",
			wantBucket: "",
			wantObject: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, object, err := parseGCSURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGCSURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("parseGCSURI() bucket = %v, want %v", bucket, tt.wantBucket)
			}
			if object != tt.wantObject {
				t.Errorf("parseGCSURI() object = %v, want %v", object, tt.wantObject)
			}
		})
	}
}

func TestBuildGCSURI(t *testing.T) {
	tests := []struct {
		name       string
		bucket     string
		objectName string
		want       string
	}{
		{
			name:       "with object",
			bucket:     "my-bucket",
			objectName: "path/to/object.txt",
			want:       "gs://my-bucket/path/to/object.txt",
		},
		{
			name:       "without object",
			bucket:     "my-bucket",
			objectName: "",
			want:       "gs://my-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGCSURI(tt.bucket, tt.objectName)
			if got != tt.want {
				t.Errorf("buildGCSURI() = %v, want %v", got, tt.want)
			}
		})
	}
}
