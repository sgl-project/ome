# V1beta1ProjectSpec

## Properties

| Name            | Type               | Description                                   | Notes           |
|-----------------|--------------------|-----------------------------------------------|-----------------|
| **config**      | **Dict[str, str]** | Config contains vendor-specific configuration | [optional]      |
| **description** | **str**            | Description is the project description        | [optional]      |
| **name**        | **str**            | Name is the project name                      | [default to ''] |
| **organization_ref** | [**V1beta1CrossReference**](V1beta1CrossReference.md) |                                               |

## Example

```python
from ome.models.v1beta1_project_spec import V1beta1ProjectSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ProjectSpec from a JSON string
v1beta1_project_spec_instance = V1beta1ProjectSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1ProjectSpec.to_json())

# convert the object into a dict
v1beta1_project_spec_dict = v1beta1_project_spec_instance.to_dict()
# create an instance of V1beta1ProjectSpec from a dict
v1beta1_project_spec_from_dict = V1beta1ProjectSpec.from_dict(v1beta1_project_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
