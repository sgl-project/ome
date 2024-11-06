package constants

import (
	"fmt"
	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	"os"
	"regexp"
	"strings"
	"time"

	"knative.dev/serving/pkg/apis/autoscaling"

	"knative.dev/pkg/network"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// OME Constants
var (
	OMEName                          = "ome"
	OMEAPIGroupName                  = "ome.io"
	KnativeAutoscalingAPIGroupName   = "autoscaling.knative.dev"
	KnativeServingAPIGroupNamePrefix = "serving.knative"
	ChainsawAPIGroupName             = "chainsaw.k8s-integration.oracle.com"
	KnativeServingAPIGroupName       = KnativeServingAPIGroupNamePrefix + ".dev"
	OMENamespace                     = getEnvOrDefault("POD_NAMESPACE", "ome")
)

// InferenceService Constants
var (
	InferenceServiceName            = "inferenceservice"
	InferenceServiceAPIName         = "inferenceservices"
	InferenceServicePodLabelKey     = OMEAPIGroupName + "/" + InferenceServiceName
	InferenceServiceConfigMapName   = "inferenceservice-config"
	DedicatedAIClusterConfigMapName = "dedicatedaicluster-config"
	DedicatedAiClusterFinalizer     = "dedicatedaiclusters.ome.io/finalizer"
)

// OME Agent Constants
var (
	AgentName                         = "ome-agent"
	AgentAppName                      = "OME_AGENT"
	AgentModelNameEnvVarKey           = AgentAppName + "_" + "MODEL_NAME"
	AgentModelStoreDirectoryEnvVarKey = AgentAppName + "_" + "MODEL_STORE_DIRECTORY"
	AgentModelFrameworkEnvVarKey      = AgentAppName + "_" + "MODEL_FRAMEWORK"
)

// InferenceService MultiModel Constants
var (
	ModelConfigFileName = "models.json"
)

// Model agent Constants
const (
	AgentContainerName    = "agent"
	AgentConfigMapKeyName = "agent"
	AgentEnableFlag       = "--enable-puller"
	AgentConfigDirArgName = "--config-dir"
	AgentModelDirArgName  = "--model-dir"
)

// InferenceService Annotations
var (
	DeploymentMode                           = OMEAPIGroupName + "/deploymentMode"
	EnableRoutingTagAnnotationKey            = OMEAPIGroupName + "/enable-tag-routing"
	AutoscalerClass                          = OMEAPIGroupName + "/autoscalerClass"
	AutoscalerMetrics                        = OMEAPIGroupName + "/metrics"
	TargetUtilizationPercentage              = OMEAPIGroupName + "/targetUtilizationPercentage"
	MinScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/min-scale"
	MaxScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/max-scale"
	RollOutDurationAnnotationKey             = KnativeServingAPIGroupName + "/rollout-duration"
	KnativeOpenshiftEnablePassthroughKey     = "serving.knative.openshift.io/enablePassthrough"
	EnableMetricAggregation                  = OMEAPIGroupName + "/enable-metric-aggregation"
	SetPrometheusAnnotation                  = OMEAPIGroupName + "/enable-prometheus-scraping"
	DedicatedAICluster                       = OMEAPIGroupName + "/dedicated-ai-cluster"
	VolcanoQueue                             = OMEAPIGroupName + "/volcano-queue"
	Scheduler                                = OMEAPIGroupName + "/scheduler"
	ContainerPrometheusPortKey               = "prometheus.ome.io/port"
	ContainerPrometheusPathKey               = "prometheus.ome.io/path"
	PrometheusPortAnnotationKey              = "prometheus.io/port"
	PrometheusPathAnnotationKey              = "prometheus.io/path"
	DefaultPrometheusPath                    = "/metrics"
	QueueProxyAggregatePrometheusMetricsPort = 9088
	DefaultPodPrometheusPort                 = "9091"
	ChainsawInject                           = ChainsawAPIGroupName + "/inject"
	ChainsawLogPath                          = ChainsawAPIGroupName + "/logPath"
	ChainsawNamespace                        = ChainsawAPIGroupName + "/namespace"
	ChainsawCompartmentID                    = ChainsawAPIGroupName + "/compartmentId"
)

// Label Constants
var (
	RayClusterLabel            = "ray.io/cluster"
	RayScheduler               = "ray.io/scheduler-name"
	RayPrioriyClass            = "ray.io/priority-class-name"
	RayClusterStartTime        = "raycluster/start-time"
	RayClusterUnavailableSince = "raycluster/unavailable-since"
	VolcanoQueueName           = "volcano.sh/queue-name"
	VolcanoScheduler           = "volcano"
	VolcanoPreemptable         = "volcano.sh/preemptable"
	CompartmentIDLabelKey      = "oci.oraclecloud.com/compartment"
)

// PrioriryClass
var (
	DedicatedAiClusterReservationPriorityClass = "volcano-reservation-low-priority"
	DedicatedAiClusterPreemptionPriorityClass  = "volcano-scheduling-high-priority"
)

// InferenceService Internal Annotations
var (
	InferenceServiceInternalAnnotationsPrefix        = "internal." + OMEAPIGroupName
	StorageInitializerSourceUriInternalAnnotationKey = InferenceServiceInternalAnnotationsPrefix + "/storage-initializer-sourceuri"
	StorageSpecAnnotationKey                         = InferenceServiceInternalAnnotationsPrefix + "/storage-spec"
	StorageSpecParamAnnotationKey                    = InferenceServiceInternalAnnotationsPrefix + "/storage-spec-param"
	StorageSpecKeyAnnotationKey                      = InferenceServiceInternalAnnotationsPrefix + "/storage-spec-key"
	LoggerInternalAnnotationKey                      = InferenceServiceInternalAnnotationsPrefix + "/logger"
	LoggerSinkUrlInternalAnnotationKey               = InferenceServiceInternalAnnotationsPrefix + "/logger-sink-url"
	LoggerModeInternalAnnotationKey                  = InferenceServiceInternalAnnotationsPrefix + "/logger-mode"
	BatcherInternalAnnotationKey                     = InferenceServiceInternalAnnotationsPrefix + "/batcher"
	BatcherMaxBatchSizeInternalAnnotationKey         = InferenceServiceInternalAnnotationsPrefix + "/batcher-max-batchsize"
	BatcherMaxLatencyInternalAnnotationKey           = InferenceServiceInternalAnnotationsPrefix + "/batcher-max-latency"
	AgentShouldInjectAnnotationKey                   = InferenceServiceInternalAnnotationsPrefix + "/agent"
	AgentModelConfigVolumeNameAnnotationKey          = InferenceServiceInternalAnnotationsPrefix + "/configVolumeName"
	AgentModelConfigMountPathAnnotationKey           = InferenceServiceInternalAnnotationsPrefix + "/configMountPath"
	AgentModelDirAnnotationKey                       = InferenceServiceInternalAnnotationsPrefix + "/modelDir"
	PredictorHostAnnotationKey                       = InferenceServiceInternalAnnotationsPrefix + "/predictor-host"
	PredictorProtocolAnnotationKey                   = InferenceServiceInternalAnnotationsPrefix + "/predictor-protocol"
)

// ome networking constants
const (
	NetworkVisibility      = "networking.ome.io/visibility"
	ClusterLocalVisibility = "cluster-local"
	ClusterLocalDomain     = "svc.cluster.local"
)

// StorageSpec Constants
var (
	DefaultStorageSpecSecret     = "storage-config"
	DefaultStorageSpecSecretPath = "/mnt/storage-secret" // #nosec G101
)

// Controller Constants
var (
	ControllerLabelName             = OMEName + "-controller-manager"
	DefaultIstioSidecarUID          = int64(1337)
	DefaultMinReplicas              = 1
	IstioInitContainerName          = "istio-init"
	IstioInterceptModeRedirect      = "REDIRECT"
	IstioInterceptionModeAnnotation = "sidecar.istio.io/interceptionMode"
	IstioSidecarUIDAnnotationKey    = OMEAPIGroupName + "/storage-initializer-uid"
	IstioSidecarStatusAnnotation    = "sidecar.istio.io/status"
	IstioSidecarInjectionLabel      = "sidecar.istio.io/inject"
)

type AutoscalerClassType string
type AutoscalerMetricsType string
type AutoScalerKPAMetricsType string

var (
	AutoScalerKPAMetricsRPS         AutoScalerKPAMetricsType = "rps"
	AutoScalerKPAMetricsConcurrency AutoScalerKPAMetricsType = "concurrency"
)

// Autoscaler Default Class
var (
	DefaultAutoscalerClass = AutoscalerClassHPA
)

// Autoscaler Class
var (
	AutoscalerClassHPA      AutoscalerClassType = "hpa"
	AutoscalerClassKEDA     AutoscalerClassType = "keda"
	AutoscalerClassExternal AutoscalerClassType = "external"
)

// Keda Autoscaler Configs
var (
	KedaScalingThreshold        = "autoscaling.keda.sh/threshold"
	KedaScalingOperator         = "autoscaling.keda.sh/operator"
	KedaPrometheusServerAddress = "autoscaling.keda.sh/prometheus.serverAddress"
	KedaPrometheusQuery         = "autoscaling.keda.sh/prometheus.query"
	KedaDefaultMinReplicas      = 1
)

// Autoscaler Metrics
var (
	AutoScalerMetricsCPU AutoscalerMetricsType = "cpu"
)

// Autoscaler Memory metrics
var (
	AutoScalerMetricsMemory AutoscalerMetricsType = "memory"
)

// Autoscaler Class Allowed List
var AutoscalerAllowedClassList = []AutoscalerClassType{
	AutoscalerClassHPA,
	AutoscalerClassKEDA,
	AutoscalerClassExternal,
}

// Autoscaler Metrics Allowed List
var AutoscalerAllowedMetricsList = []AutoscalerMetricsType{
	AutoScalerMetricsCPU,
	AutoScalerMetricsMemory,
}

// Autoscaler KPA Metrics Allowed List
var AutoScalerKPAMetricsAllowedList = []AutoScalerKPAMetricsType{
	AutoScalerKPAMetricsConcurrency,
	AutoScalerKPAMetricsRPS,
}

// Autoscaler Default Metrics Value
var (
	DefaultCPUUtilization int32 = 80
)

// Webhook Constants
var (
	PodMutatorWebhookName              = OMEName + "-pod-mutator-webhook"
	ServingRuntimeValidatorWebhookName = OMEName + "-servingRuntime-validator-webhook"
)

// GPU Constants
const (
	NvidiaGPUResourceType = "nvidia.com/gpu"
)

// InferenceService Environment Variables
const (
	CustomSpecStorageUriEnvVarKey                     = "STORAGE_URI"
	CustomSpecProtocolEnvVarKey                       = "PROTOCOL"
	CustomSpecMultiModelServerEnvVarKey               = "MULTI_MODEL_SERVER"
	ContainerPrometheusMetricsPortEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PORT"
	ContainerPrometheusMetricsPathEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PATH"
	QueueProxyAggregatePrometheusMetricsPortEnvVarKey = "AGGREGATE_PROMETHEUS_METRICS_PORT"
)

type InferenceServiceComponent string

type InferenceServiceVerb string

type InferenceServiceProtocol string

// Knative constants
const (
	KnativeLocalGateway   = "knative-serving/knative-local-gateway"
	KnativeIngressGateway = "knative-serving/knative-ingress-gateway"
	VisibilityLabel       = "networking.knative.dev/visibility"
)

var (
	LocalGatewayHost = "knative-local-gateway.istio-system.svc." + network.GetClusterDomainName()
	IstioMeshGateway = "mesh"
)

// InferenceService Component enums
const (
	Predictor InferenceServiceComponent = "predictor"
)

// InferenceService verb enums
const (
	Predict InferenceServiceVerb = "predict"
	Explain InferenceServiceVerb = "explain"
)

// InferenceService protocol enums
const (
	OpenAIProtocol          InferenceServiceProtocol = "openAI"
	CohereProtocol          InferenceServiceProtocol = "cohere"
	OpenInferenceProtocolV1 InferenceServiceProtocol = "openInference-v1"
	OpenInferenceProtocolV2 InferenceServiceProtocol = "openInference-v2"
	ProtocolUnknown         InferenceServiceProtocol = ""
)

// InferenceService Endpoint Ports
const (
	InferenceServiceDefaultHttpPort     = "8080"
	InferenceServiceDefaultAgentPortStr = "9081"
	InferenceServiceDefaultAgentPort    = 9081
	CommonDefaultHttpPort               = 80
	AggregateMetricsPortName            = "aggr-metric"
)

// Labels to put on kservice
const (
	KServiceComponentLabel = "component"
	KServiceModelLabel     = "model"
	KServiceEndpointLabel  = "endpoint"
)

// Labels for TrainedModel
const (
	ParentInferenceServiceLabel = "inferenceservice"
	InferenceServiceLabel       = "ome.io/inferenceservice"
)

// InferenceService default/canary constants
const (
	InferenceServiceDefault = "default"
	InferenceServiceCanary  = "canary"
)

// InferenceService container names
const (
	InferenceServiceContainerName   = "ome-container"
	MultiNodeProberContainerName    = "multinode-prober"
	StorageInitializerContainerName = "storage-initializer"
	MultiNodeProberContainerPort    = 8080

	// TransformerContainerName transformer container name in collocation
	TransformerContainerName = "transformer-container"
)

// DefaultModelLocalMountPath is where models will be mounted by the storage-initializer
const DefaultModelLocalMountPath = "/mnt/models"

// Multi-model InferenceService
const (
	ModelConfigVolumeName = "model-config"
	ModelDirVolumeName    = "model-dir"
	ModelConfigDir        = "/mnt/configs"
	ModelDir              = DefaultModelLocalMountPath
)

var (
	ServiceAnnotationDisallowedList = []string{
		autoscaling.MinScaleAnnotationKey,
		autoscaling.MaxScaleAnnotationKey,
		StorageInitializerSourceUriInternalAnnotationKey,
		"kubectl.kubernetes.io/last-applied-configuration",
	}

	RevisionTemplateLabelDisallowedList = []string{
		VisibilityLabel,
	}
)

// CheckResultType raw k8s deployment, resource exist check result
type CheckResultType int

const (
	CheckResultCreate  CheckResultType = 0
	CheckResultUpdate  CheckResultType = 1
	CheckResultExisted CheckResultType = 2
	CheckResultUnknown CheckResultType = 3
	CheckResultDelete  CheckResultType = 4
	CheckResultSkipped CheckResultType = 5
)

type DeploymentModeType string

const (
	Serverless       DeploymentModeType = "Serverless"
	RawDeployment    DeploymentModeType = "RawDeployment"
	MultiNodeRayVLLM DeploymentModeType = "MultiNodeRayVLLM"
)

const (
	DefaultNSKnativeServing = "knative-serving"
)

// built-in runtime servers
const (
	TGIServer    = "tgi"
	TritonServer = "triton"
	VLLMServer   = "vllm"
)

const (
	ModelClassLabel = "modelClass"
	ServiceEnvelope = "serviceEnvelope"
)

// torchserve service envelope label allowed values
const (
	ServiceEnvelopeOME   = "ome"
	ServiceEnvelopeOMEV2 = "omev2"
)

// supported model type
const (
	SupportedModelHuggingFace = "huggingface"
	SupportedModelTriton      = "triton"
)

type ProtocolVersion int

const (
	_ ProtocolVersion = iota
	V1
	V2
	GRPCV1
	GRPCV2
	Unknown
)

type HostPath struct {
	Name     string
	HostPath string
}

var OCIETCHostPaths = []HostPath{
	{"region", "/etc/region"},
	{"host-class", "/etc/hostclass"},
	// This typo (avalability) is intentional, it is used in the code in chainsaw sidecar injection,
	// so DO NOT FIX IT UNLESS YOU FIX THE CODE in chainsaw sidecar injection
	// the exact code is here https://bitbucket.oci.oraclecorp.com/projects/GENAICORE/repos/core-k8s-apps/browse/logging-flow/charts/chainsaw-sidecar-injector/values.yaml#85
	{"etc-avalability-domain", "/etc/availability-domain"},
	{"etc-fault-domain", "/etc/fault-domain"},
	{"etc-pki", "/etc/pki"},
	{"etc-ocipki", "/etc/oci-pki"},
	{"etc-identity-realm", "/etc/identity-realm"},
	{"etc-hosts", "/etc/hosts"},
}

// revision label
const (
	RevisionLabel         = "serving.knative.dev/revision"
	RawDeploymentAppLabel = "app"
)

// container state reason
const (
	StateReasonRunning          = "Running"
	StateReasonCompleted        = "Completed"
	StateReasonError            = "Error"
	StateReasonCrashLoopBackOff = "CrashLoopBackOff"
)

// CRD Kinds
const (
	IstioVirtualServiceKind = "VirtualService"
	KnativeServiceKind      = "Service"
	RayClusterKind          = "RayCluster"
	VolcanoQueueKind        = "Queue"
	VolcanoJobKind          = "Job"
)

// Volcano Job Labels
const (
	VolcanoJobLabelName = "volcano.sh/job-name"
)

// DatasetType represents the different types of data
type DatasetType string

const (
	Training   DatasetType = "training"
	Evaluation DatasetType = "evaluation"
)

// Model Agent & Model Controller
var (
	NodeInstanceShapeLabel    = "node.kubernetes.io/instance-type"
	ModelsLabelPrefix         = "models.ome/"
	TargetInstanceShapes      = "models.ome.io/target-instance-shapes"
	ModelStatusConfigMapLabel = "models.ome/basemodel-status"
	ObjectStorageUrlPrefix    = "oci://"
)

// Default training job constants
const (
	TrainingJobNameLabelKey  = "job-name"
	TrainingJobNamePrefix    = "ft-"
	TrainingJobContainerName = "genai-container"

	TrainingMaxSchedulingTimeoutDuration  = 10 * time.Minute
	TrainingK8SJobCreationTimeoutDuration = 3 * time.Minute
	TrainingK8SJobStartingTimeoutDuration = 10 * time.Minute
	TrainingK8SJobRetryTimeoutDuration    = 15 * time.Minute
	TrainingK8SJobRetryMaxAttempts        = 5
)

// Peft training Constants
const (
	PeftTrainingBadDataErrorMessagePrefix = "Data error"
)

type TrainingFailedReason string

const (
	TrainerReconcileFailed          TrainingFailedReason = "TrainerReconcileFailed"
	ModelUpdateFailed               TrainingFailedReason = "ModelUpdateFailed"
	TrainingJobStatusUpdateFailed   TrainingFailedReason = "TrainingJobStatusUpdateFailed"
	ModelCreationOrFetchFailed      TrainingFailedReason = "ModelCreationOrFetchFailed"
	K8SJobFetchFailed               TrainingFailedReason = "K8SJobFetchFailed"
	K8SJobCreationTimeout           TrainingFailedReason = "K8SJobCreationTimeout"
	K8SJobUnexpectedAdmissionError  TrainingFailedReason = "K8SJobUnexpectedAdmissionError"
	K8SJobFailed                    TrainingFailedReason = "K8SJobFailed"
	BadTrainingData                 TrainingFailedReason = "BadTrainingData"
	K8SJobPodFetchFailed            TrainingFailedReason = "K8SJobPodFetchFailed"
	K8SJobSchedulingFailed          TrainingFailedReason = "K8SJobSchedulingFailed"
	K8SJobStartingTimeout           TrainingFailedReason = "K8SJobStartingTimeout"
	TrainingContainerStartingFailed TrainingFailedReason = "TrainingContainerStartingFailed"
)

func (c CheckResultType) String() string {
	switch c {
	case CheckResultCreate:
		return "Create"
	case CheckResultUpdate:
		return "Update"
	case CheckResultExisted:
		return "Existed"
	case CheckResultUnknown:
		return "Unknown"
	case CheckResultDelete:
		return "Delete"
	case CheckResultSkipped:
		return "Skipped"
	default:
		return "Invalid"
	}
}

func GetModelsLabelWithUid(uid types.UID) string {
	return ModelsLabelPrefix + string(uid)
}

// GetRawServiceLabel generate native service label
func GetRawServiceLabel(service string) string {
	return service
}

func (e InferenceServiceComponent) String() string {
	return string(e)
}

func (v InferenceServiceVerb) String() string {
	return string(v)
}

func getEnvOrDefault(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func ModelConfigName(isvcName string) string {
	var maxLen = 20
	if len(isvcName) > maxLen {
		isvcName = isvcName[len(isvcName)-maxLen:]
	}
	return fmt.Sprintf("modelconfig-%s", isvcName)
}

func PVName(isvcName string, isvcNamespace string, componentName string) string {
	var maxSubLen = 16
	if len(isvcNamespace) > maxSubLen {
		isvcNamespace = isvcNamespace[len(isvcNamespace)-maxSubLen:]
	}
	if len(isvcName) > maxSubLen {
		isvcName = isvcName[len(isvcName)-maxSubLen:]
	}

	if len(componentName) > maxSubLen {
		componentName = componentName[len(componentName)-maxSubLen:]
	}
	return fmt.Sprintf("pv-%s-%s-%s", isvcNamespace, isvcName, componentName)
}

func PVCName(isvcName string, component string) string {
	var maxLen = 25
	if len(isvcName) > maxLen {
		isvcName = isvcName[len(isvcName)-maxLen:]
	}
	if len(component) > maxLen {
		component = component[len(component)-maxLen:]
	}
	return fmt.Sprintf("pvc-%s-%s", isvcName, component)
}

// nolint: unused
func isEnvVarMatched(envVar, matchtedValue string) bool {
	return getEnvOrDefault(envVar, "") == matchtedValue
}

func InferenceServiceURL(scheme, name, namespace, domain string) string {
	return fmt.Sprintf("%s://%s.%s.%s%s", scheme, name, namespace, domain, InferenceServicePrefix(name))
}

func InferenceServiceHostName(name string, namespace string, domain string) string {
	return fmt.Sprintf("%s.%s.%s", name, namespace, domain)
}

func DefaultPredictorServiceName(name string) string {
	return name + "-" + string(Predictor) + "-" + InferenceServiceDefault
}

func PredictorServiceName(name string) string {
	return name
}

func CanaryPredictorServiceName(name string) string {
	return name + "-" + string(Predictor) + "-" + InferenceServiceCanary
}

func DefaultServiceName(name string, component InferenceServiceComponent) string {
	return name + "-" + component.String() + "-" + InferenceServiceDefault
}

func CanaryServiceName(name string, component InferenceServiceComponent) string {
	return name + "-" + component.String() + "-" + InferenceServiceCanary
}

func InferenceServicePrefix(name string) string {
	return fmt.Sprintf("/v1/models/%s", name)
}

func PredictPath(name string, protocol InferenceServiceProtocol) string {
	path := ""
	if protocol == OpenInferenceProtocolV1 {
		path = fmt.Sprintf("/v1/models/%s:predict", name)
	} else if protocol == OpenInferenceProtocolV2 {
		path = fmt.Sprintf("/v2/models/%s/infer", name)
	}
	return path
}

func VirtualServiceHostname(name string, predictorHostName string) string {
	index := strings.Index(predictorHostName, ".")
	return name + predictorHostName[index:]
}

func PredictorURL(metadata metav1.ObjectMeta, isCanary bool) string {
	serviceName := DefaultPredictorServiceName(metadata.Name)
	if isCanary {
		serviceName = CanaryPredictorServiceName(metadata.Name)
	}
	return fmt.Sprintf("%s.%s", serviceName, metadata.Namespace)
}

// Should only match 1..65535, but for simplicity it matches 0-99999.
const portMatch = `(?::\d{1,5})?`

// HostRegExp returns an ECMAScript regular expression to match either host or host:<any port>
// for clusterLocalHost, we will also match the prefixes.
func HostRegExp(host string) string {
	localDomainSuffix := ".svc." + network.GetClusterDomainName()
	if !strings.HasSuffix(host, localDomainSuffix) {
		return exact(regexp.QuoteMeta(host) + portMatch)
	}
	prefix := regexp.QuoteMeta(strings.TrimSuffix(host, localDomainSuffix))
	clusterSuffix := regexp.QuoteMeta("." + network.GetClusterDomainName())
	svcSuffix := regexp.QuoteMeta(".svc")
	return exact(prefix + optional(svcSuffix+optional(clusterSuffix)) + portMatch)
}

func exact(regexp string) string {
	return "^" + regexp + "$"
}

func optional(regexp string) string {
	return "(" + regexp + ")?"
}

func DefaultRayHeadServiceName(name string, index int) string {
	return rayutils.CheckName(fmt.Sprintf("%s-%d", name, index))
}
