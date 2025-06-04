package constants

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"

	"knative.dev/serving/pkg/apis/autoscaling"

	"knative.dev/pkg/network"

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

// Benchmark Constants
var (
	BenchmarjJobName          = "benchmarkjob"
	BenchmarkJobConfigMapName = "benchmarkjob-config"
)

// AI Platform Constants
var (
	AIPlatformConfigMapName = "aiplatform-config"
)

// InferenceService Constants
var (
	InferenceServiceName             = "inferenceservice"
	InferenceServiceAPIName          = "inferenceservices"
	InferenceServicePodLabelKey      = OMEAPIGroupName + "/" + InferenceServiceName
	InferenceServiceConfigMapName    = "inferenceservice-config"
	DedicatedAIClusterConfigMapName  = "dedicatedaicluster-config"
	CapacityReservationConfigMapName = "capacityreservation-config"
)

// OME Agent Constants
var (
	AgentName                         = "ome-agent"
	AgentAppName                      = "OME_AGENT"
	AgentModelNameEnvVarKey           = AgentAppName + "_" + "MODEL_NAME"
	AgentModelStoreDirectoryEnvVarKey = AgentAppName + "_" + "MODEL_STORE_DIRECTORY"
	AgentModelFrameworkEnvVarKey      = AgentAppName + "_" + "MODEL_FRAMEWORK"
	AgentTensorRTLLMVersionsEnvVarKey = AgentAppName + "_" + "TENSORRTLLM_VERSION"
	AgentBaseModelTypeEnvVarKey       = AgentAppName + "_" + "MODEL_TYPE"

	// General Configuration
	AgentLocalPathEnvVarKey              = AgentAppName + "_" + "LOCAL_PATH"
	AgentNumOfGPUEnvVarKey               = AgentAppName + "_" + "NUM_OF_GPU"
	AgentDisableModelDecryptionEnvVarKey = AgentAppName + "_" + "DISABLE_MODEL_DECRYPTION"
	AgentModelBucketNameEnvVarKey        = AgentAppName + "_" + "MODEL_BUCKET_NAME"
	AgentModelNamespaceEnvVarKey         = AgentAppName + "_" + "MODEL_NAMESPACE"
	AgentModelObjectName                 = AgentAppName + "_" + "MODEL_OBJECT_NAME"

	// OCI Vault and Security
	AgentCompartmentIDEnvVarKey = AgentAppName + "_" + "COMPARTMENT_ID"
	AgentAuthTypeEnvVarKey      = AgentAppName + "_" + "AUTH_TYPE"
	AgentRegionEnvVarKey        = AgentAppName + "_" + "REGION"
	AgentVaultIDEnvVarKey       = AgentAppName + "_" + "VAULT_ID"
	AgentKeyNameEnvVarKey       = AgentAppName + "_" + "KEY_NAME"
	AgentSecretNameEnvVarKey    = AgentAppName + "_" + "SECRET_NAME"

	// Serving Sidecar Configuration
	AgentFineTunedWeightInfoFilePath      = AgentAppName + "_" + "FINE_TUNED_WEIGHT_INFO_FILE_PATH"
	AgentUnzippedFineTunedWeightDirectory = AgentAppName + "_" + "UNZIPPED_FINE_TUNED_WEIGHT_DIRECTORY"
	AgentZippedFineTunedWeightDirectory   = AgentAppName + "_" + "ZIPPED_FINE_TUNED_WEIGHT_DIRECTORY"
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
	TensorRTLLM           = "tensorrtllm"
)

