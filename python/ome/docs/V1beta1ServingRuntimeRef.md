# V1beta1ServingRuntimeRef

## Properties

| Name          | Type    | Description                                                                                                                                                                                                                         | Notes                      |
|---------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **api_group** | **str** | APIGroup of the resource being referenced Defaults to &#x60;ome.io&#x60; Specifies the Kubernetes API group of the referenced runtime.                                                                                              | [optional]                 |
| **kind**      | **str** | Kind of the runtime being referenced Defaults to ClusterServingRuntime Specifies the Kubernetes resource kind of the referenced runtime. ClusterServingRuntime is a cluster-wide runtime, while ServingRuntime is namespace-scoped. | [optional]                 |
| **name**      | **str** | Name of the runtime being referenced Identifies the specific runtime environment to be used for model execution.                                                                                                                    | [optional] [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
