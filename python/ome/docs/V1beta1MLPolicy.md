# V1beta1MLPolicy

MLPolicy represents configuration for the model training with ML-specific parameters.

## Properties

| Name          | Type                                                            | Description                              | Notes      |
|---------------|-----------------------------------------------------------------|------------------------------------------|------------|
| **mpi**       | [**V1beta1MPIMLPolicyConfig**](V1beta1MPIMLPolicyConfig.md)     |                                          | [optional] |
| **num_nodes** | **int**                                                         | Number of training nodes. Defaults to 1. | [optional] |
| **torch**     | [**V1beta1TorchMLPolicyConfig**](V1beta1TorchMLPolicyConfig.md) |                                          | [optional] |

## Example

```python
from ome.models.v1beta1_ml_policy import V1beta1MLPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1MLPolicy from a JSON string
v1beta1_ml_policy_instance = V1beta1MLPolicy.from_json(json)
# print the JSON string representation of the object
print(V1beta1MLPolicy.to_json())

# convert the object into a dict
v1beta1_ml_policy_dict = v1beta1_ml_policy_instance.to_dict()
# create an instance of V1beta1MLPolicy from a dict
v1beta1_ml_policy_from_dict = V1beta1MLPolicy.from_dict(v1beta1_ml_policy_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