// InferenceService Annotations
var (
	DeploymentMode                           = OMEAPIGroupName + "/deploymentMode"
	EnableRoutingTagAnnotationKey            = OMEAPIGroupName + "/enable-tag-routing"
	AutoscalerClass                          = OMEAPIGroupName + "/autoscalerClass"
	AutoscalerMetrics                        = OMEAPIGroupName + "/metrics"
	TargetUtilizationPercentage              = OMEAPIGroupName + "/targetUtilizationPercentage"
	DeprecationWarning                       = OMEAPIGroupName + "/deprecation-warning"
	MinScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/min-scale"
	MaxScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/max-scale"
	RollOutDurationAnnotationKey             = KnativeServingAPIGroupName + "/rollout-duration"
	KnativeOpenshiftEnablePassthroughKey     = "serving.knative.openshift.io/enablePassthrough"
	EnableMetricAggregation                  = OMEAPIGroupName + "/enable-metric-aggregation"
	SetPrometheusAnnotation                  = OMEAPIGroupName + "/enable-prometheus-scraping"
	DedicatedAICluster                       = OMEAPIGroupName + "/dedicated-ai-cluster"
	VolcanoQueue                             = OMEAPIGroupName + "/volcano-queue"
	BlockListDisableInjection                = OMEAPIGroupName + "/disable-blocklist"
	ModelInitInjectionKey                    = OMEAPIGroupName + "/inject-model-init"
	FineTunedAdapterInjectionKey             = OMEAPIGroupName + "/inject-fine-tuned-adapter"
	ServingSidecarInjectionKey               = OMEAPIGroupName + "/inject-serving-sidecar"
	FineTunedWeightFTStrategyKey             = OMEAPIGroupName + "/fine-tuned-weight-ft-strategy"
	BaseModelName                            = OMEAPIGroupName + "/base-model-name"
	BaseModelVendorAnnotationKey             = OMEAPIGroupName + "/base-model-vendor"
	ServingRuntimeKeyName                    = OMEAPIGroupName + "/serving-runtime"
	BaseModelFormat                          = OMEAPIGroupName + "/base-model-format"
	BaseModelFormatVersion                   = OMEAPIGroupName + "/base-model-format-version"
	FTServingWithMergedWeightsAnnotationKey  = OMEAPIGroupName + "/fine-tuned-serving-with-merged-weights"
	ServiceType                              = OMEAPIGroupName + "/service-type"
	LoadBalancerIP                           = OMEAPIGroupName + "/load-balancer-ip"
	EntrypointComponent                      = OMEAPIGroupName + "/entrypoint-component"
	ContainerPrometheusPortKey               = "prometheus.ome.io/port"
	ContainerPrometheusPathKey               = "prometheus.ome.io/path"
	PrometheusPortAnnotationKey              = "prometheus.io/port"
	PrometheusPathAnnotationKey              = "prometheus.io/path"
	PrometheusScrapeAnnotationKey            = "prometheus.io/scrape"
	RDMAAutoInjectAnnotationKey              = "rdma.ome.io/auto-inject"
	RDMAProfileAnnotationKey                 = "rdma.ome.io/profile"
	RDMAContainerNameAnnotationKey           = "rdma.ome.io/container-name"
	DefaultPrometheusPath                    = "/metrics"
	QueueProxyAggregatePrometheusMetricsPort = 9088
	DefaultPodPrometheusPort                 = "9091"
	ModelCategoryAnnotation                  = "models.ome.io/category"
)

// InferenceService Annotations for model encryption and decryption
var (
	BaseModelDecryptionKeyName    = OMEAPIGroupName + "/base-model-decryption-key-name"
	BaseModelDecryptionSecretName = OMEAPIGroupName + "/base-model-decryption-secret-name"
	DisableModelDecryption        = OMEAPIGroupName + "/disable-model-decryption"
)

// Label Constants
var (
	RayScheduler                          = "ray.io/scheduler-name"
	RayPrioriyClass                       = "ray.io/priority-class-name"
	RayClusterUnavailableSince            = "raycluster/unavailable-since"
	VolcanoQueueName                      = "volcano.sh/queue-name"
	VolcanoScheduler                      = "volcano"
	InferenceServiceBaseModelNameLabelKey = "base-model-name"
	InferenceServiceBaseModelSizeLabelKey = "base-model-size"
	BaseModelTypeLabelKey                 = "base-model-type"
	BaseModelVendorLabelKey               = "base-model-vendor"
	FTServingLabelKey                     = "fine-tuned-serving"
	FTServingWithMergedWeightsLabelKey    = "fine-tuned-serving-with-merged-weights"
	ServingRuntimeLabelKey                = "serving-runtime"
	FineTunedWeightFTStrategyLabelKey     = "fine-tuned-weight-ft-strategy"
)

// PrioriryClass
var (
	DedicatedAiClusterPreemptionPriorityClass = "volcano-scheduling-high-priority"

	DedicatedAiClusterPreemptionWorkloadPriorityClass = "kueue-scheduling-high-priority"
)

// InferenceService Internal Annotations
var (
	InferenceServiceInternalAnnotationsPrefix        = "internal." + OMEAPIGroupName
	StorageInitializerSourceUriInternalAnnotationKey = InferenceServiceInternalAnnotationsPrefix + "/storage-initializer-sourceuri"
	LoggerInternalAnnotationKey                      = InferenceServiceInternalAnnotationsPrefix + "/logger"
	LoggerSinkUrlInternalAnnotationKey               = InferenceServiceInternalAnnotationsPrefix + "/logger-sink-url"
	LoggerModeInternalAnnotationKey                  = InferenceServiceInternalAnnotationsPrefix + "/logger-mode"
	BatcherInternalAnnotationKey                     = InferenceServiceInternalAnnotationsPrefix + "/batcher"
	AgentShouldInjectAnnotationKey                   = InferenceServiceInternalAnnotationsPrefix + "/agent"
	AgentModelConfigVolumeNameAnnotationKey          = InferenceServiceInternalAnnotationsPrefix + "/configVolumeName"
	AgentModelConfigMountPathAnnotationKey           = InferenceServiceInternalAnnotationsPrefix + "/configMountPath"
	AgentModelDirAnnotationKey                       = InferenceServiceInternalAnnotationsPrefix + "/modelDir"
)

