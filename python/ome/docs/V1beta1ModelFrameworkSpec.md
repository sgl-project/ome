# V1beta1ModelFrameworkSpec

## Properties

| Name        | Type    | Description                                                                                                                                                                       | Notes                      |
|-------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **name**    | **str** | Name of the library in which the model is stored, e.g., \&quot;ONNX\&quot;, \&quot;TensorFlow\&quot;, \&quot;PyTorch\&quot;, \&quot;Transformer\&quot;, \&quot;TensorRTLLM\&quot; | [optional] [default to ''] |
| **version** | **str** | Version of the library. Used in validating that a runtime supports a predictor. It Can be \&quot;major\&quot;, \&quot;major.minor\&quot; or \&quot;major.minor.patch\&quot;.      | [optional]                 |

## Example

```python
from ome.models.v1beta1_model_framework_spec import V1beta1ModelFrameworkSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ModelFrameworkSpec from a JSON string
v1beta1_model_framework_spec_instance = V1beta1ModelFrameworkSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1ModelFrameworkSpec.to_json())

# convert the object into a dict
v1beta1_model_framework_spec_dict = v1beta1_model_framework_spec_instance.to_dict()
# create an instance of V1beta1ModelFrameworkSpec from a dict
v1beta1_model_framework_spec_from_dict = V1beta1ModelFrameworkSpec.from_dict(v1beta1_model_framework_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
