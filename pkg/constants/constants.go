package constants

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"

	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"

	"knative.dev/serving/pkg/apis/autoscaling"

	"knative.dev/pkg/network"

	batchv1 "k8s.io/api/batch/v1"
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

// InferenceGraph Constants
const (
	RouterHeadersPropagateEnvVar = "PROPAGATE_HEADERS"
	RouterReadinessEndpoint      = "/readyz"
	RouterPort                   = 8080
)

// InferenceService Constants
var (
	InferenceServiceName                = "inferenceservice"
	InferenceServiceAPIName             = "inferenceservices"
	InferenceServicePodLabelKey         = OMEAPIGroupName + "/" + InferenceServiceName
	InferenceServiceConfigMapName       = "inferenceservice-config"
	DedicatedAIClusterConfigMapName     = "dedicatedaicluster-config"
	CapacityReservationConfigMapName    = "capacityreservation-config"
	DedicatedAiClusterFinalizer         = "dedicatedaiclusters.ome.io/finalizer"
	ClusterCapacityReservationFinalizer = "clustercapacityreservations.ome.io/finalizer"
)

// OME Agent Constants
var (
	AgentName                           = "ome-agent"
	AgentAppName                        = "OME_AGENT"
	AgentModelNameEnvVarKey             = AgentAppName + "_" + "MODEL_NAME"
	AgentModelStoreDirectoryEnvVarKey   = AgentAppName + "_" + "MODEL_STORE_DIRECTORY"
	AgentModelFrameworkEnvVarKey        = AgentAppName + "_" + "MODEL_FRAMEWORK"
	AgentTensorRTLLMVersionsEnvVarKey   = AgentAppName + "_" + "TENSORRTLLM_VERSION"
	AgentModelFrameworkVersionEnvVarKey = AgentAppName + "_" + "MODEL_FRAMEWORK_VERSION"
	AgentBaseModelTypeEnvVarKey         = AgentAppName + "_" + "MODEL_TYPE"

	// General Configuration
	AgentLocalPathEnvVarKey      = AgentAppName + "_" + "LOCAL_PATH"
	AgentHFTokenEnvVarKey        = AgentAppName + "_" + "HF_TOKEN"
	AgentSkipSHAEnvVarKey        = AgentAppName + "_" + "SKIP_SHA"
	AgentMaxRetriesEnvVarKey     = AgentAppName + "_" + "MAX_RETRIES"
	AgentRetryIntervalEnvVarKey  = AgentAppName + "_" + "RETRY_INTERVAL_IN_SECONDS"
	AgentNumConnectionsEnvVarKey = AgentAppName + "_" + "NUM_CONNECTIONS"

	// Size Limit Configuration
	AgentDownloadSizeLimitEnvVarKey    = AgentAppName + "_" + "DOWNLOAD_SIZE_LIMIT_GB"
	AgentEnableSizeLimitCheckEnvVarKey = AgentAppName + "_" + "ENABLE_SIZE_LIMIT_CHECK"

	// Source Configuration
	AgentSourceBucketNameEnvVarKey = AgentAppName + "_" + "SOURCE_BUCKET_NAME"
	AgentSourcePrefixEnvVarKey     = AgentAppName + "_" + "SOURCE_PREFIX"
	AgentSourceRegionEnvVarKey     = AgentAppName + "_" + "SOURCE_REGION"
	AgentSourceNamespaceEnvVarKey  = AgentAppName + "_" + "SOURCE_NAMESPACE"

	// Target Configuration
	AgentTargetBucketNameEnvVarKey = AgentAppName + "_" + "TARGET_BUCKET_NAME"
	AgentTargetPrefixEnvVarKey     = AgentAppName + "_" + "TARGET_PREFIX"
	AgentTargetRegionEnvVarKey     = AgentAppName + "_" + "TARGET_REGION"
	AgentTargetNamespaceEnvVarKey  = AgentAppName + "_" + "TARGET_NAMESPACE"

	// Model Configuration
	AgentNodeShapeAliasEnvVarKey         = AgentAppName + "_" + "NODE_SHAPE_ALIAS"
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
	MinScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/min-scale"
	MaxScaleAnnotationKey                    = KnativeAutoscalingAPIGroupName + "/max-scale"
	RollOutDurationAnnotationKey             = KnativeServingAPIGroupName + "/rollout-duration"
	KnativeOpenshiftEnablePassthroughKey     = "serving.knative.openshift.io/enablePassthrough"
	EnableMetricAggregation                  = OMEAPIGroupName + "/enable-metric-aggregation"
	SetPrometheusAnnotation                  = OMEAPIGroupName + "/enable-prometheus-scraping"
	DedicatedAICluster                       = OMEAPIGroupName + "/dedicated-ai-cluster"
	VolcanoQueue                             = OMEAPIGroupName + "/volcano-queue"
	Scheduler                                = OMEAPIGroupName + "/scheduler"
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
	ContainerPrometheusPortKey               = "prometheus.ome.io/port"
	ContainerPrometheusPathKey               = "prometheus.ome.io/path"
	PrometheusPortAnnotationKey              = "prometheus.io/port"
	PrometheusPathAnnotationKey              = "prometheus.io/path"
	PrometheusScrapeAnnotationKey            = "prometheus.io/scrape"
	DefaultPrometheusPath                    = "/metrics"
	QueueProxyAggregatePrometheusMetricsPort = 9088
	DefaultPodPrometheusPort                 = "9091"
	ModelCategoryAnnotation                  = "models.ome.io/category"
)

