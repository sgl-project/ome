package benchmarkutils

import (
	"reflect"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}

func TestParseOCIStorageURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    *OCIStorageComponents
		wantErr bool
	}{
		{
			name: "valid uri",
			uri:  "oci://n/my-namespace/b/my-bucket/o/path/to/results",
			want: &OCIStorageComponents{
				Namespace: "my-namespace",
				Bucket:    "my-bucket",
				Prefix:    "path/to/results",
			},
			wantErr: false,
		},
		{
			name:    "invalid uri - missing namespace",
			uri:     "oci://n///b/my-bucket/o/results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - wrong format",
			uri:     "oci://namespace/bucket/results",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid uri - not oci scheme",
			uri:     "s3://my-bucket/results",
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid uri - multiple path segments",
			uri:  "oci://n/my-namespace/b/my-bucket/o/path/with/multiple/segments",
			want: &OCIStorageComponents{
				Namespace: "my-namespace",
				Bucket:    "my-bucket",
				Prefix:    "path/with/multiple/segments",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOCIStorageURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOCIStorageURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseOCIStorageURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildStorageArgs(t *testing.T) {
	storageUri := "oci://n/my-namespace/b/my-bucket/o/results"
	tests := []struct {
		name        string
		storageSpec *v1beta1.StorageSpec
		want        []string
	}{
		{
			name: "complete storage spec",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &storageUri,
				Parameters: &map[string]string{
					"auth":           "instance_principal",
					"config_file":    "/path/to/config",
					"profile":        "DEFAULT",
					"security_token": "token123",
					"region":         "us-phoenix-1",
				},
			},
			want: []string{
				"--upload-results",
				"--namespace", "my-namespace",
				"--bucket", "my-bucket",
				"--prefix", "results",
				"--auth", "instance_principal",
				"--config-file", "/path/to/config",
				"--profile", "DEFAULT",
				"--security-token", "token123",
				"--region", "us-phoenix-1",
			},
		},
		{
			name: "only storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &storageUri,
			},
			want: []string{
				"--upload-results",
				"--namespace", "my-namespace",
				"--bucket", "my-bucket",
				"--prefix", "results",
			},
		},
		{
			name: "only auth parameters",
			storageSpec: &v1beta1.StorageSpec{
				Parameters: &map[string]string{
					"auth":    "user_principal",
					"profile": "DEFAULT",
				},
			},
			want: []string{
				"--upload-results",
				"--auth", "user_principal",
				"--profile", "DEFAULT",
			},
		},
		{
			name:        "nil storage spec",
			storageSpec: nil,
			want:        nil,
		},
		{
			name: "invalid storage uri",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("invalid-uri"),
				Parameters: &map[string]string{
					"auth": "instance_principal",
				},
			},
			want: []string{
				"--upload-results",
				"--auth", "instance_principal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildStorageArgs(tt.storageSpec)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetInferenceService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name    string
		ref     *v1beta1.InferenceServiceReference
		isvc    *v1beta1.InferenceService
		wantErr bool
	}{
		{
			name: "valid reference",
			ref: &v1beta1.InferenceServiceReference{
				Name:      "test-isvc",
				Namespace: "default",
			},
			isvc: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			wantErr: false,
		},
		{
			name:    "nil reference",
			ref:     nil,
			isvc:    nil,
			wantErr: true,
		},
		{
			name: "non-existent service",
			ref: &v1beta1.InferenceServiceReference{
				Name:      "non-existent",
				Namespace: "default",
			},
			isvc:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme)
			if tt.isvc != nil {
				client = client.WithObjects(tt.isvc)
			}
			c := client.Build()

			got, err := GetInferenceService(c, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInferenceService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.isvc) {
				t.Errorf("GetInferenceService() = %v, want %v", got, tt.isvc)
			}
		})
	}
}

func TestBuildInferenceServiceArgs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name         string
		endpointSpec v1beta1.EndpointSpec
		namespace    string
		want         map[string]string
		wantErr      bool
	}{
		{
			name: "direct endpoint - all fields",
			endpointSpec: v1beta1.EndpointSpec{
				Endpoint: &v1beta1.Endpoint{
					URL:       "http://test-url",
					APIFormat: "openai",
					ModelName: "test-model",
				},
			},
			want: map[string]string{
				"--api-backend":    "openai",
				"--api-model-name": "test-model",
				"--api-base":       "http://test-url",
			},
			wantErr: false,
		},
		{
			name: "direct endpoint - minimal fields",
			endpointSpec: v1beta1.EndpointSpec{
				Endpoint: &v1beta1.Endpoint{
					URL:       "http://test-url",
					APIFormat: "openai",
				},
			},
			want: map[string]string{
				"--api-backend":    "openai",
				"--api-base":       "http://test-url",
				"--api-model-name": "",
			},
			wantErr: false,
		},
		{
			name:         "nil endpoint and inference service",
			endpointSpec: v1beta1.EndpointSpec{},
			want:         nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			got, err := BuildInferenceServiceArgs(client, tt.endpointSpec, tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildInferenceServiceArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildInferenceServiceArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateVolumeMounts(t *testing.T) {
	tests := []struct {
		name      string
		isvc      *v1beta1.InferenceService
		container *v1.Container
		want      *v1.Container
	}{
		{
			name: "with base model",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{
						Model: &v1beta1.ModelSpec{
							BaseModel: strPtr("test-model"),
						},
					},
				},
			},
			container: &v1.Container{},
			want: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "test-model",
						MountPath: "/model/test-model",
						ReadOnly:  true,
					},
				},
				Env: []v1.EnvVar{
					{
						Name:  "MODEL_PATH",
						Value: "/model/test-model",
					},
				},
			},
		},
		{
			name: "without base model",
			isvc: &v1beta1.InferenceService{
				Spec: v1beta1.InferenceServiceSpec{
					Predictor: v1beta1.PredictorSpec{},
				},
			},
			container: &v1.Container{},
			want:      &v1.Container{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateVolumeMounts(tt.isvc, tt.container)
			if !reflect.DeepEqual(tt.container, tt.want) {
				t.Errorf("UpdateVolumeMounts() = %v, want %v", tt.container, tt.want)
			}
		})
	}
}