// ome networking constants
const (
	NetworkVisibility      = "networking.ome.io/visibility"
	ClusterLocalVisibility = "cluster-local"
	ClusterLocalDomain     = "svc.cluster.local"
	IsvcNameHeader         = "OMe-Isvc-Name"
	IsvcNamespaceHeader    = "OME-Isvc-Namespace"
)

// StorageSpec Constants
var ()

// Controller Constants
var (
	DefaultMinReplicas = 1

	IstioSidecarInjectionLabel = "sidecar.istio.io/inject"
)

type AutoscalerClassType string
type AutoscalerMetricsType string
type AutoScalerKPAMetricsType string

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

// Autoscaler Default Metrics Value
var (
	DefaultCPUUtilization int32 = 80
)

// Webhook Constants
var (
	PodMutatorWebhookName              = OMEName + "-pod-mutator-webhook"
	ServingRuntimeValidatorWebhookName = OMEName + "-servingRuntime-validator-webhook"
	BenchmarkJobValidatorWebhookName   = OMEName + "-benchmark-job-validator-webhook"
)

// GPU/CPU resource constants
const (
	NvidiaGPUResourceType = "nvidia.com/gpu"
)

// InferenceService Environment Variables
const (
	ContainerPrometheusMetricsPortEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PORT"
	ContainerPrometheusMetricsPathEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PATH"
	QueueProxyAggregatePrometheusMetricsPortEnvVarKey = "AGGREGATE_PROMETHEUS_METRICS_PORT"

	TFewWeightPathEnvVarKey = "TFEW_PATH"

	ModelPathEnvVarKey       = "MODEL_PATH"
	ServedModelNameEnvVarKey = "SERVED_MODEL_NAME"
)

// ModelConfig Constants
const (
	ModelConfigKey = "models.json"
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
	Router    InferenceServiceComponent = "router"
	Engine    InferenceServiceComponent = "engine"
	Decoder   InferenceServiceComponent = "decoder"
)

// InferenceService protocol enums
const (
	OpenAIProtocol          InferenceServiceProtocol = "openAI"
	OpenInferenceProtocolV1 InferenceServiceProtocol = "openInference-v1"
	OpenInferenceProtocolV2 InferenceServiceProtocol = "openInference-v2"
)

// InferenceService Endpoint Ports
const (
	InferenceServiceDefaultHttpPort     = "8080"
	InferenceServiceDefaultAgentPortStr = "9081"
	InferenceServiceDefaultAgentPort    = 9081
	CommonDefaultHttpPort               = 80
	CommonISVCPort                      = 8080
	AggregateMetricsPortName            = "aggr-metric"
)

// Labels to put on kservice
const (
	KServiceComponentLabel = "component"
	KServiceEndpointLabel  = "endpoint"
)

// Labels for TrainedModel
const (
	InferenceServiceLabel = "ome.io/inferenceservice"
)

// InferenceService default/canary constants
const (
	InferenceServiceDefault = "default"
)

// DAC/InferenceService/TrainingJob container names
const (
	MainContainerName               = "ome-container"
	TrainingMainContainerName       = "trainer"
	MultiNodeProberContainerName    = "multinode-prober"
	StorageInitializerContainerName = "storage-initializer"
	ModelInitContainerName          = "model-init"
	FineTunedAdapterContainerName   = "fine-tuned-adapter"
	ServingSidecarContainerName     = "serving-sidecar"
	MultiNodeProberContainerPort    = 8080
)

// Model Agents Constants
const (
	AuthtypeOKEWorkloadIdentity = "OkeWorkloadIdentity"
)

// Serving Container Block Lists
const (
	BlocklistConfigMapVolumeName = "configmap-blocklist-volume"
	InputBlocklistSubPath        = "input.txt"
	OutputBlocklistSubPath       = "output.txt"
	InputBlocklistMountPath      = "/usr/bin/input.txt"
	OutputBlocklistMountPath     = "/usr/bin/output.txt"
)

