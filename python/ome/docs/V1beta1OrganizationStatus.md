# V1beta1OrganizationStatus

## Properties

| Name           | Type                                    | Description                                                                     | Notes      |
|----------------|-----------------------------------------|---------------------------------------------------------------------------------|------------|
| **conditions** | [**List[V1Condition]**](V1Condition.md) | Conditions represent the latest available observations of an object&#39;s state | [optional] |

## Example

```python
from ome.models.v1beta1_organization_status import V1beta1OrganizationStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1OrganizationStatus from a JSON string
v1beta1_organization_status_instance = V1beta1OrganizationStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1OrganizationStatus.to_json())

# convert the object into a dict
v1beta1_organization_status_dict = v1beta1_organization_status_instance.to_dict()
# create an instance of V1beta1OrganizationStatus from a dict
v1beta1_organization_status_from_dict = V1beta1OrganizationStatus.from_dict(v1beta1_organization_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
