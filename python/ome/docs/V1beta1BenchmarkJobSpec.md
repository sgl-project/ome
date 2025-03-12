# V1beta1BenchmarkJobSpec

BenchmarkJobSpec defines the specification for a benchmark job. All fields within this specification collectively represent the desired state and configuration of a BenchmarkJob.

## Properties

| Name                          | Type                                            | Description                                                              | Notes      |
|-------------------------------|-------------------------------------------------|--------------------------------------------------------------------------|------------|
| **additional_request_params** | **Dict[str, str]**                              | AdditionalRequestParams contains additional request parameters as a map. | [optional] |
| **dataset**                   | [**V1beta1StorageSpec**](V1beta1StorageSpec.md) |                                                                          | [optional] |
| **endpoint**                      | [**V1beta1EndpointSpec**](V1beta1EndpointSpec.md)                             |                                                                                                                                                                                 |
| **hugging_face_secret_reference** | [**V1beta1HuggingFaceSecretReference**](V1beta1HuggingFaceSecretReference.md) |                                                                                                                                                                                 | [optional]      |
| **max_requests_per_iteration**    | **int**                                                                       | MaxRequestsPerIteration specifies the maximum number of requests for a single iteration. Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency. |
| **max_time_per_iteration**        | **int**                                                                       | MaxTimePerIteration specifies the maximum time (in minutes) for a single iteration. Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency.      |
| **num_concurrency**               | **List[int]**                                                                 | NumConcurrency defines a list of concurrency levels to test during the benchmark. If not provided, defaults will be assigned via genai-bench.                                   | [optional]      |
| **output_location**               | [**V1beta1StorageSpec**](V1beta1StorageSpec.md)                               |                                                                                                                                                                                 |
| **pod_override**                  | [**V1beta1PodOverride**](V1beta1PodOverride.md)                               |                                                                                                                                                                                 | [optional]      |
| **result_folder_name**            | **str**                                                                       | ResultFolderName specifies the name of the folder that stores the benchmark result. A default name will be assigned if not specified.                                           | [optional]      |
| **service_metadata**              | [**V1beta1ServiceMetadata**](V1beta1ServiceMetadata.md)                       |                                                                                                                                                                                 | [optional]      |
| **task**                          | **str**                                                                       | Task specifies the task to benchmark, pattern: &lt;input-modality&gt;-to-&lt;output-modality&gt; (e.g., \&quot;text-to-text\&quot;, \&quot;image-to-text\&quot;).               | [default to ''] |
| **traffic_scenarios**             | **List[str]**                                                                 | TrafficScenarios contains a list of traffic scenarios to simulate during the benchmark. If not provided, defaults will be assigned via genai-bench.                             | [optional]      |

## Example

```python
from ome.models.v1beta1_benchmark_job_spec import V1beta1BenchmarkJobSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1BenchmarkJobSpec from a JSON string
v1beta1_benchmark_job_spec_instance = V1beta1BenchmarkJobSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1BenchmarkJobSpec.to_json())

# convert the object into a dict
v1beta1_benchmark_job_spec_dict = v1beta1_benchmark_job_spec_instance.to_dict()
# create an instance of V1beta1BenchmarkJobSpec from a dict
v1beta1_benchmark_job_spec_from_dict = V1beta1BenchmarkJobSpec.from_dict(v1beta1_benchmark_job_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
