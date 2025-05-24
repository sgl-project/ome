# V1beta1Endpoint

Endpoint defines a direct URL-based inference service with additional API configuration.

## Properties

| Name           | Type    | Description                                                                                                                                                                                                   | Notes           |
|----------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **api_format** | **str** | APIFormat specifies the type of API, such as \&quot;openai\&quot; or \&quot;oci-cohere\&quot;.                                                                                                                | [default to ''] |
| **model_name** | **str** | ModelName specifies the name of the model being served at the endpoint. Useful for endpoints that require model-specific configuration. For instance, for openai API, this is a required field in the payload | [optional]      |
| **url**        | **str** | URL represents the endpoint URL for the inference service.                                                                                                                                                    | [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
