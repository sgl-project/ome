# V1beta1FineTunedWeightSpec

FineTunedWeightSpec defines the desired state of FineTunedWeight

## Properties

| Name                 | Type                                                                                      | Description                                                                                                     | Notes      |
| -------------------- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------- |
| **base_model_ref**   | [**V1beta1ObjectReference**](V1beta1ObjectReference.md)                                   |                                                                                                                 |
| **compartment_id**   | **str**                                                                                   | CompartmentID is the compartment ID of the model                                                                | [optional] |
| **configuration**    | [**K8sIoApimachineryPkgRuntimeRawExtension**](K8sIoApimachineryPkgRuntimeRawExtension.md) |                                                                                                                 | [optional] |
| **disabled**         | **bool**                                                                                  | Whether the model is enabled or not                                                                             | [optional] |
| **display_name**     | **str**                                                                                   | DisplayName is the user-friendly name of the model                                                              | [optional] |
| **hyper_parameters** | [**K8sIoApimachineryPkgRuntimeRawExtension**](K8sIoApimachineryPkgRuntimeRawExtension.md) |                                                                                                                 |
| **model_type**       | **str**                                                                                   | ModelType of the fine-tuned weight, e.g., \&quot;Distillation\&quot;, \&quot;Adapter\&quot;, \&quot;Tfew\&quot; |
| **storage**          | [**V1beta1StorageSpec**](V1beta1StorageSpec.md)                                           |                                                                                                                 |
| **training_job_ref** | [**V1beta1ObjectReference**](V1beta1ObjectReference.md)                                   |                                                                                                                 | [optional] |
| **vendor**           | **str**                                                                                   | Vendor of the model, e.g., \&quot;NVIDIA\&quot;, \&quot;Meta\&quot;, \&quot;HuggingFace\&quot;                  | [optional] |
| **version**          | **str**                                                                                   |                                                                                                                 | [optional] |

## Example

```python
from ome.models.v1beta1_fine_tuned_weight_spec import V1beta1FineTunedWeightSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1FineTunedWeightSpec from a JSON string
v1beta1_fine_tuned_weight_spec_instance = V1beta1FineTunedWeightSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1FineTunedWeightSpec.to_json())

# convert the object into a dict
v1beta1_fine_tuned_weight_spec_dict = v1beta1_fine_tuned_weight_spec_instance.to_dict()
# create an instance of V1beta1FineTunedWeightSpec from a dict
v1beta1_fine_tuned_weight_spec_from_dict = V1beta1FineTunedWeightSpec.from_dict(v1beta1_fine_tuned_weight_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)
