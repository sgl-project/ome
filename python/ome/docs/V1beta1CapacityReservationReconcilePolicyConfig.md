# V1beta1CapacityReservationReconcilePolicyConfig

## Properties

| Name                                 | Type     | Description | Notes      |
|--------------------------------------|----------|-------------|------------|
| **reconcile_failed_lifecycle_state** | **bool** |             | [optional] |

## Example

```python
from ome.models.v1beta1_capacity_reservation_reconcile_policy_config import V1beta1CapacityReservationReconcilePolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CapacityReservationReconcilePolicyConfig from a JSON string
v1beta1_capacity_reservation_reconcile_policy_config_instance = V1beta1CapacityReservationReconcilePolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1CapacityReservationReconcilePolicyConfig.to_json())

# convert the object into a dict
v1beta1_capacity_reservation_reconcile_policy_config_dict = v1beta1_capacity_reservation_reconcile_policy_config_instance.to_dict()
# create an instance of V1beta1CapacityReservationReconcilePolicyConfig from a dict
v1beta1_capacity_reservation_reconcile_policy_config_from_dict = V1beta1CapacityReservationReconcilePolicyConfig.from_dict(v1beta1_capacity_reservation_reconcile_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
