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

## Example

```python
from ome.models.v1beta1_benchmark_job_status import V1beta1BenchmarkJobStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1BenchmarkJobStatus from a JSON string
v1beta1_benchmark_job_status_instance = V1beta1BenchmarkJobStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1BenchmarkJobStatus.to_json())

# convert the object into a dict
v1beta1_benchmark_job_status_dict = v1beta1_benchmark_job_status_instance.to_dict()
# create an instance of V1beta1BenchmarkJobStatus from a dict
v1beta1_benchmark_job_status_from_dict = V1beta1BenchmarkJobStatus.from_dict(v1beta1_benchmark_job_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
