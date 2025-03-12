# V1beta1TrainingJobStatus

## Properties

| Name                    | Type                                              | Description                                                                | Notes      |
|-------------------------|---------------------------------------------------|----------------------------------------------------------------------------|------------|
| **completion_time**     | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |
| **conditions**          | [**List[V1Condition]**](V1Condition.md)           | Conditions is an array of current observed job conditions.                 | [optional] |
| **jobs_status**         | [**List[V1beta1JobStatus]**](V1beta1JobStatus.md) | JobsStatus tracks the child Jobs in TrainJob.                              | [optional] |
| **last_reconcile_time** | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |
| **retry_count**         | **int**                                           | RetryCount represents the number of retries the training job has performed | [optional] |
| **start_time**          | [**V1Time**](V1Time.md)                           |                                                                            | [optional] |

## Example

```python
from ome.models.v1beta1_training_job_status import V1beta1TrainingJobStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TrainingJobStatus from a JSON string
v1beta1_training_job_status_instance = V1beta1TrainingJobStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1TrainingJobStatus.to_json())

# convert the object into a dict
v1beta1_training_job_status_dict = v1beta1_training_job_status_instance.to_dict()
# create an instance of V1beta1TrainingJobStatus from a dict
v1beta1_training_job_status_from_dict = V1beta1TrainingJobStatus.from_dict(v1beta1_training_job_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
