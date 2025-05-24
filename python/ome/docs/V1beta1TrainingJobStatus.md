# V1beta1TrainingJobStatus

## Properties

| Name                    | Type                                              | Description                                                                | Notes      |
|-------------------------|---------------------------------------------------|----------------------------------------------------------------------------|------------|
| **completion_time**     | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |
| **conditions**          | [**list[V1Condition]**](V1Condition.md)           | Conditions is an array of current observed job conditions.                 | [optional] |
| **jobs_status**         | [**list[V1beta1JobStatus]**](V1beta1JobStatus.md) | JobsStatus tracks the child Jobs in TrainJob.                              | [optional] |
| **last_reconcile_time** | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |
| **retry_count**         | **int**                                           | RetryCount represents the number of retries the training job has performed | [optional] |
| **start_time**          | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
