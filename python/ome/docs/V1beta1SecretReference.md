# V1beta1SecretReference

## Properties

| Name          | Type    | Description                              | Notes           |
|---------------|---------|------------------------------------------|-----------------|
| **key**       | **str** | Key is the key in the secret             | [default to ''] |
| **name**      | **str** | Name is the name of the secret           | [default to ''] |
| **namespace** | **str** | Namespace is the namespace of the secret | [default to ''] |

## Example

```python
from ome.models.v1beta1_secret_reference import V1beta1SecretReference

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1SecretReference from a JSON string
v1beta1_secret_reference_instance = V1beta1SecretReference.from_json(json)
# print the JSON string representation of the object
print(V1beta1SecretReference.to_json())

# convert the object into a dict
v1beta1_secret_reference_dict = v1beta1_secret_reference_instance.to_dict()
# create an instance of V1beta1SecretReference from a dict
v1beta1_secret_reference_from_dict = V1beta1SecretReference.from_dict(v1beta1_secret_reference_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
