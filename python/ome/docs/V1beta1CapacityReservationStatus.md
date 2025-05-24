# V1beta1CapacityReservationStatus

CapacityReservationStatus defines the observed status of CapacityReservation.

## Properties

| Name                                     | Type                                                                                                | Description                                                                                                                                                               | Notes      |
|------------------------------------------|-----------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **allocatable**                          | [**list[SigsK8sIoKueueApisKueueV1beta1FlavorUsage]**](SigsK8sIoKueueApisKueueV1beta1FlavorUsage.md) | Allocatable represents the resources that are available for scheduling.                                                                                                   | [optional] |
| **association_usages**                   | [**list[V1beta1AssociationUsage]**](V1beta1AssociationUsage.md)                                     | Usages of associations An association can be a DAC or a Workload                                                                                                          | [optional] |
| **capacity**                             | [**list[SigsK8sIoKueueApisKueueV1beta1FlavorUsage]**](SigsK8sIoKueueApisKueueV1beta1FlavorUsage.md) | Capacity represents the total resources available in this capacity reservation.                                                                                           | [optional] |
| **capacity_reservation_lifecycle_state** | **str**                                                                                             | CapacityReservationLifecycleState indicates the current phase of the CapacityReservation (e.g., \&quot;active\&quot;, \&quot;creating\&quot;, \&quot;Failed\&quot; etc.). | [optional] |
| **conditions**                           | [**list[V1beta1CapacityReservationCondition]**](V1beta1CapacityReservationCondition.md)             | Conditions represents health and operational states.                                                                                                                      | [optional] |
| **lifecycle_detail**                     | **str**                                                                                             | A message describing the current state in more detail that can provide actionable information.                                                                            | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
