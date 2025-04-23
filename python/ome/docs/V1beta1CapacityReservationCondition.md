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

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
