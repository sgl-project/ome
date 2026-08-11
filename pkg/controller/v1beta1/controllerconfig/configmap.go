package controllerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/ome/pkg/constants"
	"strings"
)

const (
	IngressConfigKeyName   = "ingress"
	DeployConfigName       = "deploy"
	BenchmarkJobConfigName = "benchmarkjob"
	OmeAgentConfigName     = "omeAgent"

	DefaultDomainTemplate = "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"
	DefaultIngressDomain  = "example.com"

	DefaultUrlScheme = "http"
)

// DefaultConsistentHashHeaders is the fallback request-header list used as the
// consistent-hash key for generated backend traffic policies when the ingress
// config does not set consistentHashHeaders.
var DefaultConsistentHashHeaders = []string{"x-routing-key"}

const ()

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

	OmeIngressGateway string `json:"omeIngressGateway,omitempty"`
	// OmeIngressGatewayClass labels the primary (OmeIngressGateway) gateway's
	// endpoints in status ("internal", "external", ...) — the analogue of
	// IngressGatewaySpec.Class for the cluster-default primary gateway. Free-form;
	// NO in-code default (config-driven per the no-hardcoded-magic-values
	// constraint). Must match the RFC-1123 label form so it is safe as a
	// selector / metrics / path token; invalid values are rejected at config-load.
	// Empty ⇒ the primary endpoint carries no class.
	OmeIngressGatewayClass   string    `json:"omeIngressGatewayClass,omitempty"`
	IngressDomain            string    `json:"ingressDomain,omitempty"`
	IngressClassName         *string   `json:"ingressClassName,omitempty"`
	AdditionalIngressDomains *[]string `json:"additionalIngressDomains,omitempty"`
	DomainTemplate           string    `json:"domainTemplate,omitempty"`
	UrlScheme                string    `json:"urlScheme,omitempty"`
	DisableIstioVirtualHost  bool      `json:"disableIstioVirtualHost,omitempty"`
	PathTemplate             string    `json:"pathTemplate,omitempty"`
	DisableIngressCreation   bool      `json:"disableIngressCreation,omitempty"`
	EnableGatewayAPI         bool      `json:"enableGatewayAPI,omitempty"`
	ConsistentHashHeaders    []string  `json:"consistentHashHeaders,omitempty"`
	// PerISVCSubdomain opts into a per-ISVC subdomain ingress scheme:
	// each ISVC is reached at its own host (matching status.url, rendered
	// from DomainTemplate) with a root path, instead of the default shared
	// "<SharedHostPrefix>.<IngressDomain>" host plus a "/<namespace>/<service>/"
	// path prefix. Default false preserves the shared-host scheme.
	PerISVCSubdomain bool `json:"perISVCSubdomain,omitempty"`
	// SharedHostPrefix is the subdomain label prepended to IngressDomain in the
	// shared-host scheme (PerISVCSubdomain=false): "<SharedHostPrefix>.<IngressDomain>".
	// There is intentionally NO in-code default — the value is supplied via the
	// inferenceservice-config ConfigMap (set by the Helm chart / GitOps), so the
	// host label is never a hardcoded magic string. Empty yields the bare
	// IngressDomain (no prefix).
	SharedHostPrefix string `json:"sharedHostPrefix,omitempty"`
	// AdditionalIngressGateways attaches each generated HTTPRoute to extra
	// gateways beyond the primary OmeIngressGateway — e.g. an external gateway
	// alongside the internal one. Each entry contributes its own parentRef and a
	// hostname rendered from its own IngressDomain. Empty preserves the
	// single-gateway behavior.
	AdditionalIngressGateways []IngressGatewaySpec `json:"additionalIngressGateways,omitempty"`
	// DefaultRouteTimeoutSeconds is the cluster-default Gateway API request
	// timeout (HTTPRouteRule.Timeouts.Request) applied to every generated route
	// whose ISVC does not set a per-component spec.<component>.timeoutSeconds
	// override. There is intentionally NO in-code default — the value is supplied
	// via the inferenceservice-config ConfigMap (Helm chart / GitOps), so the
	// timeout is never a hardcoded magic number. nil or non-positive means OME
	// imposes no route timeout and the gateway's own default applies.
	DefaultRouteTimeoutSeconds *int64 `json:"defaultRouteTimeoutSeconds,omitempty"`
	// NamespaceIngressGateways maps an ISVC's namespace to the gateway(s) its
	// generated HTTPRoutes attach to, overriding the cluster-default primary
	// gateway (OmeIngressGateway/IngressDomain) and AdditionalIngressGateways for
	// ISVCs in that namespace. Namespaces absent from the map fall back to the
	// cluster defaults. This lets one OME controller serve per-namespace Gateways
	// (each with its own TLS cert and DNS) while the route hostname is unchanged —
	// it is still rendered from DomainTemplate, which already embeds the
	// namespace ("<isvc>.<namespace>.<domain>").
	// A per-ISVC "ome.io/ingress-gateway" (or "ome.io/ingress-additional-gateways")
	// annotation still takes precedence over this namespace default.
	NamespaceIngressGateways map[string]NamespaceIngressGateway `json:"namespaceIngressGateways,omitempty"`
}