// Cohere volume mount paths
const (
	ModelEmptyDirVolumeName                   = "model-empty-dir"
	ModelDefaultSourcePath                    = "/mnt/model"
	ModelDefaultMountPathPrefix               = "/opt/ml"
	ModelDefaultMountPath                     = "/opt/ml/model"
	FineTunedWeightDownloadMountPath          = "/mnt/finetuned/download"
	CohereTFewFineTunedWeightVolumeMountPath  = "/opt/ml/tfew"
	CohereTFewFineTunedWeightDefaultPath      = "/opt/ml/tfew/fastertransformer/1"
	BaseModelVolumeMountSubPath               = "base"
	FineTunedWeightDownloadVolumeMountSubPath = "download"
	FineTunedWeightVolumeMountSubPath         = "finetuned"
	TensorRTModelVolumeMountSubPath           = "tensorrt_llm"
)

// Constants used for inference container arguments
const (
	LLamaVllmServedModelNameArgName         = "--served-model-name"
	LLamaVllmFTServingServedModelNamePrefix = "/data"
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
	Serverless        DeploymentModeType = "Serverless"
	RawDeployment     DeploymentModeType = "RawDeployment"
	MultiNodeRayVLLM  DeploymentModeType = "MultiNodeRayVLLM"
	PDDisaggregated   DeploymentModeType = "PDDisaggregated"
	MultiNode         DeploymentModeType = "MultiNode"
	VirtualDeployment DeploymentModeType = "VirtualDeployment"
)

// IsValid checks if the deployment mode is valid
func (d DeploymentModeType) IsValid() bool {
	switch d {
	case Serverless, RawDeployment, MultiNodeRayVLLM, MultiNode, VirtualDeployment:
		return true
	default:
		return false
	}
}

const (
	DefaultNSKnativeServing = "knative-serving"
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
	KEDAScaledObjectKind    = "ScaledObject"
	VolcanoJobKind          = "Job"
	LWSKind                 = "LeaderWorkerSet"
	GatewayKind             = "Gateway"
	ServiceKind             = "Service"
)

// Volcano Job Labels
const (
	VolcanoJobLabelName = "volcano.sh/job-name"
)

// Kueue related Labels
const (
	KueueQueueLabelKey                 = "kueue.x-k8s.io/queue-name"
	KueueWorkloadPriorityClassLabelKey = "kueue.x-k8s.io/priority-class"
	KueueEnabledLabelKey               = "kueue-enabled"
)

// Model Agent & Model Controller
var (
	NodeInstanceShapeLabel    = "node.kubernetes.io/instance-type"
	ModelsLabelPrefix         = "models.ome/"
	TargetInstanceShapes      = "models.ome.io/target-instance-shapes"
	ModelStatusConfigMapLabel = "models.ome/basemodel-status"
)

type TrainingStrategy string

const (
	TFewTrainingStrategy TrainingStrategy = "tfew"
	LoraTrainingStrategy TrainingStrategy = "lora"
)

type ServingStrategy string

// Default training job constants
const (
	TrainingJobName                   = "trainingjob"
	MergedModelWeightZippedFileSuffix = "-merged-weight"
)

type TrainingSidecarRuntime string

type TrainingRuntimeType string

// Training sidecar env variable key names and config key names

var (
	TrainingJobPodLabelKey = OMEAPIGroupName + "/" + TrainingJobName
)

var (
	StrategyConfigKey = "strategy"
)

// FineTunedWeight related constants
const (
	FineTunedWeightMergedWeightsConfigKey = "merged_weights"
)

type ModelVendor string

const (
	Meta   ModelVendor = "meta"
	Cohere ModelVendor = "cohere"
	OpenAI ModelVendor = "openai"
)

var (
// JobCompletionIndexFieldPath is the field path for the Job completion index annotation.
)

// BaseModelType enum
type BaseModelType string

const (
	ServingBaseModel BaseModelType = "Serving"
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

func LWSName(isvcName string) string {
	var maxLen = 50
	if len(isvcName) > maxLen {
		isvcName = isvcName[len(isvcName)-maxLen:]
	}
	return fmt.Sprintf("lws-%s", isvcName)
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

func DefaultRouterServiceName(name string) string {
	return name + "-" + string(Router) + "-" + InferenceServiceDefault
}

func RouterServiceName(name string) string {
	return name
}

func DecoderServiceName(name string) string {
	return name
}

func DecoderPrefix() string {
	return "^/v1/.*$"
}

func PathBasedExplainPrefix() string {
	return "(/v1/.*)$"
}

// FallbackPrefix returns the regex pattern to match any path
func FallbackPrefix() string {
	return "^/.*$"
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
