# V1beta1ObjectReference

ObjectReference contains enough information to let you inspect or modify the referred object.

## Properties

| Name          | Type    | Description                        | Notes      |
|---------------|---------|------------------------------------|------------|
| **name**      | **str** | Name of the referenced object      | [optional] |
| **namespace** | **str** | Namespace of the referenced object | [optional] |

## Example

```python
from ome.models.v1beta1_object_reference import V1beta1ObjectReference

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ObjectReference from a JSON string
v1beta1_object_reference_instance = V1beta1ObjectReference.from_json(json)
# print the JSON string representation of the object
print(V1beta1ObjectReference.to_json())

# convert the object into a dict
v1beta1_object_reference_dict = v1beta1_object_reference_instance.to_dict()
# create an instance of V1beta1ObjectReference from a dict
v1beta1_object_reference_from_dict = V1beta1ObjectReference.from_dict(v1beta1_object_reference_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