// IngressGatewaySpec identifies one gateway an HTTPRoute attaches to, plus the
// domain used to build that gateway's hostname for the route.
type IngressGatewaySpec struct {
	// OmeIngressGateway is the gateway parentRef in "namespace/name" form.
	OmeIngressGateway string `json:"omeIngressGateway"`
	// IngressDomain is the domain used to render this gateway's route hostname
	// (same scheme as the primary: per-ISVC subdomain or shared-host prefix).
	IngressDomain string `json:"ingressDomain"`
	// Class labels this gateway's endpoints in status ("internal", "external",
	// "exp", ...). Free-form; NO in-code default (config-driven per the
	// no-hardcoded-magic-values constraint). Must match the RFC-1123 label form
	// [a-z0-9]([-a-z0-9]*[a-z0-9])? so it is safe as a selector / metrics / path
	// token; invalid values are rejected at config-load. Empty ⇒ the endpoint
	// carries no class.
	Class string `json:"class,omitempty"`
}

// NamespaceIngressGateway is the per-namespace gateway override applied to every
// ISVC in a namespace (keyed by namespace in IngressConfig.NamespaceIngressGateways):
// the primary gateway its HTTPRoutes attach to plus any additional gateways,
// replacing the cluster defaults for that namespace.
type NamespaceIngressGateway struct {
	// Primary is the primary gateway (parentRef "namespace/name") and the domain
	// used to render its route hostname for ISVCs in this namespace. When
	// OmeIngressGateway is empty the cluster-default primary is kept.
	Primary IngressGatewaySpec `json:"primary"`
	// Additional attaches each generated HTTPRoute to extra gateways beyond the
	// primary (e.g. an external gateway alongside the internal one), replacing the
	// cluster-default AdditionalIngressGateways for this namespace. nil keeps the
	// cluster default; an explicit empty list means "no additional gateways".
	Additional []IngressGatewaySpec `json:"additional,omitempty"`
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
	if _, err := clientset.CoreV1().ConfigMaps(constants.OMENamespace).Get(context.TODO(), constants.InferenceServiceConfigMapName, metav1.GetOptions{}); err != nil {
		return nil, err
	}
	return &InferenceServicesConfig{}, nil
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

		// An endpoint class is copied verbatim into status and
		// used as a selector / metrics / path token, so reject a malformed one at
		// config-load rather than emit an invalid status value later.
		if err := validateIngressGatewayClasses(ingressConfig); err != nil {
			return nil, err
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

	if len(ingressConfig.ConsistentHashHeaders) == 0 {
		ingressConfig.ConsistentHashHeaders = DefaultConsistentHashHeaders
	}

	return ingressConfig, nil
}

// validateIngressGatewayClasses rejects any endpoint class that is not a valid
// RFC-1123 DNS label. The class is copied verbatim onto status endpoints and is
// meant to be safe as a selector key / metrics dimension / path segment,
// so a malformed value is a config error, not something to
// sanitize silently. Empty classes are allowed (endpoint carries no class).
func validateIngressGatewayClasses(cfg *IngressConfig) error {
	if err := validateIngressClass("omeIngressGatewayClass", cfg.OmeIngressGatewayClass); err != nil {
		return err
	}
	for i := range cfg.AdditionalIngressGateways {
		field := fmt.Sprintf("additionalIngressGateways[%d].class", i)
		if err := validateIngressClass(field, cfg.AdditionalIngressGateways[i].Class); err != nil {
			return err
		}
	}
	for ns, ng := range cfg.NamespaceIngressGateways {
		field := fmt.Sprintf("namespaceIngressGateways[%q].primary.class", ns)
		if err := validateIngressClass(field, ng.Primary.Class); err != nil {
			return err
		}
		for i := range ng.Additional {
			field := fmt.Sprintf("namespaceIngressGateways[%q].additional[%d].class", ns, i)
			if err := validateIngressClass(field, ng.Additional[i].Class); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateIngressClass(field, class string) error {
	if class == "" {
		return nil
	}
	if errs := validation.IsDNS1123Label(class); len(errs) > 0 {
		return fmt.Errorf("invalid ingress config - %s %q must be an RFC-1123 label: %s",
			field, class, strings.Join(errs, "; "))
	}
	return nil
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