// InferenceService Annotations for model encryption and decryption
var (
	BaseModelDecryptionKeyName       = OMEAPIGroupName + "/base-model-decryption-key-name"
	BaseModelDecryptionVaultID       = OMEAPIGroupName + "/base-model-decryption-vault-id"
	BaseModelDecryptionSecretName    = OMEAPIGroupName + "/base-model-decryption-secret-name"
	BaseModelDecryptionCompartmentID = OMEAPIGroupName + "/base-model-decryption-compartment-id"
	EncryptionAuthType               = OMEAPIGroupName + "/base-model-decryption-auth-type"
	DisableModelDecryption           = OMEAPIGroupName + "/disable-model-decryption"
)

// Label Constants
var (
	RayClusterLabel                       = "ray.io/cluster"
	RayScheduler                          = "ray.io/scheduler-name"
	RayPrioriyClass                       = "ray.io/priority-class-name"
	RayClusterStartTime                   = "raycluster/start-time"
	RayClusterUnavailableSince            = "raycluster/unavailable-since"
	VolcanoQueueName                      = "volcano.sh/queue-name"
	VolcanoScheduler                      = "volcano"
	VolcanoPreemptable                    = "volcano.sh/preemptable"
	CompartmentIDLabelKey                 = "oci.oraclecloud.com/compartment"
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
	DedicatedAiClusterReservationPriorityClass = "volcano-reservation-low-priority"
	DedicatedAiClusterPreemptionPriorityClass  = "volcano-scheduling-high-priority"

	DedicatedAiClusterReservationWorkloadPriorityClass = "kueue-scheduling-low-priority"
	DedicatedAiClusterPreemptionWorkloadPriorityClass  = "kueue-scheduling-high-priority"
)

// Capacity Reservation
var (
	// DedicatedServingCohort represents the cohort name for dedicated serving.
	// Currently hardcoded, but we plan to introduce additional cohorts (e.g., on-demand-serving cohort) in the future for more flexibility in the system.
	DedicatedServingCohort = "dedicated-serving"
	// DefaultPreemptionConfig defines the preemption rules for the cluster.
	// Currently hardcoded as all clusterQueues in the cluster have the same priority and follow the same preemption rules based on the capacity reservation design.
	// Future changes may introduce more granular preemption configurations as needed.
	DefaultPreemptionConfig = kueuev1beta1.ClusterQueuePreemption{
		// Disables ReclaimWithinCohort and BorrowWithinCohort by default.
		// All rules must be included. Otherwise, any missing rules will be automatically filled in, triggering a reconciler update.
		BorrowWithinCohort: &kueuev1beta1.BorrowWithinCohort{
			Policy: kueuev1beta1.BorrowWithinCohortPolicyNever,
		},
		ReclaimWithinCohort: kueuev1beta1.PreemptionPolicyNever,
		WithinClusterQueue:  kueuev1beta1.PreemptionPolicyLowerPriority,
	}
	DefaultQueueingStrategy  = kueuev1beta1.BestEffortFIFO
	DefaultStopPolicy        = kueuev1beta1.None
	DefaultFlavorFungibility = kueuev1beta1.FlavorFungibility{
		WhenCanBorrow:  kueuev1beta1.Borrow,
		WhenCanPreempt: kueuev1beta1.TryNextFlavor,
	}
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
	PodMutatorWebhookName               = OMEName + "-pod-mutator-webhook"
	ServingRuntimeValidatorWebhookName  = OMEName + "-servingRuntime-validator-webhook"
	BenchmarkJobValidatorWebhookName    = OMEName + "-benchmark-job-validator-webhook"
	TrainingRuntimeValidatorWebhookName = OMEName + "-training-runtime-validator-webhook"
)

