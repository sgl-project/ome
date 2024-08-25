package constants

import (
	"fmt"
	"os"
	"regexp"
	"strings"

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
	KnativeServingAPIGroupName       = KnativeServingAPIGroupNamePrefix + ".dev"
	OMENamespace                     = getEnvOrDefault("POD_NAMESPACE", "ome")
)

// InferenceService Constants
var (
	InferenceServiceName          = "inferenceservice"
	InferenceServiceAPIName       = "inferenceservices"
	InferenceServicePodLabelKey   = OMEAPIGroupName + "/" + InferenceServiceName
	InferenceServiceConfigMapName = "inferenceservice-config"
	DedicatedAIClusterConfigMapName = "dedicatedaicluster-config"
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
)

// Label Constants
var (
	RayClusterLabel  = "ray.io/cluster"
	RayScheduler     = "ray.io/scheduler-name"
	RayPrioriyClass  = "ray.io/priority-class-name"
	VolcanoQueueName = "volcano.sh/queue-name"
	VolcanoScheduler = "volcano"
	VolcanoPreemptable = "volcano.sh/preemptable"
)

// PrioriryClass
var (
	DedicatedAiClusterReservationPriorityClass = "volcano-reservation-low-priority"
	DedicatedAiClusterPreemptionPriorityClass = "volcano-scheduling-high-priority"
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
	AutoscalerClassExternal AutoscalerClassType = "external"
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
	StorageInitializerContainerName = "storage-initializer"

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
	Serverless    DeploymentModeType = "Serverless"
	RawDeployment DeploymentModeType = "RawDeployment"
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

// Model Agent & Model Controller
var (
	NodeInstanceShapeLabel    = "node.kubernetes.io/instance-type"
	ModelsLabelPrefix         = "models.ome/"
	TargetInstanceShapes      = "models.ome.io/target-instance-shapes"
	ModelStatusConfigMapLabel = "models.ome/basemodel-status"
	ObjectStorageUrlPrefix    = "oci://"
)

func GetModelsLabelWithUid(uid types.UID) string {
	return ModelsLabelPrefix + string(uid)
}

// GetRawServiceLabel generate native service label
func GetRawServiceLabel(service string) string {
	return "isvc." + service
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

func PVName(isvcName string, isvcNamespace string) string {
	var maxSubLen = 10
	if len(isvcNamespace) > maxSubLen {
		isvcNamespace = isvcNamespace[len(isvcNamespace)-maxSubLen:]
	}
	if len(isvcName) > maxSubLen {
		isvcName = isvcName[len(isvcName)-maxSubLen:]
	}
	return fmt.Sprintf("pv-%s-%s", isvcNamespace, isvcName)
}

func PVCName(isvcName string) string {
	var maxLen = 20
	if len(isvcName) > maxLen {
		isvcName = isvcName[len(isvcName)-maxLen:]
	}
	return fmt.Sprintf("pvc-%s", isvcName)
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
