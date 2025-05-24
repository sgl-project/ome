package controllerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

const (
	OCIConfigName                                = "ociEtc"
	IngressConfigKeyName                         = "ingress"
	DeployConfigName                             = "deploy"
	DacReconcilePolicyConfigName                 = "dacReconcilePolicy"
	CapacityReservationReconcilePolicyConfigName = "capacityReservationReconcilePolicy"
	DacReservationJobConfigName                  = "reservationJob"
	MultiNodeProberName                          = "multinodeProber"
	BenchmarkJobConfigName                       = "benchmarkjob"
	AIPlatformSecretConfigName                   = "aiplatform-config"

	DefaultDomainTemplate = "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"
	DefaultIngressDomain  = "example.com"

	DefaultUrlScheme = "http"
)

type AIPlatformConfig struct {
	SecretConfig SecretConfig `json:"secretConfig"`
}

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
	// OCIConfig contains all OCI Configuration
	OCIConfig OCIConfig `json:"ociEtc"`
	// MultiNodeProber contains all MultiNodeProber Configuration
	MultiNodeProber MultiNodeProberConfig `json:"multinodeProber"`
}

// +kubebuilder:object:generate=false
type IngressConfig struct {
	IngressGateway           string    `json:"ingressGateway,omitempty"`
	IngressServiceName       string    `json:"ingressService,omitempty"`
	LocalGateway             string    `json:"localGateway,omitempty"`
	LocalGatewayServiceName  string    `json:"localGatewayService,omitempty"`
	IngressDomain            string    `json:"ingressDomain,omitempty"`
	IngressClassName         *string   `json:"ingressClassName,omitempty"`
	AdditionalIngressDomains *[]string `json:"additionalIngressDomains,omitempty"`
	DomainTemplate           string    `json:"domainTemplate,omitempty"`
	UrlScheme                string    `json:"urlScheme,omitempty"`
	DisableIstioVirtualHost  bool      `json:"disableIstioVirtualHost,omitempty"`
	PathTemplate             string    `json:"pathTemplate,omitempty"`
	DisableIngressCreation   bool      `json:"disableIngressCreation,omitempty"`
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
type OCIConfig struct {
	// Region for all applications
	Region string `json:"region"`
	// service tenancy OCID, this is defaulted to the tenancy OCID in agent service configMap
	ServiceTenancyId string `json:"serviceTenancyId"`
	// compartment OCID, this is defaulted to the compartment OCID in agent service configMap
	ServiceCompartmentId string `json:"serviceCompartmentId"`
	// Realm for all applications
	Realm string `json:"realm"`
	// Stage for all applications
	Stage string `json:"stage"`
	// ApplicationStage for all applications
	ApplicationStage string `json:"applicationStage"`
	// InternalDomainName for all applications
	InternalDomainName string `json:"internalDomainName"`
	// PublicDomainName for all applications
	PublicDomainName string `json:"publicDomainName"`
	// AirportCode for all applications
	AirportCode string `json:"airportCode"`
	// AdNumberName for all applications
	AdNumberName string `json:"adNumberName"`
	// Namespace for service tenancy
	Namespace string `json:"namespace"`
}

// +kubebuilder:object:generate=false
type DeployConfig struct {
	DefaultDeploymentMode string `json:"defaultDeploymentMode,omitempty"`
}

// +kubebuilder:object:generate=false
type DacReconcilePolicyConfig struct {
	ReconcileFailedLifecycleState bool `json:"reconcileFailedLifecycleState,omitempty"`
	ReconcileWithKueue            bool `json:"reconcileWithKueue,omitempty"`
}

// +kubebuilder:object:generate=false
type CapacityReservationReconcilePolicyConfig struct {
	ReconcileFailedLifecycleState bool `json:"reconcileFailedLifecycleState,omitempty"`
}

// +kubebuilder:object:generate=false
type DacReservationWorkloadConfig struct {
	Image                             string `json:"image"`
	CreationFailedTimeThresholdSecond int    `json:"creationFailedTimeThresholdSecond"`
	SchedulerName                     string `json:"schedulerName"`
}

func NewInferenceServicesConfig(clientset kubernetes.Interface) (*InferenceServicesConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	icfg := &InferenceServicesConfig{}
	for _, err := range []error{
		getComponentConfig(OCIConfigName, configMap, &icfg.OCIConfig),
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
			return nil, fmt.Errorf("invalid deployment mode. Supported modes are Serverless," +
				" RawDeployment and ModelMesh")
		}
	}
	return deployConfig, nil
}

func NewOciConfig(clientset kubernetes.Interface) (*OCIConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ociConfig := &OCIConfig{}
	for _, err := range []error{
		getComponentConfig(OCIConfigName, configMap, &ociConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return ociConfig, nil
}

func NewDacReconcilePolicyConfig(clientset kubernetes.Interface) (*DacReconcilePolicyConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.DedicatedAIClusterConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	dacPolicyConfig := &DacReconcilePolicyConfig{}
	for _, err := range []error{
		getComponentConfig(DacReconcilePolicyConfigName, configMap, &dacPolicyConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return dacPolicyConfig, nil
}

func NewCapacityReservationReconcilePolicyConfig(clientset kubernetes.Interface) (*CapacityReservationReconcilePolicyConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.CapacityReservationConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	capacityReservationPolicyConfig := &CapacityReservationReconcilePolicyConfig{}
	for _, err := range []error{
		getComponentConfig(CapacityReservationReconcilePolicyConfigName, configMap, &capacityReservationPolicyConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return capacityReservationPolicyConfig, nil
}

func NewDacReservationWorkloadConfig(clientset kubernetes.Interface) (*DacReservationWorkloadConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.DedicatedAIClusterConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	dacReservationWorkloadConfig := &DacReservationWorkloadConfig{}
	for _, err := range []error{
		getComponentConfig(DacReservationJobConfigName, configMap, &dacReservationWorkloadConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return dacReservationWorkloadConfig, nil
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

func NewAIPlatformConfig(clientset kubernetes.Interface) (*AIPlatformConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.AIPlatformConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	aiPlatformConfig := &AIPlatformConfig{}
	for _, err := range []error{
		getComponentConfig(AIPlatformSecretConfigName, configMap, &aiPlatformConfig),
	} {
		if err != nil {
			return nil, err
		}
	}
	return aiPlatformConfig, nil
}
