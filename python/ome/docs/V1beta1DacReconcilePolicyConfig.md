# V1beta1DacReconcilePolicyConfig

## Properties

| Name                                 | Type     | Description | Notes      |
|--------------------------------------|----------|-------------|------------|
| **reconcile_failed_lifecycle_state** | **bool** |             | [optional] |
| **reconcile_with_kueue**             | **bool** |             | [optional] |

## Example

```python
from ome.models.v1beta1_dac_reconcile_policy_config import V1beta1DacReconcilePolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1DacReconcilePolicyConfig from a JSON string
v1beta1_dac_reconcile_policy_config_instance = V1beta1DacReconcilePolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1DacReconcilePolicyConfig.to_json())

# convert the object into a dict
v1beta1_dac_reconcile_policy_config_dict = v1beta1_dac_reconcile_policy_config_instance.to_dict()
# create an instance of V1beta1DacReconcilePolicyConfig from a dict
v1beta1_dac_reconcile_policy_config_from_dict = V1beta1DacReconcilePolicyConfig.from_dict(v1beta1_dac_reconcile_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
