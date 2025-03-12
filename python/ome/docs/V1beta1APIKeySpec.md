# V1beta1APIKeySpec

## Properties

| Name                   | Type                                                    | Description                                  | Notes |
| ---------------------- | ------------------------------------------------------- | -------------------------------------------- | ----- |
| **api_key_id**         | **str**                                                 | APIKeyId is the platform-specific API key ID |
| **api_key_secret_ref** | [**V1beta1SecretReference**](V1beta1SecretReference.md) |                                              |
| **name**               | **str**                                                 | Name is the API key name                     |

## Example

```python
from ome.models.v1beta1_api_key_spec import V1beta1APIKeySpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1APIKeySpec from a JSON string
v1beta1_api_key_spec_instance = V1beta1APIKeySpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1APIKeySpec.to_json())

# convert the object into a dict
v1beta1_api_key_spec_dict = v1beta1_api_key_spec_instance.to_dict()
# create an instance of V1beta1APIKeySpec from a dict
v1beta1_api_key_spec_from_dict = V1beta1APIKeySpec.from_dict(v1beta1_api_key_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