// GPU/CPU resource constants
const (
	NvidiaGPUResourceType = "nvidia.com/gpu"
	CPUResourceType       = "cpu"
	MemoryResourceType    = "memory"
)

// Custom scheduler constants
const (
	CustomSchedulerName = "genai-kube-scheduler"
)

// InferenceService Environment Variables
const (
	CustomSpecStorageUriEnvVarKey                     = "STORAGE_URI"
	CustomSpecProtocolEnvVarKey                       = "PROTOCOL"
	CustomSpecMultiModelServerEnvVarKey               = "MULTI_MODEL_SERVER"
	ContainerPrometheusMetricsPortEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PORT"
	ContainerPrometheusMetricsPathEnvVarKey           = "CONTAINER_PROMETHEUS_METRICS_PATH"
	QueueProxyAggregatePrometheusMetricsPortEnvVarKey = "AGGREGATE_PROMETHEUS_METRICS_PORT"

	// Cohere specific
	TFewWeightPathEnvVarKey = "TFEW_PATH"

	// Llama specific
	ModelPathEnvVarKey       = "MODEL_PATH"
	ServedModelNameEnvVarKey = "SERVED_MODEL_NAME"
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
	CommonISVCPort                      = 8080
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
	DACMainTaskName                 = "reservation"

	// TransformerContainerName transformer container name in collocation
	TransformerContainerName = "transformer-container"
)

// DAC related variables
var (
	DACReservationJobTerminationGracePeriodSeconds = int64(5)
	DACLastUpdateTimeAnnotationKey                 = "last-update-time"
	DACCapacityReservedLabelKey                    = "capacity-reserved"
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
	FineTunedWeightInfoFilePath               = "/mnt/ft-model-info.json"
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
	KEDAScaledObjectKind    = "ScaledObject"
	VolcanoJobKind          = "Job"
	LWSKind                 = "LeaderWorkerSet"
	KueueClusterQueueKind   = "ClusterQueue"
	KueueLocalQueueKind     = "LocalQueue"
	KueueCohortKind         = "Cohort"
	KueueResourceFlavorKind = "ResourceFlavor"
	KueueWorkloadKind       = "Workload"
	TrainingJobKind         = "TrainingJob"
)

// Volcano Job Labels
const (
	VolcanoJobLabelName = "volcano.sh/job-name"
)

// Kueue related Labels
const (
	KueueQueueLabelKey                     = "kueue.x-k8s.io/queue-name"
	KueueWorkloadPriorityClassLabelKey     = "kueue.x-k8s.io/priority-class"
	KueueWorkloadNamespaceSelectorLabelKey = "kueue-job"
	KueueEnabledLabelKey                   = "kueue-enabled"
)

// Model Agent & Model Controller
var (
	NodeInstanceShapeLabel    = "node.kubernetes.io/instance-type"
	ModelsLabelPrefix         = "models.ome/"
	TargetInstanceShapes      = "models.ome.io/target-instance-shapes"
	ModelStatusConfigMapLabel = "models.ome/basemodel-status"
	ObjectStorageUrlPrefix    = "oci://"
)

type TrainingStrategy string

const (
	TFewTrainingStrategy    TrainingStrategy = "tfew"
	VanillaTrainingStrategy TrainingStrategy = "vanilla"
	LoraTrainingStrategy    TrainingStrategy = "lora"
)

