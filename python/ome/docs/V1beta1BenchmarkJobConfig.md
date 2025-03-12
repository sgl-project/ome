# V1beta1BenchmarkJobConfig

## Properties

| Name           | Type                                        | Description | Notes |
| -------------- | ------------------------------------------- | ----------- | ----- |
| **pod_config** | [**V1beta1PodConfig**](V1beta1PodConfig.md) |             |

## Example

```python
from ome.models.v1beta1_benchmark_job_config import V1beta1BenchmarkJobConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1BenchmarkJobConfig from a JSON string
v1beta1_benchmark_job_config_instance = V1beta1BenchmarkJobConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1BenchmarkJobConfig.to_json())

# convert the object into a dict
v1beta1_benchmark_job_config_dict = v1beta1_benchmark_job_config_instance.to_dict()
# create an instance of V1beta1BenchmarkJobConfig from a dict
v1beta1_benchmark_job_config_from_dict = V1beta1BenchmarkJobConfig.from_dict(v1beta1_benchmark_job_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
