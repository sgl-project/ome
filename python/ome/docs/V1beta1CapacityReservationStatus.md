# V1beta1CapacityReservationStatus

CapacityReservationStatus defines the observed status of CapacityReservation.

## Properties

| Name                                     | Type                                                                                                | Description                                                                                                                                                               | Notes      |
|------------------------------------------|-----------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **allocatable**                          | [**List[SigsK8sIoKueueApisKueueV1beta1FlavorUsage]**](SigsK8sIoKueueApisKueueV1beta1FlavorUsage.md) | Allocatable represents the resources that are available for scheduling.                                                                                                   | [optional] |
| **association_usages**                   | [**List[V1beta1AssociationUsage]**](V1beta1AssociationUsage.md)                                     | Usages of associations An association can be a DAC or a Workload                                                                                                          | [optional] |
| **capacity**                             | [**List[SigsK8sIoKueueApisKueueV1beta1FlavorUsage]**](SigsK8sIoKueueApisKueueV1beta1FlavorUsage.md) | Capacity represents the total resources available in this capacity reservation.                                                                                           | [optional] |
| **capacity_reservation_lifecycle_state** | **str**                                                                                             | CapacityReservationLifecycleState indicates the current phase of the CapacityReservation (e.g., \&quot;active\&quot;, \&quot;creating\&quot;, \&quot;Failed\&quot; etc.). | [optional] |
| **conditions**                           | [**List[V1beta1CapacityReservationCondition]**](V1beta1CapacityReservationCondition.md)             | Conditions represents health and operational states.                                                                                                                      | [optional] |
| **lifecycle_detail**                     | **str**                                                                                             | A message describing the current state in more detail that can provide actionable information.                                                                            | [optional] |

## Example

```python
from ome.models.v1beta1_capacity_reservation_status import V1beta1CapacityReservationStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CapacityReservationStatus from a JSON string
v1beta1_capacity_reservation_status_instance = V1beta1CapacityReservationStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1CapacityReservationStatus.to_json())

# convert the object into a dict
v1beta1_capacity_reservation_status_dict = v1beta1_capacity_reservation_status_instance.to_dict()
# create an instance of V1beta1CapacityReservationStatus from a dict
v1beta1_capacity_reservation_status_from_dict = V1beta1CapacityReservationStatus.from_dict(v1beta1_capacity_reservation_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
