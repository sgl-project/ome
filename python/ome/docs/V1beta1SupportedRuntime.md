# V1beta1SupportedRuntime

SupportedRuntime is the schema for supported runtime result of automatic selection

## Properties

| Name     | Type    | Description | Notes           |
|----------|---------|-------------|-----------------|
| **name** | **str** |             | [default to ''] |
| **spec** | [**V1beta1ServingRuntimeSpec**](V1beta1ServingRuntimeSpec.md) |             |

## Example

```python
from ome.models.v1beta1_supported_runtime import V1beta1SupportedRuntime

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1SupportedRuntime from a JSON string
v1beta1_supported_runtime_instance = V1beta1SupportedRuntime.from_json(json)
# print the JSON string representation of the object
print(V1beta1SupportedRuntime.to_json())

# convert the object into a dict
v1beta1_supported_runtime_dict = v1beta1_supported_runtime_instance.to_dict()
# create an instance of V1beta1SupportedRuntime from a dict
v1beta1_supported_runtime_from_dict = V1beta1SupportedRuntime.from_dict(v1beta1_supported_runtime_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
