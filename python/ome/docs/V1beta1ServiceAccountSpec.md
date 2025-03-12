# V1beta1ServiceAccountSpec

## Properties

| Name       | Type               | Description                                   | Notes      |
|------------|--------------------|-----------------------------------------------|------------|
| **config** | **Dict[str, str]** | Config contains vendor-specific configuration | [optional] |
| **name**        | **str**                                               | Name is the service account name                                            |
| **permissions** | **List[str]**                                         | Permissions defines the service account permissions                         | [optional] |
| **project_ref** | [**V1beta1CrossReference**](V1beta1CrossReference.md) |                                                                             |
| **role**        | **str**                                               | Role defines the service account&#39;s role in the project, owner or member | [optional] |

## Example

```python
from ome.models.v1beta1_service_account_spec import V1beta1ServiceAccountSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ServiceAccountSpec from a JSON string
v1beta1_service_account_spec_instance = V1beta1ServiceAccountSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1ServiceAccountSpec.to_json())

# convert the object into a dict
v1beta1_service_account_spec_dict = v1beta1_service_account_spec_instance.to_dict()
# create an instance of V1beta1ServiceAccountSpec from a dict
v1beta1_service_account_spec_from_dict = V1beta1ServiceAccountSpec.from_dict(v1beta1_service_account_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
