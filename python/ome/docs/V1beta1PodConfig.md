# V1beta1PodConfig

## Properties

| Name               | Type    | Description | Notes           |
|--------------------|---------|-------------|-----------------|
| **cpu_limit**      | **str** |             | [default to ''] |
| **cpu_request**    | **str** |             | [default to ''] |
| **image**          | **str** |             | [default to ''] |
| **memory_limit**   | **str** |             | [default to ''] |
| **memory_request** | **str** |             | [default to ''] |

## Example

```python
from ome.models.v1beta1_pod_config import V1beta1PodConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1PodConfig from a JSON string
v1beta1_pod_config_instance = V1beta1PodConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1PodConfig.to_json())

# convert the object into a dict
v1beta1_pod_config_dict = v1beta1_pod_config_instance.to_dict()
# create an instance of V1beta1PodConfig from a dict
v1beta1_pod_config_from_dict = V1beta1PodConfig.from_dict(v1beta1_pod_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
