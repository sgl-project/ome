# V1beta1AIPlatformConfig

## Properties

| Name              | Type                                              | Description | Notes |
| ----------------- | ------------------------------------------------- | ----------- | ----- |
| **secret_config** | [**V1beta1SecretConfig**](V1beta1SecretConfig.md) |             |

## Example

```python
from ome.models.v1beta1_ai_platform_config import V1beta1AIPlatformConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1AIPlatformConfig from a JSON string
v1beta1_ai_platform_config_instance = V1beta1AIPlatformConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1AIPlatformConfig.to_json())

# convert the object into a dict
v1beta1_ai_platform_config_dict = v1beta1_ai_platform_config_instance.to_dict()
# create an instance of V1beta1AIPlatformConfig from a dict
v1beta1_ai_platform_config_from_dict = V1beta1AIPlatformConfig.from_dict(v1beta1_ai_platform_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
