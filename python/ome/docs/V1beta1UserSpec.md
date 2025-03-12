# V1beta1UserSpec

## Properties

| Name       | Type               | Description                                   | Notes           |
|------------|--------------------|-----------------------------------------------|-----------------|
| **config** | **Dict[str, str]** | Config contains vendor-specific configuration | [optional]      |
| **email**  | **str**            | Email is the user&#39;s email address         | [default to ''] |
| **project_ref** | [**V1beta1CrossReference**](V1beta1CrossReference.md) |                                                                  |
| **role**        | **str**                                               | Role defines the user&#39;s role in the project, owner or member | [optional] [default to ''] |

## Example

```python
from ome.models.v1beta1_user_spec import V1beta1UserSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1UserSpec from a JSON string
v1beta1_user_spec_instance = V1beta1UserSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1UserSpec.to_json())

# convert the object into a dict
v1beta1_user_spec_dict = v1beta1_user_spec_instance.to_dict()
# create an instance of V1beta1UserSpec from a dict
v1beta1_user_spec_from_dict = V1beta1UserSpec.from_dict(v1beta1_user_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
