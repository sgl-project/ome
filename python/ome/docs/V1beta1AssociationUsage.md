# V1beta1AssociationUsage

AssociationUsage defines the usage of the association.

## Properties

| Name     | Type    | Description              | Notes           |
|----------|---------|--------------------------|-----------------|
| **name** | **str** | Name of the association. | [default to ''] |
| **usage** | [**List[SigsK8sIoKueueApisKueueV1beta1FlavorUsage]**](SigsK8sIoKueueApisKueueV1beta1FlavorUsage.md) | Usage of the association. |

## Example

```python
from ome.models.v1beta1_association_usage import V1beta1AssociationUsage

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1AssociationUsage from a JSON string
v1beta1_association_usage_instance = V1beta1AssociationUsage.from_json(json)
# print the JSON string representation of the object
print(V1beta1AssociationUsage.to_json())

# convert the object into a dict
v1beta1_association_usage_dict = v1beta1_association_usage_instance.to_dict()
# create an instance of V1beta1AssociationUsage from a dict
v1beta1_association_usage_from_dict = V1beta1AssociationUsage.from_dict(v1beta1_association_usage_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
