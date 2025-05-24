# V1beta1BenchmarkJobStatus

BenchmarkJobStatus reflects the state and results of the benchmark job. It will be set and updated by the controller.

## Properties

| Name                    | Type                    | Description                                                                                                                                           | Notes           |
|-------------------------|-------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **completion_time**     | [**V1Time**](V1Time.md) |                                                                                                                                                       | [optional]      |
| **details**             | **str**                 | Details provide additional information or metadata about the benchmark job.                                                                           | [optional]      |
| **failure_message**     | **str**                 | FailureMessage contains any error messages if the benchmark job failed.                                                                               | [optional]      |
| **last_reconcile_time** | [**V1Time**](V1Time.md) |                                                                                                                                                       | [optional]      |
| **start_time**          | [**V1Time**](V1Time.md) |                                                                                                                                                       | [optional]      |
| **state**               | **str**                 | State represents the current state of the benchmark job: \&quot;Pending\&quot;, \&quot;Running\&quot;, \&quot;Completed\&quot;, \&quot;Failed\&quot;. | [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
