# V1beta1EndpointSpec

EndpointSpec defines a reference to an inference service. It supports either a Kubernetes-style reference (InferenceService) or an Endpoint struct for a direct URL. Cross-namespace references are supported for InferenceService but require appropriate RBAC permissions to access resources in the target namespace.

## Properties

| Name                  | Type                                                                        | Description | Notes      |
|-----------------------|-----------------------------------------------------------------------------|-------------|------------|
| **endpoint**          | [**V1beta1Endpoint**](V1beta1Endpoint.md)                                   |             | [optional] |
| **inference_service** | [**V1beta1InferenceServiceReference**](V1beta1InferenceServiceReference.md) |             | [optional] |

## Example

```python
from ome.models.v1beta1_endpoint_spec import V1beta1EndpointSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1EndpointSpec from a JSON string
v1beta1_endpoint_spec_instance = V1beta1EndpointSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1EndpointSpec.to_json())

# convert the object into a dict
v1beta1_endpoint_spec_dict = v1beta1_endpoint_spec_instance.to_dict()
# create an instance of V1beta1EndpointSpec from a dict
v1beta1_endpoint_spec_from_dict = V1beta1EndpointSpec.from_dict(v1beta1_endpoint_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
