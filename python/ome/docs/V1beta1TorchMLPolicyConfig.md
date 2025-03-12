# V1beta1TorchMLPolicyConfig

TorchMLPolicyConfig represents a PyTorch runtime configuration.

## Properties

| Name                  | Type                                                          | Description                                                                                                                                                                                                                                            | Notes      |
|-----------------------|---------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **elastic_policy**    | [**V1beta1TorchElasticPolicy**](V1beta1TorchElasticPolicy.md) |                                                                                                                                                                                                                                                        | [optional] |
| **num_proc_per_node** | **str**                                                       | Number of processes per node. This value is inserted into the &#x60;--nproc-per-node&#x60; argument of the &#x60;torchrun&#x60; CLI. Supported values: &#x60;auto&#x60;, &#x60;cpu&#x60;, &#x60;gpu&#x60;, or int value. Defaults to &#x60;auto&#x60;. | [optional] |

## Example

```python
from ome.models.v1beta1_torch_ml_policy_config import V1beta1TorchMLPolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TorchMLPolicyConfig from a JSON string
v1beta1_torch_ml_policy_config_instance = V1beta1TorchMLPolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1TorchMLPolicyConfig.to_json())

# convert the object into a dict
v1beta1_torch_ml_policy_config_dict = v1beta1_torch_ml_policy_config_instance.to_dict()
# create an instance of V1beta1TorchMLPolicyConfig from a dict
v1beta1_torch_ml_policy_config_from_dict = V1beta1TorchMLPolicyConfig.from_dict(v1beta1_torch_ml_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