type ServingStrategy string

const (
	VanillaServingStrategy ServingStrategy = "vanilla" // Fine-tuned weights are merged back into the model and served in same way as baseline model
	LoraServingStrategy    ServingStrategy = "lora"    // Stacked multi lora serving
)

// Default training job constants
const (
	TrainingJobName                     = "trainingjob"
	TrainingSidecarContainerName        = "training-sidecar"
	TrainingSidecarConfigMapKeyName     = "trainingSidecar"
	MergedModelWeightZippedFileSuffix   = "-merged-weight"
	DefaultTrainingZippedModelDirectory = "/mnt/ft/output"
	TrainingJobNamePrefix               = "ft-"
)

type TrainingSidecarRuntime string

const (
	PeftTrainingSidecar           TrainingSidecarRuntime = "peft"
	CohereCommand1TrainingSidecar TrainingSidecarRuntime = "cohere"
	CohereCommandRTrainingSidecar TrainingSidecarRuntime = "cohere-commandr"
)

type TrainingRuntimeType string

const (
	PeftTrainingRuntime            TrainingRuntimeType = "peft"
	CohereCommand1TrainingRuntime  TrainingRuntimeType = "cohere"
	CohereCommandRTrainingTraining TrainingRuntimeType = "cohere-commandr"
)

// Training sidecar env variable key names and config key names

var (
	TrainingSidecarInjectionKey      = OMEAPIGroupName + "/inject-training-sidecar"
	TrainingJobPodLabelKey           = OMEAPIGroupName + "/" + TrainingJobName
	TrainingRuntimeTypeAnnotationKey = OMEAPIGroupName + "/training-runtime-type"
)

