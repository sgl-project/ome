# V1beta1ModelRef

## Properties

| Name                   | Type          | Description                                                                                                                          | Notes                      |
|------------------------|---------------|--------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **api_group**          | **str**       | APIGroup of the resource being referenced Defaults to &#x60;ome.io&#x60; Specifies the Kubernetes API group of the referenced model. | [optional]                 |
| **fine_tuned_weights** | **List[str]** | Optional FineTunedWeights references References to fine-tuned weights that should be applied to the base model.                      | [optional]                 |
| **kind**               | **str**       | Kind of the model being referenced Defaults to ClusterBaseModel Specifies the Kubernetes resource kind of the referenced model.      | [optional]                 |
| **name**               | **str**       | Name of the model being referenced Identifies the specific model to be used for inference.                                           | [optional] [default to ''] |

## Example

```python
from ome.models.v1beta1_model_ref import V1beta1ModelRef

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ModelRef from a JSON string
v1beta1_model_ref_instance = V1beta1ModelRef.from_json(json)
# print the JSON string representation of the object
print(V1beta1ModelRef.to_json())

# convert the object into a dict
v1beta1_model_ref_dict = v1beta1_model_ref_instance.to_dict()
# create an instance of V1beta1ModelRef from a dict
v1beta1_model_ref_from_dict = V1beta1ModelRef.from_dict(v1beta1_model_ref_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
