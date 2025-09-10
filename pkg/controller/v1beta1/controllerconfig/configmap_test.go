package controllerconfig

import (
	"context"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewInferenceServicesConfig(t *testing.T) {
	tests := []struct {
		name           string
		configMapData  map[string]string
		expectedError  bool
		validateConfig func(*testing.T, *InferenceServicesConfig)
	}{
		{
			name: "valid config",
			configMapData: map[string]string{
				OCIConfigName: `{
					"region": "us-phoenix-1",
					"serviceTenancyId": "ocid1.tenancy.oc1..example",
					"serviceCompartmentId": "ocid1.compartment.oc1..example",
					"realm": "oc1",
					"stage": "dev",
					"applicationStage": "dev",
					"internalDomainName": "internal.example.com",
					"publicDomainName": "example.com",
					"airportCode": "PHX",
					"adNumberName": "ad1",
					"namespace": "test"
				}`,
				MultiNodeProberName: `{
					"image": "test-image",
					"cpuRequest": "100m",
					"memoryRequest": "100Mi",
					"cpuLimit": "200m",
					"memoryLimit": "200Mi",
					"startupFailureThreshold": 3,
					"startupPeriodSeconds": 10,
					"startupInitialDelaySeconds": 5,
					"startupTimeoutSeconds": 30,
					"unavailableThresholdSeconds": 60
				}`,
			},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *InferenceServicesConfig) {
				assert.Equal(t, "us-phoenix-1", cfg.OCIConfig.Region)
				assert.Equal(t, "ocid1.tenancy.oc1..example", cfg.OCIConfig.ServiceTenancyId)
				assert.Equal(t, "test-image", cfg.MultiNodeProber.Image)
				assert.Equal(t, "100m", cfg.MultiNodeProber.CPURequest)
			},
		},
		{
			name:          "missing configmap",
			configMapData: nil,
			expectedError: true,
		},
		{
			name: "invalid oci config json",
			configMapData: map[string]string{
				OCIConfigName: `invalid json`,
			},
			expectedError: true,
		},
		{
			name: "invalid multinodeprober config json",
			configMapData: map[string]string{
				OCIConfigName:       `{"region": "us-phoenix-1"}`,
				MultiNodeProberName: `invalid json`,
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			if tt.configMapData != nil {
				configMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.InferenceServiceConfigMapName,
						Namespace: constants.OMENamespace,
					},
					Data: tt.configMapData,
				}
				_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			config, err := NewInferenceServicesConfig(clientset)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestNewIngressConfig(t *testing.T) {
	tests := []struct {
		name           string
		configMapData  map[string]string
		expectedError  bool
		validateConfig func(*testing.T, *IngressConfig)
	}{
		{
			name: "valid config",
			configMapData: map[string]string{
				IngressConfigKeyName: `{
					"ingressGateway": "istio-ingress",
					"ingressService": "istio-ingress",
					"localGateway": "cluster-local-gateway",
					"localGatewayService": "cluster-local-gateway",
					"ingressDomain": "example.com",
					"ingressClassName": "nginx",
					"additionalIngressDomains": ["extra.example.com"],
					"domainTemplate": "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
					"urlScheme": "https",
					"pathTemplate": "/{{ .Namespace }}/{{ .Name }}"
				}`,
			},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *IngressConfig) {
				assert.Equal(t, "istio-ingress", cfg.IngressGateway)
				assert.Equal(t, "istio-ingress", cfg.IngressServiceName)
				assert.Equal(t, "example.com", cfg.IngressDomain)
				assert.Equal(t, "nginx", *cfg.IngressClassName)
				assert.Equal(t, "https", cfg.UrlScheme)
			},
		},
		{
			name: "missing required fields",
			configMapData: map[string]string{
				IngressConfigKeyName: `{
					"ingressDomain": "example.com"
				}`,
			},
			expectedError: true,
		},
		{
			name: "invalid path template",
			configMapData: map[string]string{
				IngressConfigKeyName: `{
					"ingressGateway": "istio-ingress",
					"ingressService": "istio-ingress",
					"pathTemplate": "{{ .Invalid }}"
				}`,
			},
			expectedError: true,
		},
		{
			name:          "default values",
			configMapData: map[string]string{},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *IngressConfig) {
				assert.Equal(t, DefaultDomainTemplate, cfg.DomainTemplate)
				assert.Equal(t, DefaultIngressDomain, cfg.IngressDomain)
				assert.Equal(t, DefaultUrlScheme, cfg.UrlScheme)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.InferenceServiceConfigMapName,
					Namespace: constants.OMENamespace,
				},
				Data: tt.configMapData,
			}
			_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			require.NoError(t, err)

			config, err := NewIngressConfig(clientset)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestNewDeployConfig(t *testing.T) {
	tests := []struct {
		name           string
		configMapData  map[string]string
		expectedError  bool
		validateConfig func(*testing.T, *DeployConfig)
	}{
		{
			name: "valid config",
			configMapData: map[string]string{
				DeployConfigName: `{
					"defaultDeploymentMode": "Serverless"
				}`,
			},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *DeployConfig) {
				assert.Equal(t, "Serverless", cfg.DefaultDeploymentMode)
			},
		},
		{
			name: "invalid json",
			configMapData: map[string]string{
				DeployConfigName: `invalid json`,
			},
			expectedError: true,
		},
		{
			name:          "empty config",
			configMapData: map[string]string{},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *DeployConfig) {
				assert.Empty(t, cfg.DefaultDeploymentMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.InferenceServiceConfigMapName,
					Namespace: constants.OMENamespace,
				},
				Data: tt.configMapData,
			}
			_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
			require.NoError(t, err)

			config, err := NewDeployConfig(clientset)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestNewReplicationJobConfig(t *testing.T) {
	tests := []struct {
		name           string
		configMapData  map[string]string
		expectedError  bool
		validateConfig func(*testing.T, *ReplicationJobConfig)
	}{
		{
			name: "valid config",
			configMapData: map[string]string{
				ReplicationJobConfigName: `{
					"podConfig": {
						"image": "replication-job:latest",
						"resources": {
							"requests": {
								"cpu": "100m",
								"memory": "128Mi"
							},
							"limits": {
								"cpu": "200m",
								"memory": "256Mi"
							}
						}
					},
					"source": {
						"authType": "InstancePrincipal",
						"enableOboToken": true,
						"oboToken": "source-token"
					},
					"target": {
						"authType": "OkeWorkloadIdentity",
						"enableOboToken": false,
						"oboToken": ""
					},
					"enableSizeLimitCheck": true,
					"downloadSizeLimit": "10Gi",
					"enableChecksumUpload": true,
					"checksumAlgorithm": "md5",
					"compartmentId": "ocid1.compartment.oc1..example"
				}`,
			},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *ReplicationJobConfig) {
				assert.Equal(t, "replication-job:latest", cfg.PodConfig.Image)
				require.NotNil(t, cfg.PodConfig.Resources)
				assert.Equal(t, resource.MustParse("100m"), cfg.PodConfig.Resources.Requests["cpu"])
				assert.Equal(t, resource.MustParse("128Mi"), cfg.PodConfig.Resources.Requests["memory"])
				assert.Equal(t, resource.MustParse("200m"), cfg.PodConfig.Resources.Limits["cpu"])
				assert.Equal(t, resource.MustParse("256Mi"), cfg.PodConfig.Resources.Limits["memory"])
				assert.Equal(t, "InstancePrincipal", cfg.Source.AuthType)
				assert.True(t, cfg.Source.EnableOboToken)
				assert.Equal(t, "source-token", cfg.Source.OboToken)
				assert.Equal(t, "OkeWorkloadIdentity", cfg.Target.AuthType)
				assert.False(t, cfg.Target.EnableOboToken)
				assert.True(t, cfg.EnableSizeLimitCheck)
				assert.Equal(t, "10Gi", cfg.DownloadSizeLimit)
				assert.True(t, cfg.EnableChecksumUpload)
				assert.Equal(t, "md5", cfg.ChecksumAlgorithm)
				assert.Equal(t, "ocid1.compartment.oc1..example", cfg.CompartmentId)
			},
		},
		{
			name: "minimal config",
			configMapData: map[string]string{
				ReplicationJobConfigName: `{
					"podConfig": {
						"image": "replication-job:latest"
					},
					"source": {
						"authType": "OkeWorkloadIdentity"
					},
					"target": {
						"authType": "OkeWorkloadIdentity"
					}
				}`,
			},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *ReplicationJobConfig) {
				assert.Equal(t, "replication-job:latest", cfg.PodConfig.Image)
				assert.Nil(t, cfg.PodConfig.Resources)
				assert.Equal(t, "OkeWorkloadIdentity", cfg.Source.AuthType)
				assert.False(t, cfg.Source.EnableOboToken)
				assert.Empty(t, cfg.Source.OboToken)
				assert.Equal(t, "OkeWorkloadIdentity", cfg.Target.AuthType)
				assert.False(t, cfg.Target.EnableOboToken)
				assert.False(t, cfg.EnableSizeLimitCheck)
				assert.Empty(t, cfg.DownloadSizeLimit)
				assert.False(t, cfg.EnableChecksumUpload)
				assert.Empty(t, cfg.ChecksumAlgorithm)
				assert.Empty(t, cfg.CompartmentId)
			},
		},
		{
			name:          "missing configmap",
			configMapData: nil,
			expectedError: true,
		},
		{
			name: "invalid json",
			configMapData: map[string]string{
				ReplicationJobConfigName: `invalid json`,
			},
			expectedError: true,
		},
		{
			name:          "empty config",
			configMapData: map[string]string{},
			expectedError: false,
			validateConfig: func(t *testing.T, cfg *ReplicationJobConfig) {
				assert.Empty(t, cfg.PodConfig.Image)
				assert.Nil(t, cfg.PodConfig.Resources)
				assert.Empty(t, cfg.Source.AuthType)
				assert.Empty(t, cfg.Target.AuthType)
				assert.False(t, cfg.EnableSizeLimitCheck)
				assert.False(t, cfg.EnableChecksumUpload)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()

			if tt.configMapData != nil {
				configMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.ReplicationJobConfigMapName,
						Namespace: constants.OMENamespace,
					},
					Data: tt.configMapData,
				}
				_, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			config, err := NewReplicationJobConfig(clientset)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestGetComponentConfig(t *testing.T) {
	type testStruct struct {
		Field string `json:"field"`
	}

	tests := []struct {
		name         string
		key          string
		data         map[string]string
		expectedData testStruct
		expectedErr  bool
	}{
		{
			name: "valid json",
			key:  "test",
			data: map[string]string{
				"test": `{"field": "value"}`,
			},
			expectedData: testStruct{Field: "value"},
			expectedErr:  false,
		},
		{
			name: "invalid json",
			key:  "test",
			data: map[string]string{
				"test": `invalid json`,
			},
			expectedErr: true,
		},
		{
			name:        "missing key",
			key:         "missing",
			data:        map[string]string{},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMap := &v1.ConfigMap{Data: tt.data}
			var result testStruct
			err := getComponentConfig(tt.key, configMap, &result)
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, result)
		})
	}
}
