# V1beta1ServiceMetadata

ServiceMetadata contains metadata fields for recording the backend model server's configuration and version details. This information helps track experiment context, enabling users to filter and query experiments based on server properties.

## Properties

| Name          | Type    | Description                                                                                                                                                   | Notes           |
|---------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| **engine**    | **str** | Engine specifies the backend model server engine. Supported values: \&quot;vLLM\&quot;, \&quot;SGLang\&quot;, \&quot;TGI\&quot;.                              | [default to ''] |
| **gpu_count** | **int** | GpuCount indicates the number of GPU cards available on the model server.                                                                                     | [default to 0]  |
| **gpu_type**  | **str** | GpuType specifies the type of GPU used by the model server. Supported values: \&quot;H100\&quot;, \&quot;A100\&quot;, \&quot;MI300\&quot;, \&quot;A10\&quot;. | [default to ''] |
| **version**   | **str** | Version specifies the version of the model server (e.g., \&quot;0.5.3\&quot;).                                                                                | [default to ''] |

## Example

```python
from ome.models.v1beta1_service_metadata import V1beta1ServiceMetadata

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ServiceMetadata from a JSON string
v1beta1_service_metadata_instance = V1beta1ServiceMetadata.from_json(json)
# print the JSON string representation of the object
print(V1beta1ServiceMetadata.to_json())

# convert the object into a dict
v1beta1_service_metadata_dict = v1beta1_service_metadata_instance.to_dict()
# create an instance of V1beta1ServiceMetadata from a dict
v1beta1_service_metadata_from_dict = V1beta1ServiceMetadata.from_dict(v1beta1_service_metadata_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
