package controllerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/ome/pkg/constants"
)

const (
	IngressConfigKeyName   = "ingress"
	DeployConfigName       = "deploy"
	MultiNodeProberName    = "multinodeProber"
	BenchmarkJobConfigName = "benchmarkjob"
	OmeAgentConfigName     = "omeAgent"

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
	// Deprecated: IngressGateway and IngressServiceName are no longer consumed by any
	// ingress strategy (they were only used by the removed Serverless/Istio VirtualService
	// path). They are retained, still shipped in the config map, and no longer required,
	// so that a controller image that predates the Serverless removal keeps starting
	// against a newer config map.
	//
	// TODO: remove these two fields once no controller image that requires them is still
	// running. Removing them is a breaking change for that image, which validates both at
	// startup and exits if either is empty. Delete together with the "ingressGateway" and
	// "ingressService" keys in config/configmap/inferenceservice.yaml and
	// charts/ome-resources/templates/ome-controller/configmap.yaml, and the gateway /
	// gatewayService values in charts/ome-resources/values.yaml.
	IngressGateway     string `json:"ingressGateway,omitempty"`
	IngressServiceName string `json:"ingressService,omitempty"`

	OmeIngressGateway        string    `json:"omeIngressGateway,omitempty"`
	IngressDomain            string    `json:"ingressDomain,omitempty"`
	IngressClassName         *string   `json:"ingressClassName,omitempty"`
	AdditionalIngressDomains *[]string `json:"additionalIngressDomains,omitempty"`
	DomainTemplate           string    `json:"domainTemplate,omitempty"`
	UrlScheme                string    `json:"urlScheme,omitempty"`
	DisableIstioVirtualHost  bool      `json:"disableIstioVirtualHost,omitempty"`
	PathTemplate             string    `json:"pathTemplate,omitempty"`
	DisableIngressCreation   bool      `json:"disableIngressCreation,omitempty"`
	EnableGatewayAPI         bool      `json:"enableGatewayAPI,omitempty"`
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

// OmeAgentConfig configures the metadata-extraction Job the BaseModel
// controller spawns for PVC-backed models. Unlike the model-agent
// DaemonSet (whose image is Helm-only), the controller creates this Job in
// Go and therefore needs the agent image + Job settings at runtime.
// +kubebuilder:object:generate=false
type OmeAgentConfig struct {
	// Image is the ome-agent container image. Required.
	Image string `json:"image"`
	// ServiceAccount the metadata Job pod runs as. Must have RBAC to
	// get/list/create/update/patch ConfigMaps in the OME namespace — the
	// agent surfaces extracted metadata via a per-model status ConfigMap,
	// not via direct CR updates.
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// CPURequest/MemoryRequest/CPULimit/MemoryLimit follow the standard
	// resource shape; empty strings fall back to the K8s default.
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
	// BackoffLimit caps Job retries. Default 2 if zero.
	BackoffLimit int32 `json:"backoffLimit,omitempty"`
	// TTLSecondsAfterFinished controls cleanup of completed Jobs.
	// Default 3600 if zero.
	TTLSecondsAfterFinished int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// NodeSelector / Tolerations / Affinity / PriorityClassName are
	// pass-through scheduling hints applied to the metadata Job pod.
	// Necessary when the PVC's CSI driver only mounts on a subset of
	// nodes, or when the cluster taints GPU nodes that hold the models.
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`
	Tolerations       []v1.Toleration   `json:"tolerations,omitempty"`
	Affinity          *v1.Affinity      `json:"affinity,omitempty"`
	PriorityClassName string            `json:"priorityClassName,omitempty"`
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

		if deployConfig.DefaultDeploymentMode != string(constants.RawDeployment) {
			return nil, fmt.Errorf("invalid deployment mode %q. The only supported mode is %s", deployConfig.DefaultDeploymentMode, constants.RawDeployment)
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

// NewOmeAgentConfig loads the omeAgent block from the inferenceservice-config
// ConfigMap in the OME namespace. A missing block yields a zero-value config
// (not an error) so the PVC path can surface PVCConfigMissing via status
// rather than failing controller startup.
func NewOmeAgentConfig(clientset kubernetes.Interface) (*OmeAgentConfig, error) {
	configMap, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cfg := &OmeAgentConfig{}
	if data, ok := configMap.Data[OmeAgentConfigName]; ok {
		if err := json.Unmarshal([]byte(data), cfg); err != nil {
			return nil, fmt.Errorf("unable to parse omeAgent config json: %w", err)
		}
	}
	return cfg, nil
}
