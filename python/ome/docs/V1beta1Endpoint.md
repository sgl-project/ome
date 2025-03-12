# V1beta1Endpoint

Endpoint defines a direct URL-based inference service with additional API configuration.

## Properties

| Name           | Type    | Description                                                                                                                                                                                                   | Notes           |
|----------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **api_format** | **str** | APIFormat specifies the type of API, such as \&quot;openai\&quot; or \&quot;oci-cohere\&quot;.                                                                                                                | [default to ''] |
| **model_name** | **str** | ModelName specifies the name of the model being served at the endpoint. Useful for endpoints that require model-specific configuration. For instance, for openai API, this is a required field in the payload | [optional]      |
| **url**        | **str** | URL represents the endpoint URL for the inference service.                                                                                                                                                    | [default to ''] |

## Example

```python
from ome.models.v1beta1_endpoint import V1beta1Endpoint

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1Endpoint from a JSON string
v1beta1_endpoint_instance = V1beta1Endpoint.from_json(json)
# print the JSON string representation of the object
print(V1beta1Endpoint.to_json())

# convert the object into a dict
v1beta1_endpoint_dict = v1beta1_endpoint_instance.to_dict()
# create an instance of V1beta1Endpoint from a dict
v1beta1_endpoint_from_dict = V1beta1Endpoint.from_dict(v1beta1_endpoint_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