var (
	CompartmentEnvVarKey              = AgentAppName + "_" + "COMPARTMENT_ID"
	NamespaceEnvVarKey                = AgentAppName + "_" + "NAMESPACE"
	OboTokenEnvVarKey                 = AgentAppName + "_" + "INPUT_OBJECT_STORE_OBO_TOKEN"
	OboTokenConfigKey                 = "obo_token"
	EnableOboTokenEnvVarKey           = AgentAppName + "_" + "INPUT_OBJECT_STORE_ENABLE_OBO_TOKEN"
	BucketNameEnvVarKey               = AgentAppName + "_" + "BUCKET_NAME"
	TrainingDataBucketNameEnvVarKey   = AgentAppName + "_" + "TRAINING_DATA_BUCKET_NAME"
	TrainingDataBucketConfigKey       = "training_data_bucket_config_key"
	TrainingDataNamespaceEnvVarKey    = AgentAppName + "_" + "TRAINING_DATA_NAMESPACE"
	TrainingDataNamespaceConfigKey    = "training_data_namespace_config_key"
	TrainingDataFileNameEnvVarKey     = AgentAppName + "_" + "TRAINING_DATA_OBJECT_NAME"
	TrainingDataFileNameConfigKey     = "trainingDataFileName"
	TrainingMetricsBucketEnvVarKey    = AgentAppName + "_" + "TRAINING_METRICS_BUCKET_NAME"
	TrainingMetricsNamespaceEnvVarKey = AgentAppName + "_" + "TRAINING_METRICS_NAMESPACE"
	TrainingMetricsObjectEnvVarKey    = AgentAppName + "_" + "TRAINING_METRICS_OBJECT_NAME"
	BatchSizeConfigKey                = "trainingBatchSize"
	EarlyStoppingPatienceConfigKey    = "earlyStoppingPatience"
	EarlyStoppingThresholdConfigKey   = "earlyStoppingThreshold"
	EpochsConfigKey                   = "totalTrainingEpochs"
	LearningRateConfigKey             = "learningRate"
	TrainingConfigTypeConfigKey       = "strategy"
	ModelDirectoryEnvVarKey           = AgentAppName + "_" + "MODEL_DIRECTORY"
	ZippedModelPathEnvVarKey          = "ZIPPED_MODEL_PATH"
	ZippedMergedModelPathEnvVarKey    = AgentAppName + "_" + "ZIPPED_MERGED_MODEL_PATH"
	LoraTrainingConfig                = "lora"
	RuntimeEnvVarKey                  = AgentAppName + "_" + "RUNTIME"
	LoraConfigRankConfigKey           = "loraR"
	ModelVendorConfigKey              = "vendor"
	TrainingNameEnvVarKey             = AgentAppName + "_" + "TRAINING_NAME"
	TrainingDataDirectoryEnvVarKey    = AgentAppName + "_" + "TRAINING_DATA_DIRECTORY"

	/*
	 * Constants specific to cohere training sidecar
	 */
	CohereLogTrainStatusEveryStepEnvVarKey = AgentAppName + "_" + "COHERE_FT_LOG_TRAIN_STATUS_EVERY_STEPS"
	LogTrainStatusEveryStepConfigKey       = "logModelMetricsIntervalInSteps"
	CohereNLastLayersEnvVarKey             = AgentAppName + "_" + "COHERE_FT_N_LAST_LAYERS"
	NLastLayersConfigKey                   = "nLastLayers"
	ModelSizeEnvVarKey                     = AgentAppName + "_" + "COHERE_FT_SIZE"
	ModelSizeConfigKey                     = "modelSize"
	StrategyEnvVarKey                      = AgentAppName + "_" + "COHERE_FT_STRATEGY"
	StrategyConfigKey                      = "strategy"
	CohereTrainingSidecarNameEnvVarKey     = AgentAppName + "_" + "COHERE_FT_NAME"
	CohereLearningRateEnvVarKey            = AgentAppName + "_" + "COHERE_FT_LEARNING_RATE"
	CohereBatchSizeEnvVarKey               = AgentAppName + "_" + "COHERE_FT_TRAIN_BATCH_SIZE"
	CohereEarlyStoppingPatienceEnvVarKey   = AgentAppName + "_" + "COHERE_FT_EARLY_STOPPING_PATIENCE"
	CohereEarlyStoppingThresholdEnvVarKey  = AgentAppName + "_" + "COHERE_FT_EARLY_STOPPING_THRESHOLD"
	CohereModelNameEnvVarKey               = AgentAppName + "_" + "COHERE_FT_BASE_MODEL"
	CohereLoraConfigAlphaEnvVarKey         = AgentAppName + "_" + "COHERE_FT_LORA_CONFIG_ALPHA"
	CohereEpochsEnvVarKey                  = AgentAppName + "_" + "COHERE_FT_TRAIN_EPOCHS"

	/*
	 * Constants specific to cohere command R training sidecar
	 */
	CohereTensorParallelEnvVarKey = AgentAppName + "_" + "COHERE_FT_TENSOR_PARALLEL_SIZE"
	TensorParallelConfigKey       = "tensor_parallel"
	BaseModelEnvVarKey            = AgentAppName + "_" + "BASE_MODEL"
	BaseModelConfigKey            = "base_model"
	ServingStrategyEnvVarKey      = AgentAppName + "_" + "COHERE_FT_SERVING_STRATEGY"

	/*
	 *Constants specific to peft training sidecar
	 */

	PeftEpochsEnvVarKey                = AgentAppName + "_" + "PEFT_FT_NUM_TRAIN_EPOCHS"
	PeftBatchSizeEnvVarKey             = AgentAppName + "_" + "PEFT_FT_TRAIN_BATCH_SIZE"
	PeftEarlyStoppingPatienceEnvVarKey = AgentAppName + "_" + "PEFT_FT_EARLY_STOPPING_PATIENCE"

	PeftEarlyStoppingThresholdEnvVarKey = AgentAppName + "_" + "PEFT_FT_EARLY_STOPPING_THRESHOLD"
	PeftTrainingDataSetFileEnvVarKey    = AgentAppName + "_" + "PEFT_FT_TRAIN_DATASET_FILE"
	PeftLoraREnvVarKey                  = AgentAppName + "_" + "PEFT_FT_LORA_R"
	LogMetricsIntervalInStepsEnvVarKey  = AgentAppName + "_" + "PEFT_FT_LOG_MODEL_METRICS_INTERNAL_IN_STEPS"
	CohereLoraConfigRankEnvVarKey       = AgentAppName + "_" + "COHERE_FT_LORA_CONFIG_RANK"
	PeftLoraConfigAlphaEnvVarKey        = AgentAppName + "_" + "PEFT_FT_LORA_ALPHA"
	PeftLearningRateEnvVarKey           = AgentAppName + "_" + "PEFT_FT_LEARNING_RATE"

	LoraAlphaConfigKey     = "loraAlpha"
	LoraDropoutEnvVarKey   = AgentAppName + "_" + "PEFT_FT_LORA_DROPOUT"
	LoraDropoutConfigKey   = "loraDropout"
	PeftModelNameEnvVarKey = AgentAppName + "_" + "PEFT_FT_MODEL_NAME"
	ModelNameConfigKey     = "modelName"
	PeftTypeEnvVarKey      = AgentAppName + "_" + "PEFT_FT_PEFT_TYPE"
)

