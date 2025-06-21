package controllerconfig

import (
	"context"
	"fmt"
	"testing"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	DefaultModelLocalMountPath = "/mnt/models"
	DefaultHTTPPort            = 8080
	DefaultGRPCPort            = 9000
	DefaultWorkers             = 1
	DefaultTimeout             = 60
	IngressGateway             = "knative-ingress-gateway.knative-serving"
	IngressService             = "istio-ingressgateway.istio-system.svc.cluster.local"
	LocalGateway               = "knative-local-gateway.knative-serving"
	LocalGatewayService        = "knative-local-gateway.istio-system.svc.cluster.local"
	Domain                     = "example.com"
	IngressClassName           = "nginx"
	AdditionalDomain           = "additional-example.com"
	AdditionalDomainExtra      = "additional-example-extra.com"
)

var (
	IngressConfigData = fmt.Sprintf(`{
		"ingressGateway":"%s",
		"ingressService":"%s",
		"localGateway":"%s",
		"localGatewayService":"%s",
		"ingressDomain":"%s",
		"ingressClassName":"%s",
		"additionalIngressDomains":["%s","%s"]
	}`,
		IngressGateway, IngressService,
		LocalGateway, LocalGatewayService,
		Domain, IngressClassName,
		AdditionalDomain, AdditionalDomainExtra)
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
				assert.Equal(t, "test-image", cfg.MultiNodeProber.Image)
				assert.Equal(t, "100m", cfg.MultiNodeProber.CPURequest)
			},
		},
		{
			name:          "missing configmap",
			configMapData: nil,
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
