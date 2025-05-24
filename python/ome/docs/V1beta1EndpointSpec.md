# V1beta1EndpointSpec

EndpointSpec defines a reference to an inference service. It supports either a Kubernetes-style reference (InferenceService) or an Endpoint struct for a direct URL. Cross-namespace references are supported for InferenceService but require appropriate RBAC permissions to access resources in the target namespace.

## Properties

| Name                  | Type                                                                        | Description | Notes      |
|-----------------------|-----------------------------------------------------------------------------|-------------|------------|
| **endpoint**          | [**V1beta1Endpoint**](V1beta1Endpoint.md)                                   |             | [optional] |
| **inference_service** | [**V1beta1InferenceServiceReference**](V1beta1InferenceServiceReference.md) |             | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
