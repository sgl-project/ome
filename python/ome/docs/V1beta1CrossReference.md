# V1beta1CrossReference

## Properties

| Name          | Type    | Description                                                                                   | Notes                      |
|---------------|---------|-----------------------------------------------------------------------------------------------|----------------------------|
| **name**      | **str** | Name is the name of the referenced resource                                                   | [optional] [default to ''] |
| **namespace** | **str** | Namespace is the namespace of the referenced resource (optional for cluster-scoped resources) | [optional]                 |

## Example

```python
from ome.models.v1beta1_cross_reference import V1beta1CrossReference

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CrossReference from a JSON string
v1beta1_cross_reference_instance = V1beta1CrossReference.from_json(json)
# print the JSON string representation of the object
print(V1beta1CrossReference.to_json())

# convert the object into a dict
v1beta1_cross_reference_dict = v1beta1_cross_reference_instance.to_dict()
# create an instance of V1beta1CrossReference from a dict
v1beta1_cross_reference_from_dict = V1beta1CrossReference.from_dict(v1beta1_cross_reference_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
