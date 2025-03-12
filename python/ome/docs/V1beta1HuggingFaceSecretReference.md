# V1beta1HuggingFaceSecretReference

HuggingFaceSecretReference defines a reference to a Kubernetes Secret containing the Hugging Face API key. This secret must reside in the same namespace as the BenchmarkJob. Cross-namespace references are not allowed for security and simplicity.

## Properties

| Name     | Type    | Description                                                                                                               | Notes                      |
|----------|---------|---------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **name** | **str** | Name of the secret containing the Hugging Face API key. The secret must reside in the same namespace as the BenchmarkJob. | [optional] [default to ''] |

## Example

```python
from ome.models.v1beta1_hugging_face_secret_reference import V1beta1HuggingFaceSecretReference

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1HuggingFaceSecretReference from a JSON string
v1beta1_hugging_face_secret_reference_instance = V1beta1HuggingFaceSecretReference.from_json(json)
# print the JSON string representation of the object
print(V1beta1HuggingFaceSecretReference.to_json())

# convert the object into a dict
v1beta1_hugging_face_secret_reference_dict = v1beta1_hugging_face_secret_reference_instance.to_dict()
# create an instance of V1beta1HuggingFaceSecretReference from a dict
v1beta1_hugging_face_secret_reference_from_dict = V1beta1HuggingFaceSecretReference.from_dict(v1beta1_hugging_face_secret_reference_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
