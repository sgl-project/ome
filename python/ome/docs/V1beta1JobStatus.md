# V1beta1JobStatus

## Properties

| Name          | Type    | Description                                                                                                                                                     | Notes           |
|---------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **active**    | **int** | Active is the number of child Jobs with at least 1 pod in a running or pending state which are not marked for deletion.                                         | [default to 0]  |
| **failed**    | **int** | Failed is the number of failed child Jobs.                                                                                                                      | [default to 0]  |
| **name**      | **str** | Name of the child Job.                                                                                                                                          | [default to ''] |
| **ready**     | **int** | Ready is the number of child Jobs where the number of ready pods and completed pods is greater than or equal to the total expected pod count for the child Job. | [default to 0]  |
| **succeeded** | **int** | Succeeded is the number of successfully completed child Jobs.                                                                                                   | [default to 0]  |
| **suspended** | **int** | Suspended is the number of child Jobs which are in a suspended state.                                                                                           | [default to 0]  |

## Example

```python
from ome.models.v1beta1_job_status import V1beta1JobStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1JobStatus from a JSON string
v1beta1_job_status_instance = V1beta1JobStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1JobStatus.to_json())

# convert the object into a dict
v1beta1_job_status_dict = v1beta1_job_status_instance.to_dict()
# create an instance of V1beta1JobStatus from a dict
v1beta1_job_status_from_dict = V1beta1JobStatus.from_dict(v1beta1_job_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