// Training pod volume name constants
const (
	ModelStorePVCSourceName = "model-storage"
	ModelEmptyDirName       = "model"
	DataEmptyDirName        = "data"
)

// common used constants
const (
	RegionFileVolumeName = "region"
	ADFileVolumeName     = "etc-avalability-domain"
	RealmFileVolumeName  = "etc-identity-realm"

	RegionFileVolumeMountPath = "/etc/region"
	ADFileVolumeMountPath     = "/etc/availability-domain"
	RealmFileVolumeMountPath  = "/etc/identity-realm"
)

// training Constants
const (
	ModelDirectoryPrefix                 = "/mnt/data/models"
	ModelStorePVCMountPath               = "/mnt/models"
	TrainingDataEmptyDirMountPath        = "/mnt/data"
	PeftTrainingOutputModelDirectoryName = "output"
	PeftTrainingMergedModelWeightSuffix  = "-merged-weight"
	PeftFineTunedWeightsDirectory        = "fine-tuned-weights"
	PeftMergedWeightsDirectory           = "base-peft-merged"
	TrainingPathPrefixEnvVarKey          = "PATH_PREFIX"
	TrainingBaselineModelEnvVarKey       = "BASELINE_MODEL"
)

// Cohere training constants
const (
	CohereTrainingRuntimePrefix                             = "cohere-finetuning"
	CohereStorePathPrefix                                   = "/mnt/cohere/"
	CohereTrainingInitModelEmptyDirMountPathFastTransformer = "/model/fastertransformer"
	CohereTrainingLargeGpuRequest                           = "8"
	CohereCommandRFTMergedModelWeightSuffix                 = "-merged-weight"
	CohereCommandRV1Version                                 = "v19-0-0"
	CohereCommandRV2Version                                 = "v20-1-0"
	CommandRBaseModelV1                                     = "command_r"
	CommandRBaseModelV2                                     = "command_r_v2"
	CohereCommandRLoraTrainingModelDirectory                = "output"
	CohereTrainingPathPrefixEnvVarKey                       = "PATH_PREFIX"
	CohereTrainingBaselineModelEnvVarKey                    = "BASELINE_MODEL"
	CohereMultiLoraBaseModelNameKeyword                     = "multi_lora"
	CohereCommandRFTRuntimePrefix                           = "cohere-commandr"
	CohereCommandRMergedWeightsDirectory                    = "model/tensorrt_llm"
	CohereCommandRTFewFTWeightsDirectory                    = "output/tfew_weights"
	CohereCommandRLoraFineTunedWeightsDirectory             = "output/"
	CohereTrainingConfigPbtxt                               = "config.pbtxt"
)

