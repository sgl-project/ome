# V1beta1BenchmarkJobSpec

BenchmarkJobSpec defines the specification for a benchmark job. All fields within this specification collectively represent the desired state and configuration of a BenchmarkJob.

## Properties

| Name                          | Type                                            | Description                                                              | Notes      |
|-------------------------------|-------------------------------------------------|--------------------------------------------------------------------------|------------|
| **additional_request_params** | **dict(str, str)**                              | AdditionalRequestParams contains additional request parameters as a map. | [optional] |
| **dataset**                   | [**V1beta1StorageSpec**](V1beta1StorageSpec.md) |                                                                          | [optional] |
| **endpoint**                      | [**V1beta1EndpointSpec**](V1beta1EndpointSpec.md)                             |                                                                                                                                                                                 |
| **hugging_face_secret_reference** | [**V1beta1HuggingFaceSecretReference**](V1beta1HuggingFaceSecretReference.md) |                                                                                                                                                                                 | [optional]      |
| **max_requests_per_iteration**    | **int**                                                                       | MaxRequestsPerIteration specifies the maximum number of requests for a single iteration. Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency. |
| **max_time_per_iteration**        | **int**                                                                       | MaxTimePerIteration specifies the maximum time (in minutes) for a single iteration. Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency.      |
| **num_concurrency**               | **list[int]**                                                                 | NumConcurrency defines a list of concurrency levels to test during the benchmark. If not provided, defaults will be assigned via genai-bench.                                   | [optional]      |
| **output_location**               | [**V1beta1StorageSpec**](V1beta1StorageSpec.md)                               |                                                                                                                                                                                 |
| **pod_override**                  | [**V1beta1PodOverride**](V1beta1PodOverride.md)                               |                                                                                                                                                                                 | [optional]      |
| **result_folder_name**            | **str**                                                                       | ResultFolderName specifies the name of the folder that stores the benchmark result. A default name will be assigned if not specified.                                           | [optional]      |
| **service_metadata**              | [**V1beta1ServiceMetadata**](V1beta1ServiceMetadata.md)                       |                                                                                                                                                                                 | [optional]      |
| **task**                          | **str**                                                                       | Task specifies the task to benchmark, pattern: &lt;input-modality&gt;-to-&lt;output-modality&gt; (e.g., \&quot;text-to-text\&quot;, \&quot;image-to-text\&quot;).               | [default to ''] |
| **traffic_scenarios**             | **list[str]**                                                                 | TrafficScenarios contains a list of traffic scenarios to simulate during the benchmark. If not provided, defaults will be assigned via genai-bench.                             | [optional]      |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
