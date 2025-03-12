# V1beta1OrganizationSpec

## Properties

| Name                | Type                                                    | Description                                             | Notes           |
|---------------------|---------------------------------------------------------|---------------------------------------------------------|-----------------|
| **config**          | **Dict[str, str]**                                      | Config contains vendor-specific configuration           | [optional]      |
| **disabled**        | **bool**                                                | Disabled indicates whether the organization is disabled | [optional]      |
| **organization_id** | **str**                                                 | OrganizationID is the platform-specific organization ID | [default to ''] |
| **secret_ref**      | [**V1beta1SecretReference**](V1beta1SecretReference.md) |                                                         | [optional]      |
| **vendor**          | **str**                                                 | Vendor specifies the AI platform vendor (e.g., \&quot;openai\&quot;, \&quot;anthropic\&quot;) |

## Example

```python
from ome.models.v1beta1_organization_spec import V1beta1OrganizationSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1OrganizationSpec from a JSON string
v1beta1_organization_spec_instance = V1beta1OrganizationSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1OrganizationSpec.to_json())

# convert the object into a dict
v1beta1_organization_spec_dict = v1beta1_organization_spec_instance.to_dict()
# create an instance of V1beta1OrganizationSpec from a dict
v1beta1_organization_spec_from_dict = V1beta1OrganizationSpec.from_dict(v1beta1_organization_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
