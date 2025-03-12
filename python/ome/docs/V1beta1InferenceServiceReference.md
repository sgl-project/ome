# V1beta1InferenceServiceReference

InferenceServiceReference defines the reference to a Kubernetes inference service.

## Properties

| Name          | Type    | Description                                                                                                                                                            | Notes           |
|---------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **name**      | **str** | Name specifies the name of the inference service to benchmark.                                                                                                         | [default to ''] |
| **namespace** | **str** | Namespace specifies the Kubernetes namespace where the inference service is deployed. Cross-namespace references are allowed but require appropriate RBAC permissions. | [default to ''] |

## Example

```python
from ome.models.v1beta1_inference_service_reference import V1beta1InferenceServiceReference

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1InferenceServiceReference from a JSON string
v1beta1_inference_service_reference_instance = V1beta1InferenceServiceReference.from_json(json)
# print the JSON string representation of the object
print(V1beta1InferenceServiceReference.to_json())

# convert the object into a dict
v1beta1_inference_service_reference_dict = v1beta1_inference_service_reference_instance.to_dict()
# create an instance of V1beta1InferenceServiceReference from a dict
v1beta1_inference_service_reference_from_dict = V1beta1InferenceServiceReference.from_dict(v1beta1_inference_service_reference_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
