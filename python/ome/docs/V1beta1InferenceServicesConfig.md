# V1beta1InferenceServicesConfig

## Properties

| Name                 | Type                                                                | Description | Notes |
| -------------------- | ------------------------------------------------------------------- | ----------- | ----- |
| **multinode_prober** | [**V1beta1MultiNodeProberConfig**](V1beta1MultiNodeProberConfig.md) |             |
| **oci_etc**          | [**V1beta1OCIConfig**](V1beta1OCIConfig.md)                         |             |

## Example

```python
from ome.models.v1beta1_inference_services_config import V1beta1InferenceServicesConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1InferenceServicesConfig from a JSON string
v1beta1_inference_services_config_instance = V1beta1InferenceServicesConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1InferenceServicesConfig.to_json())

# convert the object into a dict
v1beta1_inference_services_config_dict = v1beta1_inference_services_config_instance.to_dict()
# create an instance of V1beta1InferenceServicesConfig from a dict
v1beta1_inference_services_config_from_dict = V1beta1InferenceServicesConfig.from_dict(v1beta1_inference_services_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
