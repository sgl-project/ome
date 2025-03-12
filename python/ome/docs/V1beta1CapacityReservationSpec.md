# V1beta1CapacityReservationSpec

CapacityReservationSpec defines the desired state of Capacity Reservation.

## Properties

| Name                    | Type                                                                                                                | Description                                                                                               | Notes                      |
|-------------------------|---------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------|----------------------------|
| **allow_borrowing**     | **bool**                                                                                                            | AllowBorrowing defines if this capacity reservation can borrow resources from others.                     | [optional]                 |
| **cohort**              | **str**                                                                                                             | Cohort specifies the cohort that the cluster queue belongs to, which is used for grouping cluster queues. | [optional] [default to ''] |
| **compartment_id**      | **str**                                                                                                             | The compartment ID to use for the Capacity Reservation.                                                   | [optional]                 |
| **preemption_rule**     | [**SigsK8sIoKueueApisKueueV1beta1ClusterQueuePreemption**](SigsK8sIoKueueApisKueueV1beta1ClusterQueuePreemption.md) |                                                                                                           | [optional]                 |
| **priority_class_name** | **str**                                                                                                             | PriorityClassName is the priority class assigned to workloads associated to the Capacity Reservation.     | [optional]                 |
| **resource_groups**     | [**List[SigsK8sIoKueueApisKueueV1beta1ResourceGroup]**](SigsK8sIoKueueApisKueueV1beta1ResourceGroup.md)             | ResourceGroups defines the list of resource groups for the Capacity Reservation. These are the groups of resources that the cluster queue will reserve. Limits the number of items to 50 to avoid exceeding validation complexity limits in Kubernetes API. |

## Example

```python
from ome.models.v1beta1_capacity_reservation_spec import V1beta1CapacityReservationSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CapacityReservationSpec from a JSON string
v1beta1_capacity_reservation_spec_instance = V1beta1CapacityReservationSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1CapacityReservationSpec.to_json())

# convert the object into a dict
v1beta1_capacity_reservation_spec_dict = v1beta1_capacity_reservation_spec_instance.to_dict()
# create an instance of V1beta1CapacityReservationSpec from a dict
v1beta1_capacity_reservation_spec_from_dict = V1beta1CapacityReservationSpec.from_dict(v1beta1_capacity_reservation_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
