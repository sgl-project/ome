# V1beta1ServiceMetadata

ServiceMetadata contains metadata fields for recording the backend model server's configuration and version details. This information helps track experiment context, enabling users to filter and query experiments based on server properties.

## Properties

| Name          | Type    | Description                                                                                                                                                   | Notes           |
|---------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **engine**    | **str** | Engine specifies the backend model server engine. Supported values: \&quot;vLLM\&quot;, \&quot;SGLang\&quot;, \&quot;TGI\&quot;.                              | [default to ''] |
| **gpu_count** | **int** | GpuCount indicates the number of GPU cards available on the model server.                                                                                     | [default to 0]  |
| **gpu_type**  | **str** | GpuType specifies the type of GPU used by the model server. Supported values: \&quot;H100\&quot;, \&quot;A100\&quot;, \&quot;MI300\&quot;, \&quot;A10\&quot;. | [default to ''] |
| **version**   | **str** | Version specifies the version of the model server (e.g., \&quot;0.5.3\&quot;).                                                                                | [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
