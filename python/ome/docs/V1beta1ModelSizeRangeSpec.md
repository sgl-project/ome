# V1beta1ModelSizeRangeSpec

ModelSizeRangeSpec defines the range of model sizes supported by this runtime

## Properties

| Name    | Type    | Description                        | Notes      |
|---------|---------|------------------------------------|------------|
| **max** | **str** | Maximum size of the model in bytes | [optional] |
| **min** | **str** | Minimum size of the model in bytes | [optional] |

## Example

```python
from ome.models.v1beta1_model_size_range_spec import V1beta1ModelSizeRangeSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ModelSizeRangeSpec from a JSON string
v1beta1_model_size_range_spec_instance = V1beta1ModelSizeRangeSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1ModelSizeRangeSpec.to_json())

# convert the object into a dict
v1beta1_model_size_range_spec_dict = v1beta1_model_size_range_spec_instance.to_dict()
# create an instance of V1beta1ModelSizeRangeSpec from a dict
v1beta1_model_size_range_spec_from_dict = V1beta1ModelSizeRangeSpec.from_dict(v1beta1_model_size_range_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