const (

	// DefaultJobReplicas is the default value for the ReplicatedJob replicas.
	DefaultJobReplicas = 1

	// JobSetKind is the Kind name for the JobSet.
	JobSetKind string = "JobSet"

	// JobTrainerNode is the Job name for the trainer node.
	JobTrainerNode string = "trainer-node"

	// JobTrainerInitContainer is the init-container name for the trainer node job.
	JobTrainerInitContainer string = "training-init-container"

	// ContainerTrainer is the container name for the trainer.
	ContainerTrainer string = "trainer"

	// ContainerTrainerPort is the default port for the trainer nodes communication.
	ContainerTrainerPort int32 = 29500

	// JobInitializer is the Job name for the initializer.
	JobInitializer string = "initializer"

	// ContainerModelInitializer is the container name for the model initializer.
	ContainerModelInitializer string = "model-initializer"

	// ContainerDatasetInitializer is the container name for the dataset initializer.
	ContainerDatasetInitializer string = "dataset-initializer"

	// PodGroupKind is the Kind name for the PodGroup.
	PodGroupKind string = "PodGroup"

	// Distributed envs for torchrun.
	// Ref: https://github.com/pytorch/pytorch/blob/3a0d0885171376ed610c8175a19ba40411fc6f3f/torch/distributed/argparse_util.py#L45
	// TorchEnvNumNodes is the env name for the number of training nodes.
	TorchEnvNumNodes string = "PET_NNODES"

	// TorchEnvNumProcPerNode is the env name for the number of procs per node (e.g. number of GPUs per Pod).
	TorchEnvNumProcPerNode string = "PET_NPROC_PER_NODE"

	// TorchEnvNodeRank is the env name for the node RANK
	TorchEnvNodeRank string = "PET_NODE_RANK"

	// TorchEnvMasterAddr is the env name for the master node address.
	TorchEnvMasterAddr string = "PET_MASTER_ADDR"

	// TorchEnvMasterPort is the env name for the master node port.
	TorchEnvMasterPort string = "PET_MASTER_PORT"
)

// FineTunedWeight related constants
const (
	FineTunedWeightMergedWeightsConfigKey = "merged_weights"
	StackedServingConfigKey               = "stacked_serving"
)

type ModelVendor string

const (
	Meta   ModelVendor = "meta"
	Cohere ModelVendor = "cohere"
	OpenAI ModelVendor = "openai"
)

var (
	// JobCompletionIndexFieldPath is the field path for the Job completion index annotation.
	JobCompletionIndexFieldPath string = fmt.Sprintf("metadata.annotations['%s']", batchv1.JobCompletionIndexAnnotation)
)

// constants related to training endpoint call
const (
	TrainingEndpoint = "http://localhost:5000"
	Timeout          = 15 * time.Minute
	RetryInterval    = 1 * time.Minute
)

// constants for training data error handling (error from training container)
const (
	CohereFaxFTDataErrorMessagePrefix      = "please check dataset"
	CohereCommandRFTDataErrorMessagePrefix = "Exception during dataset conversion"
	PeftDataErrorMessagePrefix             = "Data error"
	TerminationLogPath                     = "/dev/termination-log"
)

// BaseModelType enum
type BaseModelType string

const (
	ServingBaseModel    BaseModelType = "Serving"
	FinetuningBaseModel BaseModelType = "Finetuning"
)

// Constants for aiplatform
const (
	ProjectFinalizerName = "project.ome.io.finalizers"
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

func GetPvName(trainjobName string, trainjobNamespace string, baseModelName string) string {
	var maxSubLen = 16
	if len(trainjobNamespace) > maxSubLen {
		trainjobNamespace = trainjobNamespace[len(trainjobNamespace)-maxSubLen:]
	}
	if len(trainjobName) > maxSubLen {
		trainjobName = trainjobName[len(trainjobName)-maxSubLen:]
	}

	if len(baseModelName) > maxSubLen {
		baseModelName = baseModelName[len(baseModelName)-maxSubLen:]
	}
	return fmt.Sprintf("pv-%s-%s-%s", trainjobNamespace, baseModelName, trainjobName)
}

func GetPvcName(trainjobName string, trainjobNamespace string, baseModelName string) string {
	var maxSubLen = 25
	if len(trainjobNamespace) > maxSubLen {
		trainjobNamespace = trainjobNamespace[len(trainjobNamespace)-maxSubLen:]
	}
	if len(trainjobName) > maxSubLen {
		trainjobName = trainjobName[len(trainjobName)-maxSubLen:]
	}

	if len(baseModelName) > maxSubLen {
		baseModelName = baseModelName[len(baseModelName)-maxSubLen:]
	}
	return fmt.Sprintf("pvc-%s-%s-%s", trainjobNamespace, baseModelName, trainjobName)
}
