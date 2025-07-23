package controllerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sgl-project/ome/pkg/constants"
)

const (
	IngressConfigKeyName   = "ingress"
	DeployConfigName       = "deploy"
	MultiNodeProberName    = "multinodeProber"
	BenchmarkJobConfigName = "benchmarkjob"

	DefaultDomainTemplate = "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"
	DefaultIngressDomain  = "example.com"

	DefaultUrlScheme = "http"
)

type SecretConfig struct {
	WriteToCommonNamespace bool   `json:"writeToCommonNamespace"`
	Namespace              string `json:"namespace"`
	SecretName             string `json:"secretName"`
}

type BenchmarkJobConfig struct {
	// PodConfig contains all Pod Configuration
	PodConfig PodConfig `json:"podConfig"`
}

type PodConfig struct {
	Image         string `json:"image"`
	CPURequest    string `json:"cpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
	CPULimit      string `json:"cpuLimit"`
	MemoryLimit   string `json:"memoryLimit"`
}

// +kubebuilder:object:generate=false
type InferenceServicesConfig struct {
	// MultiNodeProber contains all MultiNodeProber Configuration
	MultiNodeProber MultiNodeProberConfig `json:"multinodeProber"`
}

// +kubebuilder:object:generate=false
type IngressConfig struct {
	IngressGateway             string    `json:"ingressGateway,omitempty"`
	IngressServiceName         string    `json:"ingressService,omitempty"`
	LocalGateway               string    `json:"localGateway,omitempty"`
	LocalGatewayServiceName    string    `json:"localGatewayService,omitempty"`
	KnativeLocalGatewayService string    `json:"knativeLocalGatewayService,omitempty"`
	OmeIngressGateway          string    `json:"omeIngressGateway,omitempty"`
	IngressDomain              string    `json:"ingressDomain,omitempty"`
	IngressClassName           *string   `json:"ingressClassName,omitempty"`
	AdditionalIngressDomains   *[]string `json:"additionalIngressDomains,omitempty"`
	DomainTemplate             string    `json:"domainTemplate,omitempty"`
	UrlScheme                  string    `json:"urlScheme,omitempty"`
	DisableIstioVirtualHost    bool      `json:"disableIstioVirtualHost,omitempty"`
	PathTemplate               string    `json:"pathTemplate,omitempty"`
	DisableIngressCreation     bool      `json:"disableIngressCreation,omitempty"`
	EnableGatewayAPI           bool      `json:"enableGatewayAPI,omitempty"`
}

// +kubebuilder:object:generate=false
type MultiNodeProberConfig struct {
	Image                       string `json:"image"`
	CPURequest                  string `json:"cpuRequest"`
	MemoryRequest               string `json:"memoryRequest"`
	CPULimit                    string `json:"cpuLimit"`
	MemoryLimit                 string `json:"memoryLimit"`
	StartupFailureThreshold     int32  `json:"startupFailureThreshold"`
	StartupPeriodSeconds        int32  `json:"startupPeriodSeconds"`
	StartupInitialDelaySeconds  int32  `json:"startupInitialDelaySeconds"`
	StartupTimeoutSeconds       int32  `json:"startupTimeoutSeconds"`
	UnavailableThresholdSeconds int32  `json:"unavailableThresholdSeconds"`
}

// +kubebuilder:object:generate=false
type DeployConfig struct {
	DefaultDeploymentMode string `json:"defaultDeploymentMode,omitempty"`
}

func NewInferenceServicesConfig(clientset kubernetes.Interface) (*InferenceServicesConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	icfg := &InferenceServicesConfig{}
	for _, err := range []error{
		getComponentConfig(MultiNodeProberName, configMap, &icfg.MultiNodeProber),
	} {
		if err != nil {
			return nil, err
		}
	}
	return icfg, nil
}

func NewIngressConfig(clientset kubernetes.Interface) (*IngressConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ingressConfig := &IngressConfig{}
	if ingress, ok := configMap.Data[IngressConfigKeyName]; ok {
		err := json.Unmarshal([]byte(ingress), &ingressConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to parse ingress config json: %w", err)
		}

		if ingressConfig.IngressGateway == "" || ingressConfig.IngressServiceName == "" {
			return nil, fmt.Errorf("invalid ingress config - ingressGateway and ingressService are required")
		}
		if ingressConfig.PathTemplate != "" {
			// TODO: ensure that the generated path is valid, that is:
			// * both Name and Namespace are used to avoid collisions
			// * starts with a /
			// For now simply check that this is a valid template.
			_, err := template.New("path-template").Parse(ingressConfig.PathTemplate)
			if err != nil {
				return nil, fmt.Errorf("invalid ingress config, unable to parse pathTemplate: %w", err)
			}
			if ingressConfig.IngressDomain == "" {
				return nil, fmt.Errorf("invalid ingress config - ingressDomain is required if pathTemplate is given")
			}
		}
	}

	if ingressConfig.DomainTemplate == "" {
		ingressConfig.DomainTemplate = DefaultDomainTemplate
	}

	if ingressConfig.IngressDomain == "" {
		ingressConfig.IngressDomain = DefaultIngressDomain
	}

	if ingressConfig.UrlScheme == "" {
		ingressConfig.UrlScheme = DefaultUrlScheme
	}

	return ingressConfig, nil
}

func getComponentConfig(key string, configMap *v1.ConfigMap, componentConfig interface{}) error {
	if data, ok := configMap.Data[key]; ok {
		err := json.Unmarshal([]byte(data), componentConfig)
		if err != nil {
			return fmt.Errorf("unable to unmarshall %v json string due to %w ", key, err)
		}
	}
	return nil
}

func NewDeployConfig(clientset kubernetes.Interface) (*DeployConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	deployConfig := &DeployConfig{}
	if deploy, ok := configMap.Data[DeployConfigName]; ok {
		err := json.Unmarshal([]byte(deploy), &deployConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to parse deploy config json: %w", err)
		}

		if deployConfig.DefaultDeploymentMode == "" {
			return nil, fmt.Errorf("invalid deploy config, defaultDeploymentMode is required")
		}

		if deployConfig.DefaultDeploymentMode != string(constants.Serverless) &&
			deployConfig.DefaultDeploymentMode != string(constants.RawDeployment) {
			return nil, fmt.Errorf("invalid deployment mode. Supported modes are %s and %s", constants.Serverless, constants.RawDeployment)
		}
	}
	return deployConfig, nil
}

func NewMultiNodeProberConfig(clientset kubernetes.Interface) (*MultiNodeProberConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	multiNodeProberConfig := &MultiNodeProberConfig{}
	for _, err := range []error{
		getComponentConfig(MultiNodeProberName, configMap, &multiNodeProberConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return multiNodeProberConfig, nil
}

func NewBenchmarkJobConfig(clientset kubernetes.Interface) (*BenchmarkJobConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.BenchmarkJobConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	benchmarkJobConfig := &BenchmarkJobConfig{}
	for _, err := range []error{
		getComponentConfig(BenchmarkJobConfigName, configMap, &benchmarkJobConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return benchmarkJobConfig, nil
}
