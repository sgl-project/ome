# V1beta1ModelConfig

## Properties

| Name             | Type                                            | Description                             | Notes      |
|------------------|-------------------------------------------------|-----------------------------------------|------------|
| **input_model**  | **str**                                         | InputModel defines the base model name. | [optional] |
| **output_model** | [**V1beta1StorageSpec**](V1beta1StorageSpec.md) |                                         | [optional] |

## Example

```python
from ome.models.v1beta1_model_config import V1beta1ModelConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ModelConfig from a JSON string
v1beta1_model_config_instance = V1beta1ModelConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1ModelConfig.to_json())

# convert the object into a dict
v1beta1_model_config_dict = v1beta1_model_config_instance.to_dict()
# create an instance of V1beta1ModelConfig from a dict
v1beta1_model_config_from_dict = V1beta1ModelConfig.from_dict(v1beta1_model_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
