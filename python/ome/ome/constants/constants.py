import os

# ome K8S constants
OME_GROUP = "ome.io"
OME_KIND_INFERENCESERVICE = "InferenceService"
OME_PLURAL_INFERENCESERVICE = "inferenceservices"
OME_KIND_INFERENCEGRAPH = "InferenceGraph"
OME_PLURAL_INFERENCEGRAPH = "inferencegraphs"
OME_KIND_TRAININGJOB = "TrainingJob"
OME_PLURAL_TRAININGJOB = "trainingjobs"
OME_KIND_TRAININGRUNTIME = "TrainingRuntime"
OME_PLURAL_TRAININGRUNTIME = "trainingruntimes"
OME_KIND_CLUSTERTRAININGRUNTIME = "ClusterTrainingRuntime"
OME_PLURAL_CLUSTERTRAININGRUNTIME = "clustertrainingruntimes"
OME_KIND_BASEMODEL = "BaseModel"
OME_PLURAL_BASEMODEL = "basemodels"
OME_KIND_CLUSTERBASEMODEL = "ClusterBaseModel"
OME_PLURAL_CLUSTERBASEMODEL = "clusterbasemodels"
OME_KIND_FINETUNEDWEIGHT = "FineTunedWeight"
OME_PLURAL_FINETUNEDWEIGHT = "finetunedweights"
OME_KIND_SERVINGRUNTIME = "ServingRuntime"
OME_PLURAL_SERVINGRUNTIME = "servingruntimes"
OME_KIND_CLUSTERSERVINGRUNTIME = "ClusterServingRuntime"
OME_PLURAL_CLUSTERSERVINGRUNTIME = "clusterservingruntimes"
OME_KIND_BENCHMARKJOB = "BenchmarkJob"
OME_PLURAL_BENCHMARKJOB = "benchmarkjobs"
OME_KIND_CAPACITYRESERVATION = "CapacityReservation"
OME_PLURAL_CAPACITYRESERVATION = "capacityreservations"
OME_KIND_CLUSTERCAPACITYRESERVATION = "ClusterCapacityReservation"
OME_PLURAL_CLUSTERCAPACITYRESERVATION = "clustercapacityreservations"
OME_KIND_ORGANIZATION = "Organization"
OME_PLURAL_ORGANIZATION = "organizations"
OME_KIND_PROJECT = "Project"
OME_PLURAL_PROJECT = "projects"
OME_KIND_SERVICEACCOUNT = "ServiceAccount"
OME_PLURAL_SERVICEACCOUNT = "serviceaccounts"
OME_KIND_USER = "User"
OME_PLURAL_USER = "users"
OME_KIND_RATELIMIT = "RateLimit"
OME_PLURAL_RATELIMIT = "ratelimits"
OME_KIND_DEDICATEDAICLUSTER = "DedicatedAICluster"
OME_PLURAL_DEDICATEDAICLUSTER = "dedicatedaiclusters"
OME_V1BETA1_VERSION = "v1beta1"

OME_V1BETA1 = OME_GROUP + "/" + OME_V1BETA1_VERSION

OME_LOGLEVEL = os.environ.get("OME_LOGLEVEL", "INFO").upper()

# INFERENCESERVICE credentials common constants
INFERENCESERVICE_CONFIG_MAP_NAME = "inferenceservice-config"
INFERENCESERVICE_SYSTEM_NAMESPACE = "ome"
DEFAULT_SECRET_NAME = "ome-secret-"
DEFAULT_SA_NAME = "ome-service-credentials"
# K8S status key constants
OBSERVED_GENERATION = "observedGeneration"

# K8S metadata key constants
GENERATION = "generation"

EXPLAINER_BASE_URL_FORMAT = "{0}://{1}"
