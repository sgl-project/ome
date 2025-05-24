# V1beta1SecretConfig

## Properties

| Name                          | Type     | Description | Notes              |
|-------------------------------|----------|-------------|--------------------|
| **namespace**                 | **str**  |             | [default to '']    |
| **secret_name**               | **str**  |             | [default to '']    |
| **write_to_common_namespace** | **bool** |             | [default to False] |

## Example

```python
from ome.models.v1beta1_secret_config import V1beta1SecretConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1SecretConfig from a JSON string
v1beta1_secret_config_instance = V1beta1SecretConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1SecretConfig.to_json())

# convert the object into a dict
v1beta1_secret_config_dict = v1beta1_secret_config_instance.to_dict()
# create an instance of V1beta1SecretConfig from a dict
v1beta1_secret_config_from_dict = V1beta1SecretConfig.from_dict(v1beta1_secret_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
