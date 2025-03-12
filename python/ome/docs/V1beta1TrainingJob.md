# V1beta1TrainingJob

TrainingJob is the Schema for the TrainingJobs API

## Properties

| Name            | Type                                                                                                        | Description                                                                                                                                                                                                                                                                                        | Notes      |
|-----------------|-------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **api_version** | **str**                                                                                                     | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources  | [optional] |
| **kind**        | **str**                                                                                                     | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] |
| **metadata**    | [**V1ObjectMeta**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ObjectMeta.md) |                                                                                                                                                                                                                                                                                                    | [optional] |
| **spec**        | [**V1beta1TrainingJobSpec**](V1beta1TrainingJobSpec.md)                                                     |                                                                                                                                                                                                                                                                                                    | [optional] |
| **status**      | [**V1beta1TrainingJobStatus**](V1beta1TrainingJobStatus.md)                                                 |                                                                                                                                                                                                                                                                                                    | [optional] |

## Example

```python
from ome.models.v1beta1_training_job import V1beta1TrainingJob

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TrainingJob from a JSON string
v1beta1_training_job_instance = V1beta1TrainingJob.from_json(json)
# print the JSON string representation of the object
print(V1beta1TrainingJob.to_json())

# convert the object into a dict
v1beta1_training_job_dict = v1beta1_training_job_instance.to_dict()
# create an instance of V1beta1TrainingJob from a dict
v1beta1_training_job_from_dict = V1beta1TrainingJob.from_dict(v1beta1_training_job_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
