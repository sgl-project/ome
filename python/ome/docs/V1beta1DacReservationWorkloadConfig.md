# V1beta1DacReservationWorkloadConfig

## Properties

| Name                                      | Type    | Description | Notes           |
|-------------------------------------------|---------|-------------|-----------------|
| **creation_failed_time_threshold_second** | **int** |             | [default to 0]  |
| **image**                                 | **str** |             | [default to ''] |

## Example

```python
from ome.models.v1beta1_dac_reservation_workload_config import V1beta1DacReservationWorkloadConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1DacReservationWorkloadConfig from a JSON string
v1beta1_dac_reservation_workload_config_instance = V1beta1DacReservationWorkloadConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1DacReservationWorkloadConfig.to_json())

# convert the object into a dict
v1beta1_dac_reservation_workload_config_dict = v1beta1_dac_reservation_workload_config_instance.to_dict()
# create an instance of V1beta1DacReservationWorkloadConfig from a dict
v1beta1_dac_reservation_workload_config_from_dict = V1beta1DacReservationWorkloadConfig.from_dict(v1beta1_dac_reservation_workload_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
