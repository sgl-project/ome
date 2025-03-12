# V1beta1CapacityReservationCondition

CapacityReservationCondition defines health and operational status of the capacity reservation.

## Properties

| Name                     | Type                    | Description                                                                 | Notes           |
|--------------------------|-------------------------|-----------------------------------------------------------------------------|-----------------|
| **last_transition_time** | [**V1Time**](V1Time.md) |                                                                             | [optional]      |
| **message**              | **str**                 | Message is a human-readable message indicating details about the condition. | [optional]      |
| **reason**               | **str**                 | Reason for the condition&#39;s last transition.                             | [optional]      |
| **status**               | **str**                 | Status of the condition.                                                    | [default to ''] |
| **type**                 | **str**                 | Type of condition.                                                          | [default to ''] |

## Example

```python
from ome.models.v1beta1_capacity_reservation_condition import V1beta1CapacityReservationCondition

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CapacityReservationCondition from a JSON string
v1beta1_capacity_reservation_condition_instance = V1beta1CapacityReservationCondition.from_json(json)
# print the JSON string representation of the object
print(V1beta1CapacityReservationCondition.to_json())

# convert the object into a dict
v1beta1_capacity_reservation_condition_dict = v1beta1_capacity_reservation_condition_instance.to_dict()
# create an instance of V1beta1CapacityReservationCondition from a dict
v1beta1_capacity_reservation_condition_from_dict = V1beta1CapacityReservationCondition.from_dict(v1beta1_capacity_reservation_condition_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
