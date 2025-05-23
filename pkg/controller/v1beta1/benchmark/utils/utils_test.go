package benchmarkutils

import (
	"reflect"
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/utils/storage"
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
		want    *storage.OCIStorageComponents
		wantErr bool
	}{
		{
			name: "valid uri",
			uri:  "oci://n/my-namespace/b/my-bucket/o/path/to/results",
			want: &storage.OCIStorageComponents{
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
			want: &storage.OCIStorageComponents{
				Namespace: "my-namespace",
				Bucket:    "my-bucket",
				Prefix:    "path/with/multiple/segments",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := storage.ParseOCIStorageURI(tt.uri)
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
	ociStorageUri := "oci://n/my-namespace/b/my-bucket/o/results"
	pvcStorageUri := "pvc://my-pvc/experiment-results"
	tests := []struct {
		name        string
		storageSpec *v1beta1.StorageSpec
		want        []string
		wantErr     bool
	}{
		{
			name: "complete OCI storage spec with all parameters",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &ociStorageUri,
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
			wantErr: false,
		},
		{
			name: "OCI storage with only required parameters",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &ociStorageUri,
				Parameters: &map[string]string{
					"auth": "instance_principal",
				},
			},
			want: []string{
				"--upload-results",
				"--namespace", "my-namespace",
				"--bucket", "my-bucket",
				"--prefix", "results",
				"--auth", "instance_principal",
			},
			wantErr: false,
		},
		{
			name: "OCI storage without parameters",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &ociStorageUri,
			},
			want: []string{
				"--upload-results",
				"--namespace", "my-namespace",
				"--bucket", "my-bucket",
				"--prefix", "results",
			},
			wantErr: false,
		},
		{
			name: "invalid OCI storage URI format",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("oci://invalid/format"),
				Parameters: &map[string]string{
					"auth": "instance_principal",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "complete PVC storage spec",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &pvcStorageUri,
			},
			want: []string{
				"--experiment-base-dir", "/experiment-results",
			},
			wantErr: false,
		},
		{
			name: "PVC storage with nested path",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("pvc://my-pvc/path/to/results"),
			},
			want: []string{
				"--experiment-base-dir", "/path/to/results",
			},
			wantErr: false,
		},
		{
			name: "PVC storage with parameters (should ignore parameters)",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: &pvcStorageUri,
				Parameters: &map[string]string{
					"auth":    "instance_principal",
					"profile": "DEFAULT",
				},
			},
			want: []string{
				"--experiment-base-dir", "/experiment-results",
			},
			wantErr: false,
		},
		{
			name: "PVC storage with empty subpath",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("pvc://my-pvc/"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid PVC storage URI format",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("pvc://"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "PVC storage without subpath",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("pvc://my-pvc"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "unsupported storage scheme",
			storageSpec: &v1beta1.StorageSpec{
				StorageUri: strPtr("s3://my-bucket/path"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:        "nil storage spec",
			storageSpec: nil,
			want:        nil,
			wantErr:     true,
		},
		{
			name: "nil storage uri",
			storageSpec: &v1beta1.StorageSpec{
				Parameters: &map[string]string{
					"auth": "instance_principal",
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildStorageArgs(tt.storageSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildStorageArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.Equal(t, tt.want, got)
			}
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
		model     *v1beta1.ClusterBaseModel
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
			model: &v1beta1.ClusterBaseModel{
				Spec: v1beta1.BaseModelSpec{
					Storage: &v1beta1.StorageSpec{
						Path: strPtr("/model/test-model"),
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
			if tt.model != nil {
				UpdateVolumeMounts(tt.isvc, tt.container, &tt.model.Spec)
			} else {
				UpdateVolumeMounts(tt.isvc, tt.container, nil)
			}
			if !reflect.DeepEqual(tt.container, tt.want) {
				t.Errorf("UpdateVolumeMounts() = %v, want %v", tt.container, tt.want)
			}
		})
	}
}
