# V1beta1MLPolicyConfig

MLPolicyConfig represents the runtime-specific configuration for various technologies. One of the following specs can be set.

## Properties

| Name      | Type                                                            | Description | Notes      |
|-----------|-----------------------------------------------------------------|-------------|------------|
| **mpi**   | [**V1beta1MPIMLPolicyConfig**](V1beta1MPIMLPolicyConfig.md)     |             | [optional] |
| **torch** | [**V1beta1TorchMLPolicyConfig**](V1beta1TorchMLPolicyConfig.md) |             | [optional] |

## Example

```python
from ome.models.v1beta1_ml_policy_config import V1beta1MLPolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1MLPolicyConfig from a JSON string
v1beta1_ml_policy_config_instance = V1beta1MLPolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1MLPolicyConfig.to_json())

# convert the object into a dict
v1beta1_ml_policy_config_dict = v1beta1_ml_policy_config_instance.to_dict()
# create an instance of V1beta1MLPolicyConfig from a dict
v1beta1_ml_policy_config_from_dict = V1beta1MLPolicyConfig.from_dict(v1beta1_ml_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
