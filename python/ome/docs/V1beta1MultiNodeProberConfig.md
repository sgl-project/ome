# V1beta1MultiNodeProberConfig

## Properties

| Name                              | Type    | Description | Notes           |
|-----------------------------------|---------|-------------|-----------------|
| **cpu_limit**                     | **str** |             | [default to ''] |
| **cpu_request**                   | **str** |             | [default to ''] |
| **image**                         | **str** |             | [default to ''] |
| **memory_limit**                  | **str** |             | [default to ''] |
| **memory_request**                | **str** |             | [default to ''] |
| **startup_failure_threshold**     | **int** |             | [default to 0]  |
| **startup_initial_delay_seconds** | **int** |             | [default to 0]  |
| **startup_period_seconds**        | **int** |             | [default to 0]  |
| **startup_timeout_seconds**       | **int** |             | [default to 0]  |
| **unavailable_threshold_seconds** | **int** |             | [default to 0]  |

## Example

```python
from ome.models.v1beta1_multi_node_prober_config import V1beta1MultiNodeProberConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1MultiNodeProberConfig from a JSON string
v1beta1_multi_node_prober_config_instance = V1beta1MultiNodeProberConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1MultiNodeProberConfig.to_json())

# convert the object into a dict
v1beta1_multi_node_prober_config_dict = v1beta1_multi_node_prober_config_instance.to_dict()
# create an instance of V1beta1MultiNodeProberConfig from a dict
v1beta1_multi_node_prober_config_from_dict = V1beta1MultiNodeProberConfig.from_dict(v1beta1_multi_node_prober_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
