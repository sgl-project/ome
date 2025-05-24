# V1beta1InferenceServiceReference

InferenceServiceReference defines the reference to a Kubernetes inference service.

## Properties

| Name          | Type    | Description                                                                                                                                                            | Notes           |
|---------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **name**      | **str** | Name specifies the name of the inference service to benchmark.                                                                                                         | [default to ''] |
| **namespace** | **str** | Namespace specifies the Kubernetes namespace where the inference service is deployed. Cross-namespace references are allowed but require appropriate RBAC permissions. | [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
